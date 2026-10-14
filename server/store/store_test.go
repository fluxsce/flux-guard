package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReopenKeepsSeedAndSkipsDoneStatements(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fg.db")
	db, err := Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var users int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_USER`, nil, &users); err != nil {
		t.Fatal(err)
	}
	if users == 0 {
		t.Fatal("seed users missing")
	}
	var stmts int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_SCRIPT_STATEMENT`, nil, &stmts); err != nil {
		t.Fatal(err)
	}
	if stmts == 0 {
		t.Fatal("statement history missing")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var again int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_USER`, nil, &again); err != nil {
		t.Fatal(err)
	}
	if again != users {
		t.Fatalf("users %d -> %d", users, again)
	}
	if _, err := db.Exec(ctx, `DELETE FROM FG_USER WHERE userId = 'viewer'`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM FG_AUTH_ROLE_RESOURCE WHERE resourceId = 'fg0022:edit'`, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var left int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_USER`, nil, &left); err != nil {
		t.Fatal(err)
	}
	if left != users-1 {
		t.Fatalf("seed ran again, users %d", left)
	}
	var granted int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_ROLE_RESOURCE WHERE resourceId = 'fg0022:edit'`, nil, &granted); err != nil {
		t.Fatal(err)
	}
	if granted != 0 {
		t.Fatalf("grant restored %d", granted)
	}
}

func TestClearedHistoryReappliesFg0008(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fg.db")
	db, err := Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM FG_AUTH_ROLE_RESOURCE WHERE resourceId LIKE 'fg0008%'`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM FG_AUTH_RESOURCE WHERE resourceId LIKE 'fg0008%'`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM FG_SCRIPT_STATEMENT WHERE statementContent LIKE '%fg0008%'`, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{"fg0008", "fg0008:search", "fg0008:view", "fg0008:edit", "fg0008:release"} {
		var n int
		if err := Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_RESOURCE WHERE resourceId = '`+id+`'`, nil, &n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("resource %s missing after reopen", id)
		}
	}
	var sa, vw int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_ROLE_RESOURCE WHERE roleId = 'ROLE_SUPER_ADMIN' AND resourceId LIKE 'fg0008%'`, nil, &sa); err != nil {
		t.Fatal(err)
	}
	if sa != 5 {
		t.Fatalf("super admin grants %d", sa)
	}
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_ROLE_RESOURCE WHERE roleId = 'ROLE_VIEWER' AND resourceId LIKE 'fg0008%'`, nil, &vw); err != nil {
		t.Fatal(err)
	}
	if vw != 3 {
		t.Fatalf("viewer grants %d", vw)
	}
}

func TestRoleButtonsSplit(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, id := range []string{"fg0022:search", "fg0022:view", "fg0022:add", "fg0022:edit", "fg0022:delete", "fg0022:grant"} {
		var count int
		if err := Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_RESOURCE WHERE resourceId = '`+id+`'`, nil, &count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("resource %s missing", id)
		}
	}
	var name string
	if err := Scan(ctx, db, `SELECT resourceName FROM FG_AUTH_RESOURCE WHERE resourceId = 'fg0022:edit'`, nil, &name); err != nil {
		t.Fatal(err)
	}
	if name != "修改" {
		t.Fatalf("edit name %s", name)
	}
	var viewer int
	if err := Scan(ctx, db, `SELECT count(*) FROM FG_AUTH_ROLE_RESOURCE WHERE roleId = 'ROLE_VIEWER' AND resourceId = 'fg0022:add'`, nil, &viewer); err != nil {
		t.Fatal(err)
	}
	if viewer != 0 {
		t.Fatal("viewer was granted add")
	}
}

func TestSchemaIndexes(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, name := range []string{"IDX_FG_AUDIT_TIME", "IDX_FG_USER_ADD", "IDX_FG_HOST_CPU_ADD", "IDX_FG_TENANT_ADD"} {
		var count int
		if err := Scan(ctx, db, `SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = '`+name+`'`, nil, &count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("index %s missing", name)
		}
	}
}

func TestStatementsSplitKeepsSemicolonInString(t *testing.T) {
	got := statements(`
INSERT INTO t (a) VALUES ('a;b');
UPDATE t SET a = 1;
`)
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if got[0] != "INSERT INTO t (a) VALUES ('a;b')" {
		t.Fatalf("first %q", got[0])
	}
}
