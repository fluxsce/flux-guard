package service

import (
	"context"
	"testing"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/store"
)

func TestJavaMetricsStaySplit(t *testing.T) {
	svc := testService(t)
	started := time.Date(2026, 9, 30, 1, 0, 0, 0, time.UTC)
	_, err := svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "java-order", BatchID: "batch-java-1", AgentCode: "java",
		Java: &protocol.Java{
			AppName: "order", MainClass: "com.example.Order", ProcessID: "42",
			JavaVersion: "17.0.11", StartTime: started, HostName: "box", IPAddress: "10.0.0.8",
		},
		Samples: []protocol.Sample{
			{MetricName: "runtime.up", MetricValue: 1},
			{MetricName: "jvm.heap.used.bytes", MetricValue: 800},
			{MetricName: "jvm.heap.max.bytes", MetricValue: 2000},
			{MetricName: "jvm.nonheap.used.bytes", MetricValue: 120},
			{MetricName: "jvm.metaspace.used.bytes", MetricValue: 40},
			{MetricName: "jvm.gc.count", MetricValue: 3, Labels: map[string]string{"collector": "G1 Young"}},
			{MetricName: "jvm.gc.time.ms", MetricValue: 15, Labels: map[string]string{"collector": "G1 Young"}},
			{MetricName: "jvm.gc.count", MetricValue: 1, Labels: map[string]string{"collector": "G1 Old"}},
			{MetricName: "jvm.thread.count", MetricValue: 24},
			{MetricName: "jvm.thread.daemon.count", MetricValue: 18},
			{MetricName: "jvm.thread.runnable.count", MetricValue: 6},
			{MetricName: "jvm.thread.blocked.count", MetricValue: 2},
			{MetricName: "jvm.thread.waiting.count", MetricValue: 16},
			{MetricName: "jvm.memory.pool.used.bytes", MetricValue: 300, Labels: map[string]string{"pool": "eden"}},
			{MetricName: "jvm.memory.pool.max.bytes", MetricValue: 4096, Labels: map[string]string{"pool": "direct"}},
			{MetricName: "jvm.memory.pool.used.bytes", MetricValue: 900, Labels: map[string]string{"pool": "mystery"}},
			{MetricName: "jvm.cpu.process.percent", MetricValue: 12.5},
			{MetricName: "jvm.class.loaded", MetricValue: 4100},
			{MetricName: "jvm.class.unloaded", MetricValue: 7},
			{MetricName: "jvm.gc.pause.count", MetricValue: 2, Labels: map[string]string{"collector": "G1 Young"}},
			{MetricName: "jvm.gc.pause.max.ms", MetricValue: 18, Labels: map[string]string{"collector": "G1 Young"}},
			{MetricName: "jvm.gc.pause.time.ms", MetricValue: 25, Labels: map[string]string{"collector": "G1 Young"}},
			{MetricName: "host.cpu.usage.percent", MetricValue: 9},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var appName, mainClass, version string
	if err := store.Scan(context.Background(), svc.db, `
		SELECT appName, mainClass, javaVersion FROM FG_AGENT_JAVA WHERE tenantId = 'default' AND agentId = 'java-order'`,
		nil, &appName, &mainClass, &version); err != nil {
		t.Fatal(err)
	}
	if appName != "order" || mainClass != "com.example.Order" || version != "17.0.11" {
		t.Fatalf("identity app=%s class=%s version=%s", appName, mainClass, version)
	}
	var used int64
	var pct float64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT heapUsedBytes, heapUsedPercent FROM FG_JAVA_HEAP WHERE tenantId = 'default' AND agentId = 'java-order'`,
		nil, &used, &pct); err != nil {
		t.Fatal(err)
	}
	if used != 800 || pct != 40 {
		t.Fatalf("heap used=%d pct=%v", used, pct)
	}
	var collectors int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_JAVA_GC WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &collectors); err != nil {
		t.Fatal(err)
	}
	if collectors != 2 {
		t.Fatalf("collectors=%d", collectors)
	}
	var threads int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT threadCount FROM FG_JAVA_THREAD WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &threads); err != nil {
		t.Fatal(err)
	}
	if threads != 24 {
		t.Fatalf("threads=%d", threads)
	}
	var blocked int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT blockedCount FROM FG_JAVA_THREAD_STATE WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &blocked); err != nil {
		t.Fatal(err)
	}
	if blocked != 2 {
		t.Fatalf("blocked=%d", blocked)
	}
	var directMax int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT maxBytes FROM FG_JAVA_POOL_MAX WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &directMax); err != nil {
		t.Fatal(err)
	}
	if directMax != 4096 {
		t.Fatalf("direct max=%d", directMax)
	}
	var poolName string
	var poolUsed int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT poolName, usedBytes FROM FG_JAVA_POOL WHERE tenantId = 'default' AND agentId = 'java-order'`,
		nil, &poolName, &poolUsed); err != nil {
		t.Fatal(err)
	}
	if poolName != "eden" || poolUsed != 300 {
		t.Fatalf("pool %s used=%d", poolName, poolUsed)
	}
	var cpu float64
	var loaded int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT processCpuPercent, loadedClassCount FROM FG_JAVA_RUNTIME WHERE tenantId = 'default' AND agentId = 'java-order'`,
		nil, &cpu, &loaded); err != nil {
		t.Fatal(err)
	}
	if cpu != 12.5 || loaded != 4100 {
		t.Fatalf("cpu=%v loaded=%d", cpu, loaded)
	}
	var pauseMax int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT pauseMaxMs FROM FG_JAVA_GC_PAUSE WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &pauseMax); err != nil {
		t.Fatal(err)
	}
	if pauseMax != 18 {
		t.Fatalf("pause max=%d", pauseMax)
	}
	var hostCPU int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_HOST_CPU WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &hostCPU); err != nil {
		t.Fatal(err)
	}
	if hostCPU != 1 {
		t.Fatalf("host cpu rows=%d", hostCPU)
	}

	_, err = svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "java-order", BatchID: "batch-java-2", AgentCode: "java",
		Samples: []protocol.Sample{
			{MetricName: "jvm.heap.used.bytes", MetricValue: 900},
			{MetricName: "jvm.heap.max.bytes", MetricValue: 2000},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var rows int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_JAVA_HEAP WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("heap rows=%d", rows)
	}
	if err := store.Scan(context.Background(), svc.db, `
		SELECT appName FROM FG_AGENT_JAVA WHERE tenantId = 'default' AND agentId = 'java-order'`, nil, &appName); err != nil {
		t.Fatal(err)
	}
	if appName != "order" {
		t.Fatalf("kept app=%s", appName)
	}

	_, err = svc.Accept(context.Background(), protocol.Batch{
		TenantID: "default", AgentID: "host-box", BatchID: "batch-not-java", AgentCode: "host",
		Samples: []protocol.Sample{{MetricName: "jvm.heap.used.bytes", MetricValue: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var javaRows int
	if err := store.Scan(context.Background(), svc.db, `
		SELECT count(*) FROM FG_AGENT_JAVA WHERE agentId = 'host-box'`, nil, &javaRows); err != nil {
		t.Fatal(err)
	}
	if javaRows != 0 {
		t.Fatalf("host wrote java identity rows=%d", javaRows)
	}
}
