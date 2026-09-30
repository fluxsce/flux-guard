package sched

import (
	"testing"
	"time"

	"fluxguard/server/job/contract"
	"fluxguard/server/job/policy"
)

func TestDueModes(t *testing.T) {
	m := New(nil, "default")
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	if m.due("t", "due-job", policy.Schedule{Mode: string(contract.TriggerManual)}, now) {
		t.Fatal("manual should not auto run")
	}
	if m.due("t", "due-job", policy.Schedule{Mode: string(contract.TriggerAt), RunAt: "2026-10-01 00:00:00"}, now) {
		t.Fatal("future at should wait")
	}
	if !m.due("t", "due-job", policy.Schedule{Mode: string(contract.TriggerAt), RunAt: "2026-09-30"}, now) {
		t.Fatal("past at should run once")
	}
	m.mark("t", "due-job", now)
	if m.due("t", "due-job", policy.Schedule{Mode: string(contract.TriggerAt), RunAt: "2026-09-30"}, now) {
		t.Fatal("at should not run again")
	}
	if !m.due("t", "interval-job", policy.Schedule{Mode: string(contract.TriggerInterval), IntervalMinutes: 60}, now) {
		t.Fatal("first interval should run")
	}
	cronNow := time.Date(2026, 9, 30, 2, 0, 30, 0, time.UTC)
	cronSched := policy.Schedule{Mode: string(contract.TriggerCron), Cron: "0 2 * * *"}
	if !m.due("t", "cron-job", cronSched, cronNow) {
		t.Fatal("cron minute should match")
	}
	m.mark("t", "cron-job", cronNow)
	if m.due("t", "cron-job", cronSched, cronNow) {
		t.Fatal("same minute should not run twice")
	}
	if m.due("t", "cron-job", cronSched, cronNow.Add(time.Minute)) {
		t.Fatal("next minute should not match")
	}
	if !m.due("t", "cron-job", cronSched, time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC)) {
		t.Fatal("next day at the same minute should run")
	}
}
