package service

import (
	"context"
	"testing"

	"fluxguard/protocol"
	"fluxguard/server/store"
)

func TestHostMetricsStaySplit(t *testing.T) {
	svc := testService(t)
	_, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-host-1", AgentCode: "host",
		Host: &protocol.Host{HostName: "box", OSName: "linux", ArchName: "amd64", CPUCount: 2, MemTotalBytes: 2048},
		Samples: []protocol.Sample{
			{MetricName: "runtime.up", MetricValue: 1},
			{MetricName: "host.cpu.count", MetricValue: 2},
			{MetricName: "host.cpu.usage.percent", MetricValue: 12.5},
			{MetricName: "host.cpu.load1", MetricValue: 0.4},
			{MetricName: "host.cpu.core.usage.percent", MetricValue: 20, Labels: map[string]string{"cpu": "0"}},
			{MetricName: "host.mem.used.percent", MetricValue: 40},
			{MetricName: "host.mem.used.bytes", MetricValue: 800},
			{MetricName: "host.disk.used.percent", MetricValue: 70, Labels: map[string]string{"path": "/", "device": "sda1"}},
			{MetricName: "host.disk.used.percent", MetricValue: 10, Labels: map[string]string{"path": "/boot", "device": "sda2"}},
			{MetricName: "host.disk.io.read.bytes_per_sec", MetricValue: 2048, Labels: map[string]string{"device": "sda"}},
			{MetricName: "host.disk.io.util.percent", MetricValue: 8, Labels: map[string]string{"device": "sda"}},
			{MetricName: "host.net.receive.bytes_per_sec", MetricValue: 4096, Labels: map[string]string{"device": "eth0"}},
			{MetricName: "host.net.transmit.bytes_per_sec", MetricValue: 1024, Labels: map[string]string{"device": "eth0"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var usage, load float64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT usagePercent, loadAvg1 FROM FG_HOST_CPU WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &usage, &load); err != nil {
		t.Fatal(err)
	}
	if usage != 12.5 || load != 0.4 {
		t.Fatalf("cpu usage=%v load=%v", usage, load)
	}
	var mounts int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_HOST_DISK WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &mounts); err != nil {
		t.Fatal(err)
	}
	if mounts != 2 {
		t.Fatalf("mounts=%d", mounts)
	}
	var readBps float64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT readBytesPerSec FROM FG_HOST_DISK_IO WHERE tenantId = 'default' AND deviceName = 'sda'`, nil, &readBps); err != nil {
		t.Fatal(err)
	}
	if readBps != 2048 {
		t.Fatalf("read=%v", readBps)
	}
	var rx float64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT receiveBytesPerSec FROM FG_HOST_NET WHERE tenantId = 'default' AND nicName = 'eth0'`, nil, &rx); err != nil {
		t.Fatal(err)
	}
	if rx != 4096 {
		t.Fatalf("rx=%v", rx)
	}

	_, err = svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-host-2", AgentCode: "host",
		Samples: []protocol.Sample{
			{MetricName: "host.disk.used.percent", MetricValue: 71, Labels: map[string]string{"path": "/", "device": "sda1"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_HOST_DISK WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &mounts); err != nil {
		t.Fatal(err)
	}
	if mounts != 3 {
		t.Fatalf("mounts after insert=%d", mounts)
	}
	var hosts int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_AGENT_HOST WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &hosts); err != nil {
		t.Fatal(err)
	}
	if hosts != 1 {
		t.Fatalf("host identity rows=%d", hosts)
	}
}

func TestTCPAndFDStayOutOfNicTable(t *testing.T) {
	svc := testService(t)
	_, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-tcp-1", AgentCode: "host",
		Samples: []protocol.Sample{
			{MetricName: "host.net.tcp.established", MetricValue: 7},
			{MetricName: "host.net.tcp.listen", MetricValue: 2},
			{MetricName: "host.fd.allocated", MetricValue: 100},
			{MetricName: "host.fd.max", MetricValue: 1000},
			{MetricName: "host.net.receive.bytes_per_sec", MetricValue: 10, Labels: map[string]string{"device": "eth0"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var established, nics int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT establishedCount FROM FG_HOST_TCP WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &established); err != nil {
		t.Fatal(err)
	}
	if established != 7 {
		t.Fatalf("established=%d", established)
	}
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_HOST_NET WHERE tenantId = 'default' AND agentId = 'agent-local'`, nil, &nics); err != nil {
		t.Fatal(err)
	}
	if nics != 1 {
		t.Fatalf("nics=%d", nics)
	}
}

func TestJavaIdentityUpdates(t *testing.T) {
	svc := testService(t)
	host := &protocol.Host{HostName: "jvm", OSName: "linux", ArchName: "amd64", ReportName: "order"}
	_, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-java-1", AgentCode: "java", Host: host,
	})
	if err != nil {
		t.Fatal(err)
	}
	host.ReportName = "pay"
	_, err = svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "agent-local", BatchID: "batch-java-2", AgentCode: "java", Host: host,
	})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	var appName string
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*), COALESCE(max(appName), '') FROM FG_AGENT_JAVA WHERE tenantId = 'default' AND agentId = 'agent-local'`,
		nil, &count, &appName); err != nil {
		t.Fatal(err)
	}
	if count != 1 || appName != "pay" {
		t.Fatalf("java rows=%d app=%s", count, appName)
	}
}
