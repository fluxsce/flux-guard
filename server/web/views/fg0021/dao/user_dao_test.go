package dao

import (
	"context"
	"strings"
	"testing"

	"fluxguard/server/store"
	"fluxguard/server/web/request"
	loginsvc "fluxguard/server/web/views/login/controllers"
)

func TestCreateUpdateDeleteUser(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := loginsvc.Open(db, "default")
	users := NewUserDAO(db, svc.DropUser)
	input := UserInput{
		UserID: "ops", UserName: "ops", Password: "secret1", RealName: "运维",
		StatusFlag: "Y", TenantAdminFlag: "N", ActiveFlag: "Y",
	}
	ctx := request.With(context.Background(), request.Principal{TenantID: "default", UserID: "admin"})
	created, err := users.CreateUser(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.UserID != "ops" || created.RealName != "运维" || created.AddWho != "admin" {
		t.Fatalf("created = %+v", created)
	}
	if _, err := users.CreateUser(ctx, input); err != ErrUserExists {
		t.Fatalf("duplicate id = %v", err)
	}
	taken := input
	taken.UserID = "ops2"
	if _, err := users.CreateUser(ctx, taken); err != ErrUserNameTaken {
		t.Fatalf("duplicate name = %v", err)
	}
	if _, err := svc.Login("ops", "secret1"); err != nil {
		t.Fatal(err)
	}
	input.Password = ""
	input.RealName = "运维值班"
	updated, err := users.UpdateUser(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.RealName != "运维值班" || updated.EditWho != "admin" {
		t.Fatalf("updated = %+v", updated)
	}
	if _, err := svc.Login("ops", "secret1"); err != nil {
		t.Fatal(err)
	}
	input.Password = "secret2"
	if _, err := users.UpdateUser(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login("ops", "secret1"); err == nil {
		t.Fatal("old password still works")
	}
	if _, err := svc.Login("ops", "secret2"); err != nil {
		t.Fatal(err)
	}
	granted, err := users.SaveUserRoles(ctx, "ops", []string{"ROLE_VIEWER", "ROLE_SUPER_ADMIN"})
	if err != nil {
		t.Fatal(err)
	}
	if granted.UserID != "ops" || len(granted.RoleIDs) != 2 || !strings.Contains(granted.RoleNames, "只读用户") {
		t.Fatalf("granted = %+v", granted)
	}
	found := false
	for _, item := range users.ListUsers("default") {
		if item.UserID != "ops" {
			continue
		}
		found = item.RealName == "运维值班" && len(item.RoleIDs) == 2 && strings.Contains(item.RoleNames, "只读用户")
	}
	if !found {
		t.Fatal("user roles missing")
	}
	roles, err := users.UserRoles("default", "ops")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, role := range roles {
		if role.Checked {
			checked++
		}
	}
	if checked != 2 {
		t.Fatalf("checked roles = %d", checked)
	}
	if _, err := users.SaveUserRoles(ctx, "ops", []string{"ROLE_VIEWER"}); err != nil {
		t.Fatal(err)
	}
	roles, err = users.UserRoles("default", "ops")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if role.RoleID == "ROLE_SUPER_ADMIN" && role.Checked {
			t.Fatal("removed role still checked")
		}
		if role.RoleID == "ROLE_VIEWER" && !role.Checked {
			t.Fatal("viewer role unchecked")
		}
	}
	if err := users.DeleteUser(ctx, "admin"); err != ErrUserSelf {
		t.Fatalf("self delete = %v", err)
	}
	boss := UserInput{
		UserID: "boss", UserName: "boss", Password: "secret1", RealName: "管理员",
		StatusFlag: "Y", TenantAdminFlag: "Y", ActiveFlag: "Y",
	}
	if _, err := users.CreateUser(ctx, boss); err != nil {
		t.Fatal(err)
	}
	if err := users.DeleteUser(ctx, "boss"); err != ErrUserAdmin {
		t.Fatalf("admin delete = %v", err)
	}
	if err := users.DeleteUser(ctx, "ops"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login("ops", "secret2"); err == nil {
		t.Fatal("deleted user still logs in")
	}
	for _, item := range users.ListUsers("default") {
		if item.UserID == "ops" {
			t.Fatal("deleted user still listed")
		}
	}
}
