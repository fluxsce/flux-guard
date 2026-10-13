package dao

import (
	"context"
	"strings"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0006/models"
	dbx "github.com/fluxsce/dbx/db"
)

// JavaFilter 是 Java 应用列表的筛选条件。
type JavaFilter struct {
	AppName  string
	HostName string
}

var javaOrder = map[string]struct{}{
	"appName": {}, "hostName": {}, "ipAddress": {}, "javaVersion": {},
	"heapUsedPercent": {}, "threadCount": {}, "collectionCount": {}, "reportedTime": {},
}

// List 返回本租户每个 Java 进程的当前堆、线程和回收累计。当前值在上报时写入 FG_AGENT_JAVA。
func List(db *dbx.DB, tenantID string, filter JavaFilter, page dbx.Page) ([]models.JavaRow, int64, error) {
	if db == nil {
		return []models.JavaRow{}, 0, nil
	}
	if _, ok := javaOrder[page.OrderBy]; !ok {
		page.OrderBy = ""
	}
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE j.tenantId = @tenantId AND j.activeFlag = 'Y'"
	where += store.LikeClause(args, "j.appName", "appLike", filter.AppName)
	where += store.LikeClause(args, "j.hostName", "hostLike", filter.HostName)
	countSQL := "SELECT COUNT(*) FROM FG_AGENT_JAVA j" + where
	listSQL := `
		SELECT j.agentId, j.appName AS appName, COALESCE(j.mainClass, ''), COALESCE(j.processId, ''),
		       COALESCE(j.javaVersion, '') AS javaVersion, j.startTime, COALESCE(j.hostName, '') AS hostName,
		       COALESCE(j.ipAddress, '') AS ipAddress,
		       COALESCE(j.heapUsedPercent, 0) AS heapUsedPercent, COALESCE(j.heapUsedBytes, 0), COALESCE(j.heapMaxBytes, 0),
		       COALESCE(j.nonHeapUsedBytes, 0),
		       COALESCE(j.threadCount, 0) AS threadCount, COALESCE(j.daemonCount, 0),
		       COALESCE(j.collectionCount, 0) AS collectionCount, COALESCE(j.collectionTimeMs, 0),
		       j.reportedTime AS reportedTime
		FROM FG_AGENT_JAVA j` + where + `
		ORDER BY j.appName`
	rows, total, err := db.QueryPage(context.Background(), countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.JavaRow
	for rows.Next() {
		var item models.JavaRow
		var start, heapPct, heapUsed, heapMax, nonUsed, threads, daemon, gcCount, gcTime, reported any
		if err := rows.Scan(
			&item.AgentID, &item.AppName, &item.MainClass, &item.ProcessID, &item.JavaVersion, &start,
			&item.HostName, &item.IPAddress, &heapPct, &heapUsed, &heapMax, &nonUsed,
			&threads, &daemon, &gcCount, &gcTime, &reported,
		); err != nil {
			return out, total, err
		}
		item.StartTime = timeText(start)
		item.HeapUsedPercent = asFloat(heapPct)
		if item.HeapUsedPercent >= 0 {
			item.HeapUsedPercent = shownPercent(item.HeapUsedPercent)
		}
		item.HeapUsedBytes = asInt64(heapUsed)
		item.HeapMaxBytes = asInt64(heapMax)
		item.NonHeapUsedBytes = asInt64(nonUsed)
		item.ThreadCount = asInt64(threads)
		item.DaemonCount = asInt64(daemon)
		item.CollectionCount = asInt64(gcCount)
		item.CollectionTimeMs = asInt64(gcTime)
		item.ReportedTime = timeText(reported)
		out = append(out, item)
	}
	if out == nil {
		out = []models.JavaRow{}
	}
	attachFindings(db, tenantID, out)
	return out, total, rows.Err()
}

// Detail 返回一个进程最新一批回收器，以及与判断同一行的最近一拍。
func Detail(db *dbx.DB, tenantID, agentID string) models.JavaDetail {
	out := models.JavaDetail{Collectors: []models.GCRow{}, Beat: emptyBeat()}
	if db == nil || strings.TrimSpace(agentID) == "" {
		return out
	}
	rows, err := db.Query(context.Background(), `
		SELECT collectorName, collectionCount, collectionTimeMs, observedTime
		FROM FG_JAVA_GC
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND batchId = (
		  SELECT batchId FROM FG_JAVA_GC
		  WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  ORDER BY observedTime DESC, pointId DESC
		  LIMIT 1
		)
		ORDER BY collectorName`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID})
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var item models.GCRow
		var count, elapsed, observed any
		if err := rows.Scan(&item.CollectorName, &count, &elapsed, &observed); err != nil {
			return out
		}
		item.CollectionCount = asInt64(count)
		item.CollectionTimeMs = asInt64(elapsed)
		item.ObservedTime = timeText(observed)
		out.Collectors = append(out.Collectors, item)
	}
	out.FindingLevel, out.Findings = judgeAgent(db, tenantID, agentID)
	out.Beat = loadBeat(db, tenantID, agentID)
	out.Previous = previousGeneration(db, tenantID, agentID)
	return out
}

func emptyBeat() models.JavaBeat {
	return models.JavaBeat{OverheadPercent: -1, DeadlockCount: -1, Pauses: []models.JavaPause{}}
}

// loadBeat 读判断用的同一行：运行时最新开销、线程状态最新死锁、停顿表最新时刻的全部回收器。
func loadBeat(db *dbx.DB, tenantID, agentID string) models.JavaBeat {
	out := emptyBeat()
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID}
	runtime, err := db.Query(context.Background(), `
		SELECT pauseOverheadPercent, observedTime
		FROM FG_JAVA_RUNTIME
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY observedTime DESC, pointId DESC
		LIMIT 1`, args)
	if err == nil {
		if runtime.Next() {
			var overhead, observed any
			if err := runtime.Scan(&overhead, &observed); err == nil {
				out.OverheadPercent = asFloat(overhead)
				out.OverheadTime = timeText(observed)
			}
		}
		runtime.Close()
	}
	state, err := db.Query(context.Background(), `
		SELECT deadlockCount, COALESCE(deadlockText, ''), observedTime
		FROM FG_JAVA_THREAD_STATE
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY observedTime DESC, pointId DESC
		LIMIT 1`, args)
	if err == nil {
		if state.Next() {
			var dead, text, observed any
			if err := state.Scan(&dead, &text, &observed); err == nil {
				out.DeadlockCount = asInt64(dead)
				out.DeadlockText = strings.TrimSpace(textOf(text))
				out.DeadlockTime = timeText(observed)
			}
		}
		state.Close()
	}
	pauses, err := db.Query(context.Background(), `
		SELECT collectorName, pauseCount, pauseMaxMs, pauseTotalMs, pauseP50Ms, pauseP99Ms,
		       heapBeforeBytes, heapAfterBytes, promotedBytes, COALESCE(pauseCause, ''),
		       cycleCount, cycleTimeMs, observedTime
		FROM FG_JAVA_GC_PAUSE
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = (
		    SELECT observedTime FROM FG_JAVA_GC_PAUSE
		    WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		ORDER BY collectorName`, args)
	if err != nil {
		return out
	}
	defer pauses.Close()
	for pauses.Next() {
		var item models.JavaPause
		var count, max, total, p50, p99, before, after, promoted, cause, cycleCount, cycleTime, observed any
		if err := pauses.Scan(
			&item.CollectorName, &count, &max, &total, &p50, &p99,
			&before, &after, &promoted, &cause, &cycleCount, &cycleTime, &observed,
		); err != nil {
			return out
		}
		item.PauseCount = asInt64(count)
		item.PauseMaxMs = asInt64(max)
		item.PauseTotalMs = asInt64(total)
		item.PauseP50Ms = asInt64(p50)
		item.PauseP99Ms = asInt64(p99)
		item.HeapBeforeBytes = asInt64(before)
		item.HeapAfterBytes = asInt64(after)
		item.PromotedBytes = asInt64(promoted)
		item.PauseCause = strings.TrimSpace(textOf(cause))
		item.CycleCount = asInt64(cycleCount)
		item.CycleTimeMs = asInt64(cycleTime)
		item.ObservedTime = timeText(observed)
		out.Pauses = append(out.Pauses, item)
	}
	return out
}
