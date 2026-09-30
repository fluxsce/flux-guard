package policy

import "testing"

func TestGroup(t *testing.T) {
	if Group("demo") != "job:demo" {
		t.Fatal(Group("demo"))
	}
}

func TestValidateSchedule(t *testing.T) {
	if err := ValidateSchedule(Schedule{IntervalMinutes: 0}); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateSchedule(Schedule{Enabled: true, IntervalMinutes: 60}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchedule(Schedule{Mode: "manual", IntervalMinutes: 0}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchedule(Schedule{Mode: "at", RunAt: "2026-09-30 08:00:00"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchedule(Schedule{Mode: "at"}); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateSchedule(Schedule{Mode: "cron", Cron: "0 2 * * *"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSchedule(Schedule{Mode: "cron", Cron: "0 2 *"}); err == nil {
		t.Fatal("expected cron error")
	}
	missing := DecodeSchedule([]byte(`{"mode":"manual"}`))
	if missing.RetryCount != 1 || missing.TimeoutSeconds != 600 || missing.Mode != "manual" {
		t.Fatalf("%#v", missing)
	}
	explicit := DecodeSchedule([]byte(`{"mode":"cron","cron":"0 2 * * *","retryCount":0}`))
	if explicit.Mode != "cron" || explicit.RetryCount != 0 {
		t.Fatalf("%#v", explicit)
	}
	raw := []byte(`{"enabled":true,"mode":"interval","intervalMinutes":60,"params":{"n":1}}`)
	params := Params(raw)
	if string(params) != `{"n":1}` {
		t.Fatal(string(params))
	}
}

func TestEncodeDocumentKeepsParams(t *testing.T) {
	raw, err := EncodeDocument(Document{
		Kind:   "demo",
		Source: "external",
		Schedule: Schedule{
			Enabled: true, Mode: "manual", TimeoutSeconds: 30, RetryCount: 0,
		},
		Params: []byte(`{"path":"/tmp","n":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(Params(raw)) != `{"n":1,"path":"/tmp"}` && string(Params(raw)) != `{"path":"/tmp","n":1}` {
		t.Fatal(string(Params(raw)))
	}
	if _, err := ParseObject(`[1]`); err == nil {
		t.Fatal("expected object")
	}
	empty, err := ParseObject("  ")
	if err != nil || string(empty) != `{}` {
		t.Fatal(string(empty), err)
	}
}
