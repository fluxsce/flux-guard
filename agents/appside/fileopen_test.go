package appside

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenWatchedFileAllowsBase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "order.war")
	if err := os.WriteFile(path, []byte("war"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := []Runtime{{Instance: "war-a", Mode: "files", Base: dir, BackupDir: t.TempDir()}}
	name, size, body, err := OpenWatchedFile(list, path)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if name != "order.war" || size != 3 {
		t.Fatalf("got %s %d", name, size)
	}
	raw, _ := io.ReadAll(body)
	if string(raw) != "war" {
		t.Fatalf("body %q", raw)
	}
	if _, _, _, err := OpenWatchedFile(list, filepath.Join(t.TempDir(), "x.war")); err == nil {
		t.Fatal("outside accepted")
	}
}
