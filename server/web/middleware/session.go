package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/request"
	loginctrl "fluxguard/server/web/views/login/controllers"
)

const principalKey = "fluxguard.principal"

// Session 在管理接口入口校验登录，并把操作者写入这次请求。
// 登录票以 Cookie 为准，缓存里必须有同一张票。通过后续上 Cookie，使浏览器有效期和缓存一致。
// 缓存已不认这张票时清掉 Cookie。skip 为 true 的请求直接放行。
func Session(skip func(*gin.Context) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if skip != nil && skip(c) {
			c.Next()
			return
		}
		auth := loginctrl.Current()
		token := request.Token(c)
		if auth == nil || token == "" {
			if request.HasSessionCookie(c) {
				request.ClearSessionCookie(c)
			}
			httpx.WriteErr(c, http.StatusUnauthorized, "未登录")
			return
		}
		sess, ok := auth.Principal(token)
		if !ok {
			if request.HasSessionCookie(c) {
				request.ClearSessionCookie(c)
			}
			httpx.WriteErr(c, http.StatusUnauthorized, "未登录")
			return
		}
		request.WriteSessionCookie(c, token, sess.Until)
		c.Set(principalKey, sess.Principal)
		c.Request = c.Request.WithContext(request.Attach(c.Request.Context(), sess))
		c.Next()
	}
}

// Principal 读取这次请求上的登录用户。来源是 Session 写进 Request.Context 的会话。
func Principal(c *gin.Context) request.Principal {
	if sess, ok := request.Current(c.Request.Context()); ok {
		return sess.Principal
	}
	principal, _ := c.Get(principalKey)
	value, _ := principal.(request.Principal)
	return value
}
