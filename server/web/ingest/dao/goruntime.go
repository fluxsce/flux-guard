package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/helper/agent"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// SaveGo 更新 Go 进程自己的身份和最近一次运行时。
// 种类是 go，或仍在使用旧种类 app 时写入。其它种类什么都不写。
// 显示名留空时保留上一行。旧进程只带样本时，从样本补上协程数和堆。
func SaveGo(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	code := strings.TrimSpace(batch.AgentCode)
	if code != protocol.AgentGo && code != protocol.AgentApp {
		return nil
	}
	snap := goSnapshot(batch)
	reported := store.FormatTime(batch.SentAt)
	edited := store.FormatTime(time.Now().UTC())
	incoming := strings.TrimSpace(snap.AppName)
	version := strings.TrimSpace(snap.GoVersion)
	processID := strings.TrimSpace(snap.ProcessID)
	hostName := strings.TrimSpace(snap.HostName)
	var start any
	if !snap.StartTime.IsZero() {
		start = store.FormatTime(snap.StartTime)
	}
	var existing int
	var keptName, keptVersion, keptPid, keptHost string
	var keptStart any
	if err := store.Scan(ctx, tx, `
		SELECT count(*), COALESCE(max(appName), ''), COALESCE(max(goVersion), ''), COALESCE(max(processId), ''),
		       COALESCE(max(hostName), ''), max(startTime)
		FROM FG_AGENT_GO WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID},
		&existing, &keptName, &keptVersion, &keptPid, &keptHost, &keptStart); err != nil {
		return err
	}
	appName := incoming
	if appName == "" {
		appName = keptName
	}
	if appName == "" {
		appName = "go"
	}
	if version == "" {
		version = keptVersion
	}
	if processID == "" {
		processID = keptPid
	}
	if hostName == "" {
		hostName = keptHost
	}
	if start == nil {
		start = keptStart
	}
	args := dbx.Args{
		"agentId": batch.AgentID, "tenantId": batch.TenantID,
		"appName": clip(appName, 100), "goVersion": nullText(clip(version, 64)),
		"processId": nullText(clip(processID, 32)), "startTime": start,
		"hostName": nullText(clip(hostName, 100)), "numCpu": snap.NumCPU,
		"goroutineCount": int64(snap.GoroutineCount), "heapAllocBytes": int64(snap.HeapAllocBytes),
		"heapSysBytes": int64(snap.HeapSysBytes), "heapObjects": int64(snap.HeapObjects),
		"gcCount": int64(snap.GCCount), "gcPauseMs": snap.GCPauseMs,
		"reportedTime": reported, "editTime": edited, "addWho": batch.AgentID,
	}
	if existing > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE FG_AGENT_GO SET
			  appName = @appName, goVersion = @goVersion, processId = @processId, startTime = @startTime,
			  hostName = @hostName, numCpu = @numCpu, goroutineCount = @goroutineCount,
			  heapAllocBytes = @heapAllocBytes, heapSysBytes = @heapSysBytes, heapObjects = @heapObjects,
			  gcCount = @gcCount, gcPauseMs = @gcPauseMs, reportedTime = @reportedTime,
			  editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND agentId = @agentId`, args); err != nil {
			return err
		}
	} else if _, err := tx.Exec(ctx, `
		INSERT INTO FG_AGENT_GO (
		  agentId, tenantId, appName, goVersion, processId, startTime, hostName, numCpu,
		  goroutineCount, heapAllocBytes, heapSysBytes, heapObjects, gcCount, gcPauseMs, reportedTime,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, @tenantId, @appName, @goVersion, @processId, @startTime, @hostName, @numCpu,
		  @goroutineCount, @heapAllocBytes, @heapSysBytes, @heapObjects, @gcCount, @gcPauseMs, @reportedTime,
		  @addWho, 'system', 'INGEST', 1, 'Y'
		)`, args); err != nil {
		return err
	}
	var keptAgent string
	err := store.Scan(ctx, tx, `
		SELECT COALESCE(agentName, '') FROM FG_AGENT WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID}, &keptAgent)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	display := agent.ResolveDisplayName(keptAgent, batch.AgentCode, appName, hostName)
	_, err = tx.Exec(ctx, `
		UPDATE FG_AGENT SET
		  agentName = @agentName,
		  hostName = CASE WHEN @hostName = '' THEN hostName ELSE @hostName END,
		  editTime = @editTime, editWho = 'system'
		WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{
			"agentName": display, "hostName": hostName,
			"editTime": edited, "tenantId": batch.TenantID, "agentId": batch.AgentID,
		})
	return err
}

func goSnapshot(batch protocol.Batch) protocol.Go {
	var snap protocol.Go
	if batch.Go != nil {
		snap = *batch.Go
	}
	for _, sample := range batch.Samples {
		switch strings.TrimSpace(sample.MetricName) {
		case "go.runtime.goroutine.count", "app.runtime.goroutine.count":
			if snap.GoroutineCount == 0 {
				snap.GoroutineCount = int(sample.MetricValue)
			}
		case "go.runtime.heap.alloc.bytes", "app.runtime.heap.alloc.bytes":
			if snap.HeapAllocBytes == 0 {
				snap.HeapAllocBytes = uint64(sample.MetricValue)
			}
		case "go.runtime.heap.sys.bytes":
			if snap.HeapSysBytes == 0 {
				snap.HeapSysBytes = uint64(sample.MetricValue)
			}
		case "go.runtime.heap.objects":
			if snap.HeapObjects == 0 {
				snap.HeapObjects = uint64(sample.MetricValue)
			}
		case "go.runtime.gc.count":
			if snap.GCCount == 0 {
				snap.GCCount = uint32(sample.MetricValue)
			}
		case "go.runtime.gc.pause.ms":
			if snap.GCPauseMs == 0 {
				snap.GCPauseMs = sample.MetricValue
			}
		}
	}
	return snap
}
