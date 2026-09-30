package dao

import (
	"context"
	"testing"
	"time"

	"fluxguard/server/alert"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

func systemCtx() context.Context {
	return request.With(context.Background(), request.Principal{TenantID: "default", UserID: "system"})
}

func TestHostRuleFiresAfterDuration(t *testing.T) {
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	svc := alert.Open(db)
	rule, err := SaveRule(systemCtx(), db, svc, models.HostRuleInput{
		RuleName: "CPU 高", MetricCode: "cpu.usage", CompareOp: "gt", ThresholdValue: 80,
		DurationSec: 0, LevelCode: "critical", StatusFlag: "Y",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(context.Background(), `
		INSERT INTO FG_HOST_CPU (
		  agentId, tenantId, pointId, batchId, usagePercent, observedTime, addWho, editWho, oprSeqFlag
		) VALUES (
		  'agent-local', 'default', 'pt-alert', 'batch-alert', 95, '2026-09-28 10:00:00', 'system', 'system', 'T'
		)`, nil)
	if err != nil {
		t.Fatal(err)
	}
	Evaluate(context.Background(), db, svc)
	events, err := ListIncidents(context.Background(), db, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].StatusCode != "firing" || events[0].RuleID != rule.RuleID {
		t.Fatalf("事件 %#v", events)
	}

	pending, err := SaveRule(systemCtx(), db, svc, models.HostRuleInput{
		RuleName: "内存", MetricCode: "mem.usage", CompareOp: "gt", ThresholdValue: 10,
		DurationSec: 3600, LevelCode: "warn", StatusFlag: "Y",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(context.Background(), `
		INSERT INTO FG_HOST_MEM (
		  agentId, tenantId, pointId, batchId, usedPercent, observedTime, addWho, editWho, oprSeqFlag
		) VALUES (
		  'agent-local', 'default', 'pt-mem', 'batch-mem', 40, '2026-09-28 10:00:00', 'system', 'system', 'M'
		)`, nil)
	if err != nil {
		t.Fatal(err)
	}
	Evaluate(context.Background(), db, svc)
	events, err = ListIncidents(context.Background(), db, "default")
	if err != nil {
		t.Fatal(err)
	}
	var mem Incident
	for _, item := range events {
		if item.RuleID == pending.RuleID {
			mem = item
		}
	}
	if mem.StatusCode != "pending" {
		t.Fatalf("持续未到应保持待定 %#v", mem)
	}
	old := store.FormatTime(time.Now().UTC().Add(-2 * time.Hour))
	if _, err := db.Exec(context.Background(), `
		UPDATE FG_HOST_INCIDENT SET firstFireTime = @firstFireTime
		WHERE tenantId = 'default' AND incidentId = @incidentId`,
		dbx.Args{"firstFireTime": old, "incidentId": mem.IncidentID}); err != nil {
		t.Fatal(err)
	}
	Evaluate(context.Background(), db, svc)
	events, err = ListIncidents(context.Background(), db, "default")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range events {
		if item.IncidentID == mem.IncidentID {
			mem = item
		}
	}
	if mem.StatusCode != "firing" {
		t.Fatalf("持续到达后应触发 %#v", mem)
	}
}
