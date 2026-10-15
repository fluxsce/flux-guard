package tomcat

import (
	"os"
	"path/filepath"
	"testing"

	"fluxguard/agents/appside/core"
)

func TestListWebapps(t *testing.T) {
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

func TestReplaceWarParallelDeployName(t *testing.T) {
	base := t.TempDir()
	webapps := filepath.Join(base, "webapps")
	if err := os.Mkdir(webapps, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(webapps, "order##2.4.1.war")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
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
	if report.TargetPath != old {
		t.Fatalf("target %s", report.TargetPath)
	}
	body, err := os.ReadFile(old)
	if err != nil || string(body) != "new" {
		t.Fatalf("war %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(webapps, "order.war")); !os.IsNotExist(err) {
		t.Fatal("should not create parallel order.war")
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
	if _, err := core.Rollback(backup, report.TargetPath, report.BackupPath, "war", "order"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(dest)
	if err != nil || string(body) != "old" {
		t.Fatalf("war %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(webapps, "order")); !os.IsNotExist(err) {
		t.Fatal("exploded still present")
	}
	if _, err := core.Rollback(backup, dest, filepath.Join(t.TempDir(), "outside.war"), "war", "order"); err == nil {
		t.Fatal("outside backup accepted")
	}
}

func TestUpgradeKeepsTomcatVersionFile(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"bin", "conf", "webapps"} {
		if err := os.Mkdir(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "RELEASE-NOTES"), []byte("Apache Tomcat Version 9.0.98\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if TomcatVersion(base) != "9.0.98" {
		t.Fatalf("version %s", TomcatVersion(base))
	}
}
