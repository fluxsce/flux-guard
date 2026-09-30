package dao

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestPageAndGetAgentLogs(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	insertAgent(t, db, "default", "ag1", "主机甲")
	insertCommand(t, db, "default", "c1", "ag1", "host.collect.now", "Y", "2026-09-29 02:00:00", "ok")
	insertShell(t, db, "default", "s1", "ag1", "admin", "管理员", "INPUT", "ls -la", "2026-09-29 03:00:00")
	insertCommand(t, db, "other", "c2", "ag1", "host.collect.now", "Y", "2026-09-29 04:00:00", "其它租户")

	logs := NewLogDAO(db)
	page := dbx.Page{Page: 1, PageSize: 20}
	rows, total, err := logs.PageLogs("default", LogQuery{}, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(rows) != 2 {
		t.Fatalf("page = total %d rows %+v", total, rows)
	}
	if rows[0].Source != "SHELL" || rows[0].LogID != "s1" || rows[0].AgentName != "主机甲" {
		t.Fatalf("first = %+v", rows[0])
	}
	if rows[1].Source != "COMMAND" || rows[1].EventType != "host.collect.now" {
		t.Fatalf("second = %+v", rows[1])
	}

	filtered, total, err := logs.PageLogs("default", LogQuery{Source: "COMMAND", AgentName: "主机"}, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || filtered[0].LogID != "c1" {
		t.Fatalf("filtered = total %d rows %+v", total, filtered)
	}

	got, err := logs.GetLog(context.Background(), "default", "SHELL", "s1")
	if err != nil || got.EventType != "INPUT" || got.DetailText != "ls -la" {
		t.Fatalf("get = %+v err %v", got, err)
	}
	if _, err := logs.GetLog(context.Background(), "default", "COMMAND", "c2"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross tenant = %v", err)
	}
}

func insertAgent(t *testing.T, db *dbx.DB, tenant, agentID, name string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO FG_AGENT (
		  agentId, tenantId, agentName, hostName, ipAddress, agentTokenHash, statusFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, @tenantId, @agentName, 'h', '127.0.0.1', 'x', 'Y',
		  '2026-09-29 00:00:00', 'system', '2026-09-29 00:00:00', 'system', @agentId, 1, 'Y'
		)`, dbx.Args{"agentId": agentID, "tenantId": tenant, "agentName": name})
	if err != nil {
		t.Fatal(err)
	}
}

func insertCommand(t *testing.T, db *dbx.DB, tenant, commandID, agentID, action, status, at, detail string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO FG_COMMAND (
		  commandId, tenantId, agentId, actionName, statusFlag, resultText,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @commandId, @tenantId, @agentId, @actionName, @statusFlag, @resultText,
		  @at, 'admin', @at, 'admin', @commandId, 1, 'Y'
		)`, dbx.Args{
		"commandId": commandID, "tenantId": tenant, "agentId": agentID,
		"actionName": action, "statusFlag": status, "resultText": detail, "at": at,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func insertShell(t *testing.T, db *dbx.DB, tenant, auditID, agentID, userID, userName, event, detail, at string) {
	t.Helper()
	_, err := db.Exec(context.Background(), `
		INSERT INTO FG_SHELL_AUDIT (
		  auditId, tenantId, shellId, agentId, userId, userName, eventType, detailText,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @auditId, @tenantId, 'sh1', @agentId, @userId, @userName, @eventType, @detailText,
		  @at, @userId, @at, @userId, @auditId, 1, 'Y'
		)`, dbx.Args{
		"auditId": auditID, "tenantId": tenant, "agentId": agentID,
		"userId": userID, "userName": userName, "eventType": event, "detailText": detail, "at": at,
	})
	if err != nil {
		t.Fatal(err)
	}
}
