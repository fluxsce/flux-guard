package mongo

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeServerStatus(t *testing.T) {
	doc := bsonDoc(
		bsonDouble("ok", 1),
		bsonString("version", "7.0.0"),
		bsonSub("connections", bsonDoc(bsonInt32("current", 3), bsonInt32("available", 9))),
		bsonSub("opcounters", bsonDoc(bsonInt32("insert", 1), bsonInt32("query", 2), bsonInt32("update", 3), bsonInt32("delete", 4))),
		bsonSub("repl", bsonDoc(bsonBool("ismaster", true))),
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
}

func bsonDouble(name string, v float64) []byte {
	out := append([]byte{0x01}, cstring(name)...)
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], math.Float64bits(v))
	return append(out, n[:]...)
}

func bsonBool(name string, v bool) []byte {
	bit := byte(0)
	if v {
		bit = 1
	}
	return append(append([]byte{0x08}, cstring(name)...), bit)
}

func bsonSub(name string, doc []byte) []byte {
	return append(append([]byte{0x03}, cstring(name)...), doc...)
}
