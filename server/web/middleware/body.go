package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/protocol"
	"fluxguard/server/utils/const/limits"
)

// MaxBodyBytes 转发 limits，供测试与外部引用。数值与分层说明见 server/utils/const/limits。
const MaxBodyBytes = limits.MaxBodyBytes

// LimitBody 限制请求体字节数。超出后读取请求体会得到 http.MaxBytesError。
// 代理出站推送制品走 AppFileMaxBytes，其余管理端请求仍用 MaxBodyBytes。
func LimitBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request != nil && c.Request.Body != nil {
			limit := limits.MaxBodyBytes
			if isAppFilePush(c.Request.URL.Path) {
				limit = limits.AppFileMaxBytes
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func isAppFilePush(path string) bool {
	path = strings.TrimSpace(path)
	return path == protocol.FilePushPath
}
