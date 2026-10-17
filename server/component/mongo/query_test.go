package mongo

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"fluxguard/server/component/conn"
)

func TestQueryFilter(t *testing.T) {
	doc, err := bsonFromJSON(`{"status":"open","n":{"$gt":1},"_id":{"$oid":"00112233445566778899aabb"}}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if text(dig(got, "status")) != "open" || int64(num(dig(got, "n", "$gt"))) != 1 {
		t.Fatalf("%v", got)
	}
	if text(dig(got, "_id")) != "00112233445566778899aabb" {
		t.Fatalf("oid %v", dig(got, "_id"))
	}
	if _, err := bsonFromJSON(`{"$where":"this.a"}`); err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatal(err)
	}
	if _, err := (probe{}).Exec(context.Background(), conn.Open{Address: "127.0.0.1:1", OptionText: "standalone:none"}, "find", map[string]string{
		"database": "app", "collection": "orders", "limit": "80",
	}); err == nil || !strings.Contains(err.Error(), "1 到 50") {
		t.Fatal(err)
	}
}

func TestMongoQuery(t *testing.T) {
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
			go serveMongoQuery(c)
		}
	}()
	target := conn.Open{Address: ln.Addr().String(), OptionText: "standalone:none"}
	ctx := context.Background()
	raw, err := (probe{}).Exec(ctx, target, "find", map[string]string{"database": "app", "collection": "orders", "filter": `{"status":"open"}`, "limit": "2"})
	if err != nil {
		t.Fatal(err)
	}
	var found mongoFind
	if err := json.Unmarshal([]byte(raw), &found); err != nil {
		t.Fatal(err)
	}
	if len(found.Documents) != 1 || !strings.Contains(found.Documents[0], `"status":"open"`) || !strings.Contains(found.Documents[0], `"_id":"00112233445566778899aabb"`) {
		t.Fatalf("%+v", found)
	}
	raw, err = (probe{}).Exec(ctx, target, "count", map[string]string{"database": "app", "collection": "orders"})
	if err != nil {
		t.Fatal(err)
	}
	var counted mongoCount
	if err := json.Unmarshal([]byte(raw), &counted); err != nil || counted.Count != 3 {
		t.Fatalf("%s %v", raw, err)
	}
	raw, err = (probe{}).Exec(ctx, target, "explain", map[string]string{"database": "app", "collection": "orders", "filter": `{"status":"open"}`})
	if err != nil {
		t.Fatal(err)
	}
	var plan mongoExplain
	if err := json.Unmarshal([]byte(raw), &plan); err != nil || plan.Stage != "IXSCAN" || plan.Index != "status_1" || plan.DocsExamined != 1 || plan.KeysExamined != 1 {
		t.Fatalf("%s %v", raw, err)
	}
	raw, err = (probe{}).Exec(ctx, target, "profile", map[string]string{"database": "app"})
	if err != nil {
		t.Fatal(err)
	}
	var slow mongoProfile
	if err := json.Unmarshal([]byte(raw), &slow); err != nil || len(slow.Entries) != 1 || slow.Entries[0].Millis != 40 || slow.Entries[0].NS != "app.orders" {
		t.Fatalf("%s %v", raw, err)
	}
}

func serveMongoQuery(c net.Conn) {
	defer c.Close()
	for {
		doc, err := readCmd(c)
		if err != nil {
			return
		}
		body := bsonDoc(bsonDouble("ok", 0), bsonString("errmsg", "unexpected "+cmdNames(doc)))
		switch {
		case text(dig(doc, "find")) == "orders":
			if text(dig(doc, "filter", "status")) != "open" {
				break
			}
			body = bsonDoc(bsonDouble("ok", 1), bsonSub("cursor", bsonDoc(
				bsonInt64("id", 0),
				bsonString("ns", "app.orders"),
				bsonArray("firstBatch", bsonDoc(
					bsonObjectIDMust("_id", "00112233445566778899aabb"),
					bsonString("status", "open"),
					bsonArray("tags", bsonDoc(bsonString("0", "a"), bsonString("1", "b"))),
				)),
			)))
		case text(dig(doc, "count")) == "orders":
			body = bsonDoc(bsonDouble("ok", 1), bsonInt32("n", 3))
		case dig(doc, "explain") != nil:
			body = bsonDoc(bsonDouble("ok", 1), bsonSub("executionStats", bsonDoc(
				bsonInt32("nReturned", 1),
				bsonInt32("totalDocsExamined", 1),
				bsonInt32("totalKeysExamined", 1),
				bsonInt32("executionTimeMillis", 2),
				bsonSub("executionStages", bsonDoc(
					bsonString("stage", "IXSCAN"),
					bsonString("indexName", "status_1"),
				)),
			)))
		case text(dig(doc, "find")) == "system.profile":
			body = bsonDoc(bsonDouble("ok", 1), bsonSub("cursor", bsonDoc(
				bsonInt64("id", 0),
				bsonString("ns", "app.system.profile"),
				bsonArray("firstBatch", bsonDoc(
					bsonString("op", "query"),
					bsonString("ns", "app.orders"),
					bsonInt32("millis", 40),
					bsonString("planSummary", "IXSCAN"),
				)),
			)))
		}
		if err := writeCmd(c, body); err != nil {
			return
		}
	}
}

func bsonObjectIDMust(name, hexText string) []byte {
	raw, err := bsonObjectID(name, hexText)
	if err != nil {
		panic(err)
	}
	return raw
}
