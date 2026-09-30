package dao_test

import (
	"context"
	"testing"

	"fluxguard/server/store"
	accessdao "fluxguard/server/web/middleware/dao"
	login "fluxguard/server/web/views/login/controllers"
)

func TestViewerButtonAndModule(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	auth := login.Open(db, "default")
	session, err := auth.Login("viewer", "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	principal, ok := auth.Principal(session.Token)
	if !ok {
		t.Fatal("session missing")
	}
	access := accessdao.Open(db)
	if access.AllowButton(principal.TenantID, principal.UserID, "fg0002:edit") {
		t.Fatal("viewer must not register agents")
	}
	if !access.AllowButton(principal.TenantID, principal.UserID, "fg0001:view") {
		t.Fatal("viewer should view overview")
	}
	if !access.Allow(principal.TenantID, principal.UserID, "fg0023:view") {
		t.Fatal("viewer should open the resource catalog")
	}
	if !access.AllowButton(principal.TenantID, principal.UserID, "fg0024:view") {
		t.Fatal("viewer should open environment settings")
	}
	if access.AllowButton(principal.TenantID, principal.UserID, "fg0024:edit") {
		t.Fatal("viewer must not change environment settings")
	}
	if !access.AllowModule(principal.TenantID, principal.UserID, "fg0001") {
		t.Fatal("viewer should enter overview")
	}
	var overview accessdao.Menu
	var found bool
	for _, menu := range access.Menus(principal.TenantID, principal.UserID) {
		if menu.ResourceCode == "fg0001" {
			overview = menu
			found = true
		}
	}
	if !found {
		t.Fatal("viewer should see overview in menus")
	}
	if !hasCode(overview.Buttons, "fg0001:view") || hasCode(overview.Buttons, "fg0002:edit") {
		t.Fatal("menu buttons must follow the role grant")
	}
	if !hasCode(overview.Actions, "view") {
		t.Fatal("menu actions must list the module buttons")
	}
}

func hasCode(codes []string, code string) bool {
	for _, item := range codes {
		if item == code {
			return true
		}
	}
	return false
}
