package setting

import "testing"

func TestValidateAndClampPageSize(t *testing.T) {
	bad := Default()
	bad.RequestTimeoutSeconds = 5
	if err := Validate(bad); err == nil {
		t.Fatal("short timeout should fail")
	}
	Put("default", WebTimeout{
		RequestTimeoutSeconds: 120,
		SessionExpireHours:    12,
		DefaultPageSize:       20,
		MaxPageSize:           50,
	})
	if got := ClampPageSize("default", 0, false); got != 20 {
		t.Fatalf("default %d", got)
	}
	if got := ClampPageSize("default", 80, true); got != 50 {
		t.Fatalf("clamp %d", got)
	}
}
