package dao

import (
	"context"
	"strings"

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

// List 返回每个 Java 实例的当前占用，平铺成一行。
// 同一实例重启不另起一行；同一主机上按进程号留下的旧记录只保留最近一条。
func List(db *dbx.DB, tenantID string, filter JavaFilter, page dbx.Page) ([]models.JavaRow, int64, error) {
	if db == nil {
		return []models.JavaRow{}, 0, nil
	}
	if _, ok := javaOrder[page.OrderBy]; !ok {
		page.OrderBy = ""
	}
	instances, err := listInstances(db, tenantID, filter)
	if err != nil {
		return nil, 0, err
	}
	attachFindings(db, tenantID, instances)
	sortInstances(instances, page)
	return pageInstances(instances, page), int64(len(instances)), nil
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
