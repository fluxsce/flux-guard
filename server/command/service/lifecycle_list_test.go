package service

import "testing"

func TestLifecycleInstance(t *testing.T) {
	ext := `{"instance":"vue_test-datahub01web","action":"app.runtime.restart"}`
	if !lifecycleInstance(ext, "vue_test-datahub01web") {
		t.Fatal("same instance")
	}
	if lifecycleInstance(ext, "other") {
		t.Fatal("other instance")
	}
	if lifecycleInstance("{", "vue_test-datahub01web") {
		t.Fatal("bad json")
	}
}
