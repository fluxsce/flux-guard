package agent

import (
	"context"
	"path/filepath"
	"testing"

	"fluxguard/server/store"
	"fluxguard/server/web/request"
)

func TestUnknownAgentCodeRejected(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	agents := Open(db)
	if _, err := agents.EnsureMachineAgent("default", "python", "box-1"); err == nil {
		t.Fatal("machine code")
	}
	if _, err := agents.EnsureReportingAgent("default", "python"); err == nil {
		t.Fatal("shared code")
	}
}

func TestDevAgentToken(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	if _, _, ok := Open(db).AgentByToken("agt_dev_local"); !ok {
		t.Fatal("dev agent token")
	}
}

func TestRegisterAgentPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fg.db")
	db, err := store.Open(context.Background(), "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
	_, token, err := Open(db).RegisterAgent(ctx, RegisterInput{AgentName: "box"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.Open(context.Background(), "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tenantID, agentID, ok := Open(db).AgentByToken(token)
	if !ok || tenantID != "default" || agentID == "" {
		t.Fatalf("reopen token tenant=%s agent=%s ok=%v", tenantID, agentID, ok)
	}
}
