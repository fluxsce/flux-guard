package middleware

import (
	"testing"

	"fluxguard/server/utils/const/limits"
)

func TestMaxBodyBytes(t *testing.T) {
	if MaxBodyBytes != 200<<20 {
		t.Fatalf("body limit %d", MaxBodyBytes)
	}
	if MaxBodyBytes != limits.MaxBodyBytes {
		t.Fatal("middleware alias drifted from limits")
	}
}
