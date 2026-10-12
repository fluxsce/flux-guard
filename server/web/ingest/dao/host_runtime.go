package dao

import (
	"context"
	"strings"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

type tcpSnap struct {
	established, listen, timeWait, closeWait int64
	synSent, synRecv, finWait, lastAck       int64
	closing, tcpInUse, udpInUse, retrans     int64
	at                                       time.Time
}

type fdSnap struct {
	allocated, maximum int64
	usedPercent        float64
	at                 time.Time
}

type procSnap struct {
	count, running, sleeping, zombie, rss int64
	at                                    time.Time
}

type procTopSnap struct {
	pid, name string
	rss       int64
	cpu       float64
	at        time.Time
}

type procIOSnap struct {
	pid, name         string
	readBps, writeBps float64
	at                time.Time
}

func applyTCP(row *tcpSnap, name string, value float64, at time.Time) {
	row.at = at
	n := asInt(value)
	switch name {
	case "host.net.tcp.established":
		row.established = n
	case "host.net.tcp.listen":
		row.listen = n
	case "host.net.tcp.time_wait":
		row.timeWait = n
	case "host.net.tcp.close_wait":
		row.closeWait = n
	case "host.net.tcp.syn_sent":
		row.synSent = n
	case "host.net.tcp.syn_recv":
		row.synRecv = n
	case "host.net.tcp.fin_wait":
		row.finWait = n
	case "host.net.tcp.last_ack":
		row.lastAck = n
	case "host.net.tcp.closing":
		row.closing = n
	case "host.net.tcp.inuse":
		row.tcpInUse = n
	case "host.net.udp.inuse":
		row.udpInUse = n
	case "host.net.tcp.retransmits":
		row.retrans = n
	}
}

func applyFD(row *fdSnap, name string, value float64, at time.Time) {
	row.at = at
	switch name {
	case "host.fd.allocated":
		row.allocated = asInt(value)
	case "host.fd.max":
		row.maximum = asInt(value)
	case "host.fd.used.percent":
		row.usedPercent = value
	}
}

func applyProc(row *procSnap, tops map[string]*procTopSnap, sample protocol.Sample, at time.Time) bool {
	name := strings.TrimSpace(sample.MetricName)
	row.at = at
	switch name {
	case "host.proc.count":
		row.count = asInt(sample.MetricValue)
		return false
	case "host.proc.running.count":
		row.running = asInt(sample.MetricValue)
		return false
	case "host.proc.sleeping.count":
		row.sleeping = asInt(sample.MetricValue)
		return false
	case "host.proc.zombie.count":
		row.zombie = asInt(sample.MetricValue)
		return false
	case "host.proc.rss.total.bytes":
		row.rss = asInt(sample.MetricValue)
		return false
	case "host.proc.rss.bytes", "host.proc.cpu.percent":
		pid := clip(labelOf(sample, "pid"), 32)
		if pid == "" {
			return false
		}
		item := tops[pid]
		if item == nil {
			item = &procTopSnap{pid: pid, name: clip(labelOf(sample, "name"), 100)}
			if item.name == "" {
				item.name = pid
			}
			tops[pid] = item
		}
		if procName := clip(labelOf(sample, "name"), 100); procName != "" {
			item.name = procName
		}
		item.at = at
		if name == "host.proc.rss.bytes" {
			item.rss = asInt(sample.MetricValue)
		} else {
			item.cpu = sample.MetricValue
		}
		return true
	default:
		return false
	}
}

func writeTCP(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row tcpSnap) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_HOST_TCP (
		  pointId, batchId, agentId, tenantId, establishedCount, listenCount, timeWaitCount, closeWaitCount,
		  synSentCount, synRecvCount, finWaitCount, lastAckCount, closingCount,
		  tcpInUseCount, udpInUseCount, retransmitCount, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @establishedCount, @listenCount, @timeWaitCount, @closeWaitCount,
		  @synSentCount, @synRecvCount, @finWaitCount, @lastAckCount, @closingCount,
		  @tcpInUseCount, @udpInUseCount, @retransmitCount, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, withPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"establishedCount": row.established, "listenCount": row.listen,
		"timeWaitCount": row.timeWait, "closeWaitCount": row.closeWait,
		"synSentCount": row.synSent, "synRecvCount": row.synRecv, "finWaitCount": row.finWait,
		"lastAckCount": row.lastAck, "closingCount": row.closing,
		"tcpInUseCount": row.tcpInUse, "udpInUseCount": row.udpInUse, "retransmitCount": row.retrans,
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func writeFD(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row fdSnap) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_HOST_FD (
		  pointId, batchId, agentId, tenantId, allocatedCount, maximumCount, usedPercent, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @allocatedCount, @maximumCount, @usedPercent, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, withPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"allocatedCount": row.allocated, "maximumCount": row.maximum, "usedPercent": row.usedPercent,
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func applyProcIO(rows map[string]*procIOSnap, sample protocol.Sample, at time.Time) {
	pid := clip(labelOf(sample, "pid"), 32)
	if pid == "" {
		return
	}
	item := rows[pid]
	if item == nil {
		item = &procIOSnap{pid: pid, name: clip(labelOf(sample, "name"), 100)}
		if item.name == "" {
			item.name = pid
		}
		rows[pid] = item
	}
	if procName := clip(labelOf(sample, "name"), 100); procName != "" {
		item.name = procName
	}
	item.at = at
	switch strings.TrimSpace(sample.MetricName) {
	case "host.proc.io.read.bytes_per_sec":
		item.readBps = sample.MetricValue
	case "host.proc.io.write.bytes_per_sec":
		item.writeBps = sample.MetricValue
	}
}

func writeProcIO(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*procIOSnap) error {
	for _, item := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_HOST_PROC_IO (
			  pointId, batchId, agentId, tenantId, processId, processName, readBytesPerSec, writeBytesPerSec, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @processId, @processName, @readBytesPerSec, @writeBytesPerSec, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, withPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID,
			"processId": item.pid, "processName": item.name,
			"readBytesPerSec": item.readBps, "writeBytesPerSec": item.writeBps,
			"observedTime": store.FormatTime(item.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeProc(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row procSnap, tops map[string]*procTopSnap) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO FG_HOST_PROC (
		  pointId, batchId, agentId, tenantId, procCount, runningCount, sleepingCount, zombieCount, rssTotalBytes, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @procCount, @runningCount, @sleepingCount, @zombieCount, @rssTotalBytes, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, withPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"procCount": row.count, "runningCount": row.running, "sleepingCount": row.sleeping,
		"zombieCount": row.zombie, "rssTotalBytes": row.rss,
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	})); err != nil {
		return err
	}
	for _, item := range tops {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_HOST_PROC_TOP (
			  pointId, batchId, agentId, tenantId, processId, processName, rssBytes, cpuPercent, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @processId, @processName, @rssBytes, @cpuPercent, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, withPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID,
			"processId": item.pid, "processName": item.name, "rssBytes": item.rss, "cpuPercent": item.cpu,
			"observedTime": store.FormatTime(item.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}
