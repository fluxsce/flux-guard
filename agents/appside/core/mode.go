package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fluxguard/protocol"
)

// Mode 是一种应用运行模式的采集与换包策略。
// 新增 mode：在 modes/<name> 实现本接口，init 里 RegisterMode。
type Mode interface {
	Name() string
	CollectArtifacts(ctx context.Context, rt Runtime) ([]Artifact, error)
	FillSide(side *protocol.AppSide, rt Runtime)
	AllowKind(kind string) bool
	NeedsRestart(kind string) bool
	Replace(rt Runtime, call protocol.CommandCall, file string) (ApplyReport, error)
	ListInventory(rt Runtime) ([]Artifact, error)
	SupportUpgrade() bool
	Upgrade(rt Runtime, zipPath string) (ApplyReport, error)
	MarkStartupLog(rt Runtime) LogMark
	ProbeStartup(ctx context.Context, rt Runtime, mark LogMark) protocol.CommandStep
}

var modes = map[string]Mode{}

// RegisterMode 登记一种 mode。重复登记会覆盖。
func RegisterMode(m Mode) {
	if m == nil || strings.TrimSpace(m.Name()) == "" {
		return
	}
	modes[strings.TrimSpace(m.Name())] = m
}

// LookupMode 按配置里的 mode 名取策略。
func LookupMode(name string) (Mode, error) {
	name = strings.TrimSpace(name)
	m := modes[name]
	if m == nil {
		return nil, fmt.Errorf("不支持的 mode：%s", name)
	}
	return m, nil
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

// Replace 按运行时 mode 换包，并校验制品 kind 与 mode 匹配。
func Replace(rt Runtime, call protocol.CommandCall, file string) (ApplyReport, error) {
	m, err := LookupMode(rt.Mode)
	if err != nil {
		return ApplyReport{}, err
	}
	kind := strings.TrimSpace(call.Kind)
	if kind == "" {
		return ApplyReport{}, fmt.Errorf("缺少制品种类")
	}
	if !m.AllowKind(kind) {
		return ApplyReport{}, fmt.Errorf("%s 模式不支持制品 %s", rt.Mode, kind)
	}
	return m.Replace(rt, call, file)
}

// ListInstanceFiles 按实例短拨列目录。
func ListInstanceFiles(list []Runtime, instance string) ([]Artifact, error) {
	rt, ok := FindRuntime(list, instance)
	if !ok {
		return nil, fmt.Errorf("找不到实例 %s", instance)
	}
	m, err := LookupMode(rt.Mode)
	if err != nil {
		return nil, err
	}
	return m.ListInventory(rt)
}

// UpgradeRuntime 运行时 zip 升级。
func UpgradeRuntime(rt Runtime, zipPath string) (ApplyReport, error) {
	m, err := LookupMode(rt.Mode)
	if err != nil {
		return ApplyReport{}, err
	}
	if !m.SupportUpgrade() {
		return ApplyReport{}, fmt.Errorf("版本升级只支持 Tomcat")
	}
	return m.Upgrade(rt, zipPath)
}

// MarkStartupLog 重启前打点。
func MarkStartupLog(rt Runtime) LogMark {
	m, err := LookupMode(rt.Mode)
	if err != nil {
		return LogMark{}
	}
	return m.MarkStartupLog(rt)
}

// ProbeStartup 重启后探测启动结果。
func ProbeStartup(ctx context.Context, rt Runtime, mark LogMark) protocol.CommandStep {
	m, err := LookupMode(rt.Mode)
	if err != nil {
		return protocol.CommandStep{Name: "startup", Status: "failed", Detail: err.Error()}
	}
	return m.ProbeStartup(ctx, rt, mark)
}

// NeedsRestart 换包/回滚后是否重启。
func NeedsRestart(rt Runtime, kind string) bool {
	m, err := LookupMode(rt.Mode)
	if err != nil {
		return false
	}
	return m.NeedsRestart(kind)
}

// WaitStartup 各 mode 启动探测共用的等待。
func WaitStartup(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// StartupWait 默认启动探测等待时长。
const StartupWait = 5 * time.Second

// ProbeProcessStartup 无专用日志的 mode：等待后只看进程是否在跑。
func ProbeProcessStartup(ctx context.Context, rt Runtime) protocol.CommandStep {
	step := protocol.CommandStep{Name: "startup", Status: "done"}
	if err := WaitStartup(ctx, StartupWait); err != nil {
		step.Status = "failed"
		step.Detail = "等待启动时任务已取消"
		return step
	}
	if Running(ctx, rt) {
		step.Detail = "进程已在运行"
		return step
	}
	step.Status = "failed"
	step.Detail = "重启后进程未运行"
	return step
}
