package core

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestAppCode(t *testing.T) {
	name, version := AppCode("order##2.4.1.war")
	if name != "order" || version != "2.4.1" {
		t.Fatalf("code %s %s", name, version)
	}
}

func TestReplaceFileKeepsNameInDir(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "datahub01webApp.war")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "upload.war")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := t.TempDir()
	report, err := ReplaceFile(dir, "datahub01webApp", src, "upload.war", backup, "war-repo")
	if err != nil {
		t.Fatal(err)
	}
	if report.TargetPath != old {
		t.Fatalf("target %s", report.TargetPath)
	}
	body, err := os.ReadFile(old)
	if err != nil || string(body) != "new" {
		t.Fatalf("body %q %v", body, err)
	}
	if report.BackupPath == "" {
		t.Fatal("no backup")
	}
}

func TestUpgradeBaseKeepsPathAndBackup(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"bin", "conf", "webapps"} {
		if err := os.Mkdir(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "RELEASE-NOTES"), []byte("Apache Tomcat Version 9.0.90\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "conf", "server.xml"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "webapps", "order.war"), []byte("war"), 0o644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "tomcat.zip")
	writeZip(t, zipPath, map[string]string{
		"RELEASE-NOTES":   "Apache Tomcat Version 9.0.98\n",
		"bin/catalina.sh": "new",
	})
	backup := t.TempDir()
	if _, err := UpgradeBase(base, zipPath, backup, "tomcat-a"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(base, "conf", "server.xml"))
	if err != nil || string(body) != "keep" {
		t.Fatalf("conf %q %v", body, err)
	}
	war, err := os.ReadFile(filepath.Join(base, "webapps", "order.war"))
	if err != nil || string(war) != "war" {
		t.Fatalf("war %q %v", war, err)
	}
	matches, err := filepath.Glob(filepath.Join(backup, "tomcat-a", "runtime-*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("backup %v %v", matches, err)
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
