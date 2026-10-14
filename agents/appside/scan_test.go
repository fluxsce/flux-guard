package appside

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppCodeAndWebapps(t *testing.T) {
	name, version := AppCode("order##2.4.1.war")
	if name != "order" || version != "2.4.1" {
		t.Fatalf("code %s %s", name, version)
	}
	dir := t.TempDir()
	webapps := filepath.Join(dir, "webapps")
	if err := os.Mkdir(webapps, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"order##2.4.1.war", "manager.war", "pay.war"} {
		if err := os.WriteFile(filepath.Join(webapps, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(webapps, "order"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := ListWebapps(webapps)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].AppName == "manager" {
		t.Fatalf("%+v", found)
	}
}

func TestListFilesScansWarJarZip(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"datahub01webApp.war", "order##1.2.jar", "pack.zip", "readme.txt", "manager.war"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "child.war"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "app.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Artifact{}
	for _, item := range got {
		byName[item.AppName] = item
	}
	if byName["datahub01webApp"].Kind != "war" || byName["order"].Kind != "jar" || byName["order"].Version != "1.2" {
		t.Fatalf("%+v", got)
	}
	if byName["pack"].Kind != "static" {
		t.Fatalf("zip %+v", got)
	}
	if byName["nested"].Kind != "dir" {
		t.Fatalf("dir %+v", got)
	}
	if byName["nested/child"].Kind != "war" {
		t.Fatalf("nested war %+v", got)
	}
	if byName["nested/app"].Kind != "log" {
		t.Fatalf("log %+v", got)
	}
	if _, ok := byName["manager"]; ok {
		t.Fatal("builtin war listed")
	}
	if byName["readme"].Kind != "log" {
		t.Fatalf("readme should be log %+v", byName["readme"])
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

func TestReplaceWarKeepsBackup(t *testing.T) {
	base := t.TempDir()
	webapps := filepath.Join(base, "webapps")
	if err := os.Mkdir(webapps, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(webapps, "order.war")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(webapps, "order"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "new.war")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := t.TempDir()
	if _, err := ReplaceWar(base, "order", src, "", "tomcat-a"); err == nil {
		t.Fatal("empty backup dir")
	}
	if body, err := os.ReadFile(old); err != nil || string(body) != "old" {
		t.Fatal("old war changed")
	}
	if _, err := ReplaceWar(base, "order", src, backup, "tomcat-a"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(old)
	if err != nil || string(body) != "new" {
		t.Fatalf("war %q %v", body, err)
	}
	if _, err := os.Stat(old + ".bak"); !os.IsNotExist(err) {
		t.Fatal("bak left beside war")
	}
	matches, err := filepath.Glob(filepath.Join(backup, "tomcat-a", "order-*.war"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("backup files %v %v", matches, err)
	}
	bak, err := os.ReadFile(matches[0])
	if err != nil || string(bak) != "old" {
		t.Fatalf("backup %q %v", bak, err)
	}
	if _, err := os.Stat(filepath.Join(webapps, "order")); !os.IsNotExist(err) {
		t.Fatal("exploded dir still present")
	}
}

func TestRollbackRestoresWar(t *testing.T) {
	base := t.TempDir()
	webapps := filepath.Join(base, "webapps")
	if err := os.Mkdir(webapps, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(webapps, "order.war")
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "new.war")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	backup := t.TempDir()
	report, err := ReplaceWar(base, "order", src, backup, "tomcat-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(webapps, "order"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Rollback(backup, report.TargetPath, report.BackupPath, "war", "order"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dest)
	if err != nil || string(body) != "old" {
		t.Fatalf("war %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(webapps, "order")); !os.IsNotExist(err) {
		t.Fatal("exploded still present")
	}
	if _, err := Rollback(backup, dest, filepath.Join(t.TempDir(), "outside.war"), "war", "order"); err == nil {
		t.Fatal("outside backup accepted")
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
		"RELEASE-NOTES": "Apache Tomcat Version 9.0.98\n",
		"bin/catalina.sh": "new",
	})
	backup := t.TempDir()
	if _, err := UpgradeBase(base, zipPath, backup, "tomcat-a"); err != nil {
		t.Fatal(err)
	}
	if TomcatVersion(base) != "9.0.98" {
		t.Fatalf("version %s", TomcatVersion(base))
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
	body := []byte{}
	_ = body
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zw := zipWriter(t, file)
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
