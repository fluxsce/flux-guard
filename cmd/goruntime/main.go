package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"fluxguard/agents/goruntime"
	"fluxguard/common/agentupdate"
	"fluxguard/common/buildinfo"
	"fluxguard/common/command"
	"fluxguard/common/logger"
	"fluxguard/common/report"
	"fluxguard/protocol"
)

func main() {
	buildinfo.ExitIfRequested()
	cfg, err := report.Load(os.Getenv("FLUX_GUARD_AGENT_CONFIG"), "configs/goruntime.yaml")
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
	name := processName(cfg.ReportName)
	// 与上报分行规则一致：显示名@主机名，保证取令与心跳落在同一代理行。
	machineID := goMachineID(name)
	logger.Info(ctx, "flux-guard goruntime %s -> %s，间隔 %ds，名称 %s", buildinfo.Version, cfg.ServerURL, cfg.IntervalSec, name)
	// 自更新只走长轮询，不要求本机命令端口。
	go command.Run(ctx, command.Config{
		ServerURL:  cfg.ServerURL,
		AgentToken: cfg.AgentToken,
		AgentCode:  goruntime.Code,
		MachineID:  machineID,
	}, func(cmdCtx context.Context, call protocol.CommandCall) (protocol.CommandOutcome, error) {
		if call.Action != protocol.ActionAgentUpdate {
			return protocol.CommandOutcome{}, fmt.Errorf("不支持的动作 %s", call.Action)
		}
		msg, err := agentupdate.Apply(cmdCtx, cfg.ServerURL, cfg.AgentToken, goruntime.Code, machineID, call)
		if err != nil {
			return protocol.CommandOutcome{Message: err.Error()}, err
		}
		return protocol.CommandOutcome{Message: msg}, nil
	}, nil)
	report.Run(ctx, cfg, goruntime.Code, func(ctx context.Context) (protocol.CollectResult, error) {
		result, err := goruntime.Collect(ctx)
		if err != nil || result.Go == nil {
			return result, err
		}
		result.Go.AppName = name
		return result, nil
	})
}

func processName(reportName string) string {
	name := strings.TrimSpace(reportName)
	if name != "" {
		return name
	}
	name = strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == "" {
		return "go"
	}
	return name
}

func goMachineID(appName string) string {
	name := strings.TrimSpace(appName)
	if name == "" {
		name = "go"
	}
	host, _ := os.Hostname()
	host = strings.TrimSpace(host)
	id := name
	if host != "" {
		id = name + "@" + host
	}
	if len(id) > 64 {
		return id[:64]
	}
	return id
}
