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
	mongo, err := Normalize("mongo", "standalone:password", "app", "secret", "", false)
	if err != nil || mongo.Option != "standalone:scram256" || mongo.User != "app" || mongo.Database != "admin" || mongo.ClearSecret {
		t.Fatalf("%+v %v", mongo, err)
	}
	sha1, err := Normalize("mongo", "standalone:scram1", "app", "", "appdb", false)
	if err != nil || sha1.Option != "standalone:scram1" || sha1.Database != "appdb" {
		t.Fatalf("%+v %v", sha1, err)
	}
	if _, err := Normalize("mongo", "standalone:scram256", "", "secret", "", true); err == nil {
		t.Fatal("MongoDB 认证缺少用户名应拒绝")
	}
	clearedMongo, err := Normalize("mongo", "standalone:none", "app", "secret", "admin", true)
	if err != nil || !clearedMongo.ClearSecret || clearedMongo.User != "" || clearedMongo.Option != "standalone:none" {
		t.Fatalf("%+v %v", clearedMongo, err)
	}
	ldap, err := Normalize("mongo", "tls:plain", "app", "secret", "", true)
	if err != nil || ldap.Option != "tls:plain" || ldap.Database != "$external" {
		t.Fatalf("%+v %v", ldap, err)
	}
	cert, err := Normalize("mongo", "standalone:x509", "", "-----BEGIN CERTIFICATE-----", "", true)
	if err != nil || cert.Option != "tls:x509" || cert.Database != "$external" || cert.ClearSecret {
		t.Fatalf("%+v %v", cert, err)
	}
	aws, err := Normalize("mongo", "tls:aws", "AKIA", "secret", "admin", true)
	if err != nil || aws.Option != "tls:aws" || aws.Database != "$external" || aws.User != "AKIA" {
		t.Fatalf("%+v %v", aws, err)
	}
	if _, err := Normalize("mongo", "udp:scram256", "app", "secret", "", true); err == nil {
		t.Fatal("未知传输应拒绝")
	}
}
