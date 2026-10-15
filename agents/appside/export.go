package appside

import (
	"context"

	"fluxguard/agents/appside/core"
	"fluxguard/protocol"
)

// 对外类型别名：调用方仍用 appside.Xxx，定义在 core。
type (
	Runtime     = core.Runtime
	Artifact    = core.Artifact
	ApplyReport = core.ApplyReport
	LogMark     = core.LogMark
)

// 生产路径只转发 core（与 core.Xxx 为同一函数，无额外业务逻辑）。
// mode 实现经 register.go 空白导入登记；不要在这里再 import modes/*。
var (
	FindRuntime       = core.FindRuntime
	Replace           = core.Replace
	ListInstanceFiles = core.ListInstanceFiles
	UpgradeRuntime    = core.UpgradeRuntime
	MarkStartupLog    = core.MarkStartupLog
	NeedsRestart      = core.NeedsRestart
	Running           = core.Running
	Restart           = core.Restart
	Rollback          = core.Rollback
)

// ProbeStartup 与 core.ProbeStartup 相同，仅保持对外包路径稳定。
func ProbeStartup(ctx context.Context, rt Runtime, mark LogMark) protocol.CommandStep {
	return core.ProbeStartup(ctx, rt, mark)
}
