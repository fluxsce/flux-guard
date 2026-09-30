package report

import (
	"path/filepath"
	"strings"

	"fluxguard/common/logger"
	"fluxguard/common/yamlcfg"
)

// Config 是一个 Go 代理进程的上报配置。采集逻辑不在这里。
type Config struct {
	ServerURL     string        `mapstructure:"server_url"`
	AgentToken    string        `mapstructure:"agent_token"`
	IntervalSec   int           `mapstructure:"interval_sec"`
	ReportName    string        `mapstructure:"report_name"`
	CommandListen string        `mapstructure:"command_listen"`
	ShellEnabled  bool          `mapstructure:"shell_enabled"`
	SpoolFile     string        `mapstructure:"spool_file"`
	Log           logger.Config `mapstructure:"log"`
}

// Load 读取配置。path 为空时使用 fallback。只读本机 yaml。
func Load(path, fallback string) (Config, error) {
	cfg := Config{
		ServerURL:   "http://127.0.0.1:8080",
		AgentToken:  "agt_dev_local",
		IntervalSec: 15,
		Log:         logger.Defaults(logFile(fallback)),
	}
	if path == "" {
		path = fallback
	}
	if err := yamlcfg.Read(path, &cfg); err != nil {
		return cfg, err
	}
	if cfg.IntervalSec <= 0 {
		cfg.IntervalSec = 15
	}
	cfg.ServerURL = strings.TrimRight(cfg.ServerURL, "/")
	return cfg, nil
}

func logFile(fallback string) string {
	base := strings.ToLower(filepath.Base(fallback))
	switch {
	case strings.Contains(base, "host"):
		return "logs/host.log"
	case strings.Contains(base, "app"):
		return "logs/app.log"
	default:
		return "logs/agent.log"
	}
}
