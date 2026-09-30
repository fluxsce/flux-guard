package routes

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/middleware/dao"
	dbx "github.com/fluxsce/dbx/db"
)

// modulePattern 从管理接口路径解析模块码，例如 /fluxguard/api/fg0024/querySettings。
// auth 等非模块路径不匹配，只保留登录校验。
var modulePattern = regexp.MustCompile("^" + regexp.QuoteMeta(APIRoot) + `/(fg[0-9a-z]+)`)

// ApplyGlobalMiddleware 挂上管理端全局中间件。
// 先记访问，再挂审计，然后做崩溃恢复，最后限制请求体。
// 访问日志和审计都在崩溃恢复外层：处理函数 panic 被恢复成 500 后，完成日志和审计仍能看到这次失败。
func ApplyGlobalMiddleware(router *gin.Engine) {
	router.Use(middleware.LoggerMiddleware())
	router.Use(audit.Handler())
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		httpx.WriteErr(c, http.StatusInternalServerError, "内部错误")
	}))
	router.Use(middleware.LimitBody())
}

// ApplyAccess 在管理接口入口先校验登录，再按路径校验模块。
// 按钮由各路由上的 RequireButton 单独校验。必须在登记这些路由之前调用。
func ApplyAccess(router *gin.Engine, db *dbx.DB) {
	audit.Use(db)
	middleware.UseAccess(dao.Open(db))
	router.Use(middleware.Session(skipSession))
	router.Use(middleware.Module(middleware.Access(), moduleCode))
}

func moduleCode(c *gin.Context) string {
	matches := modulePattern.FindStringSubmatch(c.Request.URL.Path)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func skipSession(c *gin.Context) bool {
	path := c.Request.URL.Path
	if !strings.HasPrefix(path, APIRoot+"/") {
		return true
	}
	return c.Request.Method == http.MethodPost && (path == APIRoot+"/auth/login" || path == APIRoot+"/auth/captcha")
}

// PublicAPI 标记不需要登录的接口，与 gateway 的公开路由相同。
// 登录和验证码还要写在 skipSession 里，因为入口会话校验早于这条路由中间件。
func PublicAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// PermissionRequired 校验已登录，再按路径校验模块。
// 路径解析不出模块码时只要求登录，与 gateway /user 下的受保护接口相同。
func PermissionRequired() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.Session(nil),
		middleware.Module(middleware.Access(), moduleCode),
	}
}
