// Package policy 是任务策略缓存与落库。分组键为 job:{登记方给出的 code}，不含业务名。
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"fluxguard/server/job/contract"
	"fluxguard/server/utils/const/limits"
	dbx "github.com/fluxsce/dbx/db"
)

const groupPrefix = "job:"

// Schedule 所有任务共用的调度信封。参数对象在 JSON 的 params 字段，由业务自己解释。
type Schedule struct {
	Enabled         bool   `json:"enabled"`         // 停用后定时与手动都不调用
	Mode            string `json:"mode"`            // manual、interval、at、cron；空值按 interval
	IntervalMinutes int    `json:"intervalMinutes"` // mode 为 interval 时的分钟间隔
	RunAt           string `json:"runAt"`           // mode 为 at 时的一次执行时间
	Cron            string `json:"cron"`            // mode 为 cron 时的五段表达式，按 UTC
	TimeoutSeconds  int    `json:"timeoutSeconds"`  // 单次调用超时，秒
	RetryCount      int    `json:"retryCount"`      // 失败后再试几次，0 表示不重试
}

// Group 策略在 FG_SYS_SETTING 中的分组编码。
func Group(code string) string {
	return groupPrefix + strings.TrimSpace(code)
}

// TenantKey 空租户归一为 default。
func TenantKey(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return "default"
	}
	return tenantID
}

// Key 缓存键。
func Key(tenantID, code string) string {
	return TenantKey(tenantID) + "\x00" + strings.TrimSpace(code)
}

type cache struct {
	mu    sync.RWMutex
	items map[string]json.RawMessage
}

var policies = &cache{items: map[string]json.RawMessage{}}

// Forget 丢掉进程缓存。删除任务后调用，避免还读到旧策略。
func Forget(tenantID, code string) {
	if strings.TrimSpace(code) == "" {
		return
	}
	policies.mu.Lock()
	delete(policies.items, Key(tenantID, code))
	policies.mu.Unlock()
}

// Put 写入进程缓存。
func Put(tenantID, code string, raw json.RawMessage) {
	if strings.TrimSpace(code) == "" {
		return
	}
	cp := append(json.RawMessage(nil), raw...)
	policies.mu.Lock()
	policies.items[Key(tenantID, code)] = cp
	policies.mu.Unlock()
}

// Get 返回策略 JSON。没有缓存时用该 code 的默认工厂；仍没有则返回停用信封。
func Get(tenantID, code string) json.RawMessage {
	policies.mu.RLock()
	raw, ok := policies.items[Key(tenantID, code)]
	policies.mu.RUnlock()
	if ok && len(raw) > 0 {
		return append(json.RawMessage(nil), raw...)
	}
	code = strings.TrimSpace(code)
	if d, ok := defaults[code]; ok {
		return d()
	}
	return json.RawMessage(`{"enabled":false,"mode":"interval","intervalMinutes":60,"runAt":"","cron":"","timeoutSeconds":600,"retryCount":1,"params":{}}`)
}

type defaultFunc func() json.RawMessage

var defaults = map[string]defaultFunc{}

// SetDefault 登记某任务的默认策略工厂。
func SetDefault(code string, fn func() json.RawMessage) {
	defaults[strings.TrimSpace(code)] = fn
}

// ValidateSchedule 校验公共调度字段。手动不看间隔；指定时间必须能解析；cron 须为五段表达式。
func ValidateSchedule(s Schedule) error {
	switch normalizeMode(s.Mode) {
	case string(contract.TriggerManual):
	case string(contract.TriggerAt):
		if _, err := ParseAt(s.RunAt); err != nil {
			return fmt.Errorf("指定时间须为 YYYY-MM-DD HH:MM:SS 或 RFC3339")
		}
	case string(contract.TriggerCron):
		if _, err := ParseCron(s.Cron); err != nil {
			return fmt.Errorf("cron 须为五段表达式（分 时 日 月 周），例如 0 2 * * *")
		}
	case string(contract.TriggerInterval):
		if s.IntervalMinutes < limits.RetentionIntervalMinMin || s.IntervalMinutes > limits.RetentionIntervalMaxMin {
			return fmt.Errorf("执行间隔须在 %d-%d 分钟之间", limits.RetentionIntervalMinMin, limits.RetentionIntervalMaxMin)
		}
	default:
		return fmt.Errorf("调度方式无效")
	}
	timeout := s.TimeoutSeconds
	if timeout == 0 {
		timeout = limits.JobTimeoutDefaultSec
	}
	if timeout < limits.JobTimeoutMinSec || timeout > limits.JobTimeoutMaxSec {
		return fmt.Errorf("单次超时须在 %d-%d 秒之间", limits.JobTimeoutMinSec, limits.JobTimeoutMaxSec)
	}
	if s.RetryCount < 0 || s.RetryCount > limits.JobRetryMax {
		return fmt.Errorf("失败重试次数须在 0-%d 之间", limits.JobRetryMax)
	}
	return nil
}

// DecodeSchedule 从策略 JSON 抽出调度信封。未写超时和重试时补默认。
func DecodeSchedule(raw json.RawMessage) Schedule {
	var s Schedule
	_ = json.Unmarshal(raw, &s)
	s.Mode = normalizeMode(s.Mode)
	if s.IntervalMinutes <= 0 {
		s.IntervalMinutes = limits.RetentionIntervalDefaultMin
	}
	if s.TimeoutSeconds <= 0 {
		s.TimeoutSeconds = limits.JobTimeoutDefaultSec
	}
	if !jsonHas(raw, "retryCount") {
		s.RetryCount = limits.JobRetryDefault
	}
	if s.RetryCount < 0 {
		s.RetryCount = 0
	}
	return s
}

func jsonHas(raw json.RawMessage, key string) bool {
	var doc map[string]json.RawMessage
	if json.Unmarshal(raw, &doc) != nil {
		return false
	}
	_, ok := doc[key]
	return ok
}

// Document 落库的策略信封。参数对象由业务自己解释。
type Document struct {
	Title       string
	Description string
	Kind        string
	Source      string
	Scope       string
	Schedule
	Params json.RawMessage
}

type storedDocument struct {
	Title           string          `json:"title,omitempty"`
	Description     string          `json:"description,omitempty"`
	Kind            string          `json:"kind,omitempty"`
	Source          string          `json:"source,omitempty"`
	Scope           string          `json:"scope,omitempty"`
	Enabled         bool            `json:"enabled"`
	Mode            string          `json:"mode"`
	IntervalMinutes int             `json:"intervalMinutes"`
	RunAt           string          `json:"runAt"`
	Cron            string          `json:"cron"`
	TimeoutSeconds  int             `json:"timeoutSeconds"`
	RetryCount      int             `json:"retryCount"`
	Params          json.RawMessage `json:"params"`
}

// ParseObject 把用户写下的参数收成 JSON 对象。空字符串当作 {}。
func ParseObject(text string) (json.RawMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return json.RawMessage(`{}`), nil
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return nil, fmt.Errorf("参数须为 JSON 对象")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, fmt.Errorf("参数须为 JSON 对象")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("参数须为 JSON 对象")
	}
	return raw, nil
}

// EncodeDocument 写成入库 JSON。参数原样放在 params 字段。
func EncodeDocument(doc Document) (json.RawMessage, error) {
	params, err := ParseObject(string(doc.Params))
	if err != nil {
		return nil, err
	}
	return json.Marshal(storedDocument{
		Title:           strings.TrimSpace(doc.Title),
		Description:     strings.TrimSpace(doc.Description),
		Kind:            strings.TrimSpace(doc.Kind),
		Source:          strings.TrimSpace(doc.Source),
		Scope:           strings.TrimSpace(doc.Scope),
		Enabled:         doc.Enabled,
		Mode:            normalizeMode(doc.Mode),
		IntervalMinutes: doc.IntervalMinutes,
		RunAt:           strings.TrimSpace(doc.RunAt),
		Cron:            strings.TrimSpace(doc.Cron),
		TimeoutSeconds:  doc.TimeoutSeconds,
		RetryCount:      doc.RetryCount,
		Params:          params,
	})
}

// Params 取出策略里的参数对象。没有时返回空对象。
func Params(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var doc struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Params) == 0 || string(doc.Params) == "null" {
		return json.RawMessage(`{}`)
	}
	return append(json.RawMessage(nil), doc.Params...)
}

func normalizeMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case string(contract.TriggerManual), string(contract.TriggerAt), string(contract.TriggerCron):
		return strings.TrimSpace(mode)
	default:
		return string(contract.TriggerInterval)
	}
}

// ParseAt 解析指定执行时间。RFC3339 保留偏移；墙钟字符串按 UTC。
func ParseAt(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad time")
}

// Load 把已落库的 job:* 策略读进缓存。
func Load(ctx context.Context, db *dbx.DB) error {
	if db == nil {
		return nil
	}
	rows, err := db.Query(ctx, `
		SELECT tenantId, groupCode, content
		FROM FG_SYS_SETTING
		WHERE activeFlag = 'Y' AND groupCode LIKE 'job:%'`, nil)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tenantID, groupCode, content string
		if err := rows.Scan(&tenantID, &groupCode, &content); err != nil {
			return err
		}
		code := strings.TrimPrefix(groupCode, groupPrefix)
		if code == "" || code == groupCode {
			continue
		}
		Put(tenantID, code, json.RawMessage(content))
	}
	return rows.Err()
}
