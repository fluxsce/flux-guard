package retention

import (
	"context"
	"errors"
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

func TestPurgeStopsIdentityGrowth(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	archive.Put("retain", archive.Policy{
		Enabled:         true,
		Mode:            "interval",
		IntervalMinutes: 60,
		Params: archive.Params{
			OpsLogDays: 30,
			SampleDays: 14,
		},
	})
	old := store.FormatTime(time.Now().UTC().AddDate(0, 0, -40))
	fresh := store.FormatTime(time.Now().UTC())
	insertJavaIdentity(t, ctx, db, "live-java", fresh)
	insertJavaIdentity(t, ctx, db, "dead-java", old)
	insertJavaRun(t, ctx, db, "live-open", "live-java", old, "")
	insertJavaRun(t, ctx, db, "live-closed", "live-java", old, old)
	insertJavaRun(t, ctx, db, "dead-open", "dead-java", old, "")
	insertShell(t, ctx, db, "shell-closed", "C", old)
	insertShell(t, ctx, db, "shell-open", "O", old)
	insertHostIdentity(t, ctx, db, "live-host", fresh)
	insertHostIdentity(t, ctx, db, "dead-host", old)
	insertAgent(t, ctx, db, "live-agent", fresh)
	insertAgent(t, ctx, db, "dead-agent", old)
	insertAgent(t, ctx, db, "never-agent", "")

	for _, name := range []string{"java-run", "java-identity", "host-identity", "agent-identity", "shell-session"} {
		if err := runNamed(ctx, db, name, "retain"); err != nil {
			t.Fatal(name, err)
		}
	}
	assertCount(t, ctx, db, "FG_JAVA_RUN", "retain", 1)
	assertCount(t, ctx, db, "FG_AGENT_JAVA", "retain", 1)
	assertCount(t, ctx, db, "FG_AGENT_HOST", "retain", 1)
	assertCount(t, ctx, db, "FG_SHELL", "retain", 1)
	var agents int
	if err := store.Scan(ctx, db, `SELECT count(*) FROM FG_AGENT WHERE tenantId = @tenantId AND agentId IN ('live-agent','dead-agent','never-agent')`,
		dbx.Args{"tenantId": "retain"}, &agents); err != nil {
		t.Fatal(err)
	}
	if agents != 2 {
		t.Fatalf("agents remaining %d", agents)
	}
}

func runNamed(ctx context.Context, db *dbx.DB, name, tenantID string) error {
	for _, job := range NewJobs(db) {
		if job.Name() == name {
			_, err := job.Run(ctx, []string{tenantID})
			return err
		}
	}
	return errors.New("missing job " + name)
}

func assertCount(t *testing.T, ctx context.Context, db *dbx.DB, table, tenantID string, want int) {
	t.Helper()
	var count int
	err := store.Scan(ctx, db, `SELECT count(*) FROM `+table+` WHERE tenantId = @tenantId`, dbx.Args{"tenantId": tenantID}, &count)
	if err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s remaining %d, want %d", table, count, want)
	}
}

func insertJavaIdentity(t *testing.T, ctx context.Context, db *dbx.DB, agentID, reported string) {
	t.Helper()
	_, err := db.Exec(ctx, `
		INSERT INTO FG_AGENT_JAVA (
		  agentId, tenantId, appName, reportedTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, 'retain', 'app', @reported,
		  @reported, 'system', @reported, 'system', 'T', 1, 'Y'
		)`, dbx.Args{"agentId": agentID, "reported": reported})
	if err != nil {
		t.Fatal(err)
	}
}

func insertJavaRun(t *testing.T, ctx context.Context, db *dbx.DB, runID, agentID, opened, closed string) {
	t.Helper()
	var closedAt any
	if closed != "" {
		closedAt = closed
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_JAVA_RUN (
		  runId, agentId, tenantId, startTime, openedTime, closedTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @runId, @agentId, 'retain', @opened, @opened, @closed,
		  @opened, 'system', @opened, 'system', 'T', 1, 'Y'
		)`, dbx.Args{"runId": runID, "agentId": agentID, "opened": opened, "closed": closedAt})
	if err != nil {
		t.Fatal(err)
	}
}

func insertShell(t *testing.T, ctx context.Context, db *dbx.DB, shellID, status, addTime string) {
	t.Helper()
	_, err := db.Exec(ctx, `
		INSERT INTO FG_SHELL (
		  shellId, tenantId, agentId, userId, shellTitle, shellNo, statusFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @shellId, 'retain', 'agent', 'u1', '终端', 1, @status,
		  @addTime, 'system', @addTime, 'system', 'T', 1, 'Y'
		)`, dbx.Args{"shellId": shellID, "status": status, "addTime": addTime})
	if err != nil {
		t.Fatal(err)
	}
}

func insertHostIdentity(t *testing.T, ctx context.Context, db *dbx.DB, agentID, reported string) {
	t.Helper()
	_, err := db.Exec(ctx, `
		INSERT INTO FG_AGENT_HOST (
		  agentId, tenantId, hostName, osName, archName, reportedTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, 'retain', 'box', 'linux', 'amd64', @reported,
		  @reported, 'system', @reported, 'system', 'T', 1, 'Y'
		)`, dbx.Args{"agentId": agentID, "reported": reported})
	if err != nil {
		t.Fatal(err)
	}
}

func insertAgent(t *testing.T, ctx context.Context, db *dbx.DB, agentID, seen string) {
	t.Helper()
	var last any
	if seen != "" {
		last = seen
	}
	stamp := seen
	if stamp == "" {
		stamp = store.FormatTime(time.Now().UTC())
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_AGENT (
		  agentId, tenantId, agentName, agentTokenHash, lastSeenTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, 'retain', @agentId, 'hash', @seen,
		  @stamp, 'system', @stamp, 'system', 'T', 1, 'Y'
		)`, dbx.Args{"agentId": agentID, "seen": last, "stamp": stamp})
	if err != nil {
		t.Fatal(err)
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
