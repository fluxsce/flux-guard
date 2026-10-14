package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/utils/const/errcode"
	"fluxguard/server/utils/secret"
	"fluxguard/server/setting"
	"fluxguard/server/web/request"
	logindao "fluxguard/server/web/views/login/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	// ErrUserRequired 表示未填写用户。
	ErrUserRequired = errcode.New("auth.user.required", "用户不能为空")
	// ErrPasswordRequired 表示未填写密码。
	ErrPasswordRequired = errcode.New("auth.password.required", "密码不能为空")
	// ErrBadLogin 表示用户或密码不正确。
	ErrBadLogin = errcode.New("auth.bad_login", "用户或密码错误")
	// ErrDisabled 表示账号已停用。
	ErrDisabled = errcode.New("auth.disabled", "账号已停用")
	// ErrExpired 表示用户账号已过过期时间。
	ErrExpired = errcode.New("auth.expired", "账号已过期")
	// ErrCurrentPassword 表示修改口令时当前口令不对。用 400 返回，避免前端把会话 401 当成登出。
	ErrCurrentPassword = errcode.New("auth.password.current", "当前口令不正确")
	// ErrProfileMissing 表示会话用户已不存在或已停用。
	ErrProfileMissing = errcode.New("auth.profile.missing", "账号不存在或已停用")
)

const sessionKeyPrefix = "session:"

// session 是缓存里的一张登录票。Until 是票的到期时间，时长来自环境设置。
// User.Expire 是账号过期时间，和票的到期时间分开存。
type session struct {
	Token    string          `json:"token"`
	UserID   string          `json:"userId"`
	TenantID string          `json:"tenantId"`
	Until    time.Time       `json:"until"`
	User     request.Session `json:"user"`
}

// Ticket 是登录校验通过后签发并已缓存的票。
type Ticket struct {
	Token   string
	Expire  time.Time
	Session request.Session
}

// current 是本进程的登录会话缓存。中间件和改口令作废会话都读它，不经路由依赖传入。
var current *Auth

// Current 返回本进程登录会话。尚未 Open 时为 nil。
func Current() *Auth {
	return current
}

// Auth 持有本进程库连接。登录成功后的会话写在进程缓存里，过期由缓存负责。
type Auth struct {
	db       *dbx.DB
	tenantID string
}

// Open 使用已经灌好脚本的库。tenantID 是本进程所属租户，来自配置。
func Open(conn *dbx.DB, tenantID string) *Auth {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	current = &Auth{db: conn, tenantID: tenantID}
	return current
}

// DB 返回鉴权库连接。管理模块的 dao 用它查询，不另开连接。
func (a *Auth) DB() *dbx.DB {
	if a == nil {
		return nil
	}
	return a.db
}

// TenantID 返回本进程所属租户。模块补自己的菜单时写入这个租户。
func (a *Auth) TenantID() string {
	if a == nil {
		return "default"
	}
	if id := strings.TrimSpace(a.tenantID); id != "" {
		return id
	}
	return "default"
}

// Login 按本进程租户和用户 ID 校验口令与账号过期时间。
// 通过后把会话写入内存。票的有效期取环境设置 sessionExpireHours，短于账号过期时间时以账号过期为准。
func (a *Auth) Login(userID, password string) (Ticket, error) {
	found, err := logindao.Open(a.db).FindByUserID(context.Background(), a.TenantID(), strings.TrimSpace(userID))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !secret.MatchPassword(found.PasswordHash, password)) {
		return Ticket{}, ErrBadLogin
	}
	if err != nil {
		return Ticket{}, err
	}
	if found.StatusFlag != "Y" {
		return Ticket{}, ErrDisabled
	}
	if userExpired(found.Expire, time.Now()) {
		return Ticket{}, ErrExpired
	}
	until := time.Now().Add(setting.SessionTTL(found.TenantID))
	if !found.Expire.IsZero() && found.Expire.Before(until) {
		until = found.Expire
	}
	token := "usr_" + id.New()
	cached := request.Session{
		Principal: accountPrincipal(found, found.TenantAdminFlag == "Y"),
		Token: token, Expire: found.Expire, Until: until,
	}
	if err := saveSession(session{
		Token: token, UserID: found.UserID, TenantID: found.TenantID,
		Until: until, User: cached,
	}); err != nil {
		return Ticket{}, err
	}
	return Ticket{Token: token, Expire: until, Session: cached}, nil
}

func sessionKey(token string) string {
	return sessionKeyPrefix + token
}

func saveSession(sess session) error {
	ttl := time.Until(sess.Until)
	if ttl <= 0 {
		return request.Delete(context.Background(), sessionKey(sess.Token))
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return request.Put(context.Background(), sessionKey(sess.Token), raw, ttl)
}

func loadSession(token string) (session, bool) {
	raw, ok := request.Get(context.Background(), sessionKey(token))
	if !ok {
		return session{}, false
	}
	var sess session
	if err := json.Unmarshal(raw, &sess); err != nil || time.Now().After(sess.Until) {
		_ = request.Delete(context.Background(), sessionKey(token))
		return session{}, false
	}
	return sess, true
}

// Logout 作废会话。
func (a *Auth) Logout(token string) {
	if a == nil || token == "" {
		return
	}
	_ = request.Delete(context.Background(), sessionKey(token))
}

// Principal 读取仍有效的缓存会话。账号停用或过期后不再承认这张票。
func (a *Auth) Principal(token string) (request.Session, bool) {
	if a == nil || token == "" {
		return request.Session{}, false
	}
	sess, ok := loadSession(token)
	if !ok {
		return request.Session{}, false
	}
	found, err := logindao.Open(a.db).FindActive(context.Background(), sess.TenantID, sess.UserID)
	if err != nil || found.ActiveFlag != "Y" || found.StatusFlag != "Y" || userExpired(found.Expire, time.Now()) {
		a.Logout(token)
		return request.Session{}, false
	}
	until := time.Now().Add(setting.SessionTTL(found.TenantID))
	if !found.Expire.IsZero() && found.Expire.Before(until) {
		until = found.Expire
	}
	sess.Until = until
	sess.User = request.Session{
		Principal: accountPrincipal(found, sess.User.TenantAdmin),
		Token: token, Expire: found.Expire, Until: until,
	}
	if err := saveSession(sess); err != nil {
		return request.Session{}, false
	}
	return sess.User, true
}

func accountPrincipal(found logindao.Account, tenantAdmin bool) request.Principal {
	return request.Principal{
		UserID: found.UserID, TenantID: found.TenantID,
		UserName: found.UserName, RealName: found.RealName,
		TenantAdmin: tenantAdmin,
	}
}

func userExpired(expire, now time.Time) bool {
	return expire.IsZero() || !now.Before(expire)
}

// DropUser 作废该用户仍留在内存里的会话。改口令或删除用户时由用户模块调用。
func (a *Auth) DropUser(userID string) {
	if a == nil || userID == "" {
		return
	}
	a.dropMatching(func(sess session) bool {
		return sess.UserID == userID
	})
}

func (a *Auth) dropOtherSessions(tenantID, userID, keepToken string) {
	a.dropMatching(func(sess session) bool {
		return sess.Token != keepToken && sess.TenantID == tenantID && sess.UserID == userID
	})
}

func (a *Auth) dropMatching(match func(session) bool) {
	if a == nil {
		return
	}
	keys, err := request.Match(context.Background(), sessionKeyPrefix+"*")
	if err != nil {
		return
	}
	for _, key := range keys {
		raw, ok := request.Get(context.Background(), key)
		if !ok {
			continue
		}
		var sess session
		if json.Unmarshal(raw, &sess) == nil && match(sess) {
			_ = request.Delete(context.Background(), key)
		}
	}
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
