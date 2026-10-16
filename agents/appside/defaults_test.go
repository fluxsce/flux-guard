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
	if !rt.StatusAuto || !rt.KillAuto || rt.Stop != "" {
		t.Fatalf("tomcat defaults should kill when stop is unset, %+v", rt)
	}
	if !strings.Contains(rt.Status, "-Dcatalina.base=") || !strings.Contains(rt.Kill, "-Dcatalina.base=") {
		t.Fatalf("%+v", rt)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(rt.Start, "startup.bat") {
			t.Fatalf("%+v", rt)
		}
		return
	}
	if !strings.HasSuffix(rt.Start, "startup.sh") || rt.Stop != "" {
		t.Fatalf("%+v", rt)
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

func TestFillTomcatStartUsesUsualScript(t *testing.T) {
	base := t.TempDir()
	bin := filepath.Join(base, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	if err := os.WriteFile(filepath.Join(bin, "startup"+ext), []byte("ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "start"+ext), []byte("ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	rt := Runtime{Kind: "command", Mode: "tomcat", Base: base, Instance: "web"}
	if err := fillCommandDefaults(&rt); err != nil {
		t.Fatal(err)
	}
	if rt.Stop != "" || !rt.KillAuto || !rt.StatusAuto {
		t.Fatalf("%+v", rt)
	}
	if !strings.Contains(rt.Start, "startup"+ext) {
		t.Fatalf("start %s", rt.Start)
	}
}

func TestFillTomcatStartFallsBackToStartScript(t *testing.T) {
	base := t.TempDir()
	bin := filepath.Join(base, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".bat"
	}
	if err := os.WriteFile(filepath.Join(bin, "start"+ext), []byte("ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	rt := Runtime{Kind: "command", Mode: "tomcat", Base: base, Instance: "web"}
	if err := fillCommandDefaults(&rt); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rt.Start, "startup"+ext) || !strings.Contains(rt.Start, "start"+ext) {
		t.Fatalf("start %s", rt.Start)
	}
	if rt.Stop != "" || !rt.KillAuto {
		t.Fatalf("%+v", rt)
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
