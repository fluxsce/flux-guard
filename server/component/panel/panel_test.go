package panel

import (
	"testing"

	"fluxguard/server/component/catalog"
)

func TestPanelsCoverKinds(t *testing.T) {
	kinds := []string{"redis", "mysql", "mongo", "kafka", "zk", "nacos"}
	for _, kind := range kinds {
		items := Of(kind)
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
		metrics := ListMetrics(kind)
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
