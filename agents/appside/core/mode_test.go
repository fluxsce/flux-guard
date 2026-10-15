package core

import (
	"context"
	"strings"
	"testing"

	"fluxguard/protocol"
)

func TestReplaceRejectsKindMismatch(t *testing.T) {
	RegisterMode(stubMode{name: "kind-check", kinds: map[string]bool{"war": true}})

	rt := Runtime{Mode: "kind-check", Base: t.TempDir(), Instance: "t1", BackupDir: t.TempDir()}
	_, err := Replace(rt, protocol.CommandCall{Kind: "jar", AppName: "x"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "不支持制品") {
		t.Fatalf("got %v", err)
	}
}

type stubMode struct {
	name  string
	kinds map[string]bool
}

func (s stubMode) Name() string { return s.name }

func (s stubMode) CollectArtifacts(ctx context.Context, rt Runtime) ([]Artifact, error) {
	return nil, nil
}

func (s stubMode) FillSide(side *protocol.AppSide, rt Runtime) {}

func (s stubMode) AllowKind(kind string) bool { return s.kinds[kind] }

func (s stubMode) NeedsRestart(kind string) bool { return false }

func (s stubMode) Replace(rt Runtime, call protocol.CommandCall, file string) (ApplyReport, error) {
	return ApplyReport{}, nil
}

func (s stubMode) ListInventory(rt Runtime) ([]Artifact, error) { return nil, nil }

func (s stubMode) SupportUpgrade() bool { return false }

func (s stubMode) Upgrade(rt Runtime, zipPath string) (ApplyReport, error) {
	return ApplyReport{}, nil
}

func (s stubMode) MarkStartupLog(rt Runtime) LogMark { return LogMark{} }

func (s stubMode) ProbeStartup(ctx context.Context, rt Runtime, mark LogMark) protocol.CommandStep {
	return protocol.CommandStep{Name: "startup", Status: "done"}
}
