package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"fluxguard/agents/appside"
	"fluxguard/agents/host"
	"fluxguard/common/agentupdate"
	"fluxguard/common/buildinfo"
	"fluxguard/common/command"
	"fluxguard/common/logger"
	"fluxguard/common/report"
	"fluxguard/protocol"
)

func main() {
	buildinfo.ExitIfRequested()
	cfg, runtimes, err := appside.Load(os.Getenv("FLUX_GUARD_AGENT_CONFIG"))
	if err != nil {
		log.Fatal(err)
	}
	logs, err := logger.Setup(cfg.Log)
	if err != nil {
		log.Fatal(err)
	}
	defer logs.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var commandAddr atomic.Value
	machineID := host.MachineID()
	logger.Info(ctx, "flux-guard appside %s -> %s，看守 %d 套", buildinfo.Version, cfg.ServerURL, len(runtimes))
	go command.Run(ctx, command.Config{
		ServerURL:  cfg.ServerURL,
		AgentToken: cfg.AgentToken,
		AgentCode:  appside.Code,
		MachineID:  machineID,
		Listen:     cfg.CommandListen,
		FileOpen: func(path string) (string, int64, io.ReadCloser, error) {
			return appside.OpenWatchedFile(runtimes, path)
		},
		FileList: func(instance string) ([]protocol.AppArtifact, error) {
			found, err := appside.ListInstanceFiles(runtimes, instance)
			if err != nil {
				return nil, err
			}
			out := make([]protocol.AppArtifact, 0, len(found))
			for _, item := range found {
				out = append(out, protocol.AppArtifact{
					AppName: item.AppName, Kind: item.Kind, Version: item.Version, Path: item.Path,
				})
			}
			return out, nil
		},
	}, func(ctx context.Context, call protocol.CommandCall) (protocol.CommandOutcome, error) {
		return runCommand(ctx, cfg, runtimes, machineID, call)
	}, func(addr string) {
		commandAddr.Store(addr)
	})
	report.Run(ctx, cfg, appside.Code, func(ctx context.Context) (protocol.CollectResult, error) {
		addr, _ := commandAddr.Load().(string)
		result, err := appside.CollectAll(ctx, runtimes, addr)
		if err != nil {
			return result, err
		}
		for i := range result.AppSides {
			result.AppSides[i].MachineID = machineID
		}
		return result, nil
	})
}

func runCommand(ctx context.Context, cfg report.Config, list []appside.Runtime, machineID string, call protocol.CommandCall) (protocol.CommandOutcome, error) {
	if call.Action == protocol.ActionAgentUpdate {
		msg, err := agentupdate.Apply(ctx, cfg.ServerURL, cfg.AgentToken, appside.Code, machineID, call)
		if err != nil {
			return protocol.CommandOutcome{Message: err.Error()}, err
		}
		return protocol.CommandOutcome{Message: msg}, nil
	}
	switch call.Action {
	case protocol.ActionAppReplace, protocol.ActionAppUpgrade, protocol.ActionAppRollback,
		protocol.ActionAppFilePush, protocol.ActionAppFileList:
	default:
		return protocol.CommandOutcome{}, fmt.Errorf("不支持的动作 %s", call.Action)
	}
	rt, ok := appside.FindRuntime(list, call.Instance)
	if !ok {
		return protocol.CommandOutcome{}, fmt.Errorf("找不到实例 %s", call.Instance)
	}
	var steps []protocol.CommandStep
	fail := func(name, target, backup string, err error) (protocol.CommandOutcome, error) {
		steps = append(steps, protocol.CommandStep{
			Name: name, Status: "failed", TargetPath: target, BackupPath: backup, Detail: err.Error(),
		})
		return protocol.CommandOutcome{Message: err.Error(), Steps: steps}, err
	}

	if call.Action == protocol.ActionAppFileList {
		found, err := appside.ListInstanceFiles(list, call.Instance)
		if err != nil {
			return fail("list", rt.Base, "", err)
		}
		items := make([]protocol.AppArtifact, 0, len(found))
		for _, item := range found {
			items = append(items, protocol.AppArtifact{
				AppName: item.AppName, Kind: item.Kind, Version: item.Version, Path: item.Path,
			})
		}
		if err := appside.PushFileList(ctx, cfg.ServerURL, cfg.AgentToken, machineID, call, items); err != nil {
			return fail("list", rt.Base, "", err)
		}
		steps = append(steps, protocol.CommandStep{Name: "list", Status: "done", TargetPath: rt.Base, Detail: fmt.Sprintf("%d", len(items))})
		return protocol.CommandOutcome{Message: "目录清单已推送到中央", Steps: steps}, nil
	}

	if call.Action == protocol.ActionAppFilePush {
		_, _, body, err := appside.OpenWatchedFile([]appside.Runtime{rt}, call.TargetPath)
		if err != nil {
			return fail("push", call.TargetPath, "", err)
		}
		_ = body.Close()
		if err := appside.PushFile(ctx, cfg.ServerURL, cfg.AgentToken, machineID, call); err != nil {
			return fail("push", call.TargetPath, "", err)
		}
		steps = append(steps, protocol.CommandStep{Name: "push", Status: "done", TargetPath: call.TargetPath})
		return protocol.CommandOutcome{Message: "文件已推送到中央", Steps: steps}, nil
	}

	if call.Action == protocol.ActionAppRollback {
		report, err := appside.Rollback(rt.BackupDir, call.TargetPath, call.BackupPath, call.Kind, call.AppName)
		if err != nil {
			return fail("rollback", call.TargetPath, call.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "rollback", Status: "done", TargetPath: report.TargetPath, BackupPath: report.BackupPath,
		})
		if call.Kind == "static" || rt.Mode == "files" {
			return protocol.CommandOutcome{Message: "已回滚文件", Steps: steps}, nil
		}
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: rt.Instance})
		return protocol.CommandOutcome{Message: "已回滚并重启 " + rt.Instance, Steps: steps}, nil
	}

	file, err := appside.Download(ctx, cfg.ServerURL, cfg.AgentToken, machineID, call.ReleaseID, call.SHA256)
	if err != nil {
		return fail("download", "", "", err)
	}
	defer os.Remove(file)
	steps = append(steps, protocol.CommandStep{Name: "download", Status: "done"})

	switch call.Action {
	case protocol.ActionAppReplace:
		report, err := replace(rt, call, file)
		if err != nil {
			return fail("replace", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "replace", Status: "done", TargetPath: report.TargetPath, BackupPath: report.BackupPath,
		})
		if call.Kind == "static" || rt.Mode == "files" {
			msg := "文件已替换"
			if call.Kind == "static" && rt.Mode != "files" {
				msg = "静态资源已替换"
			}
			return protocol.CommandOutcome{Message: msg, Steps: steps}, nil
		}
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: rt.Instance})
		return protocol.CommandOutcome{
			Message: "已替换并重启 " + rt.Instance,
			Steps:   steps,
		}, nil
	default:
		if rt.Mode != "tomcat" {
			return fail("replace", rt.Base, "", fmt.Errorf("版本升级只支持 Tomcat"))
		}
		report, err := appside.UpgradeBase(rt.Base, file, rt.BackupDir, rt.Instance)
		if err != nil {
			return fail("replace", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "replace", Status: "done", TargetPath: report.TargetPath, BackupPath: report.BackupPath,
		})
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: rt.Instance})
		return protocol.CommandOutcome{
			Message: "运行时已升级并重启 " + rt.Instance,
			Steps:   steps,
		}, nil
	}
}

func replace(rt appside.Runtime, call protocol.CommandCall, file string) (appside.ApplyReport, error) {
	if rt.Mode == "files" {
		return appside.ReplaceFile(rt.Base, call.AppName, file, call.FileName, rt.BackupDir, rt.Instance)
	}
	switch call.Kind {
	case "war":
		return appside.ReplaceWar(rt.Base, call.AppName, file, rt.BackupDir, rt.Instance)
	case "jar":
		return appside.ReplaceJar(rt.Jar, file, rt.BackupDir, rt.Instance)
	case "node", "script":
		return appside.ReplaceStatic(rt.Base, file, call.FileName, rt.BackupDir, rt.Instance)
	case "static":
		return appside.ReplaceStatic(rt.StaticDir, file, call.FileName, rt.BackupDir, rt.Instance)
	default:
		return appside.ApplyReport{}, fmt.Errorf("制品种类无效")
	}
}
