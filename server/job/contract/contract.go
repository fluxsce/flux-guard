// Package contract 是任务接口。调度核只调用它，不包含业务逻辑。
package contract

import (
	"context"
	"encoding/json"
)

// Trigger 一次调用的来源。与策略里的调度方式用同一组取值。
type Trigger string

const (
	// TriggerManual 前端或接口立即执行。
	TriggerManual Trigger = "manual"
	// TriggerInterval 按间隔定时。
	TriggerInterval Trigger = "interval"
	// TriggerAt 到达指定时间执行一次。
	TriggerAt Trigger = "at"
	// TriggerCron 按五段 cron 表达式执行，表达式本身任意。
	TriggerCron Trigger = "cron"
)

// Meta 登记一个任务时交给目录的静态说明。
type Meta struct {
	Code        string // 任务编码，进程内唯一
	Title       string
	Description string
	Kind        string // 控制台表单种类，由登记方定义
	Source      string // catalog.SourceBuiltin 或 SourceExternal
	Scope       string // catalog.ScopeTenant 或 ScopeSystem
}

// Call 调度器交给业务的一次调用。Params 是策略里的参数对象，调度核不解释。
type Call struct {
	TenantID string          // 租户级任务为租户 ID；全站任务为 catalog.SystemTenant
	Trigger  Trigger         // 本次是手动、间隔、指定时间或 cron
	Params   json.RawMessage // 策略 JSON 的 params，缺省为 {}
}

// Result 业务返回的通用结果。Data 原样写入执行日志，调度器不解释字段。
// 成功、失败和耗时由调度器根据 error 与调用耗时记录，不放在这里。
type Result struct {
	Data json.RawMessage
}

// Known 是否为调度器认识的触发来源。
func (t Trigger) Known() bool {
	switch t {
	case TriggerManual, TriggerInterval, TriggerAt, TriggerCron:
		return true
	default:
		return false
	}
}

// Job 业务任务。实现后用 registry.Register 挂上，可登记任意多个。
type Job interface {
	// Meta 返回目录说明。Code 为空时 Register 会 panic。
	Meta() Meta
	// Run 执行一次业务。返回的 error 表示失败；Data 是成功或失败都要留下的结果对象。
	Run(ctx context.Context, call Call) (Result, error)
}
