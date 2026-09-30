package dao

import (
	"context"
	"math"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

type cpuSnap struct {
	cpuCount, coreCount                    int
	usage, user, system, idle, iowait, irq float64
	softirq, steal, load1, load5, load15   float64
	at                                     time.Time
}

type coreSnap struct {
	name  string
	usage float64
	at    time.Time
}

type memSnap struct {
	total, available, used, free, cached, buffers int64
	usedPercent                                   float64
	swapTotal, swapUsed, swapFree                 int64
	swapUsedPercent                               float64
	at                                            time.Time
}

type diskSnap struct {
	mount, device                                       string
	total, used, free, inodeTotal, inodeUsed, inodeFree int64
	usedPercent, inodeUsedPercent                       float64
	at                                                  time.Time
}

type diskIOSnap struct {
	device                                       string
	readBytes, writeBytes, reads, writes         int64
	readTimeMs, writeTimeMs                      int64
	readBps, writeBps, readLatency, writeLatency float64
	util                                         float64
	at                                           time.Time
}

type netSnap struct {
	name                                   string
	rxBytes, txBytes, rxPackets, txPackets int64
	rxErrs, txErrs, rxDrop, txDrop         int64
	rxBps, txBps                           float64
	at                                     time.Time
}

// SaveHostMetrics 把 host.* 样本写入对应的主机记录表。其它指标名直接跳过。
func SaveHostMetrics(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	var cpu cpuSnap
	var mem memSnap
	var tcp tcpSnap
	var fd fdSnap
	var proc procSnap
	cores := map[string]*coreSnap{}
	disks := map[string]*diskSnap{}
	ios := map[string]*diskIOSnap{}
	nics := map[string]*netSnap{}
	tops := map[string]*procTopSnap{}
	var hasCPU, hasMem, hasCore, hasDisk, hasIO, hasNet, hasTCP, hasFD, hasProc bool
	for _, sample := range batch.Samples {
		name := strings.TrimSpace(sample.MetricName)
		at := sample.ObservedAt
		if at.IsZero() {
			at = batch.SentAt
		}
		switch {
		case strings.HasPrefix(name, "host.cpu.core."):
			key := clip(labelOf(sample, "cpu"), 32)
			if key == "" {
				continue
			}
			row := cores[key]
			if row == nil {
				row = &coreSnap{name: key}
				cores[key] = row
			}
			row.at = at
			if name == "host.cpu.core.usage.percent" {
				row.usage = sample.MetricValue
			}
			hasCore = true
		case strings.HasPrefix(name, "host.cpu."):
			applyCPU(&cpu, name, sample.MetricValue, at)
			hasCPU = true
		case strings.HasPrefix(name, "host.mem."):
			applyMem(&mem, name, sample.MetricValue, at)
			hasMem = true
		case strings.HasPrefix(name, "host.disk.io."):
			key := clip(labelOf(sample, "device"), 100)
			if key == "" {
				continue
			}
			row := ios[key]
			if row == nil {
				row = &diskIOSnap{device: key}
				ios[key] = row
			}
			applyDiskIO(row, name, sample.MetricValue, at)
			hasIO = true
		case strings.HasPrefix(name, "host.disk."):
			key := clip(labelOf(sample, "path"), 255)
			if key == "" {
				continue
			}
			row := disks[key]
			if row == nil {
				row = &diskSnap{mount: key}
				disks[key] = row
			}
			if device := clip(labelOf(sample, "device"), 100); device != "" {
				row.device = device
			}
			applyDisk(row, name, sample.MetricValue, at)
			hasDisk = true
		case strings.HasPrefix(name, "host.net.tcp."), strings.HasPrefix(name, "host.net.udp."):
			applyTCP(&tcp, name, sample.MetricValue, at)
			hasTCP = true
		case strings.HasPrefix(name, "host.net."):
			key := clip(labelOf(sample, "device"), 64)
			if key == "" {
				continue
			}
			row := nics[key]
			if row == nil {
				row = &netSnap{name: key}
				nics[key] = row
			}
			applyNet(row, name, sample.MetricValue, at)
			hasNet = true
		case strings.HasPrefix(name, "host.fd."):
			applyFD(&fd, name, sample.MetricValue, at)
			hasFD = true
		case strings.HasPrefix(name, "host.proc."):
			applyProc(&proc, tops, sample, at)
			hasProc = true
		}
	}
	if hasCPU {
		if err := writeCPU(ctx, tx, batch, cpu); err != nil {
			return err
		}
	}
	if hasCore {
		if err := writeCores(ctx, tx, batch, cores); err != nil {
			return err
		}
	}
	if hasMem {
		if err := writeMem(ctx, tx, batch, mem); err != nil {
			return err
		}
	}
	if hasDisk {
		if err := writeDisks(ctx, tx, batch, disks); err != nil {
			return err
		}
	}
	if hasIO {
		if err := writeDiskIO(ctx, tx, batch, ios); err != nil {
			return err
		}
	}
	if hasNet {
		if err := writeNets(ctx, tx, batch, nics); err != nil {
			return err
		}
	}
	if hasTCP {
		if err := writeTCP(ctx, tx, batch, tcp); err != nil {
			return err
		}
	}
	if hasFD {
		if err := writeFD(ctx, tx, batch, fd); err != nil {
			return err
		}
	}
	if hasProc {
		if err := writeProc(ctx, tx, batch, proc, tops); err != nil {
			return err
		}
	}
	return nil
}

func applyCPU(row *cpuSnap, name string, value float64, at time.Time) {
	row.at = at
	switch name {
	case "host.cpu.count":
		row.cpuCount = int(asInt(value))
	case "host.cpu.core.count":
		row.coreCount = int(asInt(value))
	case "host.cpu.usage.percent":
		row.usage = value
	case "host.cpu.user.percent":
		row.user = value
	case "host.cpu.system.percent":
		row.system = value
	case "host.cpu.idle.percent":
		row.idle = value
	case "host.cpu.iowait.percent":
		row.iowait = value
	case "host.cpu.irq.percent":
		row.irq = value
	case "host.cpu.softirq.percent":
		row.softirq = value
	case "host.cpu.steal.percent":
		row.steal = value
	case "host.cpu.load1":
		row.load1 = value
	case "host.cpu.load5":
		row.load5 = value
	case "host.cpu.load15":
		row.load15 = value
	}
}

func applyMem(row *memSnap, name string, value float64, at time.Time) {
	row.at = at
	switch name {
	case "host.mem.total.bytes":
		row.total = asInt(value)
	case "host.mem.available.bytes":
		row.available = asInt(value)
	case "host.mem.used.bytes":
		row.used = asInt(value)
	case "host.mem.free.bytes":
		row.free = asInt(value)
	case "host.mem.cached.bytes":
		row.cached = asInt(value)
	case "host.mem.buffers.bytes":
		row.buffers = asInt(value)
	case "host.mem.used.percent":
		row.usedPercent = value
	case "host.mem.swap.total.bytes":
		row.swapTotal = asInt(value)
	case "host.mem.swap.used.bytes":
		row.swapUsed = asInt(value)
	case "host.mem.swap.free.bytes":
		row.swapFree = asInt(value)
	case "host.mem.swap.used.percent":
		row.swapUsedPercent = value
	}
}

func applyDisk(row *diskSnap, name string, value float64, at time.Time) {
	row.at = at
	switch name {
	case "host.disk.total.bytes":
		row.total = asInt(value)
	case "host.disk.used.bytes":
		row.used = asInt(value)
	case "host.disk.free.bytes":
		row.free = asInt(value)
	case "host.disk.used.percent":
		row.usedPercent = value
	case "host.disk.inodes.total":
		row.inodeTotal = asInt(value)
	case "host.disk.inodes.used":
		row.inodeUsed = asInt(value)
	case "host.disk.inodes.free":
		row.inodeFree = asInt(value)
	case "host.disk.inodes.used.percent":
		row.inodeUsedPercent = value
	}
}

func applyDiskIO(row *diskIOSnap, name string, value float64, at time.Time) {
	row.at = at
	switch name {
	case "host.disk.io.read.bytes":
		row.readBytes = asInt(value)
	case "host.disk.io.write.bytes":
		row.writeBytes = asInt(value)
	case "host.disk.io.reads":
		row.reads = asInt(value)
	case "host.disk.io.writes":
		row.writes = asInt(value)
	case "host.disk.io.read.time_ms":
		row.readTimeMs = asInt(value)
	case "host.disk.io.write.time_ms":
		row.writeTimeMs = asInt(value)
	case "host.disk.io.read.bytes_per_sec":
		row.readBps = value
	case "host.disk.io.write.bytes_per_sec":
		row.writeBps = value
	case "host.disk.io.read.latency_ms":
		row.readLatency = value
	case "host.disk.io.write.latency_ms":
		row.writeLatency = value
	case "host.disk.io.util.percent":
		row.util = value
	}
}

func applyNet(row *netSnap, name string, value float64, at time.Time) {
	row.at = at
	switch name {
	case "host.net.receive.bytes":
		row.rxBytes = asInt(value)
	case "host.net.transmit.bytes":
		row.txBytes = asInt(value)
	case "host.net.receive.packets":
		row.rxPackets = asInt(value)
	case "host.net.transmit.packets":
		row.txPackets = asInt(value)
	case "host.net.receive.errs":
		row.rxErrs = asInt(value)
	case "host.net.transmit.errs":
		row.txErrs = asInt(value)
	case "host.net.receive.drop":
		row.rxDrop = asInt(value)
	case "host.net.transmit.drop":
		row.txDrop = asInt(value)
	case "host.net.receive.bytes_per_sec":
		row.rxBps = value
	case "host.net.transmit.bytes_per_sec":
		row.txBps = value
	}
}

func withPoint(batch protocol.Batch, args dbx.Args) dbx.Args {
	args["pointId"] = id.New()
	args["batchId"] = batch.BatchID
	return args
}

func writeCPU(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row cpuSnap) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_HOST_CPU (
		  pointId, batchId, agentId, tenantId, cpuCount, coreCount, usagePercent, userPercent, systemPercent, idlePercent,
		  iowaitPercent, irqPercent, softirqPercent, stealPercent, loadAvg1, loadAvg5, loadAvg15, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @cpuCount, @coreCount, @usagePercent, @userPercent, @systemPercent, @idlePercent,
		  @iowaitPercent, @irqPercent, @softirqPercent, @stealPercent, @loadAvg1, @loadAvg5, @loadAvg15, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, withPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"cpuCount": row.cpuCount, "coreCount": row.coreCount,
		"usagePercent": row.usage, "userPercent": row.user, "systemPercent": row.system, "idlePercent": row.idle,
		"iowaitPercent": row.iowait, "irqPercent": row.irq, "softirqPercent": row.softirq, "stealPercent": row.steal,
		"loadAvg1": row.load1, "loadAvg5": row.load5, "loadAvg15": row.load15,
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func writeCores(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*coreSnap) error {
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_HOST_CPU_CORE (
			  pointId, batchId, agentId, tenantId, coreName, usagePercent, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @coreName, @usagePercent, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, withPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "coreName": row.name,
			"usagePercent": row.usage, "observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeMem(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row memSnap) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_HOST_MEM (
		  pointId, batchId, agentId, tenantId, totalBytes, availableBytes, usedBytes, freeBytes, cachedBytes, buffersBytes, usedPercent,
		  swapTotalBytes, swapUsedBytes, swapFreeBytes, swapUsedPercent, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @totalBytes, @availableBytes, @usedBytes, @freeBytes, @cachedBytes, @buffersBytes, @usedPercent,
		  @swapTotalBytes, @swapUsedBytes, @swapFreeBytes, @swapUsedPercent, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, withPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"totalBytes": row.total, "availableBytes": row.available, "usedBytes": row.used, "freeBytes": row.free,
		"cachedBytes": row.cached, "buffersBytes": row.buffers, "usedPercent": row.usedPercent,
		"swapTotalBytes": row.swapTotal, "swapUsedBytes": row.swapUsed, "swapFreeBytes": row.swapFree,
		"swapUsedPercent": row.swapUsedPercent, "observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func writeDisks(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*diskSnap) error {
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_HOST_DISK (
			  pointId, batchId, agentId, tenantId, mountPoint, deviceName, totalBytes, usedBytes, freeBytes, usedPercent,
			  inodeTotal, inodeUsed, inodeFree, inodeUsedPercent, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @mountPoint, @deviceName, @totalBytes, @usedBytes, @freeBytes, @usedPercent,
			  @inodeTotal, @inodeUsed, @inodeFree, @inodeUsedPercent, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, withPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "mountPoint": row.mount, "deviceName": nullText(row.device),
			"totalBytes": row.total, "usedBytes": row.used, "freeBytes": row.free, "usedPercent": row.usedPercent,
			"inodeTotal": row.inodeTotal, "inodeUsed": row.inodeUsed, "inodeFree": row.inodeFree,
			"inodeUsedPercent": row.inodeUsedPercent, "observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeDiskIO(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*diskIOSnap) error {
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_HOST_DISK_IO (
			  pointId, batchId, agentId, tenantId, deviceName, readBytes, writeBytes, readCount, writeCount, readTimeMs, writeTimeMs,
			  readBytesPerSec, writeBytesPerSec, readLatencyMs, writeLatencyMs, utilPercent, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @deviceName, @readBytes, @writeBytes, @readCount, @writeCount, @readTimeMs, @writeTimeMs,
			  @readBytesPerSec, @writeBytesPerSec, @readLatencyMs, @writeLatencyMs, @utilPercent, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, withPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "deviceName": row.device,
			"readBytes": row.readBytes, "writeBytes": row.writeBytes, "readCount": row.reads, "writeCount": row.writes,
			"readTimeMs": row.readTimeMs, "writeTimeMs": row.writeTimeMs,
			"readBytesPerSec": row.readBps, "writeBytesPerSec": row.writeBps,
			"readLatencyMs": row.readLatency, "writeLatencyMs": row.writeLatency, "utilPercent": row.util,
			"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeNets(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*netSnap) error {
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_HOST_NET (
			  pointId, batchId, agentId, tenantId, nicName, receiveBytes, transmitBytes, receivePackets, transmitPackets,
			  receiveErrs, transmitErrs, receiveDrop, transmitDrop, receiveBytesPerSec, transmitBytesPerSec, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @nicName, @receiveBytes, @transmitBytes, @receivePackets, @transmitPackets,
			  @receiveErrs, @transmitErrs, @receiveDrop, @transmitDrop, @receiveBytesPerSec, @transmitBytesPerSec, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, withPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "nicName": row.name,
			"receiveBytes": row.rxBytes, "transmitBytes": row.txBytes,
			"receivePackets": row.rxPackets, "transmitPackets": row.txPackets,
			"receiveErrs": row.rxErrs, "transmitErrs": row.txErrs, "receiveDrop": row.rxDrop, "transmitDrop": row.txDrop,
			"receiveBytesPerSec": row.rxBps, "transmitBytesPerSec": row.txBps,
			"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func labelOf(sample protocol.Sample, key string) string {
	if sample.Labels == nil {
		return ""
	}
	return strings.TrimSpace(sample.Labels[key])
}

func clip(value string, n int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= n {
		return value
	}
	return string(runes[:n])
}

func asInt(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int64(math.Round(value))
}
