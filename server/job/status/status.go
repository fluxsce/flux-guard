// Package status 记录任务最近一次执行，键为 (tenant, code)。
// 绑定库之后写入 FG_JOB（最近一次）和 FG_JOB_LOG（每次执行），重启可恢复。
package status

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"fluxguard/server/job/policy"
)

// Status 最近一次执行。绑定库之后同时落在 FG_JOB，重启由 Open 装回。
type Status struct {
	TenantID   string          `json:"tenantId"`
	Code       string          `json:"code"`
	Title      string          `json:"title"`
	Enabled    bool            `json:"enabled"`
	Trigger    string          `json:"trigger,omitempty"` // manual、interval 或 at
	LastStart  time.Time       `json:"lastStart,omitempty"`
	LastEnd    time.Time       `json:"lastEnd,omitempty"`
	DurationMs int64           `json:"durationMs"` // 本次调用耗时，毫秒
	OK         bool            `json:"ok"`         // error 为空则为成功
	Result     json.RawMessage `json:"result,omitempty"`
	ErrorText  string          `json:"errorText,omitempty"`
	RanAtLeast bool            `json:"ranAtLeast"`
}

type mem struct {
	mu    sync.RWMutex
	items map[string]Status
}

var global = &mem{items: map[string]Status{}}

// Report 覆盖写入最近执行状态。
func Report(s Status) {
	s.Code = strings.TrimSpace(s.Code)
	if s.Code == "" {
		return
	}
	if s.TenantID == "" {
		s.TenantID = "default"
	}
	global.mu.Lock()
	global.items[policy.Key(s.TenantID, s.Code)] = s
	global.mu.Unlock()
	persistRun(s)
}

// Get 读取某租户任务状态。
func Get(tenantID, code string) (Status, bool) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	s, ok := global.items[policy.Key(tenantID, code)]
	return s, ok
}

// List 返回全部状态快照。
func List() []Status {
	global.mu.RLock()
	defer global.mu.RUnlock()
	out := make([]Status, 0, len(global.items))
	for _, s := range global.items {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TenantID != out[j].TenantID {
			return out[i].TenantID < out[j].TenantID
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// Touch 登记尚未跑过或仅改启停的任务。
func Touch(tenantID, code, title string, enabled bool) {
	key := policy.Key(tenantID, code)
	global.mu.Lock()
	cur, ok := global.items[key]
	if ok {
		cur.Title = title
		cur.Enabled = enabled
		global.items[key] = cur
	} else {
		cur = Status{
			TenantID: policy.TenantKey(tenantID),
			Code:     code,
			Title:    title,
			Enabled:  enabled,
		}
		global.items[key] = cur
	}
	global.mu.Unlock()
	persistTouch(cur)
}
