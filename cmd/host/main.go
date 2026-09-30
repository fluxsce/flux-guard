package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

	"fluxguard/agents/host"
	"fluxguard/common/buildinfo"
	"fluxguard/common/command"
	"fluxguard/common/logger"
	"fluxguard/common/report"
	"fluxguard/common/shellrun"
	"fluxguard/protocol"
)

func main() {
	buildinfo.ExitIfRequested()
	cfg, err := report.Load(os.Getenv("FLUX_GUARD_AGENT_CONFIG"), "configs/host.yaml")
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
	ctl := report.NewControl(cfg.IntervalSec)
	var commandAddr atomic.Value
	logger.Info(ctx, "flux-guard host %s -> %s，间隔 %ds，名称 %s", buildinfo.Version, cfg.ServerURL, cfg.IntervalSec, cfg.ReportName)
	go command.Run(ctx, command.Config{
		ServerURL:  cfg.ServerURL,
		AgentToken: cfg.AgentToken,
		AgentCode:  host.Code,
		MachineID:  host.MachineID(),
		Listen:     cfg.CommandListen,
	}, func(_ context.Context, call protocol.CommandCall) (string, error) {
		return runCommand(ctl, call)
	}, func(addr string) {
		commandAddr.Store(addr)
		logger.Info(ctx, "命令端口 %s", addr)
	})
	if cfg.ShellEnabled {
		logger.Info(ctx, "终端通道已开启")
		go shellrun.Run(ctx, shellrun.Config{
			ServerURL:  cfg.ServerURL,
			AgentToken: cfg.AgentToken,
			AgentCode:  host.Code,
			MachineID:  host.MachineID(),
		})
	}
	report.RunControlled(ctx, cfg, host.Code, func(ctx context.Context) (protocol.CollectResult, error) {
		result, err := host.Collect(ctx)
		if err != nil || result.Host == nil {
			return result, err
		}
		result.Host.ReportName = strings.TrimSpace(cfg.ReportName)
		result.Host.AgentVersion = buildinfo.Version
		if addr, ok := commandAddr.Load().(string); ok && addr != "" {
			result.Host.CommandAddress = addr
		}
		return result, nil
	}, ctl)
}

func runCommand(ctl *report.Control, call protocol.CommandCall) (string, error) {
	switch call.Action {
	case protocol.ActionCollectNow:
		ctl.Nudge()
		return "已触发采集", nil
	case protocol.ActionIntervalSet:
		if err := ctl.SetInterval(call.IntervalSec); err != nil {
			return "", err
		}
		return fmt.Sprintf("上报间隔已改为 %d 秒", call.IntervalSec), nil
	default:
		return "", fmt.Errorf("不支持的动作 %s", call.Action)
	}
}
