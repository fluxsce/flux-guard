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
	if got := reportingMachineID(java); got != "java@box" {
		t.Fatalf("java machine %s", got)
	}
	restart := protocol.Request{Java: &protocol.Java{AppName: "datahub", HostName: "box", ProcessID: "99"}}
	again := protocol.Request{Java: &protocol.Java{AppName: "datahub", HostName: "box", ProcessID: "100"}}
	if reportingMachineID(restart) != "datahub@box" || reportingMachineID(restart) != reportingMachineID(again) {
		t.Fatalf("restart id %s / %s", reportingMachineID(restart), reportingMachineID(again))
	}
	named := protocol.Request{Java: &protocol.Java{AppName: "datahub", InstanceName: "datahub-2", HostName: "box", ProcessID: "7"}}
	if got := reportingMachineID(named); got != "datahub@datahub-2" {
		t.Fatalf("instance %s", got)
	}
	pod := protocol.Request{Java: &protocol.Java{AppName: "datahub", HostName: "pod-b", ProcessID: "3"}}
	if reportingMachineID(pod) == reportingMachineID(restart) {
		t.Fatal("cluster instance collapsed")
	}
	pid := protocol.Request{Java: &protocol.Java{ProcessID: "7"}}
	if got := reportingMachineID(pid); got != "7" {
		t.Fatalf("pid %s", got)
	}
}
