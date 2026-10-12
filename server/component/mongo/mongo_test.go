package mongo

import (
	"context"
	"encoding/binary"
	"math"
	"testing"
	"time"

	"fluxguard/server/component/models"
	"fluxguard/server/component/row"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func TestDecodeServerStatus(t *testing.T) {
	doc := bsonDoc(
		bsonDouble("ok", 1),
		bsonString("version", "7.0.0"),
		bsonDecimal("ignored"),
		bsonSub("connections", bsonDoc(bsonInt32("current", 3), bsonInt32("available", 9))),
		bsonSub("opcounters", bsonDoc(bsonInt64("insert", 1), bsonInt32("query", 2), bsonInt32("update", 3), bsonInt32("delete", 4))),
		bsonSub("repl", bsonDoc(bsonBool("ismaster", true), bsonTimestamp("ts", 1_700_000_000))),
	)
	got, err := decodeDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if text(dig(got, "version")) != "7.0.0" || int64(num(dig(got, "connections", "current"))) != 3 {
		t.Fatalf("%v", got)
	}
	if !truthy(dig(got, "repl", "ismaster")) || int64(num(dig(got, "opcounters", "delete"))) != 4 {
		t.Fatalf("%v", got)
	}
	if int64(num(dig(got, "repl", "ts"))) != 1_700_000_000 {
		t.Fatalf("timestamp %v", dig(got, "repl", "ts"))
	}
}

func TestSampleFromStatus(t *testing.T) {
	doc := map[string]any{
		"ok": float64(1), "version": "7.0.14", "process": "mongod", "uptime": float64(100),
		"connections": map[string]any{"current": int32(4), "available": int32(96), "active": int32(1)},
		"opcounters":  map[string]any{"insert": int32(1), "query": int32(2), "update": int32(3), "delete": int32(4), "getmore": int32(5), "command": int32(6)},
		"network":     map[string]any{"bytesIn": int64(2048), "bytesOut": int64(4096)},
		"asserts":     map[string]any{"regular": int32(1), "warning": int32(0), "msg": int32(0), "user": int32(1)},
		"mem":         map[string]any{"resident": int32(32)},
		"globalLock":  map[string]any{"currentQueue": map[string]any{"readers": int32(0), "writers": int32(2)}},
		"wiredTiger": map[string]any{
			"cache": map[string]any{
				"bytes currently in the cache":     float64(500),
				"maximum bytes configured":         float64(1000),
				"tracked dirty bytes in the cache": float64(200),
			},
			"concurrentTransactions": map[string]any{
				"read":  map[string]any{"available": int32(128)},
				"write": map[string]any{"available": int32(0)},
			},
		},
		"repl": map[string]any{"setName": "rs0", "isWritablePrimary": true, "me": "a:27017"},
	}
	sample := sampleFrom(doc)
	if sample.Role != "primary" || sample.SetName != "rs0" || math.Abs(sample.ConnUsagePercent-4) > 0.001 {
		t.Fatalf("%+v", sample)
	}
	if math.Abs(sample.CacheDirtyPercent-20) > 0.001 || sample.ResidentBytes != 32*1024*1024 {
		t.Fatalf("%+v", sample)
	}
	if sample.WriteTickets != 0 || sample.ReadTickets != 128 || sample.QueuedWriters != 2 || sample.Asserts != 2 || !sample.HasOps {
		t.Fatalf("%+v", sample)
	}
	if sample.OpQuery != 2 || sample.NetIn != 2048 || sample.UptimeSec != 100 {
		t.Fatalf("%+v", sample)
	}
}

func TestReplicaLag(t *testing.T) {
	primary := int64(1_700_000_000_000)
	sample := newSample()
	sample.Role = "primary"
	readReplica(&sample, map[string]any{
		"ok": float64(1), "set": "rs0",
		"members": map[string]any{
			"0": map[string]any{"name": "a:27017", "stateStr": "PRIMARY", "health": float64(1), "self": true, "optimeDate": primary},
			"1": map[string]any{"name": "b:27017", "stateStr": "SECONDARY", "health": float64(1), "optimeDate": primary - 12_000},
			"2": map[string]any{"name": "c:27017", "stateStr": "DOWN", "health": float64(0)},
		},
	}, "a:27017")
	if sample.ReplicaLagSec != 12 || sample.MemberCount != 3 || sample.UnhealthyMembers != 1 || sample.SetName != "rs0" {
		t.Fatalf("%+v", sample)
	}
	if !sample.Members[0].Self || sample.Members[0].Lag != 0 || sample.Members[1].Lag != 12 || sample.Members[2].Lag != -1 {
		t.Fatalf("%+v", sample.Members)
	}
}

func TestRunningFiltersSubsecond(t *testing.T) {
	sample := newSample()
	readRunning(&sample, map[string]any{
		"ok": float64(1),
		"inprog": map[string]any{
			"0": map[string]any{"op": "query", "ns": "app.orders", "secs_running": int32(3), "client": "10.0.0.8:1"},
			"1": map[string]any{"op": "insert", "ns": "app.orders", "secs_running": float64(0.2)},
			"2": map[string]any{"op": "update", "ns": "app.orders", "microsecs_running": float64(2_500_000)},
		},
	})
	if sample.SlowOps != 2 || len(sample.Running) != 2 || sample.Running[0].Secs != 3 || sample.Running[1].Op != "update" {
		t.Fatalf("%+v %v", sample.SlowOps, sample.Running)
	}
}

func TestApplyRates(t *testing.T) {
	now := time.Now()
	prev := &prior{
		Insert: 10, Query: 100, Update: 0, Delete: 0, Getmore: -1, Command: 5,
		NetIn: 1024, NetOut: 2048, Asserts: 1, Uptime: 100, At: now.Add(-10 * time.Second),
	}
	sample := Sample{
		HasOps: true, HasNet: true, HasAsserts: true, UptimeSec: 110,
		OpInsert: 20, OpQuery: 100, OpUpdate: 0, OpDelete: 0, OpGetmore: 50, OpCommand: 15,
		NetIn: 2048, NetOut: 2048, Asserts: 1,
	}
	applyRates(&sample, prev, now)
	if math.Abs(sample.OpInsertPerSec-1) > 0.001 || sample.OpQueryPerSec != 0 || sample.OpGetmorePerSec != -1 {
		t.Fatalf("%+v", sample)
	}
	if math.Abs(sample.OpsPerSec-2) > 0.001 || math.Abs(sample.NetInKbps-0.1) > 0.001 || sample.NetOutKbps != 0 || sample.AssertPerSec != 0 {
		t.Fatalf("%+v", sample)
	}
	restart := sample
	restart.UptimeSec = 5
	applyRates(&restart, prev, now)
	if restart.OpsPerSec != -1 {
		t.Fatalf("restart should not invent a rate: %v", restart.OpsPerSec)
	}
}

func TestSaveMongoSample(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := store.OpenTemp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	sample := newSample()
	sample.ConnectionsCurrent = 4
	sample.ConnectionsAvailable = 96
	sample.CacheUsedBytes = 4096
	sample.HasOps = true
	sample.OpQuery = 10
	sample.UptimeSec = 100
	sample.Role = "primary"
	sample.Version = "7.0.14"
	sample.SlowOps = 0
	status := models.Status{Up: true, Version: "7.0.14", Role: "primary", Summary: "主节点 · 连接 4"}
	if err := save(ctx, db, "default", "c1", sample, status); err != nil {
		t.Fatal(err)
	}
	var current, cache, ops any
	if err := store.Scan(ctx, db, `
		SELECT connectionsCurrent, cacheUsedBytes, opsPerSec
		FROM FG_COMPONENT_MONGO
		WHERE tenantId = @tenantId AND componentId = @componentId`,
		dbx.Args{"tenantId": "default", "componentId": "c1"}, &current, &cache, &ops); err != nil {
		t.Fatal(err)
	}
	if row.Float(current) != 4 || row.Float(cache) != 4096 || row.Float(ops) != -1 {
		t.Fatalf("current=%v cache=%v ops=%v", current, cache, ops)
	}
}

func bsonTimestamp(name string, sec uint32) []byte {
	out := append([]byte{0x11}, cstring(name)...)
	var n [8]byte
	binary.LittleEndian.PutUint32(n[4:], sec)
	return append(out, n[:]...)
}

func bsonDecimal(name string) []byte {
	return append(append([]byte{0x13}, cstring(name)...), make([]byte, 16)...)
}
