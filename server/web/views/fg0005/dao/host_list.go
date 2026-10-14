package dao

import (
	"context"
	"strings"
	"time"

	"fluxguard/server/utils/const/limits"
	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0005/models"
	dbx "github.com/fluxsce/dbx/db"
)

// List 返回本租户每台主机的当前占用。当前值在上报时写入 FG_AGENT_HOST，这里只查这一张表。
func List(db *dbx.DB, tenantID string, filter HostFilter, page dbx.Page) ([]models.HostRow, int64, error) {
	if db == nil {
		return []models.HostRow{}, 0, nil
	}
	if _, ok := hostOrder[page.OrderBy]; !ok {
		page.OrderBy = ""
	}
	args, where := hostFilterSQL(tenantID, filter)
	listSQL := `
		SELECT h.agentId, h.hostName AS hostName, COALESCE(h.reportName, ''), COALESCE(h.ipAddress, '') AS ipAddress, COALESCE(h.osName, ''),
		       COALESCE(h.cpuUsagePercent, 0) AS cpuUsagePercent, COALESCE(h.loadAvg1, 0) AS loadAvg1,
		       COALESCE(h.memUsedPercent, 0) AS memUsedPercent, COALESCE(h.memUsedBytes, 0), COALESCE(h.memTotalBytes, 0),
		       COALESCE(h.diskUsedPercent, 0) AS diskUsedPercent,
		       COALESCE(h.diskReadBytesPerSec, 0) AS diskReadBytesPerSec, COALESCE(h.diskWriteBytesPerSec, 0) AS diskWriteBytesPerSec, COALESCE(h.diskUtilPercent, 0),
		       COALESCE(h.netReceiveBytesPerSec, 0) AS netReceiveBytesPerSec, COALESCE(h.netTransmitBytesPerSec, 0) AS netTransmitBytesPerSec,
		       COALESCE(h.tcpEstablished, 0) AS tcpEstablished, COALESCE(h.tcpTimeWait, 0) AS tcpTimeWait, COALESCE(h.tcpCloseWait, 0) AS tcpCloseWait,
		       COALESCE(h.fdAllocated, 0) AS fdAllocated, COALESCE(h.fdUsedPercent, 0),
		       COALESCE(h.procCount, 0) AS procCount, COALESCE(h.zombieCount, 0) AS zombieCount,
		       h.reportedTime AS reportedTime
		FROM FG_AGENT_HOST h` + where + `
		ORDER BY h.hostName`
	rows, total, err := db.QueryPage(context.Background(), "SELECT COUNT(*) FROM FG_AGENT_HOST h"+where, listSQL, page, args)
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
		item.OnlineFlag = "N"
		if at, ok := store.TimeValue(reported); ok && time.Since(at) <= limits.OnlineWindow {
			item.OnlineFlag = "Y"
		}
		out = append(out, item)
	}
	if out == nil {
		out = []models.HostRow{}
	}
	return out, total, rows.Err()
}

func hostFilterSQL(tenantID string, filter HostFilter) (dbx.Args, string) {
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE h.tenantId = @tenantId AND h.activeFlag = 'Y'"
	if name := strings.TrimSpace(filter.HostName); name != "" {
		name = strings.NewReplacer("%", "", "_", "").Replace(strings.ToLower(name))
		args["hostLike"] = "%" + name + "%"
		where += " AND (LOWER(h.hostName) LIKE @hostLike OR LOWER(COALESCE(h.reportName, '')) LIKE @hostLike)"
	}
	where += store.LikeClause(args, "h.ipAddress", "ipLike", filter.IPAddress)
	return args, where
}
