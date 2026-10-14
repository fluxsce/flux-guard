package service

import (
	"context"
	"testing"

	"fluxguard/protocol"
	"fluxguard/server/store"
)

func TestLegacyAppSamplesLandOnGoRow(t *testing.T) {
	svc := testService(t)
	batch := protocol.Batch{
		TenantID: "default", AgentID: "go-old", BatchID: "batch-go-old-1", AgentCode: protocol.AgentApp,
		Samples: []protocol.Sample{
			{MetricName: "app.runtime.goroutine.count", MetricValue: 12},
			{MetricName: "app.runtime.heap.alloc.bytes", MetricValue: 4096},
		},
	}
	if _, err := svc.Accept(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	var name string
	var routines int
	var heap int64
	if err := store.Scan(context.Background(), svc.db, `
		SELECT appName, goroutineCount, heapAllocBytes FROM FG_AGENT_GO
		WHERE tenantId = 'default' AND agentId = 'go-old'`, nil, &name, &routines, &heap); err != nil {
		t.Fatal(err)
	}
	if name != "go" || routines != 12 || heap != 4096 {
		t.Fatalf("name %s routines %d heap %d", name, routines, heap)
	}
}
