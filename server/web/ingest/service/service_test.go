package service

import (
	"context"
	"testing"

	"fluxguard/protocol"
	"fluxguard/server/store"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, cleanup, err := store.OpenTemp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	return Open(db)
}

func TestAcceptDuplicate(t *testing.T) {
	svc := testService(t)
	batch := protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-0001", AgentCode: "host",
		Samples: []protocol.Sample{{MetricName: "runtime.cpu.count", MetricValue: 4}},
	}
	first, err := svc.Accept(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || first.SampleCount != 1 {
		t.Fatalf("first=%+v", first)
	}
	second, err := svc.Accept(context.Background(), batch)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate {
		t.Fatal("expected duplicate")
	}
	batch.Host = &protocol.Host{HostName: "box", OSName: "windows", ArchName: "amd64", CPUCount: 4, MemTotalBytes: 1024, ReportName: "机房A"}
	saved, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-0003", AgentCode: "host",
		Samples: []protocol.Sample{{MetricName: "host.cpu.count", MetricValue: 4}},
		Host:    batch.Host,
	})
	if err != nil || saved.Duplicate {
		t.Fatalf("host save %+v %v", saved, err)
	}
	got, ok := svc.HostOf("default", "agent-local")
	if !ok || got.HostName != "box" || got.CPUCount != 4 || got.ReportName != "机房A" {
		t.Fatalf("stored=%+v ok=%v", got, ok)
	}
	var agentName string
	if err := store.Scan(context.Background(), svc.db, `SELECT agentName FROM FG_AGENT WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &agentName); err != nil {
		t.Fatal(err)
	}
	if agentName != "机房A" {
		t.Fatalf("agentName=%s", agentName)
	}
}

func TestAcceptRejectsBadValue(t *testing.T) {
	svc := testService(t)
	_, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-0002", AgentCode: "host",
		Samples: []protocol.Sample{{MetricName: "runtime.cpu.count", MetricValue: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "a", BatchID: "x", AgentCode: "host",
		Samples: []protocol.Sample{{MetricName: "runtime.cpu.count", MetricValue: 1}},
	})
	if err == nil {
		t.Fatal("expected batch id error")
	}
}
