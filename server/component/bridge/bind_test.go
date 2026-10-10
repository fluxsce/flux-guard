package bridge

import (
	"testing"

	"fluxguard/server/component/catalog"
)

func TestRejectsUnknownMetric(t *testing.T) {
	_, err := valuesFor(catalog.Kafka, true, map[string]float64{"notAMetric": 1})
	if err == nil {
		t.Fatal("expected reject")
	}
}
