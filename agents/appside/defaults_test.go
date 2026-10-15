package appside

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeCommandClearsService(t *testing.T) {
	rt, err := normalizeRuntime(Runtime{
		Kind: "command", Mode: "tomcat", Instance: "a", Base: "/opt/t",
		Service: "tomcat-a", // 误写，应被丢掉
	})
	if err != nil {
		t.Fatal(err)
	}
	if rt.Service != "" {
		t.Fatalf("service should be cleared, got %q", rt.Service)
	}
}

func TestFillTomcatCommandDefaults(t *testing.T) {
	rt := Runtime{Kind: "command", Mode: "tomcat", Base: "/opt/tomcat-a", Instance: "tomcat-a"}
	if err := fillCommandDefaults(&rt); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(rt.Start, "startup.bat") || !strings.Contains(rt.Status, "catalina.base=") {
			t.Fatalf("%+v", rt)
		}
		return
	}
	if !strings.HasSuffix(rt.Start, "startup.sh") || !strings.HasSuffix(rt.Stop, "shutdown.sh") {
		t.Fatalf("%+v", rt)
	}
	if !strings.Contains(rt.Status, "pgrep") || !strings.Contains(rt.Status, "catalina.base=") {
		t.Fatalf("status %s", rt.Status)
	}
	if !strings.Contains(rt.Kill, "pkill") || !strings.Contains(rt.Kill, "catalina.base=") {
		t.Fatalf("kill %s", rt.Kill)
	}
}

func TestFillNodeCommandDefaultsFromScripts(t *testing.T) {
	base := t.TempDir()
	bin := filepath.Join(base, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	for _, name := range []string{"start", "stop", "status"} {
		if err := os.WriteFile(filepath.Join(bin, name+ext), []byte("ok"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rt := Runtime{Kind: "command", Mode: "node", Base: base, Instance: "order-api"}
	if err := fillCommandDefaults(&rt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rt.Start, "start"+ext) || !strings.Contains(rt.Stop, "stop"+ext) {
		t.Fatalf("%+v", rt)
	}
}

func TestFillNodeCommandDefaultsFallback(t *testing.T) {
	rt := Runtime{Kind: "command", Mode: "node", Base: "/opt/order-api", Instance: "order-api"}
	if err := fillCommandDefaults(&rt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rt.Start, filepath.Join("bin", "start")) {
		t.Fatalf("start %s", rt.Start)
	}
}

func TestFillScriptAndJarCommandDefaults(t *testing.T) {
	base := t.TempDir()
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	for _, name := range []string{"start", "stop", "status"} {
		if err := os.WriteFile(filepath.Join(base, name+ext), []byte("ok"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := Runtime{Kind: "command", Mode: "script", Base: base, Instance: "batch-job"}
	if err := fillCommandDefaults(&script); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script.Start, "start"+ext) {
		t.Fatalf("%+v", script)
	}
	jarPath := filepath.Join(base, "app.jar")
	if err := os.WriteFile(jarPath, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	jar := Runtime{Kind: "command", Mode: "jar", Jar: jarPath, Instance: "app-jar"}
	if err := fillCommandDefaults(&jar); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jar.Start, "start"+ext) {
		t.Fatalf("%+v", jar)
	}
}

func TestNormalizeKeepsManualCommands(t *testing.T) {
	rt, err := normalizeRuntime(Runtime{
		Kind: "command", Mode: "tomcat", Instance: "a", Base: "/opt/t",
		Start: "custom-start", Stop: "custom-stop", Status: "custom-status",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rt.Start != "custom-start" || rt.Stop != "custom-stop" || rt.Status != "custom-status" {
		t.Fatalf("%+v", rt)
	}
}
