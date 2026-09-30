package controllers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fluxguard/server/config"
	"fluxguard/server/store"
	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware/audit"
	accessdao "fluxguard/server/web/middleware/dao"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/login/models"
	dbx "github.com/fluxsce/dbx/db"
)

// AuthController 处理控制台登录、验证码、登出、当前用户和菜单。
type AuthController struct {
	access  *accessdao.Access
	captcha *Captcha
}

// NewAuthController 创建登录控制器。会话读本进程登录缓存。
func NewAuthController(db *dbx.DB) *AuthController {
	return &AuthController{access: accessdao.Open(db), captcha: NewCaptcha()}
}

// Captcha 签发图形验证码。不要求登录。
func (c *AuthController) Captcha(g *gin.Context) {
	var body models.CaptchaRequest
	if g.Request.ContentLength > 0 && !request.Bind(g, &body) {
		return
	}
	result, err := c.captcha.Generate(g.Request.Context(), &body)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrCaptchaType) {
			status = http.StatusBadRequest
		}
		httpx.WriteFail(g, status, err)
		return
	}
	httpx.WriteOK(g, result)
}

// Login 校验口令并签发登录票。成功后把会话写入内存，并写入会话 Cookie。
// 提交了验证码票时先核销，同一张票不能再次使用。
// 会话有效期和返回给前端的接口超时都取环境设置。不要求登录。
func (c *AuthController) Login(g *gin.Context) {
	var body models.LoginRequest
	if !request.Bind(g, &body) {
		return
	}
	userID := strings.TrimSpace(body.UserID)
	noteLogin(g, userID, "", loginTenant(), "登录")
	if userID == "" {
		httpx.WriteFail(g, http.StatusBadRequest, ErrUserRequired)
		return
	}
	if body.Password == "" {
		httpx.WriteFail(g, http.StatusBadRequest, ErrPasswordRequired)
		return
	}
	if body.CaptchaId != "" || body.CaptchaCode != "" {
		if err := c.captcha.Verify(body.CaptchaId, body.CaptchaCode); err != nil {
			httpx.WriteFail(g, http.StatusBadRequest, err)
			return
		}
	}
	auth := Current()
	if auth == nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "登录服务未就绪")
		return
	}
	ticket, err := auth.Login(userID, body.Password)
	if err != nil {
		httpx.WriteFail(g, http.StatusUnauthorized, err)
		return
	}
	sess := ticket.Session
	noteLogin(g, sess.UserID, sess.UserName, sess.TenantID, "登录成功")
	request.WriteSessionCookie(g, ticket.Token, ticket.Expire)
	policy := request.Timeout(sess.TenantID)
	httpx.WriteOK(g, models.LoginResult{
		Token: ticket.Token, UserID: sess.UserID, UserName: sess.UserName, RealName: sess.RealName,
		TenantID: sess.TenantID, TenantAdminFlag: adminFlag(sess.TenantAdmin), ExpireAt: store.FormatTime(sess.Expire),
		LoginTime:       store.FormatTime(time.Now().UTC()),
		Timeout:         policy.RequestTimeoutSeconds * 1000,
		DefaultPageSize: policy.DefaultPageSize,
		MaxPageSize:     policy.MaxPageSize,
	})
}

// Logout 作废当前会话，并清掉登录票 Cookie。
func (c *AuthController) Logout(g *gin.Context) {
	if sess, ok := request.Current(g.Request.Context()); ok {
		audit.Note(g, "", "USER", sess.UserID, sess.UserName, "退出登录")
	}
	if auth := Current(); auth != nil {
		auth.Logout(request.Token(g))
	}
	request.ClearSessionCookie(g)
	httpx.WriteOK(g, map[string]string{"status": "ok"})
}

// Me 返回当前登录用户。接口超时和分页档位取环境设置，供前端覆盖访问超时。
func (c *AuthController) Me(g *gin.Context) {
	sess, _ := request.Current(g.Request.Context())
	policy := request.Timeout(sess.TenantID)
	httpx.WriteOK(g, map[string]any{
		"userId": sess.UserID, "userName": sess.UserName, "realName": sess.RealName, "tenantId": sess.TenantID,
		"tenantAdminFlag": adminFlag(sess.TenantAdmin),
		"expireAt":        store.FormatTime(sess.Expire),
		"timeout":         policy.RequestTimeoutSeconds * 1000,
		"defaultPageSize": policy.DefaultPageSize,
		"maxPageSize":     policy.MaxPageSize,
	})
}

// Menus 返回当前用户可见的模块菜单，按钮授权附在每条模块上。
func (c *AuthController) Menus(g *gin.Context) {
	p := principalOf(g)
	httpx.WriteOK(g, c.access.Menus(p.TenantID, p.UserID))
}

// Profile 返回当前登录用户可修改的本人资料。
func (c *AuthController) Profile(g *gin.Context) {
	p := principalOf(g)
	auth := Current()
	if auth == nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "登录服务未就绪")
		return
	}
	profile, err := auth.Profile(p.TenantID, p.UserID)
	if !writeProfileErr(g, err, "读取失败") {
		return
	}
	httpx.WriteOK(g, profile)
}

// SaveProfile 修改当前登录用户本人的资料。身份只取会话，请求体里的用户和权限字段会被拒绝。
func (c *AuthController) SaveProfile(g *gin.Context) {
	var body models.SelfProfileInput
	if !request.BindKnown(g, &body, "profile.fields.rejected", "只能修改本人的姓名、邮箱、手机、性别和口令") {
		return
	}
	p := principalOf(g)
	detail := "修改本人资料"
	if strings.TrimSpace(body.NewPassword) != "" {
		detail = "修改本人资料，并更换了口令"
	}
	audit.Note(g, "", "USER", p.UserID, body.RealName, detail)
	auth := Current()
	if auth == nil {
		httpx.WriteErr(g, http.StatusInternalServerError, "登录服务未就绪")
		return
	}
	profile, err := auth.UpdateSelf(g.Request.Context(), request.Token(g), body)
	if !writeProfileErr(g, err, "保存失败") {
		return
	}
	name := profile.RealName
	if name == "" {
		name = profile.UserName
	}
	audit.Note(g, "", "USER", profile.UserID, name, detail)
	httpx.WriteOK(g, profile)
}

func writeProfileErr(g *gin.Context, err error, fallback string) bool {
	if err == nil {
		return true
	}
	if IsProfileInput(err) || errors.Is(err, ErrCurrentPassword) {
		httpx.WriteFail(g, http.StatusBadRequest, err)
		return false
	}
	if errors.Is(err, ErrProfileMissing) {
		httpx.WriteFail(g, http.StatusNotFound, err)
		return false
	}
	httpx.WriteErr(g, http.StatusInternalServerError, fallback)
	return false
}

// noteLogin 记下这次登录。失败时还没有会话，只能从这里带上用户和租户。
func noteLogin(g *gin.Context, userID, userName, tenantID, detail string) {
	if userID == "" {
		userID = "-"
	}
	if tenantID == "" {
		tenantID = loginTenant()
	}
	audit.SetEvent(g, &audit.Event{
		UserID: userID, UserName: userName, TenantID: tenantID,
		Action: audit.ActionLogin, ModuleCode: "login",
		TargetType: "USER", TargetID: userID, TargetName: userName,
		Detail: detail,
	})
}

// loginTenant 取本进程租户。登录服务未打开时退回配置里的租户。
func loginTenant() string {
	if auth := Current(); auth != nil {
		if id := strings.TrimSpace(auth.TenantID()); id != "" {
			return id
		}
	}
	return config.TenantID()
}

func adminFlag(tenantAdmin bool) string {
	if tenantAdmin {
		return "Y"
	}
	return "N"
}

func principalOf(g *gin.Context) request.Principal {
	sess, _ := request.Current(g.Request.Context())
	return sess.Principal
}
