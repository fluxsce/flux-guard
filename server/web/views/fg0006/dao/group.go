package dao

import (
	"context"
	"sort"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/views/fg0006/models"
	dbx "github.com/fluxsce/dbx/db"
)

type scannedInstance struct {
	row       models.JavaRow
	machineID string
	reported  time.Time
}

func listInstances(db *dbx.DB, tenantID string, filter JavaFilter) ([]models.JavaRow, error) {
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE j.tenantId = @tenantId AND j.activeFlag = 'Y'"
	where += store.LikeClause(args, "j.appName", "appLike", filter.AppName)
	where += store.LikeClause(args, "j.hostName", "hostLike", filter.HostName)
	rows, err := db.Query(context.Background(), `
		SELECT j.agentId, j.appName AS appName, COALESCE(j.mainClass, ''), COALESCE(j.processId, ''),
		       COALESCE(j.javaVersion, '') AS javaVersion, j.startTime, COALESCE(j.hostName, '') AS hostName,
		       COALESCE(j.ipAddress, '') AS ipAddress,
		       COALESCE(j.heapUsedPercent, 0) AS heapUsedPercent, COALESCE(j.heapUsedBytes, 0), COALESCE(j.heapMaxBytes, 0),
		       COALESCE(j.nonHeapUsedBytes, 0), COALESCE(j.rssBytes, -1),
		       COALESCE(j.threadCount, 0) AS threadCount, COALESCE(j.daemonCount, 0),
		       COALESCE(j.collectionCount, 0) AS collectionCount, COALESCE(j.collectionTimeMs, 0),
		       j.reportedTime AS reportedTime, COALESCE(a.machineId, '')
		FROM FG_AGENT_JAVA j
		LEFT JOIN FG_AGENT a ON a.tenantId = j.tenantId AND a.agentId = j.agentId AND a.activeFlag = 'Y'`+where,
		args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scanned []scannedInstance
	for rows.Next() {
		var item scannedInstance
		var start, heapPct, heapUsed, heapMax, nonUsed, rss, threads, daemon, gcCount, gcTime, reported any
		if err := rows.Scan(
			&item.row.AgentID, &item.row.AppName, &item.row.MainClass, &item.row.ProcessID, &item.row.JavaVersion, &start,
			&item.row.HostName, &item.row.IPAddress, &heapPct, &heapUsed, &heapMax, &nonUsed, &rss,
			&threads, &daemon, &gcCount, &gcTime, &reported, &item.machineID,
		); err != nil {
			return nil, err
		}
		item.row.StartTime = timeText(start)
		item.row.HeapUsedPercent = asFloat(heapPct)
		if item.row.HeapUsedPercent >= 0 {
			item.row.HeapUsedPercent = shownPercent(item.row.HeapUsedPercent)
		}
		item.row.HeapUsedBytes = asInt64(heapUsed)
		item.row.HeapMaxBytes = asInt64(heapMax)
		item.row.NonHeapUsedBytes = asInt64(nonUsed)
		item.row.RSSBytes = asInt64(rss)
		item.row.ThreadCount = asInt64(threads)
		item.row.DaemonCount = asInt64(daemon)
		item.row.CollectionCount = asInt64(gcCount)
		item.row.CollectionTimeMs = asInt64(gcTime)
		item.row.ReportedTime = timeText(reported)
		item.row.OnlineFlag = "N"
		if at, ok := store.TimeValue(reported); ok {
			item.reported = at
			if time.Since(at) <= limits.OnlineWindow {
				item.row.OnlineFlag = "Y"
			}
		}
		scanned = append(scanned, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return collapseInstances(scanned), nil
}

// collapseInstances 把同一主机上的历史进程收成最近一次上报。
// 配置了实例名的编号含有 @，同一台机器上的多份服务各自保留。
func collapseInstances(rows []scannedInstance) []models.JavaRow {
	best := map[string]scannedInstance{}
	var order []string
	for _, item := range rows {
		key := instanceSlot(item)
		prev, ok := best[key]
		if ok && !item.reported.After(prev.reported) {
			continue
		}
		if !ok {
			order = append(order, key)
		}
		best[key] = item
	}
	// 新编号是「服务@实例」。同一台机器已经有这种记录时，不再列出按进程号拆开的旧行。
	live := map[string]bool{}
	for _, item := range best {
		if strings.Contains(strings.TrimSpace(item.machineID), "@") {
			live[item.row.AppName+"\x00"+item.row.HostName] = true
		}
	}
	out := make([]models.JavaRow, 0, len(order))
	for _, key := range order {
		item := best[key]
		if !strings.Contains(strings.TrimSpace(item.machineID), "@") && live[item.row.AppName+"\x00"+item.row.HostName] {
			continue
		}
		out = append(out, item.row)
	}
	return out
}

func instanceSlot(item scannedInstance) string {
	machine := strings.TrimSpace(item.machineID)
	if strings.Contains(machine, "@") {
		return "s\x00" + machine
	}
	return "h\x00" + item.row.AppName + "\x00" + item.row.HostName
}

func reportedAt(row models.JavaRow) time.Time {
	at, ok := store.ParseTime(row.ReportedTime)
	if !ok {
		return time.Time{}
	}
	return at
}

func sortInstances(rows []models.JavaRow, page dbx.Page) {
	key := page.OrderBy
	if key == "" {
		key = "appName"
	}
	sort.Slice(rows, func(i, j int) bool {
		cmp := instanceCompare(rows[i], rows[j], key)
		if cmp == 0 {
			cmp = strings.Compare(rows[i].AgentID, rows[j].AgentID)
		}
		if page.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func instanceCompare(left, right models.JavaRow, key string) int {
	switch key {
	case "hostName":
		return strings.Compare(left.HostName, right.HostName)
	case "ipAddress":
		return strings.Compare(left.IPAddress, right.IPAddress)
	case "javaVersion":
		return strings.Compare(left.JavaVersion, right.JavaVersion)
	case "heapUsedPercent":
		return compareFloat(left.HeapUsedPercent, right.HeapUsedPercent)
	case "threadCount":
		return compareInt(left.ThreadCount, right.ThreadCount)
	case "collectionCount":
		return compareInt(left.CollectionCount, right.CollectionCount)
	case "reportedTime":
		return reportedAt(left).Compare(reportedAt(right))
	default:
		return strings.Compare(left.AppName, right.AppName)
	}
}

func compareFloat(left, right float64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func compareInt(left, right int64) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func pageInstances(rows []models.JavaRow, page dbx.Page) []models.JavaRow {
	if len(rows) == 0 {
		return []models.JavaRow{}
	}
	size := page.PageSize
	if size <= 0 {
		size = len(rows)
	}
	pageNo := page.Page
	if pageNo < 1 {
		pageNo = 1
	}
	start := (pageNo - 1) * size
	if start >= len(rows) {
		return []models.JavaRow{}
	}
	end := start + size
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}
