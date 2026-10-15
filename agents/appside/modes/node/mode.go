package node

import (
	"context"
	"fluxguard/agents/appside/core"
	"fmt"

	"fluxguard/protocol"
)

func init() {
	core.RegisterMode(nodeMode{})
}

type nodeMode struct{}

func (nodeMode) Name() string { return "node" }

func (nodeMode) AllowKind(kind string) bool { return kind == "node" }

func (nodeMode) NeedsRestart(kind string) bool { return true }

func (nodeMode) SupportUpgrade() bool { return false }

func (nodeMode) FillSide(side *protocol.AppSide, rt core.Runtime) {}

func (nodeMode) CollectArtifacts(ctx context.Context, rt core.Runtime) ([]core.Artifact, error) {
	_ = ctx
	pkg := ReadNodePackage(rt.Base)
	return []core.Artifact{{
		AppName: NodeAppName(rt),
		Kind:    "node",
		Version: pkg.Version,
		Path:    rt.Base,
	}}, nil
}

func (nodeMode) ListInventory(rt core.Runtime) ([]core.Artifact, error) {
	return nil, fmt.Errorf("实例 %s 是 node 模式，不支持列目录", rt.Instance)
}

func (nodeMode) Replace(rt core.Runtime, call protocol.CommandCall, file string) (core.ApplyReport, error) {
	return core.ReplaceStatic(rt.Base, file, call.FileName, rt.BackupDir, rt.Instance)
}

func (nodeMode) Upgrade(rt core.Runtime, zipPath string) (core.ApplyReport, error) {
	return core.ApplyReport{}, fmt.Errorf("node 模式不支持运行时升级")
}

func (nodeMode) MarkStartupLog(rt core.Runtime) core.LogMark { return core.LogMark{} }

func (nodeMode) ProbeStartup(ctx context.Context, rt core.Runtime, mark core.LogMark) protocol.CommandStep {
	return core.ProbeProcessStartup(ctx, rt)
}
