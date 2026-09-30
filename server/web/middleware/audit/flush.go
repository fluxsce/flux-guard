package audit

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/web/httpx"
	"fluxguard/server/web/request"
)

// Handler 在处理函数返回后，用已登记的库连接同步写入审计。
// 挂在崩溃恢复外层，panic 被写成 500 之后仍会落库。
func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		Flush(c)
	}
}

// Flush 把当前请求的审计写入账本。查询、只读和没有操作者的请求跳过。同一请求只写一次。
// 只补身份、路径、结果和失败原因；操作说明与业务对象由业务模块 Note 写入，本处不拼装文案。
func Flush(c *gin.Context) {
	if opened == nil || c == nil || alreadyFlushed(c) {
		return
	}
	path, method := requestMeta(c)
	resource := resourceFrom(c)
	event := eventFrom(c)
	if event == nil {
		action := ActionOf(path, resource)
		if action == "" || !managementPath(path) || !writeMethod(method) {
			return
		}
		event = &Event{Action: action}
	}
	fill(c, event, path, method, resource)
	if strings.TrimSpace(event.UserID) == "" || strings.TrimSpace(event.TenantID) == "" || event.Action == "" {
		return
	}
	c.Set(flushedKey, true)
	write(context.Background(), event)
}

func writeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func fill(c *gin.Context, event *Event, path, method, resource string) {
	if sess, ok := request.Current(c.Request.Context()); ok {
		if event.UserID == "" {
			event.UserID = sess.UserID
		}
		if event.TenantID == "" {
			event.TenantID = sess.TenantID
		}
		if event.UserName == "" {
			event.UserName = sess.UserName
		}
	}
	if event.RequestPath == "" {
		event.RequestPath = path
	}
	if event.RequestMethod == "" {
		event.RequestMethod = method
	}
	if event.ClientIP == "" {
		event.ClientIP = clientIP(c)
	}
	if event.ResourceCode == "" {
		event.ResourceCode = resource
	}
	if event.Action == "" {
		event.Action = ActionOf(path, event.ResourceCode)
	}
	if event.ModuleCode == "" {
		event.ModuleCode = moduleOf(path, event.ResourceCode)
	}
	if event.TargetType == "" {
		event.TargetType = targetTypeOf(path)
	}
	failed, failText := requestFailed(c)
	if failed {
		event.Result = ResultFail
		if event.Action == ActionLogin {
			event.Action = ActionLoginFail
		}
		event.FailReason = failText
		return
	}
	if event.Result == "" {
		event.Result = ResultOK
	}
	event.FailReason = ""
}

func requestMeta(c *gin.Context) (path, method string) {
	if c == nil || c.Request == nil {
		return "", ""
	}
	method = c.Request.Method
	if c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	return path, method
}

func requestFailed(c *gin.Context) (bool, string) {
	if c == nil || c.Writer == nil {
		return false, ""
	}
	status := c.Writer.Status()
	if status < http.StatusBadRequest {
		return false, ""
	}
	detail, _ := c.Get(httpx.FailTextKey)
	text, _ := detail.(string)
	if text == "" {
		text = http.StatusText(status)
	}
	return true, text
}

func clientIP(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil || host == "" {
		return c.Request.RemoteAddr
	}
	return host
}
