package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"fluxguard/agents/goruntime"
	"fluxguard/common/buildinfo"
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
	logger.Info(ctx, "flux-guard goruntime %s -> %s，间隔 %ds，名称 %s", buildinfo.Version, cfg.ServerURL, cfg.IntervalSec, name)
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
