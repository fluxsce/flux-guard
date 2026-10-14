// Package request 读取请求：绑定结构体、取单个参数、组装分页，并携带全局登录会话。
package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/httpx"
)

// errEmptyBody 表示写请求没有正文。
var errEmptyBody = errors.New("empty body")

// Bind 把 JSON 或表单、查询参数、路径参数、请求头绑进同一个结构体，再按 binding 标签校验。
// json 标签读 JSON 正文，form 标签读表单和查询，uri 标签读路径，header 标签读请求头。
func Bind(c *gin.Context, dst any) bool {
	if err := bindAll(c, dst, false); err != nil {
		writeBindErr(c, err)
		return false
	}
	return true
}

// BindKnown 与 Bind 相同，但 JSON 正文拒绝未声明字段。
// 未知字段使用调用方给出的 code 和 msg，例如本人资料只允许改自己的字段。
func BindKnown(c *gin.Context, dst any, code, msg string) bool {
	if err := bindAll(c, dst, true); err != nil {
		if code != "" && strings.Contains(err.Error(), "unknown field") {
			httpx.WriteCode(c, http.StatusBadRequest, code, msg)
			return false
		}
		writeBindErr(c, err)
		return false
	}
	return true
}

func bindAll(c *gin.Context, dst any, strict bool) error {
	switch bodyKind(c) {
	case "file":
		if err := bindForm(c, dst); err != nil {
			return err
		}
	case "form":
		if err := bindURLEncoded(c, dst); err != nil {
			return err
		}
	case "json":
		if err := bindJSON(c, dst, strict); err != nil {
			return err
		}
	}
	if q := c.Request.URL.Query(); len(q) > 0 {
		if err := binding.MapFormWithTag(dst, q, "form"); err != nil {
			return err
		}
	}
	if len(c.Params) > 0 {
		uri := make(map[string][]string, len(c.Params))
		for _, p := range c.Params {
			uri[p.Key] = []string{p.Value}
		}
		if err := binding.MapFormWithTag(dst, uri, "uri"); err != nil {
			return err
		}
	}
	if len(c.Request.Header) > 0 {
		if err := binding.MapFormWithTag(dst, c.Request.Header, "header"); err != nil {
			return err
		}
	}
	if binding.Validator == nil {
		return nil
	}
	return binding.Validator.ValidateStruct(dst)
}

func bodyKind(c *gin.Context) string {
	switch c.ContentType() {
	case "multipart/form-data":
		return "file"
	case "application/x-www-form-urlencoded":
		return "form"
	case "", "application/json":
		return "json"
	default:
		return "stream"
	}
}

func bindJSON(c *gin.Context, dst any, strict bool) error {
	if !hasBody(c) {
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			return errEmptyBody
		}
		return nil
	}
	body, err := rawBody(c)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			return errEmptyBody
		}
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		return err
	}
	return nil
}

func bindURLEncoded(c *gin.Context, dst any) error {
	if c.Request.ContentLength > maxJSONBytes {
		return &http.MaxBytesError{Limit: maxJSONBytes}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBytes)
	if err := c.Request.ParseForm(); err != nil {
		return err
	}
	return bindFormValues(c, dst)
}

func bindForm(c *gin.Context, dst any) error {
	err := c.Request.ParseMultipartForm(limits.MultipartMemory)
	if err != nil && !errors.Is(err, http.ErrNotMultipart) {
		if perr := c.Request.ParseForm(); perr != nil {
			return err
		}
	}
	return bindFormValues(c, dst)
}

func bindFormValues(c *gin.Context, dst any) error {
	values := c.Request.PostForm
	if len(values) == 0 {
		values = c.Request.Form
	}
	if len(values) == 0 {
		return nil
	}
	return binding.MapFormWithTag(dst, values, "form")
}

func hasBody(c *gin.Context) bool {
	if c.Request.Body == nil {
		return false
	}
	if c.Request.ContentLength > 0 {
		return true
	}
	return c.Request.ContentLength < 0 && (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch)
}

func writeBindErr(c *gin.Context, err error) {
	code := "request.body.invalid"
	msg := "请求体无法解析"
	var tooLarge *http.MaxBytesError
	var invalid validator.ValidationErrors
	switch {
	case errors.Is(err, errEmptyBody), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
	case errors.As(err, &tooLarge):
		code = "request.body.too_large"
		msg = "请求体过大"
	case errors.As(err, &invalid):
		code = "request.field.invalid"
		msg = fieldMessage(invalid[0])
	default:
		if strings.Contains(err.Error(), "unknown field") {
			msg = "请求体无法解析"
		}
	}
	httpx.WriteCode(c, http.StatusBadRequest, code, msg)
}

func fieldMessage(fe validator.FieldError) string {
	if fe.Tag() == "required" {
		return "缺少必填字段 " + fe.Field()
	}
	return "字段 " + fe.Field() + " 校验失败"
}
