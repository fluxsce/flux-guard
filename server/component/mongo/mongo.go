// Package mongo 读取 serverStatus、复制状态、库占用和正在执行的操作，并写入 FG_COMPONENT_MONGO_METRIC。
// 需要口令的实例当前记为连不上，不实现 SCRAM。
package mongo

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
		return newSample(), models.Status{Error: "MongoDB 口令登录尚未接入"}
	}
	addr, err := netx.HostPort(target.Address, 27017)
	if err != nil {
		return newSample(), models.Status{Error: err.Error()}
	}
	dialer := net.Dialer{Timeout: 3 * time.Second}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return newSample(), models.Status{Error: err.Error()}
	}
	defer raw.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	}
	doc, err := call(raw, bsonDoc(bsonInt32("serverStatus", 1), bsonString("$db", "admin")))
	if err != nil {
		return newSample(), models.Status{Error: err.Error()}
	}
	if num(dig(doc, "ok")) < 1 {
		msg := text(dig(doc, "errmsg"))
		if msg == "" {
			msg = "MongoDB 应答失败"
		}
		return newSample(), models.Status{Error: row.Cut(msg, 100)}
	}
	sample := sampleFrom(doc)
	me := text(dig(doc, "repl", "me"))
	if extra, err := call(raw, bsonDoc(bsonInt32("replSetGetStatus", 1), bsonString("$db", "admin"))); err != nil {
		sample.MemberNote = "不是副本集，或暂时读不到成员。"
	} else {
		readReplica(&sample, extra, me)
	}
	if extra, err := call(raw, bsonDoc(bsonInt32("listDatabases", 1), bsonString("$db", "admin"))); err != nil {
		sample.DbNote = "读不到库列表。"
	} else {
		readDatabases(&sample, extra)
	}
	if extra, err := call(raw, currentOpCommand()); err != nil {
		sample.RunningNote = "读不到正在执行的操作。"
	} else {
		readRunning(&sample, extra)
	}
	return sample, models.Status{
		Up: true, Version: sample.Version, Role: sample.Role,
		Summary: row.Cut(summaryOf(sample), 100),
	}
}

func currentOpCommand() []byte {
	return bsonDoc(
		bsonInt32("currentOp", 1),
		bsonBool("active", true),
		bsonBool("$all", true),
		bsonString("$db", "admin"),
	)
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

func save(ctx context.Context, db *dbx.DB, tenantID, componentID string, sample Sample, status models.Status) error {
	now := time.Now().UTC()
	if status.Up {
		prev, err := lastCounters(ctx, db, tenantID, componentID)
		if err != nil {
			return err
		}
		applyRates(&sample, prev, now)
	} else {
		sample = newSample()
	}
	args := dbx.Args{
		"tenantId": tenantID, "pointId": id.New(), "componentId": componentID,
		"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
		"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
		"summaryText": row.Cut(status.Summary, 100), "observedTime": store.FormatTime(now),
		"connectionsCurrent": sample.ConnectionsCurrent, "connectionsAvailable": sample.ConnectionsAvailable,
		"connectionsActive": sample.ConnectionsActive, "connUsagePercent": sample.ConnUsagePercent,
		"residentBytes": sample.ResidentBytes, "cacheUsedBytes": sample.CacheUsedBytes,
		"cacheMaxBytes": sample.CacheMaxBytes, "cacheDirtyBytes": sample.CacheDirtyBytes,
		"cacheUsagePercent": sample.CacheUsagePercent, "cacheDirtyPercent": sample.CacheDirtyPercent,
		"opsPerSec": sample.OpsPerSec, "opInsertPerSec": sample.OpInsertPerSec, "opQueryPerSec": sample.OpQueryPerSec,
		"opUpdatePerSec": sample.OpUpdatePerSec, "opDeletePerSec": sample.OpDeletePerSec,
		"opCommandPerSec": sample.OpCommandPerSec, "opGetmorePerSec": sample.OpGetmorePerSec,
		"opInsert": sample.OpInsert, "opQuery": sample.OpQuery, "opUpdate": sample.OpUpdate, "opDelete": sample.OpDelete,
		"opGetmore": sample.OpGetmore, "opCommand": sample.OpCommand,
		"replicaLagSec": sample.ReplicaLagSec, "memberCount": sample.MemberCount, "unhealthyMembers": sample.UnhealthyMembers,
		"netInKbps": sample.NetInKbps, "netOutKbps": sample.NetOutKbps, "netInBytes": sample.NetIn, "netOutBytes": sample.NetOut,
		"queuedReaders": sample.QueuedReaders, "queuedWriters": sample.QueuedWriters,
		"readTickets": sample.ReadTickets, "writeTickets": sample.WriteTickets,
		"assertPerSec": sample.AssertPerSec, "assertsTotal": sample.Asserts,
		"slowOps": sample.SlowOps, "uptimeSec": sample.UptimeSec, "dataBytes": sample.DataBytes,
		"extProperty": mongoDetail(sample),
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_COMPONENT_MONGO_METRIC (
		  tenantId, pointId, componentId, upFlag, versionText, roleName, errorText, summaryText, observedTime,
		  connectionsCurrent, connectionsAvailable, connectionsActive, connUsagePercent,
		  residentBytes, cacheUsedBytes, cacheMaxBytes, cacheDirtyBytes, cacheUsagePercent, cacheDirtyPercent,
		  opsPerSec, opInsertPerSec, opQueryPerSec, opUpdatePerSec, opDeletePerSec, opCommandPerSec, opGetmorePerSec,
		  opInsert, opQuery, opUpdate, opDelete, opGetmore, opCommand,
		  replicaLagSec, memberCount, unhealthyMembers,
		  netInKbps, netOutKbps, netInBytes, netOutBytes,
		  queuedReaders, queuedWriters, readTickets, writeTickets,
		  assertPerSec, assertsTotal, slowOps, uptimeSec, dataBytes, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @pointId, @componentId, @upFlag, @versionText, @roleName, @errorText, @summaryText, @observedTime,
		  @connectionsCurrent, @connectionsAvailable, @connectionsActive, @connUsagePercent,
		  @residentBytes, @cacheUsedBytes, @cacheMaxBytes, @cacheDirtyBytes, @cacheUsagePercent, @cacheDirtyPercent,
		  @opsPerSec, @opInsertPerSec, @opQueryPerSec, @opUpdatePerSec, @opDeletePerSec, @opCommandPerSec, @opGetmorePerSec,
		  @opInsert, @opQuery, @opUpdate, @opDelete, @opGetmore, @opCommand,
		  @replicaLagSec, @memberCount, @unhealthyMembers,
		  @netInKbps, @netOutKbps, @netInBytes, @netOutBytes,
		  @queuedReaders, @queuedWriters, @readTickets, @writeTickets,
		  @assertPerSec, @assertsTotal, @slowOps, @uptimeSec, @dataBytes, @extProperty,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	if err != nil {
		return err
	}
	return pane.Touch(ctx, db, catalog.Mongo, args)
}

func lastCounters(ctx context.Context, db *dbx.DB, tenantID, componentID string) (*prior, error) {
	var insertN, queryN, updateN, deleteN, getmoreN, commandN any
	var netIn, netOut, asserts, uptime, observed any
	err := store.Scan(ctx, db, `
		SELECT opInsert, opQuery, opUpdate, opDelete, opGetmore, opCommand,
		       netInBytes, netOutBytes, assertsTotal, uptimeSec, observedTime
		FROM FG_COMPONENT_MONGO_METRIC
		WHERE tenantId = @tenantId AND componentId = @componentId
		ORDER BY observedTime DESC
		LIMIT 1`, dbx.Args{"tenantId": tenantID, "componentId": componentID},
		&insertN, &queryN, &updateN, &deleteN, &getmoreN, &commandN, &netIn, &netOut, &asserts, &uptime, &observed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	at, ok := store.TimeValue(observed)
	if !ok {
		return nil, nil
	}
	return &prior{
		Insert: int64(row.Float(insertN)), Query: int64(row.Float(queryN)),
		Update: int64(row.Float(updateN)), Delete: int64(row.Float(deleteN)),
		Getmore: int64(row.Float(getmoreN)), Command: int64(row.Float(commandN)),
		NetIn: int64(row.Float(netIn)), NetOut: int64(row.Float(netOut)),
		Asserts: int64(row.Float(asserts)), Uptime: int64(row.Float(uptime)), At: at,
	}, nil
}
