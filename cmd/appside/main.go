package main

import (
	"context"
	"fmt"
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
	}, func(ctx context.Context, call protocol.CommandCall) (string, error) {
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

func runCommand(ctx context.Context, cfg report.Config, list []appside.Runtime, machineID string, call protocol.CommandCall) (string, error) {
	if call.Action == protocol.ActionAgentUpdate {
		return agentupdate.Apply(ctx, cfg.ServerURL, cfg.AgentToken, appside.Code, machineID, call)
	}
	switch call.Action {
	case protocol.ActionAppReplace, protocol.ActionAppUpgrade:
	default:
		return "", fmt.Errorf("不支持的动作 %s", call.Action)
	}
	rt, ok := appside.FindRuntime(list, call.Instance)
	if !ok {
		return "", fmt.Errorf("找不到实例 %s", call.Instance)
	}
	file, err := appside.Download(ctx, cfg.ServerURL, cfg.AgentToken, machineID, call.ReleaseID, call.SHA256)
	if err != nil {
		return "", err
	}
	defer os.Remove(file)
	switch call.Action {
	case protocol.ActionAppReplace:
		if err := replace(rt, call, file); err != nil {
			return "", err
		}
		if call.Kind == "static" {
			return "静态资源已替换", nil
		}
		if err := appside.Restart(ctx, rt); err != nil {
			return "", err
		}
		return "已替换并重启 " + rt.Instance, nil
	default:
		if rt.Mode != "tomcat" {
			return "", fmt.Errorf("版本升级只支持 Tomcat")
		}
		if err := appside.UpgradeBase(rt.Base, file, rt.BackupDir, rt.Instance); err != nil {
			return "", err
		}
		if err := appside.Restart(ctx, rt); err != nil {
			return "", err
		}
		return "运行时已升级并重启 " + rt.Instance, nil
	}
}

func replace(rt appside.Runtime, call protocol.CommandCall, file string) error {
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
		return fmt.Errorf("制品种类无效")
	}
}
