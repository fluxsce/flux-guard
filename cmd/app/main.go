package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"fluxguard/agents/app"
	"fluxguard/common/buildinfo"
	"fluxguard/common/logger"
	"fluxguard/common/report"
)

func main() {
	buildinfo.ExitIfRequested()
	cfg, err := report.Load(os.Getenv("FLUX_GUARD_AGENT_CONFIG"), "configs/app.yaml")
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
	logger.Info(ctx, "flux-guard app %s -> %s，间隔 %ds", buildinfo.Version, cfg.ServerURL, cfg.IntervalSec)
	report.Run(ctx, cfg, app.Code, app.Collect)
}
