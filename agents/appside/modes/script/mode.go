package script

import (
	"context"
	"fluxguard/agents/appside/core"
	"fmt"

	"fluxguard/protocol"
)

func init() {
	core.RegisterMode(scriptMode{})
}

type scriptMode struct{}

func (scriptMode) Name() string { return "script" }

func (scriptMode) AllowKind(kind string) bool { return kind == "script" }

func (scriptMode) NeedsRestart(kind string) bool { return true }

func (scriptMode) SupportUpgrade() bool { return false }

func (scriptMode) FillSide(side *protocol.AppSide, rt core.Runtime) {}

func (scriptMode) CollectArtifacts(ctx context.Context, rt core.Runtime) ([]core.Artifact, error) {
	_ = ctx
	return []core.Artifact{{
		AppName: rt.Instance,
		Kind:    "script",
		Path:    rt.Base,
	}}, nil
}

func (scriptMode) ListInventory(rt core.Runtime) ([]core.Artifact, error) {
	return nil, fmt.Errorf("实例 %s 是 script 模式，不支持列目录", rt.Instance)
}

func (scriptMode) Replace(rt core.Runtime, call protocol.CommandCall, file string) (core.ApplyReport, error) {
	return core.ReplaceStatic(rt.Base, file, call.FileName, rt.BackupDir, rt.Instance)
}

func (scriptMode) Upgrade(rt core.Runtime, zipPath string) (core.ApplyReport, error) {
	return core.ApplyReport{}, fmt.Errorf("script 模式不支持运行时升级")
}

func (scriptMode) MarkStartupLog(rt core.Runtime) core.LogMark { return core.LogMark{} }

func (scriptMode) ProbeStartup(ctx context.Context, rt core.Runtime, mark core.LogMark) protocol.CommandStep {
	return core.ProbeProcessStartup(ctx, rt)
}
