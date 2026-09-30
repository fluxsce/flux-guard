package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fluxguard/server/limits"
)

// MaxBodyBytes 转发 limits，供测试与外部引用。数值与分层说明见 server/limits。
const MaxBodyBytes = limits.MaxBodyBytes

// LimitBody 限制请求体字节数。超出后读取请求体会得到 http.MaxBytesError。
func LimitBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limits.MaxBodyBytes)
		}
		c.Next()
	}
}
