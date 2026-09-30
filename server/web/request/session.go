package request

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"fluxguard/server/cache"
	"fluxguard/server/errcode"
	"fluxguard/server/setting"
	"fluxguard/server/store"
)

type contextKey struct{}

// ErrNoSession 表示这次调用没有登录上下文。
var ErrNoSession = errcode.New("auth.session.missing", "未登录")

const (
	// SessionCookie 是登录票的 Cookie 名。值是缓存键里的会话编号。
	SessionCookie = "FG_SESSION"
	sessionPath   = "/"
)

// sessionCookieSecure 由进程入口按配置写入：enable_https 或显式 cookie_secure。
var sessionCookieSecure atomic.Bool

// ConfigureSessionCookie 设置会话 Cookie 的 Secure 位。须在开始监听前调用。
func ConfigureSessionCookie(secure bool) {
	sessionCookieSecure.Store(secure)
}

// Principal 是已登录用户，各模块从请求上下文读取，不依赖登录包。
// TenantAdmin 在登录时按 FG_USER.tenantAdminFlag 写入。之后只读这个字段。
type Principal struct {
	UserID      string
	TenantID    string
	UserName    string
	RealName    string
	TenantAdmin bool
}

// Session 是一次已认证请求上的操作者，也是缓存里的会话正文。
// At 是认证通过的时刻，写入 addTime、editTime 时用同一值。
// Expire 是用户账号的过期时间。Until 是这张登录票的到期时间，缓存 TTL 和 Cookie MaxAge 都用它。
type Session struct {
	Principal
	Token  string
	At     time.Time
	Expire time.Time
	Until  time.Time
}

// With 把操作者放进 ctx。At 为零时取当前 UTC。
func With(ctx context.Context, principal Principal) context.Context {
	return Attach(ctx, Session{Principal: principal})
}

// Attach 把完整会话放进 ctx。At 为零时取当前 UTC。
func Attach(ctx context.Context, sess Session) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if sess.At.IsZero() {
		sess.At = time.Now().UTC()
	}
	return context.WithValue(ctx, contextKey{}, sess)
}

// Current 读取 ctx 里的操作者。没有时 ok 为 false。
func Current(ctx context.Context) (Session, bool) {
	if ctx == nil {
		return Session{}, false
	}
	sess, ok := ctx.Value(contextKey{}).(Session)
	return sess, ok
}

// Require 读取 ctx 里的操作者。没有时返回 ErrNoSession。
func Require(ctx context.Context) (Session, error) {
	sess, ok := Current(ctx)
	if !ok || sess.UserID == "" || sess.TenantID == "" {
		return Session{}, ErrNoSession
	}
	return sess, nil
}

// When 把认证时刻写成库里的时间文本。
func (s Session) When() string {
	return store.FormatTime(s.At)
}

// Timeout 返回租户的访问策略。会话时长、接口超时和分页都从环境设置缓存读取。
func Timeout(tenantID string) setting.WebTimeout {
	return setting.Get(tenantID)
}

// SessionTTL 返回登录会话有效期。时长来自环境设置里的 sessionExpireHours。
func SessionTTL(tenantID string) time.Duration {
	return setting.SessionTTL(tenantID)
}

// PageSize 按环境设置收束每页条数。未传时用默认值。
func PageSize(tenantID string, requested int, provided bool) int {
	return setting.ClampPageSize(tenantID, requested, provided)
}

// Put 写入进程缓存。expiration 为 0 时使用缓存默认过期时间，负数表示永不过期。
func Put(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	return cache.GetDefaultCache().Set(ctx, key, value, expiration)
}

// Get 读取进程缓存。键不存在或已过期时 ok 为 false。
func Get(ctx context.Context, key string) ([]byte, bool) {
	value, err := cache.GetDefaultCache().Get(ctx, key)
	if err != nil || len(value) == 0 {
		return nil, false
	}
	return value, true
}

// Delete 删除进程缓存中的键。
func Delete(ctx context.Context, key string) error {
	return cache.GetDefaultCache().Delete(ctx, key)
}

// PutOnce 仅在键不存在时写入。已存在或仍未过期时 ok 为 false。
func PutOnce(ctx context.Context, key, value string, expiration time.Duration) (bool, error) {
	return cache.GetDefaultCache().SetNXString(ctx, key, value, expiration)
}

// Match 返回仍有效、且键名匹配 pattern 的键。pattern 支持前缀加 *。
func Match(ctx context.Context, pattern string) ([]string, error) {
	return cache.GetDefaultCache().Keys(ctx, pattern)
}

// Remain 返回键的剩余有效期。键不存在时 ok 为 false。
func Remain(ctx context.Context, key string) (time.Duration, bool) {
	ttl, err := cache.GetDefaultCache().TTL(ctx, key)
	if err != nil || ttl < 0 {
		return 0, false
	}
	return ttl, true
}

// Token 读取这次请求的登录票。优先 Cookie，其次 Authorization Bearer。
func Token(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if value, err := c.Cookie(SessionCookie); err == nil {
		if token := strings.TrimSpace(value); token != "" {
			return token
		}
	}
	const prefix = "Bearer "
	value := c.GetHeader("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(value[len(prefix):])
}

// HasSessionCookie 表示浏览器带上了登录票 Cookie。
func HasSessionCookie(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	value, err := c.Cookie(SessionCookie)
	return err == nil && strings.TrimSpace(value) != ""
}

// WriteSessionCookie 按登录票到期时间写 Cookie。MaxAge 与缓存 TTL 使用同一个到期时刻。
func WriteSessionCookie(c *gin.Context, token string, expire time.Time) {
	if c == nil || token == "" {
		return
	}
	maxAge := int(time.Until(expire).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, token, maxAge, sessionPath, "", sessionCookieSecure.Load(), true)
}

// ClearSessionCookie 删掉登录票 Cookie。缓存里的会话要单独删除。
func ClearSessionCookie(c *gin.Context) {
	if c == nil {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookie, "", -1, sessionPath, "", sessionCookieSecure.Load(), true)
}
