package appside

import (
	"fmt"
	"strings"

	"fluxguard/agents/appside/core"
	"fluxguard/common/report"
	"fluxguard/common/yamlcfg"
	"fluxguard/protocol"
)

// Load 读取上报配置和本进程看守的各套运行时。path 为空时使用 configs/appside.yaml。
// runtimes 列出一台机器上的全部套件。只写了 runtime 时，按一套处理。
func Load(path string) (report.Config, []Runtime, error) {
	if strings.TrimSpace(path) == "" {
		path = "configs/appside.yaml"
	}
	cfg, err := report.Load(path, "configs/appside.yaml")
	if err != nil {
		return cfg, nil, err
	}
	var file struct {
		BackupDir string         `mapstructure:"backup_dir"`
		Runtime   core.Runtime   `mapstructure:"runtime"`
		Runtimes  []core.Runtime `mapstructure:"runtimes"`
	}
	if err := yamlcfg.Read(path, &file); err != nil {
		return cfg, nil, err
	}
	backupDir := strings.TrimSpace(file.BackupDir)
	if backupDir == "" {
		return cfg, nil, fmt.Errorf("请填写 backup_dir")
	}
	list := file.Runtimes
	if len(list) == 0 && strings.TrimSpace(file.Runtime.Instance) != "" {
		list = []core.Runtime{file.Runtime}
	}
	if len(list) == 0 {
		return cfg, nil, fmt.Errorf("请在 runtimes 里列出这台机器上的 Tomcat、Node、脚本、JAR 或静态目录")
	}
	seen := map[string]bool{}
	for i := range list {
		rt, err := normalizeRuntime(list[i])
		if err != nil {
			return cfg, nil, err
		}
		if seen[rt.Instance] {
			return cfg, nil, fmt.Errorf("实例名重复：%s", rt.Instance)
		}
		seen[rt.Instance] = true
		rt.BackupDir = backupDir
		list[i] = rt
	}
	return cfg, list, nil
}

func normalizeRuntime(rt core.Runtime) (core.Runtime, error) {
	rt.Kind = strings.TrimSpace(rt.Kind)
	rt.Instance = strings.TrimSpace(rt.Instance)
	rt.Service = strings.TrimSpace(rt.Service)
	rt.Mode = strings.TrimSpace(rt.Mode)
	rt.Base = strings.TrimSpace(rt.Base)
	rt.Jar = strings.TrimSpace(rt.Jar)
	rt.StaticApp = strings.TrimSpace(rt.StaticApp)
	rt.StaticDir = strings.TrimSpace(rt.StaticDir)
	rt.Start = strings.TrimSpace(rt.Start)
	rt.Stop = strings.TrimSpace(rt.Stop)
	rt.Status = strings.TrimSpace(rt.Status)
	rt.Restart = strings.TrimSpace(rt.Restart)
	rt.Kill = strings.TrimSpace(rt.Kill)
	rt.Description = strings.TrimSpace(rt.Description)
	if runes := []rune(rt.Description); len(runes) > 500 {
		rt.Description = string(runes[:500])
	}
	if rt.Kind == "" || rt.Instance == "" || rt.Mode == "" {
		return rt, fmt.Errorf("每一套都要填写 kind、instance、mode")
	}
	if _, err := core.LookupMode(rt.Mode); err != nil {
		return rt, fmt.Errorf("runtime.mode 只允许 tomcat、node、script、jar、static、files")
	}
	switch rt.Mode {
	case "tomcat", "node", "script", "files":
		if rt.Base == "" {
			return rt, fmt.Errorf("%s 模式请填写 base", rt.Mode)
		}
	case "jar":
		if rt.Jar == "" {
			return rt, fmt.Errorf("jar 模式请填写 jar")
		}
	case "static":
		if rt.StaticDir == "" {
			return rt, fmt.Errorf("static 模式请填写 static_dir")
		}
	}
	switch rt.Kind {
	case "systemd", "windows", "docker":
		if rt.Service == "" {
			return rt, fmt.Errorf("%s 模式请填写 service", rt.Kind)
		}
	case "command":
		// command 只认 start/stop/status/restart/kill，不认 service（systemd/windows/docker 才用）。
		rt.Service = ""
		if err := fillCommandDefaults(&rt); err != nil {
			return rt, err
		}
	default:
		return rt, fmt.Errorf("runtime.kind 只允许 systemd、windows、docker、command")
	}
	return rt, nil
}

// Code 是协议里的代理种类。
const Code = protocol.AgentAppside
