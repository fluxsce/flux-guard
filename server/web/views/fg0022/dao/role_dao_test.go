package dao

import (
	"context"
	"testing"

	"fluxguard/server/store"
	accessdao "fluxguard/server/web/middleware/dao"
	"fluxguard/server/web/request"
	loginsvc "fluxguard/server/web/views/login/controllers"
	dbx "github.com/fluxsce/dbx/db"
)

func TestCreateUpdateDeleteRole(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	roles := NewRoleDAO(db)
	ctx := request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
	input := RoleInput{RoleID: "ROLE_OPS", RoleName: "运维", RoleStatus: "Y", ActiveFlag: "Y", NoteText: "值班"}
	created, err := roles.CreateRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.RoleID != "ROLE_OPS" || created.RoleName != "运维" || created.BuiltInFlag != "N" || created.AddWho != "admin" {
		t.Fatalf("created = %+v", created)
	}
	if _, err := roles.CreateRole(ctx, input); err != ErrRoleExists {
		t.Fatalf("duplicate = %v", err)
	}
	input.RoleName = "运维值班"
	updated, err := roles.UpdateRole(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RoleName != "运维值班" || updated.BuiltInFlag != "N" || updated.EditWho != "admin" || updated.NoteText != "值班" {
		t.Fatalf("updated = %+v", updated)
	}
	got, err := roles.GetRole("default", "ROLE_OPS")
	if err != nil {
		t.Fatal(err)
	}
	if got.RoleName != "运维值班" || got.NoteText != "值班" {
		t.Fatalf("get = %+v", got)
	}
	if _, err := roles.GetRole("default", "ROLE_MISSING"); err != ErrRoleMissing {
		t.Fatalf("missing = %v", err)
	}
	found := false
	for _, item := range roles.ListRoles("default") {
		if item.RoleID == "ROLE_OPS" && item.RoleName == "运维值班" && item.BuiltInFlag == "N" {
			found = true
		}
	}
	if !found {
		t.Fatal("created role missing")
	}
	if err := roles.DeleteRole(ctx, "ROLE_SUPER_ADMIN"); err != ErrRoleBuiltIn {
		t.Fatalf("builtin delete = %v", err)
	}
	if err := roles.DeleteRole(ctx, "ROLE_OPS"); err != nil {
		t.Fatal(err)
	}
	for _, item := range roles.ListRoles("default") {
		if item.RoleID == "ROLE_OPS" {
			t.Fatal("deleted role still listed")
		}
	}
}

func TestRoleResourceTreeChecked(t *testing.T) {
	tree := buildRoleResourceTree([]resourceFlat{
		{id: "group:observe", name: "运行观测", code: "group:observe", kind: "GROUP"},
		{id: "fg0200", name: "自定义", code: "fg0200", kind: "MODULE", parent: "group:observe"},
		{id: "fg0200:view", name: "查看", code: "fg0200:view", kind: "BUTTON", parent: "fg0200", granted: true},
		{id: "fg0200:edit", name: "修改", code: "fg0200:edit", kind: "BUTTON", parent: "fg0200"},
	})
	if len(tree) != 1 || tree[0].Checked {
		t.Fatal("partial buttons must leave the group unchecked")
	}
	module := tree[0].Children[0]
	if module.Checked || !module.Children[0].Checked || module.Children[1].Checked {
		t.Fatal("only the granted leaf is checked")
	}
}

func TestExpandRoleResources(t *testing.T) {
	links := []resourceLink{
		{id: "group:observe"},
		{id: "fg0200", parent: "group:observe"},
		{id: "fg0200:view", parent: "fg0200"},
		{id: "fg0200:edit", parent: "fg0200"},
	}
	buttonOnly := toSet(expandRoleResources([]string{"fg0200:view"}, links))
	if !buttonOnly["fg0200:view"] || !buttonOnly["fg0200"] || !buttonOnly["group:observe"] || buttonOnly["fg0200:edit"] {
		t.Fatalf("button grant = %v", buttonOnly)
	}
	module := toSet(expandRoleResources([]string{"fg0200"}, links))
	if !module["fg0200:view"] || !module["fg0200:edit"] || !module["group:observe"] {
		t.Fatalf("module grant = %v", module)
	}
}

func TestSaveRoleResourcesReplacesGrant(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	for _, row := range []struct {
		id, name, code, kind, path, parent string
		level, sort                        int
	}{
		{"fg0200", "自定义", "fg0200", "MODULE", "/fg0200", "group:observe", 2, 80},
		{"fg0200:view", "查看", "fg0200:view", "BUTTON", "", "fg0200", 3, 10},
		{"fg0200:edit", "修改", "fg0200:edit", "BUTTON", "", "fg0200", 3, 20},
	} {
		if _, err := db.Exec(context.Background(), `
			INSERT INTO FG_AUTH_RESOURCE (
			  resourceId, tenantId, resourceName, resourceCode, resourceType,
			  resourcePath, parentResourceId, resourceLevel, sortOrder, language,
			  resourceStatus, builtInFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @resourceId, 'default', @resourceName, @resourceCode, @resourceType,
			  @resourcePath, @parentResourceId, @resourceLevel, @sortOrder, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'
			)`, dbx.Args{
			"resourceId": row.id, "resourceName": row.name, "resourceCode": row.code, "resourceType": row.kind,
			"resourcePath": emptyNil(row.path), "parentResourceId": emptyNil(row.parent),
			"resourceLevel": row.level, "sortOrder": row.sort,
		}); err != nil {
			t.Fatal(err)
		}
	}
	svc := loginsvc.Open(db, "default")
	access := accessdao.Open(db)
	session, err := svc.Login("viewer", "fluxguard")
	if err != nil {
		t.Fatal(err)
	}
	principal, _ := svc.Principal(session.Token)
	for _, menu := range access.Menus(principal.TenantID, principal.UserID) {
		if menu.ResourceCode == "fg0200" {
			t.Fatal("ungranted module must stay hidden")
		}
	}
	ctx := request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
	roles := NewRoleDAO(db)
	if err := roles.SaveRoleResources(ctx, "ROLE_VIEWER", []string{"fg0200:view"}); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, menu := range access.Menus(principal.TenantID, principal.UserID) {
		if menu.ResourceCode == "fg0200" {
			found = true
		}
	}
	if !found {
		t.Fatal("granted module missing")
	}
	if !access.Allow(principal.TenantID, principal.UserID, "fg0200") {
		t.Fatal("checking a button must keep its module")
	}
	if access.Allow(principal.TenantID, principal.UserID, "fg0200:edit") {
		t.Fatal("viewer must not receive edit unless it is checked")
	}
	if access.Allow(principal.TenantID, principal.UserID, "fg0001:view") {
		t.Fatal("save replaces the previous grant set")
	}
	if access.AllowModule(principal.TenantID, principal.UserID, "fg0001") {
		t.Fatal("replaced grant must drop the old module")
	}
	if !access.AllowModule(principal.TenantID, principal.UserID, "fg0200") {
		t.Fatal("a button grant must keep module access")
	}
	if !access.AllowModule(principal.TenantID, principal.UserID, "fg9999") {
		t.Fatal("unknown module stays login-only")
	}
	if err := roles.SaveRoleResources(ctx, "ROLE_VIEWER", []string{"fg0200"}); err != nil {
		t.Fatal(err)
	}
	if !access.Allow(principal.TenantID, principal.UserID, "fg0200:edit") {
		t.Fatal("checking a module must grant its buttons")
	}
}

func emptyNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func toSet(ids []string) map[string]bool {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}
