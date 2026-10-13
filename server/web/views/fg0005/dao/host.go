package dao

import (
	"context"
	"strconv"
	"strings"
	"time"

	"fluxguard/common/hostnic"
	"fluxguard/common/hostvol"
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

// Detail 返回一台机器分类存放的当前磁盘、磁盘 IO、网卡和单核占用。
func Detail(db *dbx.DB, tenantID, agentID string) models.HostDetail {
	detail := models.HostDetail{
		Disks:  []models.DiskRow{},
		DiskIO: []models.DiskIORow{},
		Nets:   []models.NetRow{},
		Cores:  []models.CoreRow{},
		Procs:  []models.ProcRow{},
		ProcIO: []models.ProcIORow{},
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
	detail.ProcIO = queryProcIO(db, args)
	detail.TCP = queryTCP(db, args)
	detail.FD = queryFD(db, args)
	detail.Alerts = queryAlerts(db, args)
	return detail
}

func queryDisks(db *dbx.DB, args dbx.Args) []models.DiskRow {
	args, ok := bindLatestBatch(db, "FG_HOST_DISK", args)
	if !ok {
		return []models.DiskRow{}
	}
	rows, err := db.Query(context.Background(), `
		SELECT mountPoint, COALESCE(deviceName, ''), totalBytes, usedBytes, freeBytes, usedPercent, inodeUsedPercent, observedTime
		FROM FG_HOST_DISK
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = @observedTime AND batchId = @batchId
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
		if hostvol.Container(item.MountPoint) {
			continue
		}
		out = append(out, item)
	}
	if out == nil {
		return []models.DiskRow{}
	}
	return out
}

func queryDiskIO(db *dbx.DB, args dbx.Args) []models.DiskIORow {
	args, ok := bindLatestBatch(db, "FG_HOST_DISK_IO", args)
	if !ok {
		return []models.DiskIORow{}
	}
	rows, err := db.Query(context.Background(), `
		SELECT deviceName, readBytesPerSec, writeBytesPerSec, readLatencyMs, writeLatencyMs, utilPercent, observedTime
		FROM FG_HOST_DISK_IO
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = @observedTime AND batchId = @batchId
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
		if hostvol.Partition(item.DeviceName) {
			continue
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
	args, ok := bindLatestBatch(db, "FG_HOST_NET", args)
	if !ok {
		return []models.NetRow{}
	}
	rows, err := db.Query(context.Background(), `
		SELECT nicName, receiveBytesPerSec, transmitBytesPerSec, receiveErrs, transmitErrs, receiveDrop, transmitDrop, observedTime
		FROM FG_HOST_NET
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = @observedTime AND batchId = @batchId
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
		if hostnic.Container(item.NicName) {
			continue
		}
		out = append(out, item)
	}
	if out == nil {
		return []models.NetRow{}
	}
	return out
}

func queryCores(db *dbx.DB, args dbx.Args) []models.CoreRow {
	args, ok := bindLatestBatch(db, "FG_HOST_CPU_CORE", args)
	if !ok {
		return []models.CoreRow{}
	}
	rows, err := db.Query(context.Background(), `
		SELECT coreName, usagePercent
		FROM FG_HOST_CPU_CORE
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = @observedTime AND batchId = @batchId
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
	args, ok := bindLatestBatch(db, "FG_HOST_PROC_TOP", args)
	if !ok {
		return []models.ProcRow{}
	}
	rows, err := db.Query(context.Background(), `
		SELECT processId, processName, rssBytes, cpuPercent, observedTime
		FROM FG_HOST_PROC_TOP
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = @observedTime AND batchId = @batchId
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

func queryProcIO(db *dbx.DB, args dbx.Args) []models.ProcIORow {
	args, ok := bindLatestBatch(db, "FG_HOST_PROC_IO", args)
	if !ok {
		return []models.ProcIORow{}
	}
	rows, err := db.Query(context.Background(), `
		SELECT processId, processName, readBytesPerSec, writeBytesPerSec, observedTime
		FROM FG_HOST_PROC_IO
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND observedTime = @observedTime AND batchId = @batchId
		ORDER BY readBytesPerSec + writeBytesPerSec DESC, processName`, args)
	if err != nil {
		return []models.ProcIORow{}
	}
	defer rows.Close()
	var out []models.ProcIORow
	for rows.Next() {
		var item models.ProcIORow
		var readBps, writeBps, observed any
		if err := rows.Scan(&item.ProcessID, &item.ProcessName, &readBps, &writeBps, &observed); err != nil {
			return out
		}
		if omitProcIO(item.ProcessID, item.ProcessName) {
			continue
		}
		item.ReadBytesPerSec = asFloat(readBps)
		item.WriteBytesPerSec = asFloat(writeBps)
		item.ObservedTime = timeText(observed)
		out = append(out, item)
	}
	if out == nil {
		return []models.ProcIORow{}
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
		ORDER BY observedTime DESC
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
		ORDER BY observedTime DESC
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

// bindLatestBatch 先沿 (tenantId, agentId, observedTime) 取最新一行，再把这一批的时间写回参数。
// table 只能是调用方写死的表名。找不到时返回 false。
func bindLatestBatch(db *dbx.DB, table string, args dbx.Args) (dbx.Args, bool) {
	var batchID string
	var observed any
	err := store.Scan(context.Background(), db, `
		SELECT batchId, observedTime FROM `+table+`
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY observedTime DESC
		LIMIT 1`, args, &batchID, &observed)
	if err != nil || strings.TrimSpace(batchID) == "" {
		return nil, false
	}
	next := dbx.Args{}
	for key, value := range args {
		next[key] = value
	}
	next["batchId"] = batchID
	next["observedTime"] = observed
	return next, true
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
