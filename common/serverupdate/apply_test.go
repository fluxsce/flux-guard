package serverupdate

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStageKeepsConfigDataAndScriptsBackup(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "web", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "server.yaml"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "data", "keep.txt"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "dist", "index.html"), []byte("old-page"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "run", "start.sh"), []byte("old-start"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := binBase
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := os.WriteFile(filepath.Join(root, bin), []byte("old-bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "pack.tar.gz")
	name := "flux-guard-99.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
	writePack(t, archive, map[string]string{
		"flux-guard/" + bin:                      "new-bin",
		"flux-guard/web/dist/index.html":         "new-page",
		"flux-guard/scripts/run/start.sh":        "new-start",
		"flux-guard/configs/server.yaml":         "packed",
		"flux-guard/configs/server.yaml.example": "example",
		"flux-guard/data/keep.txt":               "packed-db",
		"flux-guard/logs/server.log":             "packed-log",
	})
	sum := fileSHA(t, archive)
	version, err := stage(root, archive, name, sum)
	if err != nil {
		t.Fatal(err)
	}
	if version != "99.0.0" {
		t.Fatalf("version %s", version)
	}
	assertFile(t, filepath.Join(root, "configs", "server.yaml"), "local")
	assertFile(t, filepath.Join(root, "configs", "server.yaml.example"), "example")
	assertFile(t, filepath.Join(root, "data", "keep.txt"), "db")
	if _, err := os.Stat(filepath.Join(root, "logs", "server.log")); !os.IsNotExist(err) {
		t.Fatal("logs from the pack must not be written")
	}
	assertFile(t, filepath.Join(root, "web", "dist", "index.html"), "new-page")
	assertFile(t, filepath.Join(root, "scripts", "run", "start.sh"), "new-start")
	assertFile(t, filepath.Join(root, bin), "old-bin")
	assertFile(t, filepath.Join(root, bin+".new"), "new-bin")
	assertFile(t, filepath.Join(root, "data", backupDir, "web-dist", "index.html"), "old-page")
	assertFile(t, filepath.Join(root, "data", backupDir, "scripts", "run", "start.sh"), "old-start")
	if _, err := os.Stat(lockPath(root)); err != nil {
		t.Fatal(err)
	}
}

func TestStageRejectsSameVersionAndWrongDigest(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(t.TempDir(), "pack.tar.gz")
	writePack(t, archive, map[string]string{
		"flux-guard/flux-guard-server":   "bin",
		"flux-guard/web/dist/index.html": "page",
	})
	if _, err := stage(root, archive, "flux-guard-0.0.1-linux-amd64.tar.gz", fileSHA(t, archive)); err == nil {
		t.Fatal("older or equal version must fail")
	}
	name := "flux-guard-99.0.0-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"
	if _, err := stage(root, archive, name, strings.Repeat("ab", 32)); err == nil {
		t.Fatal("wrong sha256 must fail")
	}
	if _, err := stage(root, archive, "notes.txt", fileSHA(t, archive)); err == nil {
		t.Fatal("bad file name must fail")
	}
}

func TestHelperScriptsCoverBootAndScriptStart(t *testing.T) {
	unix := unixScript("/opt/flux-guard", "/opt/flux-guard/flux-guard-server", "/opt/flux-guard/flux-guard-server.new")
	for _, part := range []string{"systemctl start", "systemctl stop", "launchctl bootout", "scripts/run/start.sh"} {
		if !strings.Contains(unix, part) {
			t.Fatalf("unix helper missing %s", part)
		}
	}
	windows := windowsScript(`C:\flux-guard`, `C:\flux-guard\flux-guard-server.exe`, `C:\flux-guard\flux-guard-server.exe.new`)
	for _, part := range []string{"schtasks /Query", "schtasks /End", "schtasks /Run", "start.ps1", "taskkill /F /IM flux-guard-server.exe"} {
		if !strings.Contains(windows, part) {
			t.Fatalf("windows helper missing %s", part)
		}
	}
}

func TestVersionLess(t *testing.T) {
	if !versionLess("1.2.9", "1.2.10") || !versionLess("dev", "1.0.0") {
		t.Fatal("expected newer")
	}
	if versionLess("1.3.0", "1.2.9") || versionLess("1.2.0", "1.2.0") {
		t.Fatal("downgrade or same version")
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("%s = %q", path, body)
	}
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func writePack(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
