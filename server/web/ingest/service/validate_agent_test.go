package service

import (
	"testing"

	"fluxguard/protocol"
)

func TestUnknownAgentRejected(t *testing.T) {
	err := validate(protocol.Batch{
		TenantID: "default", AgentID: "agent-1", BatchID: "batch-0001", AgentCode: "python",
		Samples: []protocol.Sample{{MetricName: "custom.cpu", MetricValue: 1}},
	})
	if err == nil || err.Error() != "代理种类无效" {
		t.Fatal(err)
	}
}
