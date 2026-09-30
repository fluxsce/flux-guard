package audit

import "testing"

func TestFillFailKeepsDetailSeparate(t *testing.T) {
	event := &Event{
		Action:     ActionCreate,
		TargetType: "USER",
		TargetID:   "a",
		TargetName: "a",
		Detail:     "新增用户 a",
	}
	event.Result = ResultFail
	event.FailReason = "口令至少 6 位"
	if event.Detail != "新增用户 a" {
		t.Fatalf("detail mutated: %q", event.Detail)
	}
	if event.FailReason != "口令至少 6 位" {
		t.Fatalf("failReason = %q", event.FailReason)
	}
}
