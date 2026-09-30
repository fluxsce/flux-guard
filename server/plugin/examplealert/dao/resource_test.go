package dao

import (
	"context"
	"testing"

	"fluxguard/server/store"
	accessdao "fluxguard/server/web/middleware/dao"
	login "fluxguard/server/web/views/login/controllers"
)

func TestEnsureDoesNotGrant(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `
		DELETE FROM FG_AUTH_ROLE_RESOURCE WHERE resourceId IN ('fg0100', 'fg0100:view')`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `
		DELETE FROM FG_AUTH_RESOURCE WHERE resourceId IN ('fg0100', 'fg0100:view')`, nil); err != nil {
		t.Fatal(err)
	}
	Ensure(db, "default")
	svc := login.Open(db, "default")
	session, err := svc.Login("viewer", "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	principal, _ := svc.Principal(session.Token)
	access := accessdao.Open(db)
	for _, menu := range access.Menus(principal.TenantID, principal.UserID) {
		if menu.ResourceCode == "fg0100" {
			t.Fatal("plugin menu must stay hidden until granted")
		}
	}
	if access.Allow(principal.TenantID, principal.UserID, "fg0100") {
		t.Fatal("plugin resource must not be granted by ensure")
	}
}
