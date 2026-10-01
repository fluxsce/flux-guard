package redis

import (
	"bufio"
	"context"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestParseInfo(t *testing.T) {
	sample := ParseInfo("redis_version:7.2.0\nrole:master\nused_memory:2048\nconnected_clients:4\ninstantaneous_ops_per_sec:8\nconnected_slaves:1\nuptime_in_seconds:10\nkeyspace_hits:3\nkeyspace_misses:1\nmem_fragmentation_ratio:1.6\ndb0:keys=4,expires=1,avg_ttl=0\ndb2:keys=8,expires=0,avg_ttl=0\n")
	if sample.Role != "master" || sample.UsedMemoryBytes != 2048 || sample.HitRate != 75 || sample.MemFragmentation != 1.6 || sample.KeyCount != 12 {
		t.Fatalf("%+v", sample)
	}
	if len(sample.DBs) != 2 || sample.DBs[0].Name != "db0" || sample.DBs[1].Keys != 8 {
		t.Fatalf("%+v", sample.DBs)
	}
	slow := ParseSlowlog([]any{[]any{"1", "170", "150000", []any{"KEYS", "*"}}})
	if len(slow) != 1 || slow[0].Usec != 150000 || slow[0].Command != "KEYS *" {
		t.Fatalf("%+v", slow)
	}
	if ParseInfo("role:master\n").HitRate != -1 {
		t.Fatal("没有命中统计时应为 -1")
	}
}

func TestParseCapacityAndReplication(t *testing.T) {
	text := strings.Join([]string{
		"role:master",
		"used_memory:800",
		"maxmemory:1000",
		"used_memory_peak:900",
		"blocked_clients:2",
		"instantaneous_input_kbps:3.5",
		"instantaneous_output_kbps:4",
		"evicted_keys:9",
		"expired_keys:8",
		"rejected_connections:7",
		"rdb_last_bgsave_status:ok",
		"aof_enabled:1",
		"aof_last_write_status:err",
		"slave0:ip=10.0.0.2,port=6379,state=online,offset=1,lag=12",
		"keyspace_hits:1",
		"keyspace_misses:1",
	}, "\n")
	sample := ParseInfo(text)
	if sample.MemUsagePercent != 80 || sample.UsedMemoryPeakBytes != 900 || sample.BlockedClients != 2 {
		t.Fatalf("容量 %+v", sample)
	}
	if sample.NetInKbps != 3.5 || sample.ReplicaLagSec != 12 || sample.MasterLinkDown != -1 {
		t.Fatalf("复制 %+v", sample)
	}
	if sample.RdbOk != 1 || sample.AofOk != 0 || sample.EvictedKeys != 9 || sample.HitRate != 50 {
		t.Fatalf("状态 %+v", sample)
	}
	down := ParseInfo("role:slave\nmaster_link_status:down\n")
	if down.MasterLinkDown != 1 || down.ReplicaLagSec != -1 {
		t.Fatalf("%+v", down)
	}
}

func TestApplyRates(t *testing.T) {
	now := time.Date(2026, 9, 30, 6, 0, 20, 0, time.UTC)
	sample := Sample{KeyspaceHits: 10, KeyspaceMisses: 10, EvictedKeys: 5, ExpiredKeys: 4, RejectedConnections: 2, UptimeSec: 100}
	prev := &prior{Hits: 8, Misses: 2, Evicted: 1, Expired: 0, Rejected: 2, Uptime: 80, At: now.Add(-20 * time.Second)}
	applyRates(&sample, prev, now)
	if math.Abs(sample.HitRate-20) > 0.001 || math.Abs(sample.EvictedPerSec-0.2) > 0.001 || math.Abs(sample.ExpiredPerSec-0.2) > 0.001 || sample.RejectedPerSec != 0 {
		t.Fatalf("%+v", sample)
	}
	restart := Sample{KeyspaceHits: 1, KeyspaceMisses: 0, UptimeSec: 5}
	applyRates(&restart, prev, now)
	if restart.HitRate != -1 {
		t.Fatalf("重启后不应沿用旧计数 %+v", restart)
	}
}

func TestSaveIntervalHitRate(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	status := models.Status{Up: true, Version: "7.2.4", Role: "master"}
	first := Sample{KeyspaceHits: 1, KeyspaceMisses: 1, EvictedKeys: 1, UptimeSec: 10, HitRate: 50}
	if err := save(ctx, db, "t1", "c1", first, status); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	second := Sample{KeyspaceHits: 3, KeyspaceMisses: 1, EvictedKeys: 1, UptimeSec: 40, HitRate: 75}
	if err := save(ctx, db, "t1", "c1", second, status); err != nil {
		t.Fatal(err)
	}
	var hit any
	if err := store.Scan(ctx, db, `
		SELECT hitRate FROM FG_REDIS_METRIC
		WHERE tenantId = @tenantId AND componentId = @componentId
		ORDER BY observedTime DESC, pointId DESC LIMIT 1`,
		dbx.Args{"tenantId": "t1", "componentId": "c1"}, &hit); err != nil {
		t.Fatal(err)
	}
	got, ok := hit.(float64)
	if !ok {
		t.Fatalf("%T %v", hit, hit)
	}
	if math.Abs(got-100) > 0.001 {
		t.Fatalf("区间命中率=%v，应为 100", got)
	}
}

func TestParseClusterAndSentinel(t *testing.T) {
	state, nodes, fail, size := ParseCluster("cluster_state:fail\ncluster_known_nodes:6\ncluster_slots_fail:2\ncluster_size:3\n")
	if state != "fail" || nodes != 6 || fail != 2 || size != 3 {
		t.Fatalf("%s %d %d %d", state, nodes, fail, size)
	}
	view := ParseSentinel([]string{"ip", "10.0.0.1", "port", "6379", "flags", "master,s_down", "num-slaves", "1", "num-sentinels", "3"})
	if !view.Down || view.IP != "10.0.0.1" || view.Slaves != 1 || view.Sentinels != 3 {
		t.Fatalf("%+v", view)
	}
}

func TestCollectLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		if _, err := readArray(bufio.NewReader(raw)); err != nil {
			return
		}
		info := "redis_version:7.2.0\r\nrole:slave\r\nused_memory:1024\r\nconnected_clients:1\r\ninstantaneous_ops_per_sec:1\r\nconnected_slaves:0\r\nuptime_in_seconds:2\r\n"
		_, _ = raw.Write([]byte("$" + strconv.Itoa(len(info)) + "\r\n" + info + "\r\n"))
	}()
	_, status := collect(context.Background(), conn.Open{Address: ln.Addr().String()})
	if !status.Up || !strings.Contains(status.Summary, "slave") {
		t.Fatalf("%+v", status)
	}
}

func readArray(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		head, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(head, "$")))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		out = append(out, string(buf[:size]))
	}
	return out, nil
}
