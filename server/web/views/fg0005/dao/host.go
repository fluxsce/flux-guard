package dao

import (
	"context"
	"strconv"
	"strings"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

// HostFilter 是主机列表的筛选条件。
type HostFilter struct {
	HostName  string
	IPAddress string
}

// hostOrder 是主机列表允许的排序列，对应 SELECT 别名。
var hostOrder = map[string]struct{}{
	"hostName": {}, "ipAddress": {}, "cpuUsagePercent": {}, "loadAvg1": {},
	"memUsedPercent": {}, "diskUsedPercent": {}, "diskReadBytesPerSec": {},
	"diskWriteBytesPerSec": {}, "netReceiveBytesPerSec": {}, "netTransmitBytesPerSec": {},
	"tcpEstablished": {}, "tcpTimeWait": {}, "tcpCloseWait": {}, "fdAllocated": {},
	"procCount": {}, "zombieCount": {}, "reportedTime": {},
}

// List 返回本租户每台主机的当前占用。磁盘取占用最高的挂载点，IO 和流量按设备求和。
func List(db *dbx.DB, tenantID string, filter HostFilter, page dbx.Page) ([]models.HostRow, int64, error) {
	if db == nil {
		return []models.HostRow{}, 0, nil
	}
	if _, ok := hostOrder[page.OrderBy]; !ok {
		page.OrderBy = ""
	}
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE h.tenantId = @tenantId AND h.activeFlag = 'Y'"
	if name := strings.TrimSpace(filter.HostName); name != "" {
		name = strings.NewReplacer("%", "", "_", "").Replace(strings.ToLower(name))
		args["hostLike"] = "%" + name + "%"
		where += " AND (LOWER(h.hostName) LIKE @hostLike OR LOWER(COALESCE(h.reportName, '')) LIKE @hostLike)"
	}
	where += store.LikeClause(args, "h.ipAddress", "ipLike", filter.IPAddress)
	countSQL := "SELECT COUNT(*) FROM FG_AGENT_HOST h" + where
	listSQL := `
		SELECT h.agentId, h.hostName AS hostName, COALESCE(h.reportName, ''), COALESCE(h.ipAddress, '') AS ipAddress, COALESCE(h.osName, ''),
		       COALESCE(c.usagePercent, 0) AS cpuUsagePercent, COALESCE(c.loadAvg1, 0) AS loadAvg1,
		       COALESCE(m.usedPercent, 0) AS memUsedPercent, COALESCE(m.usedBytes, 0), COALESCE(m.totalBytes, 0),
		       COALESCE(d.usedPercent, 0) AS diskUsedPercent,
		       COALESCE(io.readBytesPerSec, 0) AS diskReadBytesPerSec, COALESCE(io.writeBytesPerSec, 0) AS diskWriteBytesPerSec, COALESCE(io.utilPercent, 0),
		       COALESCE(n.receiveBytesPerSec, 0) AS netReceiveBytesPerSec, COALESCE(n.transmitBytesPerSec, 0) AS netTransmitBytesPerSec,
		       COALESCE(t.establishedCount, 0) AS tcpEstablished, COALESCE(t.timeWaitCount, 0) AS tcpTimeWait, COALESCE(t.closeWaitCount, 0) AS tcpCloseWait,
		       COALESCE(f.allocatedCount, 0) AS fdAllocated, COALESCE(f.usedPercent, 0),
		       COALESCE(p.procCount, 0) AS procCount, COALESCE(p.zombieCount, 0) AS zombieCount,
		       h.reportedTime AS reportedTime
		FROM FG_AGENT_HOST h
		LEFT JOIN FG_HOST_CPU c
		  ON c.tenantId = h.tenantId AND c.pointId = (
		    SELECT pointId FROM FG_HOST_CPU
		    WHERE tenantId = h.tenantId AND agentId = h.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		LEFT JOIN FG_HOST_MEM m
		  ON m.tenantId = h.tenantId AND m.pointId = (
		    SELECT pointId FROM FG_HOST_MEM
		    WHERE tenantId = h.tenantId AND agentId = h.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		LEFT JOIN (
		  SELECT d.tenantId, d.agentId, MAX(d.usedPercent) AS usedPercent
		  FROM FG_HOST_DISK d
		  WHERE d.activeFlag = 'Y' AND d.batchId = (
		    SELECT batchId FROM FG_HOST_DISK latest
		    WHERE latest.tenantId = d.tenantId AND latest.agentId = d.agentId AND latest.activeFlag = 'Y'
		    ORDER BY latest.observedTime DESC, latest.pointId DESC
		    LIMIT 1
		  )
		  GROUP BY d.tenantId, d.agentId
		) d ON d.tenantId = h.tenantId AND d.agentId = h.agentId
		LEFT JOIN (
		  SELECT io.tenantId, io.agentId,
		         SUM(io.readBytesPerSec) AS readBytesPerSec,
		         SUM(io.writeBytesPerSec) AS writeBytesPerSec,
		         MAX(io.utilPercent) AS utilPercent
		  FROM FG_HOST_DISK_IO io
		  WHERE io.activeFlag = 'Y' AND io.batchId = (
		    SELECT batchId FROM FG_HOST_DISK_IO latest
		    WHERE latest.tenantId = io.tenantId AND latest.agentId = io.agentId AND latest.activeFlag = 'Y'
		    ORDER BY latest.observedTime DESC, latest.pointId DESC
		    LIMIT 1
		  )
		  GROUP BY io.tenantId, io.agentId
		) io ON io.tenantId = h.tenantId AND io.agentId = h.agentId
		LEFT JOIN (
		  SELECT n.tenantId, n.agentId,
		         SUM(n.receiveBytesPerSec) AS receiveBytesPerSec,
		         SUM(n.transmitBytesPerSec) AS transmitBytesPerSec
		  FROM FG_HOST_NET n
		  WHERE n.activeFlag = 'Y' AND n.batchId = (
		    SELECT batchId FROM FG_HOST_NET latest
		    WHERE latest.tenantId = n.tenantId AND latest.agentId = n.agentId AND latest.activeFlag = 'Y'
		    ORDER BY latest.observedTime DESC, latest.pointId DESC
		    LIMIT 1
		  )
		  GROUP BY n.tenantId, n.agentId
		) n ON n.tenantId = h.tenantId AND n.agentId = h.agentId
		LEFT JOIN FG_HOST_TCP t
		  ON t.tenantId = h.tenantId AND t.pointId = (
		    SELECT pointId FROM FG_HOST_TCP
		    WHERE tenantId = h.tenantId AND agentId = h.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		LEFT JOIN FG_HOST_FD f
		  ON f.tenantId = h.tenantId AND f.pointId = (
		    SELECT pointId FROM FG_HOST_FD
		    WHERE tenantId = h.tenantId AND agentId = h.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		LEFT JOIN FG_HOST_PROC p
		  ON p.tenantId = h.tenantId AND p.pointId = (
		    SELECT pointId FROM FG_HOST_PROC
		    WHERE tenantId = h.tenantId AND agentId = h.agentId AND activeFlag = 'Y'
		    ORDER BY observedTime DESC, pointId DESC
		    LIMIT 1
		  )
		` + where + `
		ORDER BY h.hostName`
	rows, total, err := db.QueryPage(context.Background(), countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.HostRow
	for rows.Next() {
		var item models.HostRow
		var cpu, load, memPct, memUsed, memTotal any
		var disk, readBps, writeBps, util, rx, tx any
		var established, timeWait, closeWait, fdAlloc, fdPct, procCount, zombie any
		var reported any
		if err := rows.Scan(
			&item.AgentID, &item.HostName, &item.ReportName, &item.IPAddress, &item.OSName,
			&cpu, &load, &memPct, &memUsed, &memTotal, &disk, &readBps, &writeBps, &util, &rx, &tx,
			&established, &timeWait, &closeWait, &fdAlloc, &fdPct, &procCount, &zombie, &reported,
		); err != nil {
			return out, total, err
		}
		item.CPUUsagePercent = asFloat(cpu)
		item.LoadAvg1 = asFloat(load)
		item.MemUsedPercent = asFloat(memPct)
		item.MemUsedBytes = asInt64(memUsed)
		item.MemTotalBytes = asInt64(memTotal)
		item.DiskUsedPercent = asFloat(disk)
		item.DiskReadBytesPerSec = asFloat(readBps)
		item.DiskWriteBytesPerSec = asFloat(writeBps)
		item.DiskUtilPercent = asFloat(util)
		item.NetReceiveBytesPerSec = asFloat(rx)
		item.NetTransmitBytesPerSec = asFloat(tx)
		item.TCPEstablished = asInt64(established)
		item.TCPTimeWait = asInt64(timeWait)
		item.TCPCloseWait = asInt64(closeWait)
		item.FDAllocated = asInt64(fdAlloc)
		item.FDUsedPercent = asFloat(fdPct)
		item.ProcCount = asInt64(procCount)
		item.ZombieCount = asInt64(zombie)
		item.ReportedTime = timeText(reported)
		out = append(out, item)
	}
	if out == nil {
		out = []models.HostRow{}
	}
	return out, total, rows.Err()
}

// Detail 返回一台机器分类存放的当前磁盘、磁盘 IO、网卡和单核占用。
func Detail(db *dbx.DB, tenantID, agentID string) models.HostDetail {
	detail := models.HostDetail{
		Disks:  []models.DiskRow{},
		DiskIO: []models.DiskIORow{},
		Nets:   []models.NetRow{},
		Cores:  []models.CoreRow{},
		Procs:  []models.ProcRow{},
		Alerts: []models.HostAlert{},
	}
	if db == nil || agentID == "" {
		return detail
	}
	args := dbx.Args{"tenantId": tenantID, "agentId": agentID}
	detail.Disks = queryDisks(db, args)
	detail.DiskIO = queryDiskIO(db, args)
	detail.Nets = queryNets(db, args)
	detail.Cores = queryCores(db, args)
	detail.Procs = queryProcs(db, args)
	detail.TCP = queryTCP(db, args)
	detail.FD = queryFD(db, args)
	detail.Alerts = queryAlerts(db, args)
	return detail
}

func queryDisks(db *dbx.DB, args dbx.Args) []models.DiskRow {
	rows, err := db.Query(context.Background(), `
		SELECT mountPoint, COALESCE(deviceName, ''), totalBytes, usedBytes, freeBytes, usedPercent, inodeUsedPercent, observedTime
		FROM FG_HOST_DISK
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		`+latestBatchSQL("FG_HOST_DISK")+`
		ORDER BY usedPercent DESC, mountPoint`, args)
	if err != nil {
		return []models.DiskRow{}
	}
	defer rows.Close()
	var out []models.DiskRow
	for rows.Next() {
		var item models.DiskRow
		var total, used, free, pct, inode, observed any
		if err := rows.Scan(&item.MountPoint, &item.DeviceName, &total, &used, &free, &pct, &inode, &observed); err != nil {
			return out
		}
		item.TotalBytes = asInt64(total)
		item.UsedBytes = asInt64(used)
		item.FreeBytes = asInt64(free)
		item.UsedPercent = asFloat(pct)
		item.InodeUsedPercent = asFloat(inode)
		item.ObservedTime = timeText(observed)
		out = append(out, item)
	}
	if out == nil {
		return []models.DiskRow{}
	}
	return out
}

func queryDiskIO(db *dbx.DB, args dbx.Args) []models.DiskIORow {
	rows, err := db.Query(context.Background(), `
		SELECT deviceName, readBytesPerSec, writeBytesPerSec, readLatencyMs, writeLatencyMs, utilPercent, observedTime
		FROM FG_HOST_DISK_IO
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		`+latestBatchSQL("FG_HOST_DISK_IO")+`
		ORDER BY utilPercent DESC, deviceName`, args)
	if err != nil {
		return []models.DiskIORow{}
	}
	defer rows.Close()
	var out []models.DiskIORow
	for rows.Next() {
		var item models.DiskIORow
		var readBps, writeBps, readLat, writeLat, util, observed any
		if err := rows.Scan(&item.DeviceName, &readBps, &writeBps, &readLat, &writeLat, &util, &observed); err != nil {
			return out
		}
		item.ReadBytesPerSec = asFloat(readBps)
		item.WriteBytesPerSec = asFloat(writeBps)
		item.ReadLatencyMs = asFloat(readLat)
		item.WriteLatencyMs = asFloat(writeLat)
		item.UtilPercent = asFloat(util)
		item.ObservedTime = timeText(observed)
		out = append(out, item)
	}
	if out == nil {
		return []models.DiskIORow{}
	}
	return out
}

func queryNets(db *dbx.DB, args dbx.Args) []models.NetRow {
	rows, err := db.Query(context.Background(), `
		SELECT nicName, receiveBytesPerSec, transmitBytesPerSec, receiveErrs, transmitErrs, receiveDrop, transmitDrop, observedTime
		FROM FG_HOST_NET
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		`+latestBatchSQL("FG_HOST_NET")+`
		ORDER BY nicName`, args)
	if err != nil {
		return []models.NetRow{}
	}
	defer rows.Close()
	var out []models.NetRow
	for rows.Next() {
		var item models.NetRow
		var rx, tx, rxErr, txErr, rxDrop, txDrop, observed any
		if err := rows.Scan(&item.NicName, &rx, &tx, &rxErr, &txErr, &rxDrop, &txDrop, &observed); err != nil {
			return out
		}
		item.ReceiveBytesPerSec = asFloat(rx)
		item.TransmitBytesPerSec = asFloat(tx)
		item.ReceiveErrs = asInt64(rxErr)
		item.TransmitErrs = asInt64(txErr)
		item.ReceiveDrop = asInt64(rxDrop)
		item.TransmitDrop = asInt64(txDrop)
		item.ObservedTime = timeText(observed)
		out = append(out, item)
	}
	if out == nil {
		return []models.NetRow{}
	}
	return out
}

func queryCores(db *dbx.DB, args dbx.Args) []models.CoreRow {
	rows, err := db.Query(context.Background(), `
		SELECT coreName, usagePercent
		FROM FG_HOST_CPU_CORE
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		`+latestBatchSQL("FG_HOST_CPU_CORE")+`
		ORDER BY coreName`, args)
	if err != nil {
		return []models.CoreRow{}
	}
	defer rows.Close()
	var out []models.CoreRow
	for rows.Next() {
		var item models.CoreRow
		var usage any
		if err := rows.Scan(&item.CoreName, &usage); err != nil {
			return out
		}
		item.UsagePercent = asFloat(usage)
		out = append(out, item)
	}
	if out == nil {
		return []models.CoreRow{}
	}
	return out
}

func queryProcs(db *dbx.DB, args dbx.Args) []models.ProcRow {
	rows, err := db.Query(context.Background(), `
		SELECT processId, processName, rssBytes, cpuPercent, observedTime
		FROM FG_HOST_PROC_TOP
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		`+latestBatchSQL("FG_HOST_PROC_TOP")+`
		ORDER BY rssBytes DESC, processName`, args)
	if err != nil {
		return []models.ProcRow{}
	}
	defer rows.Close()
	var out []models.ProcRow
	for rows.Next() {
		var item models.ProcRow
		var rss, cpu, observed any
		if err := rows.Scan(&item.ProcessID, &item.ProcessName, &rss, &cpu, &observed); err != nil {
			return out
		}
		item.RSSBytes = asInt64(rss)
		item.CPUPercent = asFloat(cpu)
		item.ObservedTime = timeText(observed)
		out = append(out, item)
	}
	if out == nil {
		return []models.ProcRow{}
	}
	return out
}

func queryTCP(db *dbx.DB, args dbx.Args) models.TcpRow {
	var established, listen, timeWait, closeWait, synSent, synRecv any
	var finWait, lastAck, closing, inUse, udp, retrans any
	err := store.Scan(context.Background(), db, `
		SELECT establishedCount, listenCount, timeWaitCount, closeWaitCount, synSentCount, synRecvCount,
		       finWaitCount, lastAckCount, closingCount, tcpInUseCount, udpInUseCount, retransmitCount
		FROM FG_HOST_TCP
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY observedTime DESC, pointId DESC
		LIMIT 1`, args,
		&established, &listen, &timeWait, &closeWait, &synSent, &synRecv,
		&finWait, &lastAck, &closing, &inUse, &udp, &retrans)
	if err != nil {
		return models.TcpRow{}
	}
	return models.TcpRow{
		EstablishedCount: asInt64(established), ListenCount: asInt64(listen),
		TimeWaitCount: asInt64(timeWait), CloseWaitCount: asInt64(closeWait),
		SynSentCount: asInt64(synSent), SynRecvCount: asInt64(synRecv),
		FinWaitCount: asInt64(finWait), LastAckCount: asInt64(lastAck), ClosingCount: asInt64(closing),
		TCPInUseCount: asInt64(inUse), UDPInUseCount: asInt64(udp), RetransmitCount: asInt64(retrans),
	}
}

func queryFD(db *dbx.DB, args dbx.Args) models.FdRow {
	var allocated, maximum, pct any
	err := store.Scan(context.Background(), db, `
		SELECT allocatedCount, maximumCount, usedPercent
		FROM FG_HOST_FD
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY observedTime DESC, pointId DESC
		LIMIT 1`, args,
		&allocated, &maximum, &pct)
	if err != nil {
		return models.FdRow{}
	}
	return models.FdRow{AllocatedCount: asInt64(allocated), MaximumCount: asInt64(maximum), UsedPercent: asFloat(pct)}
}

func queryAlerts(db *dbx.DB, args dbx.Args) []models.HostAlert {
	rows, err := db.Query(context.Background(), `
		SELECT i.incidentId, COALESCE(r.ruleName, ''), i.metricCode, i.levelCode, i.statusCode,
		       i.currentValue, i.thresholdValue, i.compareOp, i.firstFireTime
		FROM FG_HOST_INCIDENT i
		LEFT JOIN FG_ALERT_RULE r ON r.tenantId = i.tenantId AND r.ruleId = i.ruleId AND r.alertType = 'host'
		WHERE i.tenantId = @tenantId AND i.agentId = @agentId AND i.activeFlag = 'Y'
		  AND i.statusCode IN ('pending', 'firing', 'acked')
		ORDER BY i.firstFireTime DESC
		LIMIT 20`, args)
	if err != nil {
		return []models.HostAlert{}
	}
	defer rows.Close()
	out := []models.HostAlert{}
	for rows.Next() {
		var item models.HostAlert
		var current, threshold, fired any
		if err := rows.Scan(&item.IncidentID, &item.RuleName, &item.MetricCode, &item.LevelCode, &item.StatusCode, &current, &threshold, &item.CompareOp, &fired); err != nil {
			return out
		}
		item.CurrentValue = asFloat(current)
		item.ThresholdValue = asFloat(threshold)
		item.FirstFireTime = timeText(fired)
		out = append(out, item)
	}
	return out
}

func latestBatchSQL(table string) string {
	return `AND batchId = (
		SELECT batchId FROM ` + table + `
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY observedTime DESC, pointId DESC
		LIMIT 1
	)`
}

func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case []byte:
		f, _ := strconv.ParseFloat(string(n), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case []byte:
		i, err := strconv.ParseInt(string(n), 10, 64)
		if err != nil {
			f, _ := strconv.ParseFloat(string(n), 64)
			return int64(f)
		}
		return i
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			f, _ := strconv.ParseFloat(n, 64)
			return int64(f)
		}
		return i
	default:
		return 0
	}
}

func timeText(v any) string {
	if t, ok := store.TimeValue(v); ok {
		return t.UTC().Format(time.RFC3339)
	}
	return ""
}
