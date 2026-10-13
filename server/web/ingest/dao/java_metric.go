package dao

import (
	"context"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"

	dbx "github.com/fluxsce/dbx/db"
)

// jvm.* 样本按类别只插入。身份仍由 SaveJava 更新。
// 堆：jvm.heap.used.bytes / committed.bytes / max.bytes，非堆与元空间同形。
// GC：jvm.gc.count、jvm.gc.time.ms，标签 collector 区分回收器，值为累计数。
// 停顿：jvm.gc.pause.count / max.ms / time.ms / p50.ms / p99.ms，同一回收器，值为这一拍的区间。
// 开销 jvm.gc.pause.overhead.percent 未知为 -1。周期 jvm.gc.cycle.* 不计入停顿。死锁 jvm.thread.deadlock.count，未知不写。
// 内存池：jvm.memory.pool.used.bytes / committed.bytes，标签 pool 只接受分代、代码缓存和堆外。
// 运行时：jvm.cpu.process.percent、jvm.class.loaded、jvm.class.unloaded。
// 线程：jvm.thread.count、jvm.thread.daemon.count、jvm.thread.peak.count。
// 状态：jvm.thread.runnable.count、blocked.count、waiting.count。不带栈。
// 上限：jvm.memory.pool.max.bytes，标签 pool=direct 时写入直接内存上限。

type heapSnap struct {
	used, committed, max          int64
	nonUsed, nonCommitted, nonMax int64
	metaUsed, metaCommitted       int64
	usedPercent                   float64
	at                            time.Time
}

type gcSnap struct {
	name  string
	count int64
	time  int64
	at    time.Time
}

type threadSnap struct {
	count, daemon, peak int64
	at                  time.Time
}

type poolSnap struct {
	name               string
	used, committed    int64
	max                int64
	hasUsed, hasCommit bool
	hasMax             bool
	at                 time.Time
}

type stateSnap struct {
	runnable, blocked, waiting int64
	deadlock                   int64
	hasDeadlock                bool
	deadlockText               string
	at                         time.Time
}

type runtimeSnap struct {
	cpu               float64
	loaded, unloaded  int64
	overhead          float64
	hasCPU, hasLoaded bool
	hasUnloaded       bool
	hasOverhead       bool
	at                time.Time
}

type pauseSnap struct {
	name                    string
	count, max, total       int64
	p50, p99                int64
	before, after, promoted int64
	cycleCount, cycleTime   int64
	cause                   string
	at                      time.Time
}

// SaveJavaMetrics 把 jvm.* 样本写入 Java 曲线表。种类不是 java，或本批没有这些指标时什么都不写。
func SaveJavaMetrics(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	if strings.TrimSpace(batch.AgentCode) != protocol.AgentJava {
		return nil
	}
	var heap heapSnap
	var threads threadSnap
	var state stateSnap
	var runtime runtimeSnap
	gcs := map[string]*gcSnap{}
	pools := map[string]*poolSnap{}
	pauses := map[string]*pauseSnap{}
	var hasHeap, hasThread, hasState, hasGC, hasRuntime, hasPool, hasPoolMax, hasPause bool
	for _, sample := range batch.Samples {
		name := strings.TrimSpace(sample.MetricName)
		if !strings.HasPrefix(name, "jvm.") {
			continue
		}
		at := sample.ObservedAt
		if at.IsZero() {
			at = batch.SentAt
		}
		switch name {
		case "jvm.heap.used.bytes":
			heap.used = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.heap.committed.bytes":
			heap.committed = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.heap.max.bytes":
			heap.max = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.heap.used.percent":
			heap.usedPercent = sample.MetricValue
			heap.at = at
			hasHeap = true
		case "jvm.nonheap.used.bytes":
			heap.nonUsed = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.nonheap.committed.bytes":
			heap.nonCommitted = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.nonheap.max.bytes":
			heap.nonMax = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.metaspace.used.bytes":
			heap.metaUsed = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.metaspace.committed.bytes":
			heap.metaCommitted = asInt(sample.MetricValue)
			heap.at = at
			hasHeap = true
		case "jvm.gc.count":
			row := javaGC(gcs, sample)
			row.count = asInt(sample.MetricValue)
			row.at = at
			hasGC = true
		case "jvm.gc.time.ms":
			row := javaGC(gcs, sample)
			row.time = asInt(sample.MetricValue)
			row.at = at
			hasGC = true
		case "jvm.thread.count":
			threads.count = asInt(sample.MetricValue)
			threads.at = at
			hasThread = true
		case "jvm.thread.daemon.count":
			threads.daemon = asInt(sample.MetricValue)
			threads.at = at
			hasThread = true
		case "jvm.thread.peak.count":
			threads.peak = asInt(sample.MetricValue)
			threads.at = at
			hasThread = true
		case "jvm.thread.runnable.count":
			state.runnable = asInt(sample.MetricValue)
			state.at = at
			hasState = true
		case "jvm.thread.blocked.count":
			state.blocked = asInt(sample.MetricValue)
			state.at = at
			hasState = true
		case "jvm.thread.waiting.count":
			state.waiting = asInt(sample.MetricValue)
			state.at = at
			hasState = true
		case "jvm.thread.deadlock.count":
			state.deadlock = asInt(sample.MetricValue)
			state.hasDeadlock = true
			state.deadlockText = clip(labelOf(sample, "detail"), 500)
			state.at = at
			hasState = true
		case "jvm.memory.pool.used.bytes":
			if row := javaPool(pools, sample); row != nil {
				row.used = asInt(sample.MetricValue)
				row.hasUsed = true
				row.at = at
				hasPool = true
			}
		case "jvm.memory.pool.committed.bytes":
			if row := javaPool(pools, sample); row != nil {
				row.committed = asInt(sample.MetricValue)
				row.hasCommit = true
				row.at = at
				hasPool = true
			}
		case "jvm.memory.pool.max.bytes":
			if row := javaPool(pools, sample); row != nil && row.name == "direct" {
				row.max = asInt(sample.MetricValue)
				row.hasMax = true
				row.at = at
				hasPoolMax = true
			}
		case "jvm.cpu.process.percent":
			runtime.cpu = sample.MetricValue
			runtime.hasCPU = true
			runtime.at = at
			hasRuntime = true
		case "jvm.class.loaded":
			runtime.loaded = asInt(sample.MetricValue)
			runtime.hasLoaded = true
			runtime.at = at
			hasRuntime = true
		case "jvm.class.unloaded":
			runtime.unloaded = asInt(sample.MetricValue)
			runtime.hasUnloaded = true
			runtime.at = at
			hasRuntime = true
		case "jvm.gc.pause.overhead.percent":
			runtime.overhead = sample.MetricValue
			runtime.hasOverhead = true
			runtime.at = at
			hasRuntime = true
		case "jvm.gc.pause.count":
			row := javaPause(pauses, sample)
			row.count = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.max.ms":
			row := javaPause(pauses, sample)
			row.max = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.time.ms":
			row := javaPause(pauses, sample)
			row.total = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.p50.ms":
			row := javaPause(pauses, sample)
			row.p50 = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.p99.ms":
			row := javaPause(pauses, sample)
			row.p99 = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.heap.before.bytes":
			row := javaPause(pauses, sample)
			row.before = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.heap.after.bytes":
			row := javaPause(pauses, sample)
			row.after = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.pause.promoted.bytes":
			row := javaPause(pauses, sample)
			row.promoted = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.cycle.count":
			row := javaPause(pauses, sample)
			row.cycleCount = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		case "jvm.gc.cycle.time.ms":
			row := javaPause(pauses, sample)
			row.cycleTime = asInt(sample.MetricValue)
			row.at = at
			hasPause = true
		}
	}
	if hasHeap {
		if heap.max > 0 && heap.used >= 0 {
			heap.usedPercent = float64(heap.used) / float64(heap.max) * 100
		} else if heap.max <= 0 {
			heap.usedPercent = -1
		}
		if err := writeJavaHeap(ctx, tx, batch, heap); err != nil {
			return err
		}
	}
	if hasGC {
		if err := writeJavaGC(ctx, tx, batch, gcs); err != nil {
			return err
		}
	}
	if hasThread {
		if err := writeJavaThread(ctx, tx, batch, threads); err != nil {
			return err
		}
	}
	if hasState {
		if err := writeJavaState(ctx, tx, batch, state); err != nil {
			return err
		}
	}
	if hasPool {
		if err := writeJavaPool(ctx, tx, batch, pools); err != nil {
			return err
		}
	}
	if hasPoolMax {
		if err := writeJavaPoolMax(ctx, tx, batch, pools); err != nil {
			return err
		}
	}
	if hasRuntime {
		if err := writeJavaRuntime(ctx, tx, batch, runtime); err != nil {
			return err
		}
	}
	if hasPause {
		if err := writeJavaPause(ctx, tx, batch, pauses); err != nil {
			return err
		}
	}
	return touchJavaCurrent(ctx, tx, batch, hasHeap, heap, hasThread, threads, hasGC, gcs)
}

// touchJavaCurrent 把本批堆、线程和回收累计写回身份行。没到的指标保持上一行。
func touchJavaCurrent(ctx context.Context, tx *dbx.DB, batch protocol.Batch, hasHeap bool, heap heapSnap, hasThread bool, threads threadSnap, hasGC bool, gcs map[string]*gcSnap) error {
	sets := make([]string, 0, 6)
	args := dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID}
	if hasHeap {
		sets = append(sets, "heapUsedPercent = @heapUsedPercent", "heapUsedBytes = @heapUsedBytes", "heapMaxBytes = @heapMaxBytes", "nonHeapUsedBytes = @nonHeapUsedBytes")
		args["heapUsedPercent"] = heap.usedPercent
		args["heapUsedBytes"] = heap.used
		args["heapMaxBytes"] = heap.max
		args["nonHeapUsedBytes"] = heap.nonUsed
	}
	if hasThread {
		sets = append(sets, "threadCount = @threadCount", "daemonCount = @daemonCount")
		args["threadCount"] = threads.count
		args["daemonCount"] = threads.daemon
	}
	if hasGC {
		var count, elapsed int64
		for _, row := range gcs {
			count += row.count
			elapsed += row.time
		}
		sets = append(sets, "collectionCount = @collectionCount", "collectionTimeMs = @collectionTimeMs")
		args["collectionCount"] = count
		args["collectionTimeMs"] = elapsed
		if _, err := tx.Exec(ctx, `
			UPDATE FG_JAVA_RUN SET
			  collectionCount = @collectionCount, collectionTimeMs = @collectionTimeMs,
			  editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND closedTime IS NULL`,
			dbx.Args{
				"collectionCount": count, "collectionTimeMs": elapsed,
				"editTime": store.FormatTime(time.Now().UTC()),
				"tenantId": batch.TenantID, "agentId": batch.AgentID,
			}); err != nil {
			return err
		}
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, "UPDATE FG_AGENT_JAVA SET "+strings.Join(sets, ", ")+" WHERE tenantId = @tenantId AND agentId = @agentId", args)
	return err
}

func javaGC(rows map[string]*gcSnap, sample protocol.Sample) *gcSnap {
	name := collectorName(sample)
	row := rows[name]
	if row == nil {
		row = &gcSnap{name: name}
		rows[name] = row
	}
	return row
}

func collectorName(sample protocol.Sample) string {
	name := clip(labelOf(sample, "collector"), 100)
	if name == "" {
		return "gc"
	}
	return name
}

func javaPool(rows map[string]*poolSnap, sample protocol.Sample) *poolSnap {
	name := poolKind(labelOf(sample, "pool"))
	if name == "" {
		return nil
	}
	row := rows[name]
	if row == nil {
		row = &poolSnap{name: name}
		rows[name] = row
	}
	return row
}

func poolKind(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "eden", "survivor", "old", "code", "direct", "mapped", "classspace":
		return strings.ToLower(strings.TrimSpace(name))
	default:
		return ""
	}
}

func javaPause(rows map[string]*pauseSnap, sample protocol.Sample) *pauseSnap {
	name := collectorName(sample)
	row := rows[name]
	if row == nil {
		row = &pauseSnap{name: name, p50: -1, p99: -1, before: -1, after: -1, promoted: -1}
		rows[name] = row
	}
	if cause := clip(labelOf(sample, "cause"), 200); cause != "" {
		row.cause = cause
	}
	return row
}

func writeJavaHeap(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row heapSnap) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_JAVA_HEAP (
		  pointId, batchId, agentId, tenantId,
		  heapUsedBytes, heapCommittedBytes, heapMaxBytes, heapUsedPercent,
		  nonHeapUsedBytes, nonHeapCommittedBytes, nonHeapMaxBytes,
		  metaspaceUsedBytes, metaspaceCommittedBytes, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId,
		  @heapUsedBytes, @heapCommittedBytes, @heapMaxBytes, @heapUsedPercent,
		  @nonHeapUsedBytes, @nonHeapCommittedBytes, @nonHeapMaxBytes,
		  @metaspaceUsedBytes, @metaspaceCommittedBytes, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, javaPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"heapUsedBytes": row.used, "heapCommittedBytes": row.committed, "heapMaxBytes": row.max, "heapUsedPercent": row.usedPercent,
		"nonHeapUsedBytes": row.nonUsed, "nonHeapCommittedBytes": row.nonCommitted, "nonHeapMaxBytes": row.nonMax,
		"metaspaceUsedBytes": row.metaUsed, "metaspaceCommittedBytes": row.metaCommitted,
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func writeJavaGC(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*gcSnap) error {
	for _, row := range rows {
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_JAVA_GC (
			  pointId, batchId, agentId, tenantId, collectorName, collectionCount, collectionTimeMs, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @collectorName, @collectionCount, @collectionTimeMs, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, javaPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "collectorName": row.name,
			"collectionCount": row.count, "collectionTimeMs": row.time,
			"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeJavaThread(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row threadSnap) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_JAVA_THREAD (
		  pointId, batchId, agentId, tenantId, threadCount, daemonCount, peakCount, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @threadCount, @daemonCount, @peakCount, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, javaPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"threadCount": row.count, "daemonCount": row.daemon, "peakCount": row.peak,
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func writeJavaState(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row stateSnap) error {
	deadlock := int64(-1)
	if row.hasDeadlock {
		deadlock = row.deadlock
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_JAVA_THREAD_STATE (
		  pointId, batchId, agentId, tenantId, runnableCount, blockedCount, waitingCount, deadlockCount, deadlockText, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @runnableCount, @blockedCount, @waitingCount, @deadlockCount, @deadlockText, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, javaPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"runnableCount": row.runnable, "blockedCount": row.blocked, "waitingCount": row.waiting,
		"deadlockCount": deadlock, "deadlockText": nullText(row.deadlockText),
		"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

func writeJavaPoolMax(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*poolSnap) error {
	for _, row := range rows {
		if !row.hasMax || row.max <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_JAVA_POOL_MAX (
			  pointId, batchId, agentId, tenantId, poolName, maxBytes, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @poolName, @maxBytes, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, javaPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "poolName": row.name, "maxBytes": row.max,
			"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeJavaPool(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*poolSnap) error {
	for _, row := range rows {
		if !row.hasUsed && !row.hasCommit {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_JAVA_POOL (
			  pointId, batchId, agentId, tenantId, poolName, usedBytes, committedBytes, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @poolName, @usedBytes, @committedBytes, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, javaPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "poolName": row.name,
			"usedBytes": row.used, "committedBytes": row.committed,
			"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func writeJavaRuntime(ctx context.Context, tx *dbx.DB, batch protocol.Batch, row runtimeSnap) error {
	cpu := row.cpu
	if !row.hasCPU {
		cpu = -1
	}
	overhead := -1.0
	if row.hasOverhead {
		overhead = row.overhead
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_JAVA_RUNTIME (
		  pointId, batchId, agentId, tenantId, processCpuPercent, loadedClassCount, unloadedClassCount, pauseOverheadPercent, observedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @pointId, @batchId, @agentId, @tenantId, @processCpuPercent, @loadedClassCount, @unloadedClassCount, @pauseOverheadPercent, @observedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, javaPoint(batch, dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"processCpuPercent": cpu, "loadedClassCount": row.loaded, "unloadedClassCount": row.unloaded,
		"pauseOverheadPercent": overhead,
		"observedTime":         store.FormatTime(row.at), "addWho": batch.AgentID,
	}))
	return err
}

// pauseKnown 区分没采到和测到 0。只有次数、时长、分位、堆或周期里至少有一项时才落库。
func pauseKnown(row *pauseSnap) bool {
	if row.count != 0 || row.max != 0 || row.total != 0 || row.cycleCount != 0 || row.cycleTime != 0 {
		return true
	}
	if row.p50 >= 0 || row.p99 >= 0 || row.before >= 0 || row.after >= 0 || row.promoted >= 0 {
		return true
	}
	return strings.TrimSpace(row.cause) != ""
}

func writeJavaPause(ctx context.Context, tx *dbx.DB, batch protocol.Batch, rows map[string]*pauseSnap) error {
	for _, row := range rows {
		if !pauseKnown(row) {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_JAVA_GC_PAUSE (
			  pointId, batchId, agentId, tenantId, collectorName, pauseCount, pauseMaxMs, pauseTotalMs,
			  pauseP50Ms, pauseP99Ms, heapBeforeBytes, heapAfterBytes, promotedBytes, pauseCause,
			  cycleCount, cycleTimeMs, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @pointId, @batchId, @agentId, @tenantId, @collectorName, @pauseCount, @pauseMaxMs, @pauseTotalMs,
			  @pauseP50Ms, @pauseP99Ms, @heapBeforeBytes, @heapAfterBytes, @promotedBytes, @pauseCause,
			  @cycleCount, @cycleTimeMs, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, javaPoint(batch, dbx.Args{
			"agentId": batch.AgentID, "tenantId": batch.TenantID, "collectorName": row.name,
			"pauseCount": row.count, "pauseMaxMs": row.max, "pauseTotalMs": row.total,
			"pauseP50Ms": row.p50, "pauseP99Ms": row.p99,
			"heapBeforeBytes": row.before, "heapAfterBytes": row.after, "promotedBytes": row.promoted,
			"pauseCause": nullText(row.cause), "cycleCount": row.cycleCount, "cycleTimeMs": row.cycleTime,
			"observedTime": store.FormatTime(row.at), "addWho": batch.AgentID,
		})); err != nil {
			return err
		}
	}
	return nil
}

func javaPoint(batch protocol.Batch, args dbx.Args) dbx.Args {
	args["pointId"] = id.New()
	args["batchId"] = batch.BatchID
	return args
}
