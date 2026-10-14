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
	// 种子行「本机开发代理」不是系统底名，上报 reportName 只写入主机专表，不冲掉显示名。
	if agentName != "本机开发代理" {
		t.Fatalf("agentName=%s", agentName)
	}
}

func TestDisplayNameFillOnceAndKeepManual(t *testing.T) {
	svc := testService(t)
	if _, err := svc.db.Exec(context.Background(), `
		INSERT INTO FG_AGENT (
		  agentId, tenantId, agentName, agentCode, machineId, agentTokenHash, statusFlag,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  'host-box', 'default', 'host-a1b2c3d4', 'host', 'machine-box', 'x', 'Y',
		  'system', 'system', 'INGEST', 1, 'Y'
		)`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "host-box", BatchID: "batch-name-1", AgentCode: "host",
		Host: &protocol.Host{HostName: "box", OSName: "linux", ArchName: "amd64", ReportName: "本机主机", MachineID: "machine-box"},
	}); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := store.Scan(context.Background(), svc.db, `
		SELECT agentName FROM FG_AGENT WHERE agentId = 'host-box'`, nil, &name); err != nil {
		t.Fatal(err)
	}
	if name != "本机主机" {
		t.Fatalf("fill once got %s", name)
	}
	if _, err := svc.db.Exec(context.Background(), `
		UPDATE FG_AGENT SET agentName = '机房A' WHERE agentId = 'host-box'`, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "host-box", BatchID: "batch-name-2", AgentCode: "host",
		Host: &protocol.Host{HostName: "box", OSName: "linux", ArchName: "amd64", ReportName: "本机主机", MachineID: "machine-box"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(context.Background(), svc.db, `
		SELECT agentName FROM FG_AGENT WHERE agentId = 'host-box'`, nil, &name); err != nil {
		t.Fatal(err)
	}
	if name != "机房A" {
		t.Fatalf("manual kept %s", name)
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
