package profile

import "testing"

func TestRedisModes(t *testing.T) {
	spec, err := Normalize("redis", "sentinel:acl", "app", "secret", "mymaster", true)
	if err != nil || spec.Option != "sentinel:acl" || spec.User != "app" || spec.Database != "mymaster" {
		t.Fatalf("%+v %v", spec, err)
	}
	if _, err := Normalize("redis", "sentinel:password", "", "secret", "", true); err == nil {
		t.Fatal("哨兵缺少主节点名应拒绝")
	}
	cleared, err := Normalize("redis", "standalone:none", "old", "secret", "0", true)
	if err != nil || !cleared.ClearSecret || cleared.User != "" {
		t.Fatalf("%+v %v", cleared, err)
	}
	if _, err := Normalize("mysql", "server:password", "", "secret", "", true); err == nil {
		t.Fatal("MySQL 缺少账号应拒绝")
	}
	kafka, err := Normalize("kafka", "tls:scram512", "app", "secret", "", true)
	if err != nil || kafka.Option != "tls:scram512" || kafka.User != "app" || kafka.ClearSecret {
		t.Fatalf("%+v %v", kafka, err)
	}
	if _, err := Normalize("kafka", "cluster:plain", "", "secret", "", true); err == nil {
		t.Fatal("Kafka 认证缺少用户名应拒绝")
	}
}
