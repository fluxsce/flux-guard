// Package appside 是应用代理。一台机器装一个进程，配置里列出这台机器上的各套 Tomcat、JAR 和静态目录。
// 应用进程退出后，本进程继续上报，并执行换包和运行时升级。主机指标和 JVM 指标不在这里采集。
package appside

import (
	"fmt"
	"strings"

	"fluxguard/common/report"
	"fluxguard/common/yamlcfg"
	"fluxguard/protocol"
)

// Runtime 是这一进程看守的一套应用。一台机器可以列多套。中央不下发这些路径。
type Runtime struct {
	Kind      string `mapstructure:"kind"`
	Instance  string `mapstructure:"instance"`
	Service   string `mapstructure:"service"`
	Mode      string `mapstructure:"mode"`
	Base      string `mapstructure:"base"`
	Jar       string `mapstructure:"jar"`
	StaticApp string `mapstructure:"static_app"`
	StaticDir string `mapstructure:"static_dir"`
	Start     string `mapstructure:"start"`
	Stop      string `mapstructure:"stop"`
	Status    string `mapstructure:"status"`
	BackupDir string `mapstructure:"-"`
}

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
		BackupDir string    `mapstructure:"backup_dir"`
		Runtime   Runtime   `mapstructure:"runtime"`
		Runtimes  []Runtime `mapstructure:"runtimes"`
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
		list = []Runtime{file.Runtime}
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

func normalizeRuntime(rt Runtime) (Runtime, error) {
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
	if rt.Kind == "" || rt.Instance == "" || rt.Mode == "" {
		return rt, fmt.Errorf("每一套都要填写 kind、instance、mode")
	}
	switch rt.Mode {
	case "tomcat", "node", "script":
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
	default:
		return rt, fmt.Errorf("runtime.mode 只允许 tomcat、node、script、jar、static")
	}
	switch rt.Kind {
	case "systemd", "windows", "docker":
		if rt.Service == "" {
			return rt, fmt.Errorf("%s 模式请填写 service", rt.Kind)
		}
	case "command":
		if err := fillCommandDefaults(&rt); err != nil {
			return rt, err
		}
	default:
		return rt, fmt.Errorf("runtime.kind 只允许 systemd、windows、docker、command")
	}
	return rt, nil
}

// FindRuntime 按实例名找到这一套。命令没带实例名且只有一套时用那一套。
func FindRuntime(list []Runtime, instance string) (Runtime, bool) {
	instance = strings.TrimSpace(instance)
	if instance == "" && len(list) == 1 {
		return list[0], true
	}
	for _, rt := range list {
		if rt.Instance == instance {
			return rt, true
		}
	}
	return Runtime{}, false
}

// Code 是协议里的代理种类。
const Code = protocol.AgentAppside
