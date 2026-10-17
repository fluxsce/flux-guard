package core

import "strings"
import "testing"

func TestDescribeLifecycleTomcatCommand(t *testing.T) {
	rt := Runtime{
		Kind: "command", Mode: "tomcat", KillAuto: true,
		Base: "/data/vue_test", Start: "/data/vue_test/bin/startup.sh",
	}
	got := DescribeLifecycle(rt, "restart")
	if !strings.Contains(got, "未配置停止脚本") || !strings.Contains(got, "-Dcatalina.base=/data/vue_test") || !strings.Contains(got, "startup.sh") {
		t.Fatalf("%s", got)
	}
	withStop := rt
	withStop.Stop = "/data/vue_test/bin/shutdown.sh"
	got = DescribeLifecycle(withStop, "restart")
	if !strings.Contains(got, "shutdown.sh") || !strings.Contains(got, "然后执行启动脚本") {
		t.Fatalf("%s", got)
	}
	scripted := rt
	scripted.Restart = "/data/vue_test/bin/restart.sh"
	got = DescribeLifecycle(scripted, "restart")
	if !strings.Contains(got, "只执行重启脚本") || strings.Contains(got, "强杀") {
		t.Fatalf("%s", got)
	}
}

func TestDescribeLifecycleSystemd(t *testing.T) {
	rt := Runtime{Kind: "systemd", Service: "tomcat"}
	if DescribeLifecycle(rt, "restart") != "systemctl restart tomcat" {
		t.Fatal(DescribeLifecycle(rt, "restart"))
	}
	if DescribeLifecycle(rt, "stop") != "systemctl stop tomcat" {
		t.Fatal(DescribeLifecycle(rt, "stop"))
	}
}
