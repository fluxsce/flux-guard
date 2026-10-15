package jar

import (
	"context"
	"fluxguard/agents/appside/core"
	"fmt"

	"fluxguard/protocol"
)

func init() {
	core.RegisterMode(jarMode{})
}

type jarMode struct{}

func (jarMode) Name() string { return "jar" }

func (jarMode) AllowKind(kind string) bool { return kind == "jar" }

func (jarMode) NeedsRestart(kind string) bool { return true }

func (jarMode) SupportUpgrade() bool { return false }

func (jarMode) FillSide(side *protocol.AppSide, rt core.Runtime) {}

func (jarMode) CollectArtifacts(ctx context.Context, rt core.Runtime) ([]core.Artifact, error) {
	_ = ctx
	name, version := core.AppCode(rt.Jar)
	if name == "" {
		return nil, nil
	}
	return []core.Artifact{{
		AppName: name,
		Kind:    "jar",
		Version: version,
		Path:    rt.Jar,
	}}, nil
}

func (jarMode) ListInventory(rt core.Runtime) ([]core.Artifact, error) {
	return nil, fmt.Errorf("实例 %s 是 jar 模式，不支持列目录", rt.Instance)
}

func (jarMode) Replace(rt core.Runtime, call protocol.CommandCall, file string) (core.ApplyReport, error) {
	_ = call
	return core.ReplaceJar(rt.Jar, file, rt.BackupDir, rt.Instance)
}

func (jarMode) Upgrade(rt core.Runtime, zipPath string) (core.ApplyReport, error) {
	return core.ApplyReport{}, fmt.Errorf("jar 模式不支持运行时升级")
}

func (jarMode) MarkStartupLog(rt core.Runtime) core.LogMark { return core.LogMark{} }

func (jarMode) ProbeStartup(ctx context.Context, rt core.Runtime, mark core.LogMark) protocol.CommandStep {
	return core.ProbeProcessStartup(ctx, rt)
}
