package shell

import (
	"context"
	"testing"
	"time"

	"fluxguard/server/store"
)

func TestManyShells(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s := OpenStore(db)
	first, err := s.Open(ctx, "default", "admin", "管理员", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Open(ctx, "default", "admin", "管理员", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Open(ctx, "default", "viewer", "只读", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	if first.ShellID == second.ShellID || first.ShellNo != 1 || second.ShellNo != 2 || other.ShellNo != 1 {
		t.Fatalf("shells %+v %+v %+v", first, second, other)
	}
	list, err := s.List(ctx, "default", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("open %d", len(list))
	}
	if err := s.Close(ctx, "default", "viewer", first.ShellID, false); err != errDenied {
		t.Fatalf("close other: %v", err)
	}
	if err := s.Close(ctx, "default", "admin", second.ShellID, false); err != nil {
		t.Fatal(err)
	}
	list, err = s.List(ctx, "default", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("after close %d", len(list))
	}
}

func TestIdleShellIsDropped(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s := OpenStore(db)
	view, err := s.Open(ctx, "default", "admin", "管理员", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	pipes.backdate(view.ShellID, time.Now().Add(-2*time.Minute))
	list, err := s.List(ctx, "default", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("idle still open %d", len(list))
	}
	if pipes.get(view.ShellID) != nil {
		t.Fatal("idle lane still held")
	}
}

func TestShellAuditRecordsOpenCloseInput(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s := OpenStore(db)
	view, err := s.Open(ctx, "default", "admin", "管理员", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteInput(ctx, "default", "admin", view.ShellID, "ls\r"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, "default", "admin", view.ShellID, false); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAudits(ctx, "default", "agent-local", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 3 {
		t.Fatalf("audits %d %+v", len(list), list)
	}
	seen := map[string]string{}
	for _, item := range list {
		seen[item.EventType] = item.DetailText
	}
	if _, ok := seen[eventOpen]; !ok {
		t.Fatalf("missing OPEN %+v", list)
	}
	if seen[eventInput] != "ls" {
		t.Fatalf("input %q", seen[eventInput])
	}
	if _, ok := seen[eventClose]; !ok {
		t.Fatalf("missing CLOSE %+v", list)
	}
}

func TestReapDropsIdleWithoutAgent(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	s := OpenStore(db)
	view, err := s.Open(ctx, "default", "admin", "管理员", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	pipes.backdate(view.ShellID, time.Now().Add(-2*time.Minute))
	if err := s.reapIdle(ctx); err != nil {
		t.Fatal(err)
	}
	if pipes.get(view.ShellID) != nil {
		t.Fatal("reap left the lane")
	}
	list, err := s.List(ctx, "default", "agent-local")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("reap left %d", len(list))
	}
}
