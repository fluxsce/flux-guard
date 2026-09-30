// Package middleware 放管理端 HTTP 中间件。路由装配只负责挂上，不在引擎里写实现。
package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fluxguard/common/id"
	"fluxguard/common/logger"
)

const (
	// TraceIDKey 是跟踪号在 Gin 上下文里的键。
	TraceIDKey = "trace_id"
	// TraceIDHeader 是响应里带回的跟踪号。
	TraceIDHeader = "X-Trace-ID"
	// RequestIDHeader 与跟踪号写同一个值，方便调用方按请求号对日志。
	RequestIDHeader = "X-Request-ID"
)

type traceCtxKey struct{}

// Visit 是一次访问的起点。Finish 用它写完成日志。
type Visit struct {
	Start time.Time
	Trace string
}

// LoggerMiddleware 记录请求开始和完成，并在响应头带回跟踪号。
// 挂在崩溃恢复外层，处理函数 panic 时完成日志仍能记下 500。
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		visit := Begin(c.Writer, c.Request)
		c.Set(TraceIDKey, visit.Trace)
		ctx := logger.With(c.Request.Context(), "trace_id", visit.Trace)
		ctx = context.WithValue(ctx, traceCtxKey{}, visit.Trace)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}
		Finish(c.Request, c.Writer.Status(), size, visit)
	}
}

// Begin 写访问开始日志，并在响应头放上跟踪号。只记路径，不记查询串和请求头。
// method、path、client_ip、trace_id 写成独立字段，msg 只留事件名，采集时按字段取值。
func Begin(w http.ResponseWriter, r *http.Request) Visit {
	trace := traceOf(r)
	if w != nil {
		w.Header().Set(TraceIDHeader, trace)
		w.Header().Set(RequestIDHeader, trace)
	}
	method, path, ip := requestFields(r)
	logger.Info(accessFields(r, trace, method, path, ip), "请求开始")
	return Visit{Start: time.Now(), Trace: trace}
}

// Finish 写访问完成日志。状态、耗时、响应字节数与开始日志一样按字段写出。
func Finish(r *http.Request, status, size int, visit Visit) {
	if status == 0 {
		status = http.StatusOK
	}
	if size < 0 {
		size = 0
	}
	method, path, ip := requestFields(r)
	ctx := logger.With(accessFields(r, visit.Trace, method, path, ip),
		"status", status,
		"duration_ms", elapsedMS(time.Since(visit.Start)),
		"response_size", size,
	)
	logger.Info(ctx, "请求完成")
}

// accessFields 把一次访问的固定键放进上下文。同名键以后写的为准，一行里不会重复。
func accessFields(r *http.Request, trace, method, path, ip string) context.Context {
	return logger.With(reqContext(r),
		"trace_id", trace,
		"method", method,
		"path", path,
		"client_ip", ip,
	)
}

func reqContext(r *http.Request) context.Context {
	if r == nil || r.Context() == nil {
		return context.Background()
	}
	return r.Context()
}

func requestFields(r *http.Request) (method, path, ip string) {
	if r == nil {
		return "-", "-", "-"
	}
	method = r.Method
	if method == "" {
		method = "-"
	}
	if r.URL != nil {
		path = r.URL.Path
	}
	if path == "" {
		path = "-"
	}
	return method, path, clientIP(r)
}

func traceOf(r *http.Request) string {
	if r != nil {
		if v := cleanTrace(r.Header.Get(TraceIDHeader)); v != "" {
			return v
		}
		if v := cleanTrace(r.Header.Get(RequestIDHeader)); v != "" {
			return v
		}
	}
	return "TRACE-" + id.New()
}

func cleanTrace(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > 128 || strings.ContainsAny(v, "\r\n\t ") {
		return ""
	}
	return v
}

func elapsedMS(d time.Duration) int64 {
	ms := d.Milliseconds()
	if d > 0 && ms == 0 {
		return 1
	}
	return ms
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		if r.RemoteAddr == "" {
			return "-"
		}
		return r.RemoteAddr
	}
	return host
}
