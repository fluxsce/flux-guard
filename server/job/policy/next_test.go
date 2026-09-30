package policy

import (
	"testing"
	"time"
)

func TestNextStart(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 30, 0, time.UTC)
	if _, ok := NextStart(Schedule{Mode: "manual"}, time.Time{}, now); ok {
		t.Fatal("manual")
	}
	at, ok := NextStart(Schedule{Mode: "at", RunAt: "2026-10-01 00:00:00"}, time.Time{}, now)
	if !ok || !at.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("future at %v %v", at, ok)
	}
	due, ok := NextStart(Schedule{Mode: "at", RunAt: "2026-09-30"}, time.Time{}, now)
	if !ok || !due.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("overdue at %v", due)
	}
	if _, ok := NextStart(Schedule{Mode: "at", RunAt: "2026-09-30"}, now, now); ok {
		t.Fatal("at already ran")
	}
	first, ok := NextStart(Schedule{Mode: "interval", IntervalMinutes: 60}, time.Time{}, now)
	if !ok || !first.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("first interval %v", first)
	}
	last := time.Date(2026, 9, 30, 11, 10, 0, 0, time.UTC)
	next, ok := NextStart(Schedule{Mode: "interval", IntervalMinutes: 60}, last, now)
	want := time.Date(2026, 9, 30, 12, 10, 0, 0, time.UTC)
	if !ok || !next.Equal(want) {
		t.Fatalf("interval %v", next)
	}
	cronLast := time.Date(2026, 9, 30, 2, 0, 30, 0, time.UTC)
	cronNext, ok := NextStart(Schedule{Mode: "cron", Cron: "0 2 * * *"}, cronLast, cronLast)
	wantCron := time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)
	if !ok || !cronNext.Equal(wantCron) {
		t.Fatalf("cron %v", cronNext)
	}
}
