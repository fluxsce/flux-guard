package logger

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWritesFile(t *testing.T) {
	prev := log.Writer()
	flags := log.Flags()
	t.Cleanup(func() {
		log.SetOutput(prev)
		log.SetFlags(flags)
		SetOutput(os.Stderr)
	})
	path := filepath.Join(t.TempDir(), "logs", "server.log")
	closer, err := Setup(Config{File: path, Console: false, MaxSizeMB: 1, MaxBackups: 2, MaxAgeDays: 3})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closer.Close() })
	log.Print("hello-log")
	Info(context.Background(), "登记模块 %s %s", "fg0021", "/fluxguard/api/fg0021")
	Warn(context.Background(), "监听退出")
	Error(context.Background(), "预警计算: %s", "超时")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`"msg":"hello-log"`,
		`"level":"info"`,
		`"caller":"logger_test.go:`,
		`"msg":"登记模块 fg0021 /fluxguard/api/fg0021"`,
		`"level":"warn"`,
		`"msg":"监听退出"`,
		`"level":"error"`,
		`"msg":"预警计算: 超时"`,
		`"stack":"`,
		"fluxguard/common/logger.TestSetupWritesFile",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Count(text, "TestSetupWritesFile") != 1 {
		t.Fatalf("error stack %s", text)
	}
	assertJSONLines(t, text)
}

func TestFillKeepsGrowthBounded(t *testing.T) {
	cfg := fill(Config{})
	if cfg.Level != "info" || cfg.MaxSizeMB != 20 || cfg.MaxBackups != 5 || cfg.MaxAgeDays != 14 {
		t.Fatalf("%+v", cfg)
	}
}

func TestLevelDropsDebug(t *testing.T) {
	prev := log.Writer()
	t.Cleanup(func() {
		log.SetOutput(prev)
		SetOutput(os.Stderr)
	})
	path := filepath.Join(t.TempDir(), "level.log")
	closer, err := Setup(Config{File: path, Console: false, Level: "info"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closer.Close() })
	Debug(context.Background(), "hidden-debug")
	Info(context.Background(), "shown-info")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "hidden-debug") || !strings.Contains(text, "shown-info") {
		t.Fatalf("level filter %s", text)
	}
}

func TestWithWritesContextFields(t *testing.T) {
	prev := log.Writer()
	t.Cleanup(func() {
		log.SetOutput(prev)
		SetOutput(os.Stderr)
	})
	path := filepath.Join(t.TempDir(), "ctx.log")
	closer, err := Setup(Config{File: path, Console: false, Level: "info"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closer.Close() })
	ctx := With(context.Background(), "trace_id", "TRACE-1")
	ctx = With(ctx, "trace_id", "TRACE-2", "tenant", "default")
	Info(ctx, "带上上下文")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "TRACE-1") || !strings.Contains(text, `"trace_id":"TRACE-2"`) || !strings.Contains(text, `"tenant":"default"`) {
		t.Fatalf("context fields %s", text)
	}
	assertJSONLines(t, text)
}

func assertJSONLines(t *testing.T, text string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("not json: %s", line)
		}
		for _, key := range []string{"time", "level", "caller", "msg"} {
			if _, ok := obj[key]; !ok {
				t.Fatalf("missing %s in %s", key, line)
			}
		}
	}
}
