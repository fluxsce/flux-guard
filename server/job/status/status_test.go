package status

import "testing"

func TestReportTouch(t *testing.T) {
	Report(Status{TenantID: "t1", Code: "demo", Title: "演示", OK: true, DurationMs: 12, Result: []byte(`{"n":3}`), RanAtLeast: true})
	st, ok := Get("t1", "demo")
	if !ok || string(st.Result) != `{"n":3}` || st.DurationMs != 12 || !st.OK {
		t.Fatalf("%#v", st)
	}
	Touch("t1", "demo", "演示", false)
	st, _ = Get("t1", "demo")
	if string(st.Result) != `{"n":3}` || st.Enabled {
		t.Fatalf("touch should keep history %#v", st)
	}
}
