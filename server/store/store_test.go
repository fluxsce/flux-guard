package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReopenKeepsSeedAndCreatesMissingTable(t *testing.T) {
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
	if _, err := db.Exec(ctx, `DROP TABLE FG_EVENT_POINT`, nil); err != nil {
		t.Fatal(err)
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
	var table int
	if err := Scan(ctx, db, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'FG_EVENT_POINT'`, nil, &table); err != nil {
		t.Fatal(err)
	}
	if table != 1 {
		t.Fatal("missing table was not created")
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
