package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware/audit"
	"fluxguard/server/web/middleware/dao"
)

var access *dao.Access

// UseAccess 登记本进程的授权查询。入口模块校验和按钮校验共用这一份。
func UseAccess(next *dao.Access) {
	access = next
}

// Access 返回本进程的授权查询。尚未登记时为 nil。
func Access() *dao.Access {
	return access
}

// RequireButton 按路由上写死的按钮码鉴权。登录和模块已在入口完成。
// 操作者只取会话上下文。tenantAdminFlag 为 Y 的租户管理员直接放行，其余按角色资源校验。
func RequireButton(button string, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		audit.MarkResource(c, button)
		principal := Principal(c)
		if principal.UserID == "" || principal.TenantID == "" {
			httpx.WriteErr(c, http.StatusUnauthorized, "未登录")
			return
		}
		if principal.TenantAdmin {
			next(c)
			return
		}
		if access == nil || !access.AllowButton(principal.TenantID, principal.UserID, button) {
			httpx.WriteErr(c, http.StatusForbidden, "没有权限")
			return
		}
		next(c)
	}
}
