package agentupdate

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestUntarAndMergeKeepsLiveConfig(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "pkg.tar.gz")
	writeAgentTar(t, archive, map[string]string{
		"flux-guard-agent-host/flux-guard-agent-host":           "bin-v2",
		"flux-guard-agent-host/configs/host.yaml.example":       "example",
		"flux-guard-agent-host/configs/host.yaml":               "packaged",
		"flux-guard-agent-host/scripts/run/start.sh":            "#!/bin/sh\n",
	})
	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := untar(archive, stage); err != nil {
		t.Fatal(err)
	}
	payload, err := payloadRoot(stage)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "install")
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs", "host.yaml"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeTree(payload, root, "flux-guard-agent-host"); err != nil {
		t.Fatal(err)
	}
	live, err := os.ReadFile(filepath.Join(root, "configs", "host.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(live) != "local" {
		t.Fatalf("live config overwritten: %q", live)
	}
	example, err := os.ReadFile(filepath.Join(root, "configs", "host.yaml.example"))
	if err != nil {
		t.Fatal(err)
	}
	if string(example) != "example" {
		t.Fatalf("example %q", example)
	}
	if findBinary(payload, "flux-guard-agent-host") == "" {
		t.Fatal("binary missing in payload")
	}
}

func writeAgentTar(t *testing.T, path string, files map[string]string) {
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
