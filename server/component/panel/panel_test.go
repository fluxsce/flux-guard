package panel

import (
	"context"
	"testing"

	"fluxguard/server/component/catalog"
	"fluxguard/server/store"
)

func TestPanelsCoverKinds(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, kind := range catalog.Codes() {
		items, err := Of(ctx, db, "default", kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) < 2 {
			t.Fatalf("%s panels missing", kind)
		}
		if items[0].PanelCode != "overview" {
			t.Fatalf("%s should open on overview", kind)
		}
		last := items[len(items)-1]
		if last.PanelCode != "terminal" || !last.Terminal {
			t.Fatalf("%s should end with its own terminal", kind)
		}
		known := map[string]struct{}{}
		for _, code := range catalog.Columns(kind) {
			known[code] = struct{}{}
		}
		metrics, err := ListMetrics(ctx, db, "default", kind)
		if err != nil {
			t.Fatal(err)
		}
		if len(metrics) == 0 {
			t.Fatalf("%s list should show a current value", kind)
		}
		for _, code := range metrics {
			if _, ok := known[code]; !ok {
				t.Fatalf("%s list metric %s is not a column", kind, code)
			}
		}
	}
}
