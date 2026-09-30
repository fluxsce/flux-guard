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

// List 返回本租户每个 Java 进程的当前堆、线程和回收累计。
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
		       COALESCE(h.heapUsedPercent, 0) AS heapUsedPercent, COALESCE(h.heapUsedBytes, 0), COALESCE(h.heapMaxBytes, 0),
		       COALESCE(h.nonHeapUsedBytes, 0),
		       COALESCE(t.threadCount, 0) AS threadCount, COALESCE(t.daemonCount, 0),
		       COALESCE(g.collectionCount, 0) AS collectionCount, COALESCE(g.collectionTimeMs, 0),
		       j.reportedTime AS reportedTime
		FROM FG_AGENT_JAVA j
		LEFT JOIN FG_JAVA_HEAP h
		  ON h.tenantId = j.tenantId AND h.pointId = (
		    SELECT pointId FROM FG_JAVA_HEAP
		    WHERE tenantId = j.tenantId AND agentId = j.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		LEFT JOIN FG_JAVA_THREAD t
		  ON t.tenantId = j.tenantId AND t.pointId = (
		    SELECT pointId FROM FG_JAVA_THREAD
		    WHERE tenantId = j.tenantId AND agentId = j.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		LEFT JOIN (
		  SELECT g.tenantId, g.agentId,
		         SUM(g.collectionCount) AS collectionCount,
		         SUM(g.collectionTimeMs) AS collectionTimeMs
		  FROM FG_JAVA_GC g
		  WHERE g.activeFlag = 'Y' AND g.batchId = (
		    SELECT batchId FROM FG_JAVA_GC latest
		    WHERE latest.tenantId = g.tenantId AND latest.agentId = g.agentId AND latest.activeFlag = 'Y'
		    ORDER BY latest.observedTime DESC, latest.pointId DESC
		    LIMIT 1
		  )
		  GROUP BY g.tenantId, g.agentId
		) g ON g.tenantId = j.tenantId AND g.agentId = j.agentId
		` + where + `
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

// Detail 返回一个进程最新一批回收器。
func Detail(db *dbx.DB, tenantID, agentID string) models.JavaDetail {
	out := models.JavaDetail{Collectors: []models.GCRow{}}
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
	return out
}
