package agentpack

import (
	"os"
	"path/filepath"
	"testing"

	"fluxguard/protocol"
)

func TestFileName(t *testing.T) {
	name, err := FileName(protocol.AgentHost)
	if err != nil || name != "flux-guard-agent-host.tar.gz" {
		t.Fatalf("host = %q %v", name, err)
	}
	name, err = FileName(protocol.AgentGo)
	if err != nil || name != "flux-guard-agent-goruntime.tar.gz" {
		t.Fatalf("go = %q %v", name, err)
	}
	name, err = FileName(protocol.AgentAppside)
	if err != nil || name != "flux-guard-agent-appside.tar.gz" {
		t.Fatalf("appside = %q %v", name, err)
	}
	if _, err := FileName("java"); err == nil {
		t.Fatal("java should fail")
	}
}

func TestOpenFromAgentsDir(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "flux-guard-agent-host.tar.gz")
	if err := os.WriteFile(path, []byte("pack"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	file, name, err := Open(protocol.AgentHost)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if name != "flux-guard-agent-host.tar.gz" {
		t.Fatalf("name = %s", name)
	}
	body, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "pack" {
		t.Fatalf("body = %q", body)
	}
}
