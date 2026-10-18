package command

import (
	"encoding/json"
	"testing"

	"fluxguard/protocol"
)

func TestScrubAndMarshalKeepsLocalLogBytes(t *testing.T) {
	raw, err := scrubAndMarshal(protocol.CommandResult{
		CommandID: "c1",
		OK:        true,
		Message:   "已启动",
		Steps: []protocol.CommandStep{{
			Name: "startup", Status: "done", Detail: "容器已启动\xff尾部",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got protocol.CommandResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.CommandID != "c1" || !got.OK {
		t.Fatalf("回执被改掉: %+v", got)
	}
	if len(got.Steps) != 1 || got.Steps[0].Detail == "" {
		t.Fatalf("步骤丢失: %+v", got.Steps)
	}
}
