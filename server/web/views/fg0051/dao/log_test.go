package dao

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestPageAndGetLogs(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	insertLog(t, db, "default", "a1", "admin", "管理员", "LOGIN", "2026-09-29 01:00:00", "登录成功")
	insertLog(t, db, "default", "a2", "ops", "运维", "UPDATE", "2026-09-29 02:00:00", "修改用户")
	insertLog(t, db, "other", "a3", "ops", "运维", "UPDATE", "2026-09-29 03:00:00", "其它租户")

	logs := NewLogDAO(db)
	page := dbx.Page{Page: 1, PageSize: 20}
	rows, total, err := logs.PageLogs("default", LogQuery{}, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 2 || rows[0].AuditID != "a2" {
		t.Fatalf("page = total %d rows %+v", total, rows)
	}

	filtered, total, err := logs.PageLogs("default", LogQuery{UserName: "运维", Action: "UPDATE"}, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(filtered) != 1 || filtered[0].DetailText != "修改用户" {
		t.Fatalf("filtered = total %d rows %+v", total, filtered)
	}

	ranged, total, err := logs.PageLogs("default", LogQuery{FromTime: "2026-09-29 01:30:00"}, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || ranged[0].AuditID != "a2" {
		t.Fatalf("ranged = total %d rows %+v", total, ranged)
	}

	got, err := logs.GetLog(context.Background(), "default", "a1")
	if err != nil || got.Action != "LOGIN" || got.UserName != "管理员" {
		t.Fatalf("get = %+v err %v", got, err)
	}
	if _, err := logs.GetLog(context.Background(), "default", "a3"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross tenant = %v", err)
	}
}

func insertLog(t *testing.T, db *dbx.DB, tenant, auditID, userID, userName, action, at, detail string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO FG_AUTH_AUDIT_LOG (
		  auditId, tenantId, userId, userName, action, resultFlag, detailText,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @auditId, @tenantId, @userId, @userName, @action, 'Y', @detailText,
		  @at, @userId, @at, @userId, @auditId, 1, 'Y'
		)`, dbx.Args{
		"auditId": auditID, "tenantId": tenant, "userId": userID, "userName": userName,
		"action": action, "detailText": detail, "at": at,
	})
	if err != nil {
		t.Fatal(err)
	}
}
