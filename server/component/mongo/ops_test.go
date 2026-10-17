package mongo

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"fluxguard/server/component/conn"
)

func TestIndexKeyOrder(t *testing.T) {
	doc := bsonDoc(bsonSub("key", bsonDoc(bsonInt32("status", 1), bsonInt32("created", -1))))
	got, err := decodeDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if keyText(dig(got, "key")) != "status:1, created:-1" {
		t.Fatalf("%s", keyText(dig(got, "key")))
	}
}

func TestMongoOpRejected(t *testing.T) {
	target := conn.Open{Address: "127.0.0.1:1", OptionText: "standalone:none"}
	cases := []struct {
		command string
		args    map[string]string
		text    string
	}{
		{"dropDatabase", map[string]string{"database": "admin"}, "不能删除"},
		{"dropCollection", map[string]string{"database": "app", "collection": "system.profile"}, "不能操作系统集合"},
		{"createIndex", map[string]string{"database": "app", "collection": "orders", "keys": "a:1,b:-1", "ttl": "60"}, "过期索引只能有一个字段"},
		{"createIndex", map[string]string{"database": "app", "collection": "orders", "keys": "a:1", "unique": "yes"}, "开关只能是 Y 或 N"},
		{"killOp", map[string]string{"op": "0"}, "操作编号无效"},
		{"dropIndex", map[string]string{"database": "app", "collection": "orders", "name": "_id_"}, "不能删除 _id_"},
		{"shell", nil, "这种组件没有这项操作"},
	}
	for _, item := range cases {
		_, err := (probe{}).Exec(context.Background(), target, item.command, item.args)
		if err == nil || !strings.Contains(err.Error(), item.text) {
			t.Fatalf("%s: %v", item.command, err)
		}
	}
}

func TestMongoExec(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveMongoOp(c)
		}
	}()
	target := conn.Open{Address: ln.Addr().String(), OptionText: "standalone:none"}
	ctx := context.Background()
	probe := probe{}

	raw, err := probe.Exec(ctx, target, "databases", nil)
	if err != nil {
		t.Fatal(err)
	}
	var databases mongoDatabases
	if err := json.Unmarshal([]byte(raw), &databases); err != nil {
		t.Fatal(err)
	}
	if len(databases.Databases) != 2 || databases.Databases[0].Name != "app" || databases.Databases[0].Bytes != 4096 {
		t.Fatalf("%+v", databases)
	}

	raw, err = probe.Exec(ctx, target, "collections", map[string]string{"database": "app"})
	if err != nil {
		t.Fatal(err)
	}
	var collections mongoCollections
	if err := json.Unmarshal([]byte(raw), &collections); err != nil {
		t.Fatal(err)
	}
	if len(collections.Collections) != 2 || collections.Collections[0].Name != "orders" || collections.Collections[1].Name != "system.profile" {
		t.Fatalf("%+v", collections)
	}

	raw, err = probe.Exec(ctx, target, "stats", map[string]string{"database": "app", "collection": "orders"})
	if err != nil {
		t.Fatal(err)
	}
	var stats mongoStats
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Count != 3 || stats.NIndexes != 2 || stats.NS != "app.orders" {
		t.Fatalf("%+v", stats)
	}

	raw, err = probe.Exec(ctx, target, "indexes", map[string]string{"database": "app", "collection": "orders"})
	if err != nil {
		t.Fatal(err)
	}
	var indexes mongoIndexes
	if err := json.Unmarshal([]byte(raw), &indexes); err != nil {
		t.Fatal(err)
	}
	if len(indexes.Indexes) != 2 || indexes.Indexes[0].Name != "_id_" || indexes.Indexes[1].Key != "status:1, created:-1" || !indexes.Indexes[1].Unique {
		t.Fatalf("%+v", indexes)
	}

	for _, item := range []struct {
		command string
		args    map[string]string
	}{
		{"createDatabase", map[string]string{"database": "app", "collection": "orders"}},
		{"dropDatabase", map[string]string{"database": "app"}},
		{"dropCollection", map[string]string{"database": "app", "collection": "orders"}},
		{"renameCollection", map[string]string{"database": "app", "collection": "orders", "name": "sales"}},
		{"createIndex", map[string]string{"database": "app", "collection": "orders", "keys": "status:1, created:-1", "name": "status_1", "unique": "Y"}},
		{"dropIndex", map[string]string{"database": "app", "collection": "orders", "name": "status_1"}},
		{"killOp", map[string]string{"op": "42"}},
	} {
		raw, err = probe.Exec(ctx, target, item.command, item.args)
		if err != nil {
			t.Fatalf("%s: %v", item.command, err)
		}
		var ack mongoAck
		if err := json.Unmarshal([]byte(raw), &ack); err != nil || !ack.OK {
			t.Fatalf("%s: %s %v", item.command, raw, err)
		}
	}
}

func serveMongoOp(c net.Conn) {
	defer c.Close()
	for {
		doc, err := readCmd(c)
		if err != nil {
			return
		}
		body := bsonDoc(bsonDouble("ok", 0), bsonString("errmsg", "unexpected "+cmdNames(doc)))
		switch {
		case num(dig(doc, "listDatabases")) == 1:
			body = bsonDoc(
				bsonDouble("ok", 1),
				bsonArray("databases",
					bsonDoc(bsonString("name", "app"), bsonDouble("sizeOnDisk", 4096), bsonBool("empty", false)),
					bsonDoc(bsonString("name", "admin"), bsonInt32("sizeOnDisk", 100)),
				),
			)
		case num(dig(doc, "listCollections")) == 1:
			body = bsonDoc(bsonDouble("ok", 1), bsonSub("cursor", bsonDoc(
				bsonInt64("id", 9),
				bsonString("ns", "app.$cmd.listCollections"),
				bsonArray("firstBatch", bsonDoc(bsonString("name", "orders"), bsonString("type", "collection"))),
			)))
		case intOfMust(dig(doc, "getMore")) == 9:
			if text(dig(doc, "collection")) != "$cmd.listCollections" {
				body = bsonDoc(bsonDouble("ok", 0), bsonString("errmsg", "wrong collection"))
				break
			}
			body = bsonDoc(bsonDouble("ok", 1), bsonSub("cursor", bsonDoc(
				bsonInt64("id", 0),
				bsonString("ns", "app.$cmd.listCollections"),
				bsonArray("nextBatch", bsonDoc(bsonString("name", "system.profile"), bsonString("type", "collection"))),
			)))
		case text(dig(doc, "collStats")) == "orders":
			body = bsonDoc(
				bsonDouble("ok", 1), bsonString("ns", "app.orders"),
				bsonInt32("count", 3), bsonInt32("size", 120), bsonInt32("avgObjSize", 40),
				bsonInt32("storageSize", 4096), bsonInt32("nindexes", 2), bsonInt32("totalIndexSize", 8192),
			)
		case text(dig(doc, "listIndexes")) == "orders":
			body = bsonDoc(bsonDouble("ok", 1), bsonSub("cursor", bsonDoc(
				bsonInt64("id", 0),
				bsonString("ns", "app.orders"),
				bsonArray("firstBatch",
					bsonDoc(
						bsonString("name", "status_1"),
						bsonSub("key", bsonDoc(bsonInt32("status", 1), bsonInt32("created", -1))),
						bsonBool("unique", true),
					),
					bsonDoc(bsonString("name", "_id_"), bsonSub("key", bsonDoc(bsonInt32("_id", 1)))),
				),
			)))
		case text(dig(doc, "create")) == "orders" && text(dig(doc, "$db")) == "app":
			body = bsonDoc(bsonDouble("ok", 1))
		case num(dig(doc, "dropDatabase")) == 1 && text(dig(doc, "$db")) == "app":
			body = bsonDoc(bsonDouble("ok", 1))
		case text(dig(doc, "drop")) == "orders" && text(dig(doc, "$db")) == "app":
			body = bsonDoc(bsonDouble("ok", 1))
		case text(dig(doc, "renameCollection")) == "app.orders" && text(dig(doc, "to")) == "app.sales" && !truthy(dig(doc, "dropTarget")):
			body = bsonDoc(bsonDouble("ok", 1))
		case text(dig(doc, "createIndexes")) == "orders":
			batch := asSlice(dig(doc, "indexes"))
			if len(batch) == 1 {
				item, _ := batch[0].(map[string]any)
				if keyText(dig(item, "key")) == "status:1, created:-1" && text(dig(item, "name")) == "status_1" && truthy(dig(item, "unique")) {
					body = bsonDoc(bsonDouble("ok", 1))
				}
			}
		case text(dig(doc, "dropIndexes")) == "orders" && text(dig(doc, "index")) == "status_1":
			body = bsonDoc(bsonDouble("ok", 1))
		case num(dig(doc, "killOp")) == 1 && intOfMust(dig(doc, "op")) == 42:
			body = bsonDoc(bsonDouble("ok", 1))
		}
		if err := writeCmd(c, body); err != nil {
			return
		}
	}
}

func cmdNames(doc map[string]any) string {
	names := make([]string, 0, len(doc))
	for key := range doc {
		if key == "\x00" || key == "$db" {
			continue
		}
		names = append(names, key)
	}
	return strings.Join(names, ",")
}
