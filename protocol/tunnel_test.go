package protocol

import "testing"

func TestTunnelChannelOK(t *testing.T) {
	if !TunnelChannelOK(TunnelChannelShell) || !TunnelChannelOK(TunnelChannelAppTail) {
		t.Fatal("known channels")
	}
	if TunnelChannelOK("evil") {
		t.Fatal("unknown channel must reject")
	}
}
