package tomcat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fluxguard/agents/appside/core"
)

func TestListRuntimeLogs(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs")
	if err := os.Mkdir(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalina.out", "localhost.log", "readme.md"} {
		if err := os.WriteFile(filepath.Join(logs, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ListRuntimeLogs(logs)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]core.Artifact{}
	for _, item := range got {
		byName[item.AppName] = item
	}
	if byName["logs/catalina"].Kind != "log" || byName["logs/localhost"].Kind != "log" {
		t.Fatalf("%+v", got)
	}
	if _, ok := byName["logs/readme"]; ok {
		t.Fatal("md should not be listed")
	}
}

func TestJudgeTomcatStartup(t *testing.T) {
	okText := "INFO Server startup in [1234] milliseconds"
	v, ok := judgeTomcatStartup(okText, true)
	if !ok || !strings.Contains(v, "启动成功") {
		t.Fatalf("%s %v", v, ok)
	}
	bad := "SEVERE: Failed to initialize connector\nLifecycleException: service"
	v, ok = judgeTomcatStartup(bad, false)
	if ok || !strings.Contains(v, "启动失败") {
		t.Fatalf("%s %v", v, ok)
	}
	v, ok = judgeTomcatStartup("", true)
	if !ok || !strings.Contains(v, "尚无新日志") {
		t.Fatalf("empty %s %v", v, ok)
	}
}

func TestPickTomcatLogPrefersCatalinaOut(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "catalina.2026-10-15.log"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalina.out"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := pickTomcatLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "catalina.out" {
		t.Fatalf("got %s", path)
	}
}

func TestReadLogAfterOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.out")
	old := "SEVERE: old failure\n"
	neu := "INFO Server startup in [1] milliseconds\n"
	if err := os.WriteFile(path, []byte(old+neu), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readLogAfter(path, int64(len(old)), 2400)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "SEVERE") {
		t.Fatalf("should ignore pre-offset history: %q", got)
	}
	if !strings.Contains(got, "Server startup") {
		t.Fatalf("%q", got)
	}
}
