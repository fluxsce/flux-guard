package netx

import "testing"

func TestHostPort(t *testing.T) {
	got, err := HostPort("127.0.0.1", 6379)
	if err != nil || got != "127.0.0.1:6379" {
		t.Fatalf("got %s %v", got, err)
	}
	if _, err := HostPort("10.0.0.1:1,10.0.0.2:1", 6379); err == nil {
		t.Fatal("单个地址不应接受逗号")
	}
	list, err := HostPorts("kafka:9092, 10.0.0.2", 9092)
	if err != nil || len(list) != 2 || list[1] != "10.0.0.2:9092" {
		t.Fatalf("%v %v", list, err)
	}
	base, err := BaseURL("nacos.local", 8848)
	if err != nil || base != "http://nacos.local:8848" {
		t.Fatalf("%s %v", base, err)
	}
}
