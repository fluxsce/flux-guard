package dao

import (
	"context"
	"sort"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0006/models"
	dbx "github.com/fluxsce/dbx/db"
)

const restartSQL = `
	SELECT r.openedTime, r.startTime, COALESCE(r.processId, '')
	FROM FG_JAVA_RUN r
	WHERE r.tenantId = @tenantId AND r.agentId = @agentId AND r.activeFlag = 'Y'
	  AND EXISTS (
	    SELECT 1 FROM FG_JAVA_RUN p
	    WHERE p.tenantId = r.tenantId AND p.agentId = r.agentId AND p.activeFlag = 'Y'
	      AND (p.openedTime < r.openedTime OR (p.openedTime = r.openedTime AND p.runId < r.runId))
	  )
	ORDER BY r.openedTime`

// loadRestartTimes 是窗口内每次重启后第一拍的时间。第一代本身不算重启。
func loadRestartTimes(db *dbx.DB, tenantID, agentID string, span SeriesSpan) []time.Time {
	var out []time.Time
	for _, item := range loadRestarts(db, tenantID, agentID, span) {
		if at, ok := store.ParseTime(item.ObservedTime); ok {
			out = append(out, at)
		}
	}
	return out
}

func loadRestarts(db *dbx.DB, tenantID, agentID string, span SeriesSpan) []models.JavaRestart {
	if db == nil || agentID == "" {
		return nil
	}
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID}
	query := restartSQL
	if clause := spanClause(span, args); clause != "" {
		query = strings.Replace(query, "ORDER BY r.openedTime", strings.ReplaceAll(clause, "observedTime", "r.openedTime")+" ORDER BY r.openedTime", 1)
	}
	var out []models.JavaRestart
	for _, row := range queryWide(db, query, args, 200, 3) {
		item := models.JavaRestart{
			ObservedTime: timeText(row[0]),
			StartTime:    timeText(row[1]),
			ProcessID:    strings.TrimSpace(textOf(row[2])),
		}
		if item.ObservedTime == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// crossesRestart 判断较旧的一拍和较新的一拍中间是否换过进程。
func crossesRestart(older, newer time.Time, breaks []time.Time) bool {
	for _, at := range breaks {
		if older.Before(at) && !newer.Before(at) {
			return true
		}
	}
	return false
}

func mergeRestarts(db *dbx.DB, tenantID, agentID string, span SeriesSpan, inferred []models.JavaRestart) []models.JavaRestart {
	seen := map[int64]struct{}{}
	var out []models.JavaRestart
	add := func(item models.JavaRestart) {
		at, ok := store.ParseTime(item.ObservedTime)
		if !ok {
			return
		}
		key := at.Unix()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		item.ObservedTime = timeText(at)
		out = append(out, item)
	}
	for _, item := range loadRestarts(db, tenantID, agentID, span) {
		add(item)
	}
	for _, item := range inferred {
		add(item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ObservedTime < out[j].ObservedTime })
	if out == nil {
		out = []models.JavaRestart{}
	}
	return out
}

// previousGeneration 返回上一代结束时的累计。样本还在时带上各回收器。
func previousGeneration(db *dbx.DB, tenantID, agentID string) *models.JavaGeneration {
	if db == nil || agentID == "" {
		return nil
	}
	rows := queryWide(db, `
		SELECT COALESCE(processId, ''), startTime, openedTime, closedTime, collectionCount, collectionTimeMs
		FROM FG_JAVA_RUN
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND closedTime IS NOT NULL
		ORDER BY closedTime DESC`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID}, 1, 6)
	if len(rows) == 0 {
		return nil
	}
	row := rows[0]
	closed, ok := store.TimeValue(row[3])
	if !ok {
		return nil
	}
	gen := &models.JavaGeneration{
		ProcessID:        strings.TrimSpace(textOf(row[0])),
		StartTime:        timeText(row[1]),
		OpenedTime:       timeText(row[2]),
		ClosedTime:       timeText(closed),
		CollectionCount:  asInt64(row[4]),
		CollectionTimeMs: asInt64(row[5]),
		Collectors:       collectorsBefore(db, tenantID, agentID, closed),
	}
	return gen
}

func collectorsBefore(db *dbx.DB, tenantID, agentID string, closed time.Time) []models.GCRow {
	out := []models.GCRow{}
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID, "closedTime": store.FormatTime(closed)}
	rows, err := db.Query(context.Background(), `
		SELECT collectorName, collectionCount, collectionTimeMs, observedTime
		FROM FG_JAVA_GC
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime < @closedTime
		  AND batchId = (
		    SELECT batchId FROM FG_JAVA_GC
		    WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y' AND observedTime < @closedTime
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		ORDER BY collectorName`, args)
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
		out = append(out, item)
	}
	return out
}
