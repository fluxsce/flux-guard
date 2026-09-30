// Package archive 是数据归档任务扩展。编码与策略只在本包，调度核不引用。
package archive

import (
	"context"
	"encoding/json"
	"fmt"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	"fluxguard/server/job/policy"
	"fluxguard/server/limits"
)

const (
	// Code 任务编码。
	Code = "archive"
	// Kind 控制台表单种类。
	Kind = "archive"
)

func init() {
	catalog.Register(catalog.Spec{
		Code:        Code,
		Title:       "数据归档",
		Description: "按保留天数硬删审计、运维、已恢复预警与主机样本等过期行。未恢复预警与未完成命令不删。",
		Kind:        Kind,
		Source:      catalog.SourceBuiltin,
		Scope:       catalog.ScopeTenant,
	})
	policy.SetDefault(Code, func() json.RawMessage {
		raw, _ := json.Marshal(Default())
		return raw
	})
}

// Params 归档任务的参数对象。天数 <= 0 表示该类本轮不删。
type Params struct {
	AuditLogDays int `json:"auditLogDays"`
	OpsLogDays   int `json:"opsLogDays"`
	AlertDays    int `json:"alertDays"`
	SampleDays   int `json:"sampleDays"`
}

// Policy 数据归档任务策略：调度信封加参数对象。
// Title、Kind、Source 只在控制台新建的任务上落库，内置任务留空。
type Policy struct {
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Source          string `json:"source,omitempty"`
	Scope           string `json:"scope,omitempty"`
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	IntervalMinutes int    `json:"intervalMinutes"`
	RunAt           string `json:"runAt"`
	Cron            string `json:"cron"`
	TimeoutSeconds  int    `json:"timeoutSeconds"`
	RetryCount      int    `json:"retryCount"`
	Params          Params `json:"params"`
}

type paramsProbe struct {
	AuditLogDays *int `json:"auditLogDays"`
	OpsLogDays   *int `json:"opsLogDays"`
	AlertDays    *int `json:"alertDays"`
	SampleDays   *int `json:"sampleDays"`
}

type policyProbe struct {
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Kind            string       `json:"kind"`
	Source          string       `json:"source"`
	Scope           string       `json:"scope"`
	Enabled         *bool        `json:"enabled"`
	Mode            string       `json:"mode"`
	IntervalMinutes int          `json:"intervalMinutes"`
	RunAt           string       `json:"runAt"`
	Cron            string       `json:"cron"`
	TimeoutSeconds  int          `json:"timeoutSeconds"`
	RetryCount      *int         `json:"retryCount"`
	Params          *paramsProbe `json:"params"`
}

// Default 未落库时的归档策略。
func Default() Policy {
	return Policy{
		Enabled:         true,
		Mode:            string(contract.TriggerInterval),
		IntervalMinutes: limits.RetentionIntervalDefaultMin,
		TimeoutSeconds:  limits.JobTimeoutDefaultSec,
		RetryCount:      limits.JobRetryDefault,
		Params: Params{
			AuditLogDays: limits.RetentionAuditDefaultDays,
			OpsLogDays:   limits.RetentionOpsDefaultDays,
			AlertDays:    limits.RetentionAlertDefaultDays,
			SampleDays:   limits.RetentionSampleDefaultDays,
		},
	}
}

// Parse 解析归档策略。缺字段补默认；未写 enabled 时保持默认启用。
func Parse(raw json.RawMessage) Policy {
	v := Default()
	if len(raw) == 0 {
		return v
	}
	var doc policyProbe
	if json.Unmarshal(raw, &doc) != nil {
		return v
	}
	if doc.Title != "" {
		v.Title = doc.Title
	}
	if doc.Description != "" {
		v.Description = doc.Description
	}
	if doc.Kind != "" {
		v.Kind = doc.Kind
	}
	if doc.Source != "" {
		v.Source = doc.Source
	}
	if doc.Scope != "" {
		v.Scope = doc.Scope
	}
	if doc.Enabled != nil {
		v.Enabled = *doc.Enabled
	}
	if doc.Mode != "" {
		v.Mode = doc.Mode
	}
	if doc.IntervalMinutes > 0 {
		v.IntervalMinutes = doc.IntervalMinutes
	}
	if doc.RunAt != "" {
		v.RunAt = doc.RunAt
	}
	if doc.Cron != "" {
		v.Cron = doc.Cron
	}
	if doc.TimeoutSeconds > 0 {
		v.TimeoutSeconds = doc.TimeoutSeconds
	}
	if doc.RetryCount != nil {
		v.RetryCount = *doc.RetryCount
	}
	if doc.Params != nil {
		applyParams(&v.Params, *doc.Params)
	}
	return merge(v)
}

// ParseParams 解析一次调用传入的参数对象。缺字段补默认天数。
func ParseParams(raw json.RawMessage) Params {
	v := Default().Params
	if len(raw) == 0 || string(raw) == "null" {
		return v
	}
	var doc paramsProbe
	if json.Unmarshal(raw, &doc) != nil {
		return v
	}
	applyParams(&v, doc)
	return mergeParams(v)
}

func applyParams(dst *Params, src paramsProbe) {
	if src.AuditLogDays != nil {
		dst.AuditLogDays = *src.AuditLogDays
	}
	if src.OpsLogDays != nil {
		dst.OpsLogDays = *src.OpsLogDays
	}
	if src.AlertDays != nil {
		dst.AlertDays = *src.AlertDays
	}
	if src.SampleDays != nil {
		dst.SampleDays = *src.SampleDays
	}
}

func merge(v Policy) Policy {
	d := Default()
	if v.Mode == "" {
		v.Mode = d.Mode
	}
	if v.IntervalMinutes <= 0 {
		v.IntervalMinutes = d.IntervalMinutes
	}
	if v.TimeoutSeconds <= 0 {
		v.TimeoutSeconds = d.TimeoutSeconds
	}
	if v.RetryCount < 0 {
		v.RetryCount = d.RetryCount
	}
	v.Params = mergeParams(v.Params)
	return v
}

func mergeParams(v Params) Params {
	d := Default().Params
	if v.AuditLogDays < 0 {
		v.AuditLogDays = d.AuditLogDays
	}
	if v.OpsLogDays < 0 {
		v.OpsLogDays = d.OpsLogDays
	}
	if v.AlertDays < 0 {
		v.AlertDays = d.AlertDays
	}
	if v.SampleDays < 0 {
		v.SampleDays = d.SampleDays
	}
	return v
}

// Validate 校验归档策略。
func Validate(v Policy) error {
	if err := policy.ValidateSchedule(policy.Schedule{
		Enabled:         v.Enabled,
		Mode:            v.Mode,
		IntervalMinutes: v.IntervalMinutes,
		RunAt:           v.RunAt,
		Cron:            v.Cron,
		TimeoutSeconds:  v.TimeoutSeconds,
		RetryCount:      v.RetryCount,
	}); err != nil {
		return err
	}
	for _, item := range []struct {
		name string
		days int
	}{
		{"审计日志", v.Params.AuditLogDays},
		{"运维日志", v.Params.OpsLogDays},
		{"预警记录", v.Params.AlertDays},
		{"样本数据", v.Params.SampleDays},
	} {
		if item.days < limits.RetentionDaysMin || item.days > limits.RetentionDaysMax {
			return fmt.Errorf("%s归档天数须在 %d-%d 之间", item.name, limits.RetentionDaysMin, limits.RetentionDaysMax)
		}
	}
	return nil
}

// Encode 写成入库 JSON。
func Encode(v Policy) (json.RawMessage, error) {
	return json.Marshal(merge(v))
}

// Put 写入归档策略缓存。
func Put(tenantID string, v Policy) {
	raw, err := Encode(v)
	if err != nil {
		return
	}
	policy.Put(tenantID, Code, raw)
}

// Get 读租户归档策略。
func Get(tenantID string) Policy {
	return Parse(policy.Get(tenantID, Code))
}

type paramsCtxKey struct{}

// WithParams 把本次调用的参数对象放进上下文，删除逻辑只读这一份。
func WithParams(ctx context.Context, p Params) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, paramsCtxKey{}, mergeParams(p))
}

// HasParams 上下文里是否带了本次调用的参数。带了就不再回看内置归档任务的启停。
func HasParams(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(paramsCtxKey{}).(Params)
	return ok
}

// Days 本次调用的保留天数。上下文里没有时回落到已保存策略。
func Days(ctx context.Context, tenantID string) Params {
	if ctx != nil {
		if p, ok := ctx.Value(paramsCtxKey{}).(Params); ok {
			return p
		}
	}
	return Get(tenantID).Params
}
