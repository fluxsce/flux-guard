package controllers

import (
	"testing"

	"fluxguard/protocol"
)

func TestReportingMachineID(t *testing.T) {
	host := protocol.Request{Host: &protocol.Host{MachineID: "os-1"}, Java: &protocol.Java{HostName: "box", ProcessID: "9"}}
	if got := reportingMachineID(host); got != "os-1" {
		t.Fatalf("host machine %s", got)
	}
	java := protocol.Request{Java: &protocol.Java{HostName: "box", ProcessID: "42"}}
	if got := reportingMachineID(java); got != "box:42" {
		t.Fatalf("java machine %s", got)
	}
	pid := protocol.Request{Java: &protocol.Java{ProcessID: "7"}}
	if got := reportingMachineID(pid); got != "7" {
		t.Fatalf("pid %s", got)
	}
}
