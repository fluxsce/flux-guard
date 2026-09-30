package shell

import (
	"context"
	"testing"
	"time"

	"fluxguard/server/store"
)

func TestInputStaysWithOwner(t *testing.T) {
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
	if err := s.WriteInput(ctx, "default", "viewer", view.ShellID, "whoami"); err != errNotOwner {
		t.Fatalf("viewer write %v", err)
	}
	if _, err := s.ReadOutput(ctx, "default", "viewer", view.ShellID, 0); err != errNotOwner {
		t.Fatalf("viewer read %v", err)
	}
	if err := s.WriteInput(ctx, "default", "admin", view.ShellID, "whoami"); err != nil {
		t.Fatal(err)
	}
	in, err := s.TakeInput(ctx, "default", "agent-local", view.ShellID, time.Second)
	if err != nil || in.Closed || in.Data != "whoami" {
		t.Fatalf("take %+v %v", in, err)
	}
	if err := s.PutOutput(ctx, "default", "agent-local", view.ShellID, "admin\r\n"); err != nil {
		t.Fatal(err)
	}
	chunk, err := s.ReadOutput(ctx, "default", "admin", view.ShellID, 0)
	if err != nil || chunk.Text != "admin\r\n" || chunk.Closed {
		t.Fatalf("chunk %+v %v", chunk, err)
	}
	if err := s.Close(ctx, "default", "admin", view.ShellID, false); err != nil {
		t.Fatal(err)
	}
	in, err = s.TakeInput(ctx, "default", "agent-local", view.ShellID, time.Second)
	if err != nil || !in.Closed {
		t.Fatalf("after close %+v %v", in, err)
	}
}
