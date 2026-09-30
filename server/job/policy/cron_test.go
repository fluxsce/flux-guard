package policy

import (
	"testing"
	"time"
)

func TestParseCron(t *testing.T) {
	expr, err := ParseCron("0 2 * * *")
	if err != nil {
		t.Fatal(err)
	}
	hit := time.Date(2026, 9, 30, 2, 0, 40, 0, time.UTC)
	if !expr.Match(hit) || expr.Match(hit.Add(time.Minute)) {
		t.Fatal("daily 02:00")
	}
	step, err := ParseCron("*/15 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	if !step.Match(time.Date(2026, 9, 30, 1, 45, 0, 0, time.UTC)) || step.Match(time.Date(2026, 9, 30, 1, 46, 0, 0, time.UTC)) {
		t.Fatal("step")
	}
	both, err := ParseCron("0 0 1 * MON")
	if err != nil {
		t.Fatal(err)
	}
	if !both.Match(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("day of month")
	}
	if !both.Match(time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("weekday")
	}
	if both.Match(time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("neither day nor weekday")
	}
	named, err := ParseCron("0 0 * JAN SUN")
	if err != nil || !named.Match(time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatal(err)
	}
	if _, err := ParseCron("60 * * * *"); err == nil {
		t.Fatal("expected range error")
	}
	if _, err := ParseCron("* * *"); err == nil {
		t.Fatal("expected field count error")
	}
}
