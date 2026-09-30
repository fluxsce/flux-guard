package retention

import (
	"context"
	"strings"
	"testing"
	"time"

	"fluxguard/server/job/ext/archive"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestLimitedDeleteSQL(t *testing.T) {
	sqliteSQL, err := limitedDelete("sqlite", "FG_AUTH_AUDIT_LOG", "addTime", "", 2000)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"rowid", "LIMIT 2000", "@tenantId", "@before"} {
		if !strings.Contains(sqliteSQL, part) {
			t.Fatalf("sqlite sql missing %q: %q", part, sqliteSQL)
		}
	}
	mysqlSQL, err := limitedDelete("mysql", "FG_COMMAND", "addTime", "statusFlag IN ('Y', 'N')", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"DELETE FROM FG_COMMAND", "LIMIT 100", "statusFlag IN"} {
		if !strings.Contains(mysqlSQL, part) {
			t.Fatalf("mysql sql missing %q: %q", part, mysqlSQL)
		}
	}
	if _, err := limitedDelete("sqlite", "FG_AUTH;DROP", "addTime", "", 1); err == nil {
		t.Fatal("expected invalid table")
	}
}

func TestPurgeExpiredAudit(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	archive.Put("default", archive.Policy{
		Enabled:         true,
		Mode:            "interval",
		IntervalMinutes: 60,
		Params: archive.Params{
			AuditLogDays: 7,
		},
	})

	old := store.FormatTime(time.Now().UTC().AddDate(0, 0, -30))
	fresh := store.FormatTime(time.Now().UTC())
	insertAudit(t, ctx, db, "old-1", old)
	insertAudit(t, ctx, db, "new-1", fresh)

	var audit *TableJob
	for _, job := range NewJobs(db) {
		if job.Name() == "audit-log" {
			audit = job
			break
		}
	}
	if audit == nil {
		t.Fatal("missing audit-log job")
	}
	if _, err := audit.Run(ctx, []string{"default"}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_AUDIT_LOG WHERE tenantId = @tenantId`,
		dbx.Args{"tenantId": "default"}, &count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("remaining %d", count)
	}
}

func insertAudit(t *testing.T, ctx context.Context, db *dbx.DB, auditID, addTime string) {
	t.Helper()
	_, err := db.Exec(ctx, `
		INSERT INTO FG_AUTH_AUDIT_LOG (
		  auditId, tenantId, userId, userName, action, resultFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @auditId, 'default', 'u1', 'u1', 'create', 'Y',
		  @addTime, 'system', @addTime, 'system', 'T', 1, 'Y'
		)`, dbx.Args{"auditId": auditID, "addTime": addTime})
	if err != nil {
		t.Fatal(err)
	}
}
