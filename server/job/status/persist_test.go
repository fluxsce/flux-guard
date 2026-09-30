package status

import (
	"context"
	"testing"
	"time"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestReloadAfterRestart(t *testing.T) {
	ctx := context.Background()
	conn, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := Open(ctx, conn); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	Report(Status{
		TenantID: "default", Code: "demo", Title: "演示", Enabled: true,
		Trigger: "schedule", LastStart: end, LastEnd: end,
		OK: true, DurationMs: 40, Result: []byte(`{"ok":true}`), RanAtLeast: true,
	})
	Report(Status{
		TenantID: "default", Code: "demo", Title: "演示", Enabled: true,
		Trigger: "manual", LastStart: end, LastEnd: end,
		OK: false, DurationMs: 8, Result: []byte(`{}`), ErrorText: "boom", RanAtLeast: true,
	})

	global.mu.Lock()
	global.items = map[string]Status{}
	global.mu.Unlock()

	if err := load(ctx, conn); err != nil {
		t.Fatal(err)
	}
	st, ok := Get("default", "demo")
	if !ok || st.OK || st.ErrorText != "boom" || !st.RanAtLeast || st.DurationMs != 8 || string(st.Result) != `{}` {
		t.Fatalf("reloaded %#v", st)
	}
	var n int
	if err := store.Scan(ctx, conn, `SELECT count(*) FROM FG_JOB_LOG WHERE tenantId = @tenantId AND jobCode = @jobCode`,
		dbx.Args{"tenantId": "default", "jobCode": "demo"}, &n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("log rows %d", n)
	}
	var jobs int
	if err := store.Scan(ctx, conn, `SELECT count(*) FROM FG_JOB WHERE tenantId = @tenantId AND jobCode = @jobCode`,
		dbx.Args{"tenantId": "default", "jobCode": "demo"}, &jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 {
		t.Fatalf("job rows %d", jobs)
	}
	logs, err := Recent("default", "demo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("logs %d", len(logs))
	}
	var failed *Log
	for i := range logs {
		if logs[i].ErrorText == "boom" {
			failed = &logs[i]
		}
	}
	if failed == nil || failed.OK || failed.DurationMs != 8 {
		t.Fatalf("%#v", logs)
	}
}
