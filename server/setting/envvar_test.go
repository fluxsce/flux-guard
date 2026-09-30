package setting

import "testing"

func TestMergeEnvVarsByKind(t *testing.T) {
	first, err := MergeEnvVars([]EnvVarInput{
		{Name: "API_BASE", Kind: KindText, Value: "https://ops.local"},
		{Name: "PAGE_HINT", Kind: KindNumber, Value: "20"},
		{Name: "TRACE", Kind: KindBool, Value: "yes"},
		{Name: "TOKEN", Kind: KindSecret, Value: "secret-token"},
	}, DefaultEnvVars())
	if err != nil {
		t.Fatal(err)
	}
	PutEnvVars("default", first)
	if got, ok := GetEnv("default", "API_BASE"); !ok || got != "https://ops.local" {
		t.Fatalf("text %q %v", got, ok)
	}
	if got, _ := GetEnv("default", "TRACE"); got != "true" {
		t.Fatalf("bool %q", got)
	}
	token, ok := GetEnv("default", "TOKEN")
	if !ok || token != "secret-token" {
		t.Fatalf("secret plain %q %v", token, ok)
	}
	masked := MaskEnvVars(first)
	var shown string
	for _, item := range masked {
		if item.Name == "TOKEN" {
			shown = item.Value
		}
	}
	if shown != "********" {
		t.Fatalf("mask %q", shown)
	}
	again, err := MergeEnvVars([]EnvVarInput{
		{Name: "TOKEN", Kind: KindSecret, Value: "********"},
	}, first)
	if err != nil {
		t.Fatal(err)
	}
	PutEnvVars("default", again)
	if got, _ := GetEnv("default", "TOKEN"); got != "secret-token" {
		t.Fatalf("kept secret %q", got)
	}
	if _, err := MergeEnvVars([]EnvVarInput{{Name: "PAGE_HINT", Kind: KindNumber, Value: "abc"}}, DefaultEnvVars()); err == nil {
		t.Fatal("number should fail")
	}
}

func TestExpandEnv(t *testing.T) {
	PutEnvVars("expand", EnvVars{Items: []EnvVar{{Name: "HOST", Kind: KindText, Value: "ops"}}})
	if got := Expand("expand", "https://${HOST}/v1"); got != "https://ops/v1" {
		t.Fatalf("expand %q", got)
	}
	if got := Expand("expand", "${MISSING:local}"); got != "local" {
		t.Fatalf("fallback %q", got)
	}
}
