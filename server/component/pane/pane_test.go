package pane

import (
	"context"
	"fmt"
	"testing"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/panel"
	"fluxguard/server/component/row"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestPaneWidth(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, kind := range catalog.All() {
		if kind.PaneTable == "" || !ident.MatchString(kind.PaneTable) {
			t.Fatalf("%s 缺少当前值表", kind.KindCode)
		}
		var count any
		if err := store.Scan(ctx, db, fmt.Sprintf(`SELECT COUNT(*) FROM pragma_table_info('%s')`, kind.PaneTable), nil, &count); err != nil {
			t.Fatal(err)
		}
		n := int(row.Float(count))
		if n == 0 || n > MaxColumns {
			t.Fatalf("%s 列数 %d，应在 1 到 %d", kind.PaneTable, n, MaxColumns)
		}
		rows, err := db.Query(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info('%s')`, kind.PaneTable), nil)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]struct{}{}
		for rows.Next() {
			var name any
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			text := row.Text(name)
			if len(text) > MaxColumns {
				rows.Close()
				t.Fatalf("%s.%s 字段名超过 %d", kind.PaneTable, text, MaxColumns)
			}
			names[text] = struct{}{}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		for _, code := range panel.ListMetrics(kind.KindCode) {
			if _, ok := names[code]; !ok {
				t.Fatalf("%s 缺少列表列 %s", kind.PaneTable, code)
			}
		}
	}
}

func TestTouchLatest(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	args := dbx.Args{
		"tenantId": "t1", "componentId": "c1", "observedTime": "2026-10-01 09:00:00",
		"usedMemoryBytes": 1024, "memUsagePercent": 12.5, "aofOk": -1, "slowlogLen": 3,
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	if err := Touch(ctx, db, catalog.Redis, args); err != nil {
		t.Fatal(err)
	}
	args["usedMemoryBytes"] = 2048
	args["observedTime"] = "2026-10-01 09:01:00"
	if err := Touch(ctx, db, catalog.Redis, args); err != nil {
		t.Fatal(err)
	}
	found, err := Latest(ctx, db, "FG_COMPONENT_REDIS", "t1", []string{"c1"}, panel.ListMetrics(catalog.Redis))
	if err != nil {
		t.Fatal(err)
	}
	got := found["c1"]
	if got["usedMemoryBytes"] != 2048 || got["aofOk"] != -1 || got["slowlogLen"] != 3 {
		t.Fatalf("%v", got)
	}
	var n any
	if err := store.Scan(ctx, db, `SELECT COUNT(*) FROM FG_COMPONENT_REDIS WHERE tenantId = @tenantId`, dbx.Args{"tenantId": "t1"}, &n); err != nil {
		t.Fatal(err)
	}
	if int(row.Float(n)) != 1 {
		t.Fatalf("当前值应只有一行，实际 %v", n)
	}
}
