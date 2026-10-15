package static

import (
	"context"
	"fluxguard/agents/appside/core"
	"fmt"

	"fluxguard/protocol"
)

func init() {
	core.RegisterMode(staticMode{})
}

type staticMode struct{}

func (staticMode) Name() string { return "static" }

func (staticMode) AllowKind(kind string) bool { return kind == "static" }

func (staticMode) NeedsRestart(kind string) bool { return false }

func (staticMode) SupportUpgrade() bool { return false }

func (staticMode) FillSide(side *protocol.AppSide, rt core.Runtime) {}

func (staticMode) CollectArtifacts(ctx context.Context, rt core.Runtime) ([]core.Artifact, error) {
	_ = ctx
	name := rt.StaticApp
	if name == "" {
		name = rt.Instance
	}
	return []core.Artifact{{
		AppName: name,
		Kind:    "static",
		Path:    rt.StaticDir,
	}}, nil
}

func (staticMode) ListInventory(rt core.Runtime) ([]core.Artifact, error) {
	return nil, fmt.Errorf("实例 %s 是 static 模式，不支持列目录", rt.Instance)
}

func (staticMode) Replace(rt core.Runtime, call protocol.CommandCall, file string) (core.ApplyReport, error) {
	return core.ReplaceStatic(rt.StaticDir, file, call.FileName, rt.BackupDir, rt.Instance)
}

func (staticMode) Upgrade(rt core.Runtime, zipPath string) (core.ApplyReport, error) {
	return core.ApplyReport{}, fmt.Errorf("static 模式不支持运行时升级")
}

func (staticMode) MarkStartupLog(rt core.Runtime) core.LogMark { return core.LogMark{} }

func (staticMode) ProbeStartup(ctx context.Context, rt core.Runtime, mark core.LogMark) protocol.CommandStep {
	_ = ctx
	_ = rt
	_ = mark
	return protocol.CommandStep{Name: "startup", Status: "done", Detail: "静态资源无需启动探测"}
}
