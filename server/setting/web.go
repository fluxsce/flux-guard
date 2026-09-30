// Package setting 保存租户级环境设置，并给登录会话、接口超时和列表分页共用。
package setting

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"fluxguard/server/limits"
)

const (
	// GroupWebTimeout 管理端访问：接口超时、会话时长、分页档位。
	GroupWebTimeout = "webTimeout"

	// PageSizeFloor 每页条数下限。
	PageSizeFloor = limits.PageSizeFloor
	// PageSizeCeiling 每页条数的代码上限。环境设置不能超过它。
	PageSizeCeiling = limits.PageSizeCeiling
	// DefaultListPageSize 未配置时的默认每页条数。
	DefaultListPageSize = limits.PageSizeDefault
)

// WebTimeout 管理端访问策略。
// RequestTimeoutSeconds 只表示接口/前端预算，不再直接等于 http.Server.ReadTimeout。
type WebTimeout struct {
	RequestTimeoutSeconds int `json:"requestTimeoutSeconds"`
	SessionExpireHours    int `json:"sessionExpireHours"`
	DefaultPageSize       int `json:"defaultPageSize"`
	MaxPageSize           int `json:"maxPageSize"`
}

type cache struct {
	mu      sync.RWMutex
	timeout map[string]WebTimeout
}

var global = &cache{timeout: map[string]WebTimeout{}}

var (
	applierMu sync.RWMutex
	applier   func(apiTimeoutSec int)
)

// RegisterHTTPTimeoutApplier 由进程入口注册。
// 保存访问策略后只回写 WriteTimeout（见 limits.ServerWrite）；ReadTimeout 固定服务大包上传。
func RegisterHTTPTimeoutApplier(fn func(apiTimeoutSec int)) {
	applierMu.Lock()
	applier = fn
	applierMu.Unlock()
}

func notifyHTTPTimeout(apiTimeoutSec int) {
	applierMu.RLock()
	fn := applier
	applierMu.RUnlock()
	if fn != nil {
		fn(apiTimeoutSec)
	}
}

func tenantKey(tenantID string) string {
	if tenantID == "" {
		return "default"
	}
	return tenantID
}

// Default 返回未落库时的访问策略。
func Default() WebTimeout {
	return WebTimeout{
		RequestTimeoutSeconds: limits.APIRequestDefaultSec,
		SessionExpireHours:    limits.SessionDefaultHours,
		DefaultPageSize:       DefaultListPageSize,
		MaxPageSize:           PageSizeCeiling,
	}
}

// Parse 解析分组 JSON。缺字段用默认值补齐。
func Parse(content string) WebTimeout {
	v := Default()
	if content == "" {
		return v
	}
	_ = json.Unmarshal([]byte(content), &v)
	return merge(v)
}

func merge(v WebTimeout) WebTimeout {
	d := Default()
	if v.RequestTimeoutSeconds <= 0 {
		v.RequestTimeoutSeconds = d.RequestTimeoutSeconds
	}
	if v.SessionExpireHours <= 0 {
		v.SessionExpireHours = d.SessionExpireHours
	}
	if v.DefaultPageSize <= 0 {
		v.DefaultPageSize = d.DefaultPageSize
	}
	if v.MaxPageSize <= 0 {
		v.MaxPageSize = d.MaxPageSize
	}
	return v
}

// Validate 校验超时和分页是否在允许区间。
func Validate(v WebTimeout) error {
	if v.RequestTimeoutSeconds < limits.APIRequestMinSec || v.RequestTimeoutSeconds > limits.APIRequestMaxSec {
		return fmt.Errorf("接口超时须在 %d-%d 秒之间", limits.APIRequestMinSec, limits.APIRequestMaxSec)
	}
	if v.SessionExpireHours < limits.SessionMinHours || v.SessionExpireHours > limits.SessionMaxHours {
		return fmt.Errorf("会话有效期须在 %d-%d 小时之间", limits.SessionMinHours, limits.SessionMaxHours)
	}
	if v.MaxPageSize < PageSizeFloor || v.MaxPageSize > PageSizeCeiling {
		return fmt.Errorf("每页条数上限须在 %d-%d 之间", PageSizeFloor, PageSizeCeiling)
	}
	if v.DefaultPageSize < PageSizeFloor || v.DefaultPageSize > v.MaxPageSize {
		return fmt.Errorf("默认每页条数须在 %d-%d 之间", PageSizeFloor, v.MaxPageSize)
	}
	return nil
}

// Get 返回租户访问策略。尚未加载时返回默认值。
func Get(tenantID string) WebTimeout {
	tenantID = tenantKey(tenantID)
	global.mu.RLock()
	defer global.mu.RUnlock()
	if v, ok := global.timeout[tenantID]; ok {
		return v
	}
	return Default()
}

// Put 写入缓存，并通知 HTTP 服务按接口超时更新写超时。
func Put(tenantID string, v WebTimeout) {
	tenantID = tenantKey(tenantID)
	merged := merge(v)
	global.mu.Lock()
	global.timeout[tenantID] = merged
	global.mu.Unlock()
	notifyHTTPTimeout(merged.RequestTimeoutSeconds)
}

// SessionTTL 返回登录会话有效期。
func SessionTTL(tenantID string) time.Duration {
	hours := Get(tenantID).SessionExpireHours
	if hours < limits.SessionMinHours {
		hours = limits.SessionDefaultHours
	}
	return time.Duration(hours) * time.Hour
}

// ClampPageSize 按租户分页策略收束每页条数。未传时用默认值。
func ClampPageSize(tenantID string, requested int, provided bool) int {
	v := merge(Get(tenantID))
	if v.MaxPageSize > PageSizeCeiling || v.MaxPageSize < PageSizeFloor {
		v.MaxPageSize = PageSizeCeiling
	}
	if v.DefaultPageSize < PageSizeFloor || v.DefaultPageSize > v.MaxPageSize {
		v.DefaultPageSize = DefaultListPageSize
		if v.DefaultPageSize > v.MaxPageSize {
			v.DefaultPageSize = v.MaxPageSize
		}
	}
	if !provided || requested < PageSizeFloor {
		return v.DefaultPageSize
	}
	if requested > v.MaxPageSize {
		return v.MaxPageSize
	}
	return requested
}
