package controllers

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/server/utils/secret"
	"fluxguard/server/web/request"
	logindao "fluxguard/server/web/views/login/dao"
	"fluxguard/server/web/views/login/models"
)

// profileError 是返回给本人的资料校验说明，不含库错误。
type profileError struct {
	msg string
}

func (e profileError) Error() string {
	return e.msg
}

func invalidProfile(msg string) error {
	return profileError{msg: msg}
}

// IsProfileInput 表示错误是本人资料校验失败，可以原样返回给当前用户。
func IsProfileInput(err error) bool {
	var invalid profileError
	return errors.As(err, &invalid)
}

// Profile 读取会话用户本人的资料。
func (a *Auth) Profile(tenantID, userID string) (models.SelfProfile, error) {
	return a.loadSelf(tenantID, userID)
}

// UpdateSelf 只改会话用户自己的姓名、邮箱、手机、性别，以及在校验当前口令后的新口令。
// 登录名、租户、角色、管理员标记和启停状态不在更新语句里。
// 操作者和编辑时间取自 ctx 上的登录会话。
func (a *Auth) UpdateSelf(ctx context.Context, keepToken string, in models.SelfProfileInput) (models.SelfProfile, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.SelfProfile{}, err
	}
	actor := sess.Principal
	next, changePwd, err := normalizeSelf(in)
	if err != nil {
		return models.SelfProfile{}, err
	}
	users := logindao.Open(a.db)
	var password any
	var pwdUpdate any
	changeFlag := "N"
	if changePwd {
		hash, err := users.PasswordHash(ctx, actor.TenantID, actor.UserID)
		if errors.Is(err, sql.ErrNoRows) {
			return models.SelfProfile{}, ErrProfileMissing
		}
		if err != nil {
			return models.SelfProfile{}, err
		}
		if !secret.MatchPassword(hash, in.CurrentPassword) {
			return models.SelfProfile{}, ErrCurrentPassword
		}
		hashed, err := secret.HashPassword(next.NewPassword)
		if err != nil {
			return models.SelfProfile{}, invalidProfile("新口令不符合要求")
		}
		password = hashed
		pwdUpdate = sess.When()
		changeFlag = "Y"
	}
	now := sess.When()
	err = users.UpdateSelf(ctx, logindao.ProfileUpdate{
		RealName: next.RealName, Email: nullIfEmpty(next.Email), Mobile: nullIfEmpty(next.Mobile),
		Gender: next.Gender, ChangePwd: changeFlag, Password: password, PwdUpdateTime: pwdUpdate,
		EditWho: actor.UserID, EditTime: now, OprSeqFlag: id.New(),
		TenantID: actor.TenantID, UserID: actor.UserID,
	})
	if err != nil {
		return models.SelfProfile{}, err
	}
	if changePwd {
		a.dropOtherSessions(actor.TenantID, actor.UserID, keepToken)
	}
	return a.loadSelf(actor.TenantID, actor.UserID)
}

func (a *Auth) loadSelf(tenantID, userID string) (models.SelfProfile, error) {
	out, err := logindao.Open(a.db).LoadSelf(context.Background(), tenantID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.SelfProfile{}, ErrProfileMissing
	}
	return out, err
}

type normalizedSelf struct {
	RealName    string
	Email       string
	Mobile      string
	Gender      int
	NewPassword string
}

func normalizeSelf(in models.SelfProfileInput) (normalizedSelf, bool, error) {
	name := strings.TrimSpace(in.RealName)
	if name == "" || utf8.RuneCountInString(name) > 50 || hasControl(name) {
		return normalizedSelf{}, false, invalidProfile("姓名需为 1 到 50 个字符")
	}
	email := strings.TrimSpace(in.Email)
	if email != "" && !validEmail(email) {
		return normalizedSelf{}, false, invalidProfile("邮箱格式不正确")
	}
	mobile := strings.TrimSpace(in.Mobile)
	if mobile != "" && !validMobile(mobile) {
		return normalizedSelf{}, false, invalidProfile("手机号格式不正确")
	}
	if in.Gender < 0 || in.Gender > 2 {
		return normalizedSelf{}, false, invalidProfile("性别取值无效")
	}
	current := in.CurrentPassword
	next := in.NewPassword
	if current == "" && next == "" {
		return normalizedSelf{name, email, mobile, in.Gender, ""}, false, nil
	}
	if current == "" || next == "" {
		return normalizedSelf{}, false, invalidProfile("修改口令需要同时填写当前口令和新口令")
	}
	if utf8.RuneCountInString(next) < 8 || len(next) > 72 {
		return normalizedSelf{}, false, invalidProfile("新口令长度需为 8 到 72 个字符")
	}
	return normalizedSelf{name, email, mobile, in.Gender, next}, true, nil
}

func validEmail(value string) bool {
	if len(value) > 255 || hasControl(value) || strings.ContainsAny(value, " \t") {
		return false
	}
	at := strings.IndexByte(value, '@')
	if at <= 0 || at != strings.LastIndexByte(value, '@') || at == len(value)-1 {
		return false
	}
	domain := value[at+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

func validMobile(value string) bool {
	if utf8.RuneCountInString(value) > 20 {
		return false
	}
	for _, r := range value {
		if r >= '0' && r <= '9' || r == '+' || r == '-' || r == ' ' {
			continue
		}
		return false
	}
	return true
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
