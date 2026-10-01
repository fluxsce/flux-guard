// Package mongo 用 OP_MSG 读取 serverStatus，并写入 FG_COMPONENT_MONGO_METRIC。
// 需要口令的实例当前记为连不上，不实现 SCRAM。
package mongo

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	"fluxguard/server/component/netx"
	"fluxguard/server/component/pane"
	"fluxguard/server/component/row"
	"fluxguard/server/component/runtime"
	"fluxguard/server/component/series"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	runtime.Register(catalog.Mongo, probe{})
}

type probe struct{}

// Sample 是一轮 MongoDB 采集。
type Sample struct {
	ConnectionsCurrent   int64
	ConnectionsAvailable int64
	OpInsert             int64
	OpQuery              int64
	OpUpdate             int64
	OpDelete             int64
	Version              string
	Role                 string
}

func (probe) Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error) {
	sample, status := collect(ctx, target)
	if db != nil {
		if err := save(ctx, db, target.TenantID, target.ComponentID, sample, status); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (probe) Series(ctx context.Context, db *dbx.DB, tenantID, componentID string, limit int) ([]models.Point, error) {
	return series.Read(ctx, db, "FG_COMPONENT_MONGO_METRIC", tenantID, componentID, catalog.Columns(catalog.Mongo), limit, series.Span{})
}

func collect(ctx context.Context, target conn.Open) (Sample, models.Status) {
	if target.UserName != "" || target.Secret != "" {
		return Sample{}, models.Status{Error: "MongoDB 口令登录尚未接入"}
	}
	addr, err := netx.HostPort(target.Address, 27017)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	doc, err := call(raw, bsonDoc(bsonInt32("serverStatus", 1), bsonString("$db", "admin")))
	if err != nil {
		return Sample{}, models.Status{Error: err.Error()}
	}
	if okv := num(dig(doc, "ok")); okv < 1 {
		msg := text(dig(doc, "errmsg"))
		if msg == "" {
			msg = "MongoDB 应答失败"
		}
		return Sample{}, models.Status{Error: row.Cut(msg, 100)}
	}
	sample := Sample{
		ConnectionsCurrent:   int64(num(dig(doc, "connections", "current"))),
		ConnectionsAvailable: int64(num(dig(doc, "connections", "available"))),
		OpInsert:             int64(num(dig(doc, "opcounters", "insert"))),
		OpQuery:              int64(num(dig(doc, "opcounters", "query"))),
		OpUpdate:             int64(num(dig(doc, "opcounters", "update"))),
		OpDelete:             int64(num(dig(doc, "opcounters", "delete"))),
		Version:              text(dig(doc, "version")),
		Role:                 "standalone",
	}
	if truthy(dig(doc, "repl", "ismaster")) || truthy(dig(doc, "repl", "isWritablePrimary")) {
		sample.Role = "primary"
	} else if truthy(dig(doc, "repl", "secondary")) {
		sample.Role = "secondary"
	}
	return sample, models.Status{
		Up: true, Version: sample.Version, Role: sample.Role,
		Summary: fmt.Sprintf("角色 %s · 连接 %d", sample.Role, sample.ConnectionsCurrent),
	}
}

func call(raw net.Conn, doc []byte) (map[string]any, error) {
	msg := opMessage(doc)
	if _, err := raw.Write(msg); err != nil {
		return nil, err
	}
	var sizeBuf [4]byte
	if _, err := io.ReadFull(raw, sizeBuf[:]); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint32(sizeBuf[:]))
	if size < 21 || size > 8*1024*1024 {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	rest := make([]byte, size-4)
	if _, err := io.ReadFull(raw, rest); err != nil {
		return nil, err
	}
	if len(rest) < 16 {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	payload := rest[12:]
	flags := binary.LittleEndian.Uint32(payload[:4])
	body := payload[4:]
	if flags&1 != 0 && len(body) >= 4 {
		body = body[:len(body)-4]
	}
	if len(body) < 5 || body[0] != 0 {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	return decodeDoc(body[1:])
}

func opMessage(doc []byte) []byte {
	size := 16 + 4 + 1 + len(doc)
	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:], uint32(size))
	binary.LittleEndian.PutUint32(buf[4:], 1)
	binary.LittleEndian.PutUint32(buf[12:], 2013)
	buf[20] = 0
	copy(buf[21:], doc)
	return buf
}

func bsonDoc(elems ...[]byte) []byte {
	body := []byte{}
	for _, elem := range elems {
		body = append(body, elem...)
	}
	size := 4 + len(body) + 1
	out := make([]byte, 4, size)
	binary.LittleEndian.PutUint32(out, uint32(size))
	out = append(out, body...)
	return append(out, 0)
}

func bsonInt32(name string, v int32) []byte {
	out := append([]byte{0x10}, cstring(name)...)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(v))
	return append(out, n[:]...)
}

func bsonString(name, value string) []byte {
	out := append([]byte{0x02}, cstring(name)...)
	raw := append([]byte(value), 0)
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(raw)))
	out = append(out, n[:]...)
	return append(out, raw...)
}

func cstring(name string) []byte {
	return append([]byte(name), 0)
}

func decodeDoc(b []byte) (map[string]any, error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	size := int(binary.LittleEndian.Uint32(b[:4]))
	if size < 5 || size > len(b) {
		return nil, fmt.Errorf("MongoDB 应答无效")
	}
	return decodeElems(b[4 : size-1])
}

func decodeElems(b []byte) (map[string]any, error) {
	out := map[string]any{}
	i := 0
	for i < len(b) {
		typ := b[i]
		i++
		name, n, err := readCString(b[i:])
		if err != nil {
			return nil, err
		}
		i += n
		val, used, err := decodeValue(typ, b[i:])
		if err != nil {
			return nil, err
		}
		out[name] = val
		i += used
	}
	return out, nil
}

func decodeValue(typ byte, b []byte) (any, int, error) {
	switch typ {
	case 0x01:
		if len(b) < 8 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(b[:8])), 8, nil
	case 0x02:
		if len(b) < 4 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		n := int(binary.LittleEndian.Uint32(b[:4]))
		if n < 1 || 4+n > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return string(b[4 : 4+n-1]), 4 + n, nil
	case 0x03, 0x04:
		if len(b) < 4 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		n := int(binary.LittleEndian.Uint32(b[:4]))
		if n < 5 || n > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		doc, err := decodeElems(b[4 : n-1])
		return doc, n, err
	case 0x05:
		if len(b) < 5 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		n := int(binary.LittleEndian.Uint32(b[:4]))
		if 5+n > len(b) {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return nil, 5 + n, nil
	case 0x07:
		if len(b) < 12 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return nil, 12, nil
	case 0x08:
		if len(b) < 1 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return b[0] != 0, 1, nil
	case 0x09, 0x11, 0x12:
		if len(b) < 8 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		if typ == 0x12 {
			return int64(binary.LittleEndian.Uint64(b[:8])), 8, nil
		}
		return nil, 8, nil
	case 0x0A:
		return nil, 0, nil
	case 0x10:
		if len(b) < 4 {
			return nil, 0, fmt.Errorf("MongoDB 应答无效")
		}
		return int32(binary.LittleEndian.Uint32(b[:4])), 4, nil
	default:
		return nil, 0, fmt.Errorf("MongoDB 应答含未识别类型")
	}
}

func readCString(b []byte) (string, int, error) {
	for i, c := range b {
		if c == 0 {
			return string(b[:i]), i + 1, nil
		}
	}
	return "", 0, fmt.Errorf("MongoDB 应答无效")
}

func dig(v any, path ...string) any {
	cur := v
	for _, key := range path {
		doc, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = doc[key]
	}
	return cur
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	default:
		return 0
	}
}

func text(v any) string {
	s, _ := v.(string)
	return s
}

func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(time.Now().UTC()),
		"connectionsCurrent": sample.ConnectionsCurrent, "connectionsAvailable": sample.ConnectionsAvailable,
		"opInsert": sample.OpInsert, "opQuery": sample.OpQuery, "opUpdate": sample.OpUpdate, "opDelete": sample.OpDelete,
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_MONGO_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  connectionsCurrent, connectionsAvailable, opInsert, opQuery, opUpdate, opDelete,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @connectionsCurrent, @connectionsAvailable, @opInsert, @opQuery, @opUpdate, @opDelete,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.Mongo, args)
}
