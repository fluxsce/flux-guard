package appfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveWithProgress(t *testing.T) {
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	payload := bytes.Repeat([]byte("x"), 1024)
	if err := SaveWithProgress("tenant1", "job1", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatal(err)
	}
	if !Ready("tenant1", "job1") {
		t.Fatal("expected ready")
	}
	prog := ReadProgress("tenant1", "job1")
	if prog.Status != "ready" {
		t.Fatalf("status=%s", prog.Status)
	}
	if prog.BytesReceived != int64(len(payload)) {
		t.Fatalf("bytes=%d", prog.BytesReceived)
	}
	path, err := Path("tenant1", "job1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path + ".progress")); err != nil {
		t.Fatal(err)
	}
	Remove("tenant1", "job1")
	if Ready("tenant1", "job1") {
		t.Fatal("expected removed")
	}
}

func TestPurgeOlderThan(t *testing.T) {
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	payload := []byte("old")
	if err := SaveWithProgress("tenant1", "oldjob", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatal(err)
	}
	path, err := Path("tenant1", "oldjob")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-3 * time.Hour)
	_ = os.Chtimes(path, past, past)
	_ = os.Chtimes(path+".progress", past, past)

	n, err := PurgeOlderThan(2 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("purged=%d", n)
	}
	if Ready("tenant1", "oldjob") {
		t.Fatal("stale job should be gone")
	}
}
