package redis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"fluxguard/server/component/conn"
)

func TestHashSlot(t *testing.T) {
	if crc16([]byte("123456789")) != 0x31C3 {
		t.Fatalf("crc16=%x", crc16([]byte("123456789")))
	}
	if hashSlot("{user}.a") != hashSlot("{user}.b") {
		t.Fatal("哈希标签应落在同一槽")
	}
	if slot := hashSlot("foo"); slot < 0 || slot >= 16384 {
		t.Fatalf("槽位 %d", slot)
	}
}

func TestCheckRedisOp(t *testing.T) {
	if _, err := (probe{}).Exec(context.Background(), conn.Open{Address: "127.0.0.1:1"}, "flushall", nil); err == nil {
		t.Fatal("应拒绝未登记的命令")
	}
	if _, err := (probe{}).Exec(context.Background(), conn.Open{Address: "127.0.0.1:1"}, "delete", map[string]string{"name": ""}); err == nil {
		t.Fatal("应拒绝空键名")
	}
	if _, err := (probe{}).Exec(context.Background(), conn.Open{Address: "127.0.0.1:1", OptionText: "cluster:none"}, "scan", map[string]string{"database": "1"}); err == nil || !strings.Contains(err.Error(), "0 号库") {
		t.Fatal(err)
	}
	if _, err := (probe{}).Exec(context.Background(), conn.Open{Address: "127.0.0.1:1"}, "expire", map[string]string{"name": "a", "seconds": "-1"}); err == nil {
		t.Fatal("应拒绝负数过期时间")
	}
}

func TestRedisBrowse(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	addr, stop := serveRedis(t, func(cmd []string) string {
		mu.Lock()
		seen = append(seen, strings.Join(cmd, " "))
		mu.Unlock()
		switch cmd[0] {
		case "INFO":
			return respBulk("db0:keys=2,expires=1\n")
		case "CONFIG":
			return respBulks("databases", "16")
		case "SELECT":
			return respSimple("OK")
		case "SCAN":
			return respArray(respBulk("0"), respArray(respBulk("user:1"), respBulk("user:2")))
		case "TYPE":
			return respSimple("string")
		case "TTL":
			return respInt(90)
		case "STRLEN":
			return respInt(5)
		case "GET":
			return respBulk("hello")
		case "EXISTS":
			return respInt(1)
		case "DEL":
			return respInt(1)
		case "PERSIST":
			return respInt(1)
		default:
			return respErr("unexpected " + cmd[0])
		}
	})
	defer stop()
	target := conn.Open{Address: addr}
	listed, err := (probe{}).Exec(context.Background(), target, "databases", nil)
	if err != nil || !strings.Contains(listed, `"name":"db0"`) || !strings.Contains(listed, `"keys":2`) {
		t.Fatalf("%v %s", err, listed)
	}
	scanned, err := (probe{}).Exec(context.Background(), target, "scan", map[string]string{"database": "2", "match": "user:*"})
	if err != nil || !strings.Contains(scanned, "user:1") || !strings.Contains(scanned, `"cursor":"0"`) {
		t.Fatalf("%v %s", err, scanned)
	}
	mu.Lock()
	gotCmds := append([]string(nil), seen...)
	mu.Unlock()
	if !containsCmd(gotCmds, "SELECT 2") || !containsCmd(gotCmds, "SCAN 0 MATCH user:* COUNT 100") {
		t.Fatalf("未按库查找 %v", gotCmds)
	}
	got, err := (probe{}).Exec(context.Background(), target, "key", map[string]string{"name": "user:1"})
	if err != nil || !strings.Contains(got, `"text":"hello"`) || !strings.Contains(got, `"ttl":90`) {
		t.Fatalf("%v %s", err, got)
	}
	if _, err := (probe{}).Exec(context.Background(), target, "delete", map[string]string{"name": "user:1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := (probe{}).Exec(context.Background(), target, "expire", map[string]string{"name": "user:1", "seconds": "0"}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	gotCmds = append([]string(nil), seen...)
	mu.Unlock()
	if !containsCmd(gotCmds, "PERSIST user:1") || !containsCmd(gotCmds, "DEL user:1") {
		t.Fatalf("%v", gotCmds)
	}
}

func TestRedisClusterAndSentinel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	types := 0
	scans := 0
	go acceptRedis(ln, func(cmd []string) string {
		switch cmd[0] {
		case "CLUSTER":
			return respArray(respArray(respInt(0), respInt(16383), respArray(respBulk(host), respBulk(port))))
		case "INFO":
			return respBulk("db0:keys=3,expires=0\n")
		case "SENTINEL":
			return respBulks("ip", host, "port", port, "flags", "master", "num-slaves", "1", "num-sentinels", "1")
		case "SCAN":
			scans++
			if scans > 16 {
				return respArray(respBulk("0"), respArray(respBulk("b")))
			}
			return respArray(respBulk("4"), respArray(respBulk("a")))
		case "TYPE":
			types++
			if types == 1 {
				return respErr("MOVED 0 " + ln.Addr().String())
			}
			return respSimple("hash")
		case "TTL":
			return respInt(-1)
		case "HLEN":
			return respInt(1)
		case "HSCAN":
			return respArray(respBulk("0"), respArray(respBulk("field"), respBulk("value")))
		case "ASKING":
			return respSimple("OK")
		default:
			return respErr("unexpected " + cmd[0])
		}
	})
	target := conn.Open{Address: ln.Addr().String(), OptionText: "cluster:none"}
	listed, err := (probe{}).Exec(context.Background(), target, "databases", nil)
	if err != nil || !strings.Contains(listed, "集群只有 0 号库") || !strings.Contains(listed, `"keys":3`) {
		t.Fatalf("%v %s", err, listed)
	}
	first, err := (probe{}).Exec(context.Background(), target, "scan", nil)
	if err != nil || !strings.Contains(first, `"cursor":"4"`) || !strings.Contains(first, `"a"`) {
		t.Fatalf("%v %s", err, first)
	}
	var page redisScanResult
	if err := json.Unmarshal([]byte(first), &page); err != nil {
		t.Fatal(err)
	}
	second, err := (probe{}).Exec(context.Background(), target, "scan", map[string]string{"cursor": page.Cursor})
	if err != nil || !strings.Contains(second, `"b"`) || !strings.Contains(second, `"cursor":"0"`) {
		t.Fatalf("%v %s", err, second)
	}
	got, err := (probe{}).Exec(context.Background(), target, "key", map[string]string{"name": "field"})
	if err != nil || !strings.Contains(got, `"type":"hash"`) || !strings.Contains(got, `"value":"value"`) {
		t.Fatalf("%v %s", err, got)
	}

	sentinel := conn.Open{Address: ln.Addr().String(), OptionText: "sentinel:none", DatabaseName: "mymaster"}
	again, err := (probe{}).Exec(context.Background(), sentinel, "databases", nil)
	if err != nil || !strings.Contains(again, `"keys":3`) {
		t.Fatalf("哨兵 %v %s", err, again)
	}
}

func containsCmd(seen []string, want string) bool {
	for _, item := range seen {
		if item == want {
			return true
		}
	}
	return false
}

func serveRedis(t *testing.T, handle func(cmd []string) string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go acceptRedis(ln, handle)
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func acceptRedis(ln net.Listener, handle func(cmd []string) string) {
	for {
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		go func(raw net.Conn) {
			defer raw.Close()
			reader := bufio.NewReader(raw)
			for {
				cmd, err := readArray(reader)
				if err != nil {
					return
				}
				if _, err := io.WriteString(raw, handle(cmd)); err != nil {
					return
				}
			}
		}(raw)
	}
}

func respSimple(text string) string { return "+" + text + "\r\n" }
func respErr(text string) string    { return "-" + text + "\r\n" }
func respInt(n int) string          { return fmt.Sprintf(":%d\r\n", n) }
func respBulk(text string) string   { return fmt.Sprintf("$%d\r\n%s\r\n", len(text), text) }

func respArray(parts ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(parts))
	for _, part := range parts {
		b.WriteString(part)
	}
	return b.String()
}

func respBulks(values ...string) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = respBulk(value)
	}
	return respArray(parts...)
}
