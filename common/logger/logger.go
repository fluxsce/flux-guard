// Package logger 用 log/slog 写全局日志，并把输出写到文件，按大小和天数轮转。
// 每条日志一行 JSON。固定字段是 time、level、caller、msg，上下文键值写在同一对象里。
// 级别为 debug、info、warn、error。低于配置级别的记录直接丢掉。
// error 的 stack 字段带上调用链，仍保持这一行，便于容器按行采集。
package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 是 log 配置。三个进程的 YAML 都用这一组字段。
type Config struct {
	File       string `mapstructure:"file"`
	Level      string `mapstructure:"level"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Console    bool   `mapstructure:"console"`
}

// Defaults 是未写配置时的值。file 由各进程指定，例如 logs/server.log。
func Defaults(file string) Config {
	return Config{
		File:       file,
		Level:      "info",
		MaxSizeMB:  20,
		MaxBackups: 5,
		MaxAgeDays: 14,
		Console:    true,
	}
}

var (
	mu       sync.Mutex
	minLevel = slog.LevelInfo
	handler  = newJSON(&sink{w: os.Stderr}, slog.LevelInfo)
)

// Setup 开始记录日志。file 为空时只写到窗口。返回的 Closer 在进程退出前关闭日志文件。
// 标准库 log 也接到同一处，进程退出时的 log.Fatal 仍能落盘。
func Setup(cfg Config) (io.Closer, error) {
	cfg = fill(cfg)
	level := parseLevel(cfg.Level)
	var writers []io.Writer
	if cfg.Console || cfg.File == "" {
		writers = append(writers, os.Stderr)
	}
	var file *lumberjack.Logger
	if cfg.File != "" {
		dir := filepath.Dir(cfg.File)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
		file = &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   true,
			LocalTime:  true,
		}
		writers = append(writers, file)
	}
	out := &sink{w: io.MultiWriter(writers...)}
	install(out, level)
	log.SetFlags(0)
	log.SetOutput(&stdBridge{out: out})
	if file == nil {
		return nopCloser{}, nil
	}
	return file, nil
}

// SetOutput 把后续日志写到 w。测试用来截获输出，级别保持不变。
func SetOutput(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	mu.Lock()
	level := minLevel
	mu.Unlock()
	install(&sink{w: w}, level)
}

// install 换上输出目标和最低级别。编码不在这把锁里。
func install(out *sink, level slog.Level) {
	mu.Lock()
	minLevel = level
	handler = newJSON(out, level)
	mu.Unlock()
}

// current 读取当前输出。锁只包住取指针，随后的编码不再占这把锁。
func current() *jsonHandler {
	mu.Lock()
	defer mu.Unlock()
	return handler
}

// attrKey 是日志字段在 context 里的键。
type attrKey struct{}

// With 把键值放进 ctx，之后同一次操作里的日志会带上这些字段。
// 参数成对出现：键、值、键、值。键必须是字符串。空 ctx 按 Background。同名键以后写的为准。
func With(ctx context.Context, args ...any) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	next := append([]slog.Attr{}, attrsOf(ctx)...)
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok || key == "" {
			continue
		}
		next = putAttr(next, slog.Any(key, args[i+1]))
	}
	return context.WithValue(ctx, attrKey{}, next)
}

// attrsOf 取出 ctx 里已经放好的日志字段。
func attrsOf(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	attrs, _ := ctx.Value(attrKey{}).([]slog.Attr)
	return attrs
}

// putAttr 按键写入。同名键覆盖，一行里不会出现两个 trace_id。
func putAttr(attrs []slog.Attr, item slog.Attr) []slog.Attr {
	for i := range attrs {
		if attrs[i].Key == item.Key {
			attrs[i] = item
			return attrs
		}
	}
	return append(attrs, item)
}

// orBackground 把空 ctx 换成 Background，避免把 nil 传进 WithValue。
func orBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// Debug 写一条调试日志。默认级别是 info，调试日志不输出。format 与 log.Printf 相同。
// ctx 是这次操作的上下文，空则按 Background。
func Debug(ctx context.Context, format string, args ...any) {
	emit(ctx, slog.LevelDebug, format, args...)
}

// Info 写一条信息日志。format 与 log.Printf 相同。
// ctx 是这次操作的上下文，空则按 Background。
func Info(ctx context.Context, format string, args ...any) {
	emit(ctx, slog.LevelInfo, format, args...)
}

// Warn 写一条警告日志。format 与 log.Printf 相同。
// ctx 是这次操作的上下文，空则按 Background。
func Warn(ctx context.Context, format string, args ...any) {
	emit(ctx, slog.LevelWarn, format, args...)
}

// Error 写一条错误日志，并附上从调用点往上的调用链。format 与 log.Printf 相同。
// ctx 是这次操作的上下文，空则按 Background。
func Error(ctx context.Context, format string, args ...any) {
	emit(ctx, slog.LevelError, format, args...)
}

// emit 是四个级别的共同出口。低于配置级别时直接返回，不拼正文。
func emit(ctx context.Context, level slog.Level, format string, args ...any) {
	ctx = orBackground(ctx)
	h := current()
	if h == nil || !h.enabled(level) {
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	rec := slog.NewRecord(time.Now(), level, msg, pcs[0])
	if attrs := attrsOf(ctx); len(attrs) > 0 {
		rec.AddAttrs(attrs...)
	}
	if level >= slog.LevelError {
		if chain := callChain(); chain != "" {
			rec.AddAttrs(slog.String("stack", chain))
		}
	}
	_ = h.Handle(ctx, rec)
}

// callChain 从 Error 的调用方开始，向上收集调用链。日志包自身和运行时帧不写入。
func callChain() string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	var buf strings.Builder
	for {
		frame, more := frames.Next()
		file := filepath.ToSlash(frame.File)
		if frame.Function != "" && !skipFrame(file, frame.Function) {
			fmt.Fprintf(&buf, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		}
		if !more {
			break
		}
	}
	return strings.TrimRight(buf.String(), "\n")
}

// skipFrame 丢掉日志包自身和运行时的帧，调用链从业务函数开始。
func skipFrame(file, function string) bool {
	if strings.HasSuffix(function, "runtime.goexit") || strings.HasPrefix(function, "runtime.") {
		return true
	}
	return strings.Contains(file, "/runtime/") || strings.Contains(file, "/common/logger/logger.go")
}

// sink 保证一整行一次写完，slog 和标准库 log 共用这一把锁。
type sink struct {
	mu sync.Mutex
	w  io.Writer
}

// writeLine 在锁内写出一行。JSON 在进锁之前已经编好，并发时只排队写盘。
func (s *sink) writeLine(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.w.Write(append(p, '\n'))
	return err
}

// stdBridge 把标准库 log 的一行收成同一份 JSON，进程退出时的 log.Fatal 仍落在采集流里。
type stdBridge struct {
	out *sink
}

// Write 把标准库 log 的正文收成 info 级别的一行 JSON。
func (b *stdBridge) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\r\n")
	line, err := marshalLine(time.Now(), "info", "", msg, nil, "")
	if err != nil {
		return 0, err
	}
	if err := b.out.writeLine(line); err != nil {
		return 0, err
	}
	return len(p), nil
}

// jsonHandler 把一条记录排成一行 JSON。
type jsonHandler struct {
	out *sink
	min slog.Level
}

// newJSON 用给定的输出和最低级别做 JSON 行处理器。
func newJSON(out *sink, min slog.Level) *jsonHandler {
	return &jsonHandler{out: out, min: min}
}

// enabled 判断这条日志是否达到配置的最低级别。
func (h *jsonHandler) enabled(level slog.Level) bool {
	return level >= h.min
}

// Handle 编出一行 JSON 并写出。调用链留在 stack 字段里，不另起一行。
func (h *jsonHandler) Handle(_ context.Context, r slog.Record) error {
	fields := map[string]any{}
	var stack string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			stack = a.Value.String()
			return true
		}
		if reserved(a.Key) {
			return true
		}
		fields[a.Key] = a.Value.Any()
		return true
	})
	line, err := marshalLine(r.Time, strings.ToLower(r.Level.String()), callerOf(r.PC), r.Message, fields, stack)
	if err != nil {
		return err
	}
	return h.out.writeLine(line)
}

// marshalLine 编出固定字段的 JSON。上下文键值并进同一对象，不覆盖 time、level、caller、msg、stack。
func marshalLine(at time.Time, level, caller, msg string, fields map[string]any, stack string) ([]byte, error) {
	obj := map[string]any{
		"time":   at.Format("2006-01-02T15:04:05.000Z07:00"),
		"level":  level,
		"caller": caller,
		"msg":    msg,
	}
	for key, val := range fields {
		obj[key] = val
	}
	if stack != "" {
		obj["stack"] = stack
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obj); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// reserved 是采集端依赖的固定字段。上下文里的同名键丢掉，避免冲掉时间和级别。
func reserved(key string) bool {
	switch key {
	case "time", "level", "caller", "msg", "stack":
		return true
	default:
		return false
	}
}

// callerOf 把调用点收成文件名和行号。
func callerOf(pc uintptr) string {
	if pc == 0 {
		return "?:0"
	}
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if frame.File == "" {
		return "?:0"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(frame.File), frame.Line)
}

// nopCloser 在只写窗口、没有日志文件时供调用方 Close。
type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// fill 补上未写的配置。文件路径去掉首尾空白，级别空则按 info。
func fill(cfg Config) Config {
	cfg.File = strings.TrimSpace(cfg.File)
	cfg.Level = strings.TrimSpace(cfg.Level)
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 20
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 5
	}
	if cfg.MaxAgeDays <= 0 {
		cfg.MaxAgeDays = 14
	}
	return cfg
}

// parseLevel 把配置里的级别名换成 slog 级别。无法识别时按 info。
func parseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
