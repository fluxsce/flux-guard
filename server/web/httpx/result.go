// Package httpx 写出管理端统一响应。
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Result 管理端统一响应，与前端 ApiResult 对应。
// 查询的 Data 是列表或详情。新增和修改的 Data 必须是与列表同一行的记录，前端据此改表格，不再整表查询。
// 失败时 Code 是稳定错误码，Error 是默认中文，前端按 Code 做多语言。
type Result struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Code  string `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
}

// FailTextKey 是失败文案在本次请求上的键。审计读取它作为失败说明，不读取请求体。
const FailTextKey = "fluxguard.failText"

func writeResult(c *gin.Context, status int, body Result) {
	if !body.OK && body.Error != "" {
		c.Set(FailTextKey, body.Error)
	}
	if c.Writer.Written() {
		return
	}
	c.JSON(status, body)
	if status != http.StatusOK {
		c.Abort()
	}
}

// WriteOK 写出管理端统一成功响应。
func WriteOK(c *gin.Context, data any) {
	writeResult(c, http.StatusOK, Result{OK: true, Data: data})
}

// WriteCode 写出带明确错误码的失败响应。code 稳定，msg 是默认中文。
func WriteCode(c *gin.Context, status int, code, msg string) {
	if code == "" {
		code = statusCode(status)
	}
	writeResult(c, status, Result{OK: false, Code: code, Error: msg})
}

// WriteErr 写出失败响应。错误码按 HTTP 状态归类，具体业务请用 WriteFail。
func WriteErr(c *gin.Context, status int, msg string) {
	WriteCode(c, status, statusCode(status), msg)
}

// WriteFail 写出业务错误。err 实现 Code() 时使用该错误码，否则按 HTTP 状态归类。
func WriteFail(c *gin.Context, status int, err error) {
	code := statusCode(status)
	msg := ""
	if err != nil {
		msg = err.Error()
		var coded interface{ Code() string }
		if errors.As(err, &coded) {
			if value := coded.Code(); value != "" {
				code = value
			}
		}
	}
	WriteCode(c, status, code, msg)
}

func statusCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "auth.unauthorized"
	case http.StatusForbidden:
		return "auth.forbidden"
	case http.StatusNotFound:
		return "request.not_found"
	case http.StatusConflict:
		return "request.conflict"
	case http.StatusInternalServerError:
		return "request.internal"
	default:
		return "request.invalid"
	}
}
