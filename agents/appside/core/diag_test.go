package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandLineHasArg(t *testing.T) {
	arg := "-Dcatalina.base=/opt/app"
	if !commandLineHasArg("java "+arg+" start", arg) {
		t.Fatal("bare token")
	}
	if commandLineHasArg("java -Dcatalina.base=/opt/app-other start", arg) {
		t.Fatal("longer base must not match")
	}
	if !commandLineHasArg(`java "`+arg+`"`, arg) {
		t.Fatal("quoted")
	}
	if commandLineHasArg("", arg) || commandLineHasArg("java", "") {
		t.Fatal("empty")
	}
}

func TestParseJcmdList(t *testing.T) {
	arg := "-Dcatalina.base=/data/web"
	text := "1 ignore\n4242 org.apache.catalina.startup.Bootstrap start " + arg + "\n99 java -Dcatalina.base=/data/web-bak\n"
	got := parseJcmdList(text, arg)
	if len(got) != 1 || got[0].PID != 4242 {
		t.Fatalf("%+v", got)
	}
}

func TestParseWindowsJava(t *testing.T) {
	arg := "-Dcatalina.base=E:\\opt\\web"
	text := "8\tE:\\jdk\\bin\\java.exe\tjava.exe " + arg + "\n" +
		"9\tE:\\jdk\\bin\\java.exe\tjava.exe -Dcatalina.base=E:\\opt\\web-other\n"
	got := parseWindowsJava(text, arg)
	if len(got) != 1 || got[0].PID != 8 || got[0].Exe != `E:\jdk\bin\java.exe` {
		t.Fatalf("%+v", got)
	}
}

func TestThreadDumpWritesFile(t *testing.T) {
	base := t.TempDir()
	prevFind, prevRun := findJVMs, commandRunner
	findJVMs = func(string) ([]jvmProc, error) {
		return []jvmProc{{PID: 4242, Exe: filepath.Join(base, "java")}}, nil
	}
	commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if !strings.Contains(name, "jcmd") {
			t.Fatalf("bin %s", name)
		}
		if len(args) < 2 || args[0] != "4242" || args[1] != "Thread.print" {
			t.Fatalf("args %v", args)
		}
		return []byte("Full thread dump\n\"http-nio-8080-exec-1\" #20\n"), nil
	}
	t.Cleanup(func() {
		findJVMs = prevFind
		commandRunner = prevRun
	})
	rt := Runtime{Mode: "tomcat", Base: base, Instance: "web"}
	got, err := ThreadDump(context.Background(), rt)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != 4242 || !strings.Contains(got.Path, "flux-guard-threaddump-") || !strings.HasSuffix(got.Path, ".txt") {
		t.Fatalf("%+v", got)
	}
	body, err := os.ReadFile(got.Path)
	if err != nil || !strings.Contains(string(body), "http-nio-8080") {
		t.Fatalf("file %v %s", err, body)
	}
	if !strings.Contains(got.Detail, "http-nio-8080") {
		t.Fatalf("detail %s", got.Detail)
	}
}

func TestThreadDumpQuitWhenJcmdMissing(t *testing.T) {
	base := t.TempDir()
	prevFind, prevRun, prevQuit := findJVMs, commandRunner, quitProcess
	var quitPID int
	findJVMs = func(string) ([]jvmProc, error) {
		return []jvmProc{{PID: 7}}, nil
	}
	commandRunner = func(context.Context, string, ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	}
	quitProcess = func(pid int) error {
		quitPID = pid
		return nil
	}
	t.Cleanup(func() {
		findJVMs, commandRunner, quitProcess = prevFind, prevRun, prevQuit
	})
	got, err := ThreadDump(context.Background(), Runtime{Mode: "tomcat", Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if quitPID != 7 || got.Path != "" || !strings.Contains(got.Message, "SIGQUIT") {
		t.Fatalf("pid %d %+v", quitPID, got)
	}
}

func TestThreadDumpNoQuitWhenJcmdMissing(t *testing.T) {
	prevFind, prevRun, prevQuit := findJVMs, commandRunner, quitProcess
	findJVMs = func(string) ([]jvmProc, error) {
		return []jvmProc{{PID: 7}}, nil
	}
	commandRunner = func(context.Context, string, ...string) ([]byte, error) {
		return nil, exec.ErrNotFound
	}
	quitProcess = func(int) error { return errQuitUnsupported }
	t.Cleanup(func() {
		findJVMs, commandRunner, quitProcess = prevFind, prevRun, prevQuit
	})
	_, err := ThreadDump(context.Background(), Runtime{Mode: "tomcat", Base: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "未找到 jcmd") {
		t.Fatal(err)
	}
}

func TestHeapDump(t *testing.T) {
	base := t.TempDir()
	prevFind, prevRun := findJVMs, commandRunner
	findJVMs = func(string) ([]jvmProc, error) {
		return []jvmProc{{PID: 9, Exe: `C:\jdk\bin\java.exe`}}, nil
	}
	commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if len(args) < 3 || args[1] != "GC.heap_dump" {
			t.Fatalf("%s %v", name, args)
		}
		if err := os.WriteFile(args[2], []byte("JAVA PROFILE"), 0o644); err != nil {
			return nil, err
		}
		return []byte("Heap dump file created"), nil
	}
	t.Cleanup(func() {
		findJVMs = prevFind
		commandRunner = prevRun
	})
	got, err := HeapDump(context.Background(), Runtime{Mode: "tomcat", Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if got.Bytes != 12 || !strings.HasSuffix(got.Path, ".hprof") || !strings.Contains(got.Message, "可达对象") {
		t.Fatalf("%+v", got)
	}
}

func TestHeapDumpRemovesPartial(t *testing.T) {
	base := t.TempDir()
	prevFind, prevRun := findJVMs, commandRunner
	var written string
	findJVMs = func(string) ([]jvmProc, error) {
		return []jvmProc{{PID: 9}}, nil
	}
	commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		written = args[len(args)-1]
		_ = os.WriteFile(written, []byte("partial"), 0o644)
		return []byte("attach failed"), errors.New("exit status 1")
	}
	t.Cleanup(func() {
		findJVMs = prevFind
		commandRunner = prevRun
	})
	_, err := HeapDump(context.Background(), Runtime{Mode: "tomcat", Base: base})
	if err == nil || !strings.Contains(err.Error(), "attach failed") {
		t.Fatal(err)
	}
	if written != "" {
		if _, statErr := os.Stat(written); !os.IsNotExist(statErr) {
			t.Fatalf("partial dump left %s", written)
		}
	}
}

func TestDiagRejects(t *testing.T) {
	if _, err := ThreadDump(context.Background(), Runtime{Mode: "jar", Base: t.TempDir()}); err == nil {
		t.Fatal("jar")
	}
	prev := findJVMs
	findJVMs = func(string) ([]jvmProc, error) { return nil, nil }
	t.Cleanup(func() { findJVMs = prev })
	if _, err := HeapDump(context.Background(), Runtime{Mode: "tomcat", Base: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "没有在运行") {
		t.Fatal(err)
	}
	findJVMs = func(string) ([]jvmProc, error) {
		return []jvmProc{{PID: 2}, {PID: 3}}, nil
	}
	if _, err := ThreadDump(context.Background(), Runtime{Mode: "tomcat", Base: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "多个") {
		t.Fatal(err)
	}
}
