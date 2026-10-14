package protocol

import "testing"

func TestKnownAgent(t *testing.T) {
	for _, code := range []string{AgentHost, AgentGo, AgentApp, AgentJava, AgentAppside, " host "} {
		if !KnownAgent(code) {
			t.Fatalf("accept %q", code)
		}
	}
	for _, code := range []string{"", "Host", "python", "java2", "app-extra"} {
		if KnownAgent(code) {
			t.Fatalf("reject %q", code)
		}
	}
}
