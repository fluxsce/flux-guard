package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	"fluxguard/agents/appside"
	"fluxguard/agents/host"
	"fluxguard/common/agenttunnel"
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
	// 会话隧道：日志跟读等交互流；与命令长轮询、上报分立。
	tun := agenttunnel.New(agenttunnel.Config{
		ServerURL:  cfg.ServerURL,
		AgentToken: cfg.AgentToken,
		AgentCode:  appside.Code,
		MachineID:  machineID,
	})
	tun.Handle(protocol.TunnelChannelAppTail, appside.AppTailHandle(runtimes))
	go tun.Run(ctx)
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
		protocol.ActionAppStart, protocol.ActionAppStop, protocol.ActionAppRestart,
		protocol.ActionAppThreadDump, protocol.ActionAppHeapDump,
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

	if call.Action == protocol.ActionAppStart || call.Action == protocol.ActionAppStop || call.Action == protocol.ActionAppRestart {
		return runLifecycle(ctx, rt, call.Action, steps, fail)
	}
	if call.Action == protocol.ActionAppThreadDump || call.Action == protocol.ActionAppHeapDump {
		return runDiag(ctx, rt, call.Action, steps, fail)
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
			Detail: "把备份移回目标路径",
		})
		if !appside.NeedsRestart(rt, call.Kind) {
			return protocol.CommandOutcome{Message: "已回滚文件", Steps: steps}, nil
		}
		mark := appside.MarkStartupLog(rt)
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: appside.DescribeLifecycle(rt, "restart")})
		return finishWithStartup(ctx, rt, mark, steps, "已回滚并重启 "+rt.Instance)
	}

	file, err := appside.Download(ctx, cfg.ServerURL, cfg.AgentToken, machineID, call.ReleaseID, call.SHA256)
	if err != nil {
		return fail("download", "", "", err)
	}
	defer os.Remove(file)
	steps = append(steps, protocol.CommandStep{Name: "download", Status: "done", Detail: "从中央拉取制品并校验 SHA256"})

	switch call.Action {
	case protocol.ActionAppReplace:
		report, err := appside.Replace(rt, call, file)
		if err != nil {
			return fail("replace", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "replace", Status: "done", TargetPath: report.TargetPath, BackupPath: report.BackupPath,
			Detail: replaceDetail(call.Kind),
		})
		if !appside.NeedsRestart(rt, call.Kind) {
			msg := "文件已替换"
			if call.Kind == "static" && rt.Mode != "files" {
				msg = "静态资源已替换"
			}
			return protocol.CommandOutcome{Message: msg, Steps: steps}, nil
		}
		mark := appside.MarkStartupLog(rt)
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: appside.DescribeLifecycle(rt, "restart")})
		return finishWithStartup(ctx, rt, mark, steps, "已替换并重启 "+rt.Instance)
	default:
		report, err := appside.UpgradeRuntime(rt, file)
		if err != nil {
			return fail("replace", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "replace", Status: "done", TargetPath: report.TargetPath, BackupPath: report.BackupPath,
			Detail: "用发行包替换 Tomcat 程序目录，配置保留",
		})
		mark := appside.MarkStartupLog(rt)
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", report.TargetPath, report.BackupPath, err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: appside.DescribeLifecycle(rt, "restart")})
		return finishWithStartup(ctx, rt, mark, steps, "运行时已升级并重启 "+rt.Instance)
	}
}

func runDiag(
	ctx context.Context,
	rt appside.Runtime,
	action string,
	steps []protocol.CommandStep,
	fail func(name, target, backup string, err error) (protocol.CommandOutcome, error),
) (protocol.CommandOutcome, error) {
	switch action {
	case protocol.ActionAppThreadDump:
		got, err := appside.ThreadDump(ctx, rt)
		if err != nil {
			return fail("threaddump", rt.Base, "", err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "threaddump", Status: "done", TargetPath: got.Path, Detail: got.Detail,
		})
		return protocol.CommandOutcome{Message: got.Message, Steps: steps}, nil
	case protocol.ActionAppHeapDump:
		got, err := appside.HeapDump(ctx, rt)
		if err != nil {
			return fail("heapdump", rt.Base, "", err)
		}
		steps = append(steps, protocol.CommandStep{
			Name: "heapdump", Status: "done", TargetPath: got.Path, Detail: got.Detail,
		})
		return protocol.CommandOutcome{Message: got.Message, Steps: steps}, nil
	default:
		return protocol.CommandOutcome{}, fmt.Errorf("不支持的动作 %s", action)
	}
}

func runLifecycle(
	ctx context.Context,
	rt appside.Runtime,
	action string,
	steps []protocol.CommandStep,
	fail func(name, target, backup string, err error) (protocol.CommandOutcome, error),
) (protocol.CommandOutcome, error) {
	switch action {
	case protocol.ActionAppStop:
		if err := appside.Stop(ctx, rt); err != nil {
			return fail("stop", rt.Base, "", err)
		}
		steps = append(steps, protocol.CommandStep{Name: "stop", Status: "done", Detail: appside.DescribeLifecycle(rt, "stop")})
		return protocol.CommandOutcome{Message: "已停止 " + rt.Instance, Steps: steps}, nil
	case protocol.ActionAppStart:
		mark := appside.MarkStartupLog(rt)
		if err := appside.Start(ctx, rt); err != nil {
			return fail("start", rt.Base, "", err)
		}
		steps = append(steps, protocol.CommandStep{Name: "start", Status: "done", Detail: appside.DescribeLifecycle(rt, "start")})
		return finishWithStartup(ctx, rt, mark, steps, "已启动 "+rt.Instance)
	case protocol.ActionAppRestart:
		mark := appside.MarkStartupLog(rt)
		if err := appside.Restart(ctx, rt); err != nil {
			return fail("restart", rt.Base, "", err)
		}
		steps = append(steps, protocol.CommandStep{Name: "restart", Status: "done", Detail: appside.DescribeLifecycle(rt, "restart")})
		return finishWithStartup(ctx, rt, mark, steps, "已重启 "+rt.Instance)
	default:
		return protocol.CommandOutcome{}, fmt.Errorf("不支持的动作 %s", action)
	}
}

// finishWithStartup 在重启成功后再探一次启动结果（进程 + 自重启偏移后的日志）。
func finishWithStartup(ctx context.Context, rt appside.Runtime, mark appside.LogMark, steps []protocol.CommandStep, msg string) (protocol.CommandOutcome, error) {
	probe := appside.ProbeStartup(ctx, rt, mark)
	steps = append(steps, probe)
	if probe.Status == "failed" {
		line := firstLine(probe.Detail)
		if line == "" {
			line = "启动探测失败"
		}
		return protocol.CommandOutcome{Message: line, Steps: steps}, fmt.Errorf("%s", line)
	}
	if line := firstLine(probe.Detail); line != "" {
		msg = msg + "；" + line
	}
	return protocol.CommandOutcome{Message: msg, Steps: steps}, nil
}

func replaceDetail(kind string) string {
	switch strings.TrimSpace(kind) {
	case "war":
		return "覆盖 webapps 中的 WAR，旧包已备份，并清掉已展开目录"
	case "jar":
		return "覆盖 JAR，旧文件已备份"
	case "static":
		return "写入静态文件，旧文件已备份"
	default:
		return "写入目标文件，旧文件已备份"
	}
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return strings.TrimSpace(text[:i])
	}
	return text
}
