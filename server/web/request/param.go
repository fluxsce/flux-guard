package request

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"fluxguard/server/utils/const/limits"
)

const (
	jsonParamKey = "request.jsonParams"
	rawBodyKey   = "request.rawBody"
	// maxJSONBytes 见 limits.MaxJSONBytes：JSON/表单进内存；大包走流或 multipart。
	maxJSONBytes = limits.MaxJSONBytes
)

type rawBodyState struct {
	data []byte
	err  error
}

// rawBody 只缓存 JSON 正文，供 Param 和 Bind 重复读取。超过上限直接拒绝，不把剩余字节读进来。
func rawBody(c *gin.Context) ([]byte, error) {
	if cached, ok := c.Get(rawBodyKey); ok {
		state, _ := cached.(rawBodyState)
		return state.data, state.err
	}
	state := rawBodyState{}
	if c.Request != nil && c.Request.Body != nil {
		if c.Request.ContentLength > maxJSONBytes {
			state.err = &http.MaxBytesError{Limit: maxJSONBytes}
		} else {
			data, err := io.ReadAll(io.LimitReader(c.Request.Body, maxJSONBytes+1))
			switch {
			case err != nil:
				state.err = err
			case int64(len(data)) > maxJSONBytes:
				state.err = &http.MaxBytesError{Limit: maxJSONBytes}
			default:
				state.data = data
				c.Request.Body = io.NopCloser(bytes.NewReader(data))
			}
		}
	}
	c.Set(rawBodyKey, state)
	return state.data, state.err
}

// Param 按 gateway 的顺序取一个参数：查询串、路径、再按 Content-Type 读正文。
// 只读查询串时，参数放在 JSON 或表单里就会拿不到。defaultValue 只在各处都没有时使用。
func Param(c *gin.Context, key string, defaultValue ...string) string {
	if value := c.Query(key); value != "" {
		return value
	}
	if value := c.Param(key); value != "" {
		return value
	}
	if value := paramFromBody(c, key); value != "" {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func paramFromBody(c *gin.Context, key string) string {
	ct := c.ContentType()
	switch {
	case strings.Contains(ct, "application/json"):
		return paramFromJSON(c, key)
	case strings.Contains(ct, "application/x-www-form-urlencoded"), strings.Contains(ct, "multipart/form-data"):
		return c.PostForm(key)
	default:
		return ""
	}
}

func paramFromJSON(c *gin.Context, key string) string {
	if cached, ok := c.Get(jsonParamKey); ok {
		data, _ := cached.(map[string]any)
		return jsonField(data, key)
	}
	body, err := rawBody(c)
	if err != nil || len(body) == 0 {
		c.Set(jsonParamKey, map[string]any{})
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		c.Set(jsonParamKey, map[string]any{})
		return ""
	}
	c.Set(jsonParamKey, data)
	return jsonField(data, key)
}

func jsonField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}
