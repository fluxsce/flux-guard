package mongo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"fluxguard/server/component/row"
)

const queryDocCap = 4000

func queryFilter(raw string) error {
	if _, err := bsonFromJSON(raw); err != nil {
		return err
	}
	return nil
}

func queryLimit(raw string) (int32, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 50 {
		return 0, fmt.Errorf("条数要在 1 到 50 之间")
	}
	return int32(n), nil
}

type mongoFind struct {
	Documents []string `json:"documents"`
	Note      string   `json:"note,omitempty"`
}

func opFind(raw net.Conn, db, coll, filter string, limit int32) (mongoFind, error) {
	filterDoc, err := bsonFromJSON(filter)
	if err != nil {
		return mongoFind{}, err
	}
	doc, err := command(raw, bsonDoc(
		bsonString("find", coll),
		bsonSub("filter", filterDoc),
		bsonInt32("limit", limit),
		bsonInt32("batchSize", limit),
		bsonBool("singleBatch", true),
		bsonString("$db", db),
	))
	if err != nil {
		return mongoFind{}, err
	}
	batch, truncated, err := readBatches(context.Background(), raw, doc, db, coll, int(limit))
	if err != nil {
		return mongoFind{}, err
	}
	out := make([]string, 0, len(batch))
	note := ""
	for _, item := range batch {
		text, err := orderedJSON(item)
		if err != nil {
			return mongoFind{}, err
		}
		if len(text) > queryDocCap {
			text = text[:queryDocCap] + "…"
			note = "有的文档很长，这里只保留开头。"
		}
		out = append(out, text)
	}
	if truncated && note == "" {
		note = fmt.Sprintf("这里只列出前 %d 条。", limit)
	}
	return mongoFind{Documents: out, Note: note}, nil
}

type mongoCount struct {
	Count int64 `json:"count"`
}

func opCount(raw net.Conn, db, coll, filter string) (mongoCount, error) {
	filterDoc, err := bsonFromJSON(filter)
	if err != nil {
		return mongoCount{}, err
	}
	doc, err := command(raw, bsonDoc(
		bsonString("count", coll),
		bsonSub("query", filterDoc),
		bsonString("$db", db),
	))
	if err != nil {
		return mongoCount{}, err
	}
	return mongoCount{Count: int64(num(dig(doc, "n")))}, nil
}

type mongoExplain struct {
	NReturned    int64  `json:"nReturned"`
	DocsExamined int64  `json:"docsExamined"`
	KeysExamined int64  `json:"keysExamined"`
	Millis       int64  `json:"millis"`
	Stage        string `json:"stage,omitempty"`
	Index        string `json:"index,omitempty"`
}

func opExplain(raw net.Conn, db, coll, filter string) (mongoExplain, error) {
	filterDoc, err := bsonFromJSON(filter)
	if err != nil {
		return mongoExplain{}, err
	}
	findDoc := bsonDoc(bsonString("find", coll), bsonSub("filter", filterDoc), bsonInt32("limit", 1))
	doc, err := command(raw, bsonDoc(
		bsonSub("explain", findDoc),
		bsonString("verbosity", "executionStats"),
		bsonString("$db", db),
	))
	if err != nil {
		return mongoExplain{}, err
	}
	stats := dig(doc, "executionStats")
	stage := text(dig(stats, "executionStages", "stage"))
	index := text(dig(stats, "executionStages", "indexName"))
	if index == "" {
		index = text(dig(stats, "executionStages", "inputStage", "indexName"))
	}
	if stage == "" {
		stage = text(dig(doc, "queryPlanner", "winningPlan", "stage"))
	}
	if index == "" {
		index = text(dig(doc, "queryPlanner", "winningPlan", "inputStage", "indexName"))
	}
	return mongoExplain{
		NReturned: int64(num(dig(stats, "nReturned"))), DocsExamined: int64(num(dig(stats, "totalDocsExamined"))),
		KeysExamined: int64(num(dig(stats, "totalKeysExamined"))), Millis: int64(num(dig(stats, "executionTimeMillis"))),
		Stage: row.Cut(stage, 64), Index: row.Cut(index, 128),
	}, nil
}

type mongoProfileRow struct {
	Op     string `json:"op"`
	NS     string `json:"ns"`
	Millis int64  `json:"millis"`
	Client string `json:"client,omitempty"`
	Plan   string `json:"plan,omitempty"`
}

type mongoProfile struct {
	Entries []mongoProfileRow `json:"entries"`
	Note    string            `json:"note,omitempty"`
}

func opProfile(raw net.Conn, db string) (mongoProfile, error) {
	resp, err := call(raw, bsonDoc(
		bsonString("find", "system.profile"),
		bsonSub("sort", bsonDoc(bsonInt32("ts", -1))),
		bsonInt32("limit", 20),
		bsonInt32("batchSize", 20),
		bsonBool("singleBatch", true),
		bsonString("$db", db),
	))
	if err != nil {
		return mongoProfile{}, err
	}
	if err := opFail(resp); err != nil {
		if profileMissing(err) {
			return mongoProfile{Entries: []mongoProfileRow{}, Note: "没有历史慢日志。分析器没开时，system.profile 不存在。"}, nil
		}
		return mongoProfile{}, err
	}
	batch, _, err := readBatches(context.Background(), raw, resp, db, "system.profile", 20)
	if err != nil {
		return mongoProfile{}, err
	}
	out := make([]mongoProfileRow, 0, len(batch))
	for _, item := range batch {
		out = append(out, mongoProfileRow{
			Op: row.Cut(text(dig(item, "op")), 24), NS: row.Cut(text(dig(item, "ns")), 80),
			Millis: int64(num(dig(item, "millis"))), Client: row.Cut(text(dig(item, "client")), 64),
			Plan: row.Cut(text(dig(item, "planSummary")), 80),
		})
	}
	return mongoProfile{Entries: out}, nil
}

func denseArray(doc map[string]any, order []string) ([]any, bool) {
	if len(order) == 0 {
		return nil, false
	}
	out := make([]any, len(order))
	for i, key := range order {
		if key != strconv.Itoa(i) {
			return nil, false
		}
		out[i] = doc[key]
	}
	return out, true
}

func profileMissing(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "system.profile") || strings.Contains(msg, "not found") || strings.Contains(msg, "NamespaceNotFound")
}

func orderedJSON(value any) (string, error) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, value); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeJSON(buf *bytes.Buffer, value any) error {
	switch item := value.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if item {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		raw, err := json.Marshal(item)
		if err != nil {
			return err
		}
		buf.Write(raw)
	case int32:
		buf.WriteString(strconv.FormatInt(int64(item), 10))
	case int64:
		buf.WriteString(strconv.FormatInt(item, 10))
	case float64:
		buf.WriteString(strconv.FormatFloat(item, 'f', -1, 64))
	case []byte:
		raw, err := json.Marshal(fmt.Sprintf("%x", item))
		if err != nil {
			return err
		}
		buf.Write(raw)
	case map[string]any:
		order, _ := item["\x00"].([]string)
		if len(order) == 0 {
			for key := range item {
				if key == "\x00" {
					continue
				}
				order = append(order, key)
			}
		}
		if values, ok := denseArray(item, order); ok {
			buf.WriteByte('[')
			for i, value := range values {
				if i > 0 {
					buf.WriteByte(',')
				}
				if err := writeJSON(buf, value); err != nil {
					return err
				}
			}
			buf.WriteByte(']')
			return nil
		}
		buf.WriteByte('{')
		first := true
		for _, key := range order {
			if key == "\x00" {
				continue
			}
			if !first {
				buf.WriteByte(',')
			}
			first = false
			raw, err := json.Marshal(key)
			if err != nil {
				return err
			}
			buf.Write(raw)
			buf.WriteByte(':')
			if err := writeJSON(buf, item[key]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		raw, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("MongoDB 应答无效")
		}
		buf.Write(raw)
	}
	return nil
}
