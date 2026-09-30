package archive

import (
	"encoding/json"
	"testing"

	"fluxguard/server/job/catalog"
)

func TestRegistered(t *testing.T) {
	s, ok := catalog.ByCode(Code)
	if !ok || s.Kind != Kind || s.Source != catalog.SourceBuiltin {
		t.Fatalf("%#v", s)
	}
}

func TestRoundTrip(t *testing.T) {
	ok := Default()
	if err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(ok)
	if err != nil {
		t.Fatal(err)
	}
	got := Parse(raw)
	if got.Params.AuditLogDays != ok.Params.AuditLogDays || !got.Enabled || got.Mode != ok.Mode {
		t.Fatalf("%#v", got)
	}
	partial := Parse(json.RawMessage(`{"mode":"manual","params":{"auditLogDays":30}}`))
	if !partial.Enabled || partial.Mode != "manual" || partial.Params.AuditLogDays != 30 || partial.Params.OpsLogDays != ok.Params.OpsLogDays {
		t.Fatalf("%#v", partial)
	}
	bad := ok
	bad.IntervalMinutes = 0
	if err := Validate(bad); err == nil {
		t.Fatal("interval 0 should fail")
	}
	manual := ok
	manual.Mode = "manual"
	manual.IntervalMinutes = 0
	if err := Validate(manual); err != nil {
		t.Fatal(err)
	}
	at := ok
	at.Mode = "at"
	at.RunAt = ""
	if err := Validate(at); err == nil {
		t.Fatal("at without time should fail")
	}
}
