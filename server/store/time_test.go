package store

import (
	"testing"
	"time"
)

func TestTimeValueKeepsStoredUTCClock(t *testing.T) {
	// MySQL parseTime 且 Loc=Local 时，会把库存的 UTC 墙钟贴上本地时区。
	local := time.Date(2026, 10, 12, 10, 48, 0, 0, time.FixedZone("CST", 8*3600))
	got, ok := TimeValue(local)
	if !ok {
		t.Fatal("time.Time")
	}
	if got.Format(time.RFC3339) != "2026-10-12T10:48:00Z" {
		t.Fatalf("clock shifted: %s", got.Format(time.RFC3339))
	}
	text, ok := TimeValue("2026-10-12 10:48:00")
	if !ok || text.Format(time.RFC3339) != "2026-10-12T10:48:00Z" {
		t.Fatalf("text: %s %v", text.Format(time.RFC3339), ok)
	}
}
