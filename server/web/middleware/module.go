package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/middleware/dao"
)

// Module 按路径上的模块码鉴权。解析不出模块时只保留登录校验。
// 租户管理员直接放行。目录里没有该 MODULE 时放行。有模块时，拥有模块或任一模块下按钮即通过。
func Module(access *dao.Access, code func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		module := ""
		if code != nil {
			module = code(c)
		}
		if module == "" {
			c.Next()
			return
		}
		principal := Principal(c)
		if principal.UserID == "" || principal.TenantID == "" {
			httpx.WriteErr(c, http.StatusUnauthorized, "未登录")
			return
		}
		if principal.TenantAdmin {
			c.Next()
			return
		}
		if access == nil || !access.AllowModule(principal.TenantID, principal.UserID, module) {
			httpx.WriteErr(c, http.StatusForbidden, "没有权限")
			return
		}
		c.Next()
	}
}
