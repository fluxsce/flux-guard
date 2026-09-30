package protocol

import "strings"

const (
	// IngestMethod 与 IngestPath 是代理上报的唯一接口。
	IngestMethod = "POST"
	IngestPath   = "/api/ingest/v1/batches"

	// BearerPrefix 是 Authorization 头的前缀，后面是代理令牌。
	BearerPrefix = "Bearer "
)

// IngestPattern 是上报方法加路径。管理端用 Gin 时直接挂 IngestPath。
func IngestPattern() string {
	return IngestMethod + " " + IngestPath
}

// AuthorizationValue 组成上报请求的 Authorization 头。
func AuthorizationValue(token string) string {
	return BearerPrefix + token
}

// BearerToken 从 Authorization 头取出代理令牌。格式不对时返回空串。
func BearerToken(header string) string {
	if !strings.HasPrefix(header, BearerPrefix) {
		return ""
	}
	return strings.TrimSpace(header[len(BearerPrefix):])
}

// RetryableStatus 表示网络失败或 5xx 时用同一个 batchId 再送一次。
// 4xx 是请求本身被拒绝，不重试。
func RetryableStatus(code int) bool {
	return code >= 500
}
