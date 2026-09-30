package dao

import (
	"context"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"
	"fluxguard/server/web/ingest/models"
	dbx "github.com/fluxsce/dbx/db"
)

// ErrDuplicate 表示同一租户下该批次号已经接收过。
var ErrDuplicate = errors.New("duplicate batch")

// InsertBatch 记下这一批。已存在时返回 ErrDuplicate，调用方按成功且重复处理。
func InsertBatch(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	var existing int
	if err := store.Scan(ctx, tx, `SELECT count(*) FROM FG_INGEST_BATCH WHERE tenantId = @tenantId AND batchId = @batchId`,
		dbx.Args{"tenantId": batch.TenantID, "batchId": batch.BatchID}, &existing); err != nil {
		return err
	}
	if existing > 0 {
		return ErrDuplicate
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_INGEST_BATCH (
		  batchId, tenantId, agentId, agentCode, sampleCount, eventCount, acceptedFlag,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (@batchId, @tenantId, @agentId, @agentCode, @sampleCount, @eventCount, 'Y', @addWho, 'system', 'INGEST', 1, 'Y')`,
		dbx.Args{
			"batchId": batch.BatchID, "tenantId": batch.TenantID, "agentId": batch.AgentID, "agentCode": batch.AgentCode,
			"sampleCount": len(batch.Samples), "eventCount": len(batch.Events), "addWho": batch.AgentID,
		})
	return err
}

// InsertEvents 写入本批事件。
func InsertEvents(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	for _, event := range batch.Events {
		labels, err := marshalLabels(event.Labels)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO FG_EVENT_POINT (
			  eventId, tenantId, batchId, agentId, agentCode, eventName, severity, messageText, labelJson, observedTime,
			  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @eventId, @tenantId, @batchId, @agentId, @agentCode, @eventName, @severity, @messageText, @labelJson, @observedTime,
			  @addWho, 'system', 'INGEST', 1, 'Y'
			)`, dbx.Args{
			"eventId": id.New(), "tenantId": batch.TenantID, "batchId": batch.BatchID, "agentId": batch.AgentID,
			"agentCode": batch.AgentCode, "eventName": strings.TrimSpace(event.EventName), "severity": event.Severity,
			"messageText": nullText(event.MessageText), "labelJson": labels, "observedTime": store.FormatTime(event.ObservedAt),
			"addWho": batch.AgentID,
		}); err != nil {
			return err
		}
	}
	return nil
}

// SaveHost 更新主机资料。没有主机块时什么都不写。
func SaveHost(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	if batch.Host == nil {
		return nil
	}
	host := batch.Host
	reported := store.FormatTime(batch.SentAt)
	edited := store.FormatTime(time.Now().UTC())
	var boot any
	if !host.BootTime.IsZero() {
		boot = store.FormatTime(host.BootTime)
	}
	incomingName := strings.TrimSpace(host.ReportName)
	var existing int
	var keptName, keptCommand, keptIP, keptAgent string
	if err := store.Scan(ctx, tx, `
		SELECT count(*), COALESCE(max(reportName), ''), COALESCE(max(commandAddress), '')
		FROM FG_AGENT_HOST WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID}, &existing, &keptName, &keptCommand); err != nil {
		return err
	}
	reportName := incomingName
	if reportName == "" {
		reportName = keptName
	}
	commandAddress := strings.TrimSpace(host.CommandAddress)
	if commandAddress == "" {
		commandAddress = keptCommand
	}
	args := dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID, "hostName": strings.TrimSpace(host.HostName),
		"osName": host.OSName, "osVersion": nullText(host.OSVersion), "archName": host.ArchName,
		"cpuCount": host.CPUCount, "memTotalBytes": host.MemTotalBytes, "ipAddress": nullText(host.IPAddress),
		"bootTime": boot, "reportName": nullText(reportName), "reportedTime": reported, "editTime": edited,
		"machineId": nullText(host.MachineID), "kernelVersion": nullText(host.KernelVersion),
		"agentVersion": nullText(host.AgentVersion), "ipList": nullText(host.IPList),
		"commandAddress": nullText(commandAddress),
		"addWho":         batch.AgentID,
	}
	if existing > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE FG_AGENT_HOST SET
			  hostName = @hostName, osName = @osName, osVersion = @osVersion, archName = @archName,
			  cpuCount = @cpuCount, memTotalBytes = @memTotalBytes, ipAddress = @ipAddress, bootTime = @bootTime,
			  reportName = @reportName, machineId = @machineId, kernelVersion = @kernelVersion,
			  agentVersion = @agentVersion, ipList = @ipList, commandAddress = @commandAddress,
			  reportedTime = @reportedTime, editTime = @editTime, editWho = 'system',
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND agentId = @agentId`, args); err != nil {
			return err
		}
	} else if _, err := tx.Exec(ctx, `
		INSERT INTO FG_AGENT_HOST (
		  agentId, tenantId, hostName, osName, osVersion, archName, cpuCount, memTotalBytes,
		  ipAddress, bootTime, reportName, machineId, kernelVersion, agentVersion, ipList, commandAddress,
		  reportedTime, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, @tenantId, @hostName, @osName, @osVersion, @archName, @cpuCount, @memTotalBytes,
		  @ipAddress, @bootTime, @reportName, @machineId, @kernelVersion, @agentVersion, @ipList, @commandAddress,
		  @reportedTime, @addWho, 'system', 'INGEST', 1, 'Y'
		)`, args); err != nil {
		return err
	}
	_ = store.Scan(ctx, tx, `
		SELECT COALESCE(ipAddress, ''), COALESCE(agentName, '')
		FROM FG_AGENT WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID}, &keptIP, &keptAgent)
	ip := strings.TrimSpace(host.IPAddress)
	if ip == "" {
		ip = keptIP
	}
	name := incomingName
	if name == "" {
		name = keptAgent
	}
	_, err := tx.Exec(ctx, `
		UPDATE FG_AGENT
		SET hostName = @hostName, osName = @osName, ipAddress = @ipAddress, agentName = @agentName,
		    editTime = @editTime, editWho = 'system'
		WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{
			"hostName": strings.TrimSpace(host.HostName), "osName": host.OSName, "ipAddress": ip, "agentName": name,
			"editTime": edited, "tenantId": batch.TenantID, "agentId": batch.AgentID,
		})
	return err
}

// SaveJava 更新 Java 应用资料。代理种类不是 java 时什么都不写。
// 应用名优先取 java.appName，没有时仍接受 host.reportName。空字段保留上一行。
func SaveJava(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	if strings.TrimSpace(batch.AgentCode) != protocol.AgentJava {
		return nil
	}
	reported := store.FormatTime(batch.SentAt)
	edited := store.FormatTime(time.Now().UTC())
	incoming, mainClass, processID, version := "", "", "", ""
	hostName, ip := "", ""
	var start any
	if batch.Java != nil {
		app := batch.Java
		incoming = strings.TrimSpace(app.AppName)
		mainClass = strings.TrimSpace(app.MainClass)
		processID = strings.TrimSpace(app.ProcessID)
		version = strings.TrimSpace(app.JavaVersion)
		hostName = strings.TrimSpace(app.HostName)
		ip = strings.TrimSpace(app.IPAddress)
		if !app.StartTime.IsZero() {
			start = store.FormatTime(app.StartTime)
		}
	}
	if batch.Host != nil {
		if incoming == "" {
			incoming = strings.TrimSpace(batch.Host.ReportName)
		}
		if hostName == "" {
			hostName = strings.TrimSpace(batch.Host.HostName)
		}
		if ip == "" {
			ip = strings.TrimSpace(batch.Host.IPAddress)
		}
	}
	var existing int
	var keptApp, keptMain, keptPid, keptVersion, keptHost, keptIP string
	var keptStart any
	if err := store.Scan(ctx, tx, `
		SELECT count(*), COALESCE(max(appName), ''), COALESCE(max(mainClass), ''), COALESCE(max(processId), ''),
		       COALESCE(max(javaVersion), ''), COALESCE(max(hostName), ''), COALESCE(max(ipAddress), ''), max(startTime)
		FROM FG_AGENT_JAVA WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID},
		&existing, &keptApp, &keptMain, &keptPid, &keptVersion, &keptHost, &keptIP, &keptStart); err != nil {
		return err
	}
	appName := incoming
	if appName == "" {
		appName = keptApp
	}
	if appName == "" {
		appName = "java"
	}
	if mainClass == "" {
		mainClass = keptMain
	}
	if processID == "" {
		processID = keptPid
	}
	if version == "" {
		version = keptVersion
	}
	if hostName == "" {
		hostName = keptHost
	}
	if ip == "" {
		ip = keptIP
	}
	if start == nil {
		start = keptStart
	}
	args := dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"appName": clip(appName, 100), "mainClass": nullText(clip(mainClass, 255)),
		"processId": nullText(clip(processID, 32)), "javaVersion": nullText(clip(version, 64)),
		"startTime": start, "hostName": nullText(clip(hostName, 100)), "ipAddress": nullText(clip(ip, 128)),
		"reportedTime": reported, "editTime": edited, "addWho": batch.AgentID,
	}
	if existing > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE FG_AGENT_JAVA SET
			  appName = @appName, mainClass = @mainClass, processId = @processId, javaVersion = @javaVersion,
			  startTime = @startTime, hostName = @hostName, ipAddress = @ipAddress,
			  reportedTime = @reportedTime, editTime = @editTime, editWho = 'system',
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND agentId = @agentId`, args); err != nil {
			return err
		}
	} else if _, err := tx.Exec(ctx, `
		INSERT INTO FG_AGENT_JAVA (
		  agentId, tenantId, appName, mainClass, processId, javaVersion, startTime, hostName, ipAddress, reportedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, @tenantId, @appName, @mainClass, @processId, @javaVersion, @startTime, @hostName, @ipAddress, @reportedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, args); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE FG_AGENT SET
		  agentName = @agentName,
		  hostName = CASE WHEN @hostName = '' THEN hostName ELSE @hostName END,
		  ipAddress = CASE WHEN @ipAddress = '' THEN ipAddress ELSE @ipAddress END,
		  editTime = @editTime, editWho = 'system'
		WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{
			"agentName": clip(appName, 100), "hostName": strings.TrimSpace(hostName), "ipAddress": strings.TrimSpace(ip),
			"editTime": edited, "tenantId": batch.TenantID, "agentId": batch.AgentID,
		})
	return err
}

// HostOf 返回这台代理最近上报的机器资料。
func HostOf(ctx context.Context, conn *dbx.DB, tenantID, agentID string) (protocol.Host, bool) {
	var item protocol.Host
	var boot any
	err := store.Scan(ctx, conn, `
		SELECT hostName, osName, COALESCE(osVersion, ''), archName, cpuCount, memTotalBytes, COALESCE(ipAddress, ''), bootTime, COALESCE(reportName, ''),
		       COALESCE(machineId, ''), COALESCE(kernelVersion, ''), COALESCE(agentVersion, ''), COALESCE(ipList, '')
		FROM FG_AGENT_HOST
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID},
		&item.HostName, &item.OSName, &item.OSVersion, &item.ArchName, &item.CPUCount, &item.MemTotalBytes, &item.IPAddress, &boot, &item.ReportName,
		&item.MachineID, &item.KernelVersion, &item.AgentVersion, &item.IPList,
	)
	if err != nil {
		return protocol.Host{}, false
	}
	if t, ok := store.TimeValue(boot); ok {
		item.BootTime = t
	}
	return item, true
}

// LatestEvents 返回该租户最新的事件，新的在前。
func LatestEvents(ctx context.Context, conn *dbx.DB, tenantID string, limit int) []models.Event {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT eventId, tenantId, agentId, agentCode, eventName, severity, COALESCE(messageText, ''), COALESCE(labelJson, ''), observedTime, batchId
		FROM FG_EVENT_POINT
		WHERE tenantId = @tenantId AND activeFlag = 'Y'
		ORDER BY observedTime DESC, eventId DESC ` + conn.LimitSQL()
	rows, err := conn.Query(ctx, query, dbx.BindPage(dbx.Args{"tenantId": tenantID}, dbx.Page{Page: 1, PageSize: limit}))
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []models.Event
	for rows.Next() {
		var item models.Event
		var labels string
		var observed any
		if err := rows.Scan(&item.EventID, &item.TenantID, &item.AgentID, &item.AgentCode, &item.EventName, &item.Severity, &item.MessageText, &labels, &observed, &item.BatchID); err != nil {
			return out
		}
		item.Labels = unmarshalLabels(labels)
		if t, ok := store.TimeValue(observed); ok {
			item.ObservedAt = t
		}
		out = append(out, item)
	}
	return out
}

// Stats 统计库里的事件和最近一次接收时间。
func Stats(ctx context.Context, conn *dbx.DB, tenantID string) models.Stats {
	var stats models.Stats
	var last any
	_ = store.Scan(ctx, conn, `
		SELECT
		  (SELECT count(*) FROM FG_EVENT_POINT WHERE tenantId = @tenantId AND activeFlag = 'Y'),
		  (SELECT max(addTime) FROM FG_INGEST_BATCH WHERE tenantId = @tenantId AND activeFlag = 'Y')`,
		dbx.Args{"tenantId": tenantID}, &stats.EventCount, &last)
	if t, ok := store.TimeValue(last); ok {
		stats.LastReceivedTime = &t
	}
	return stats
}
