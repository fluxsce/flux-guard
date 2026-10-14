package agent

import (
	"context"
	"path/filepath"
	"testing"

	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
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

func TestEnsureMachineAgentSplitsByCode(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	agents := Open(db)
	hostID, err := agents.EnsureMachineAgent("default", "host", "machine-shared")
	if err != nil {
		t.Fatal(err)
	}
	sideID, err := agents.EnsureMachineAgent("default", "appside", "machine-shared")
	if err != nil {
		t.Fatal(err)
	}
	if hostID == sideID {
		t.Fatal("same row for different agent codes")
	}
	again, err := agents.EnsureMachineAgent("default", "host", "machine-shared")
	if err != nil || again != hostID {
		t.Fatalf("host again %s %v", again, err)
	}
}

func TestRenameAgentKeepsUnique(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	ctx := request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
	agents := Open(db)
	first, _, err := agents.RegisterAgent(ctx, RegisterInput{AgentName: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := agents.RegisterAgent(ctx, RegisterInput{AgentName: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agents.RenameAgent(ctx, second.AgentID, "alpha"); err != ErrAgentNameTaken {
		t.Fatalf("taken: %v", err)
	}
	view, err := agents.RenameAgent(ctx, first.AgentID, "机房A")
	if err != nil || view.AgentName != "机房A" {
		t.Fatalf("rename %+v %v", view, err)
	}
}

func TestDeleteAgentsHardRemoves(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	ctx := request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
	agents := Open(db)
	view, token, err := agents.RegisterAgent(ctx, RegisterInput{AgentName: "gone"})
	if err != nil {
		t.Fatal(err)
	}
	n, err := agents.DeleteAgents(ctx, []string{view.AgentID, view.AgentID, " "})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("deleted %d", n)
	}
	if _, _, ok := agents.AgentByToken(token); ok {
		t.Fatal("token still valid")
	}
	for _, item := range agents.ListAgents("default") {
		if item.AgentID == view.AgentID {
			t.Fatal("still listed")
		}
	}
	var left int
	if err := store.Scan(ctx, db, `
		SELECT count(*) FROM FG_AGENT WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": "default", "agentId": view.AgentID}, &left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("row remains %d", left)
	}
	if _, err := agents.DeleteAgents(ctx, nil); err != ErrAgentIDsRequired {
		t.Fatalf("empty ids: %v", err)
	}
}
