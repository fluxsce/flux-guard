package status

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"fluxguard/common/id"
	"fluxguard/common/logger"
	"fluxguard/server/job/policy"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	dbMu sync.RWMutex
	db   *dbx.DB
)

// Open 绑定库并装入已落库的最近执行，供重启后恢复。
func Open(ctx context.Context, conn *dbx.DB) error {
	dbMu.Lock()
	db = conn
	dbMu.Unlock()
	if conn == nil {
		return nil
	}
	return load(ctx, conn)
}

func currentDB() *dbx.DB {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return db
}

func load(ctx context.Context, conn *dbx.DB) error {
	rows, err := conn.Query(ctx, `
		SELECT tenantId, jobCode, title, enabledFlag, lastTrigger,
		       lastStart, lastEnd, lastDurationMs, lastOk, lastResult, lastError, ranFlag
		FROM FG_JOB
		WHERE activeFlag = 'Y'`, nil)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			tenantID, code, title, enabled, lastOK, ran string
			durationMs                                  int64
			triggerRaw, resultRaw, errRaw               any
			startRaw, endRaw                            any
		)
		if err := rows.Scan(&tenantID, &code, &title, &enabled, &triggerRaw,
			&startRaw, &endRaw, &durationMs, &lastOK, &resultRaw, &errRaw, &ran); err != nil {
			return err
		}
		st := Status{
			TenantID:   tenantID,
			Code:       code,
			Title:      title,
			Enabled:    enabled == "Y",
			Trigger:    textOf(triggerRaw),
			DurationMs: durationMs,
			OK:         lastOK == "Y",
			Result:     json.RawMessage(textOf(resultRaw)),
			ErrorText:  textOf(errRaw),
			RanAtLeast: ran == "Y",
		}
		if t, ok := store.TimeValue(startRaw); ok {
			st.LastStart = t
		}
		if t, ok := store.TimeValue(endRaw); ok {
			st.LastEnd = t
		}
		global.mu.Lock()
		global.items[policy.Key(st.TenantID, st.Code)] = st
		global.mu.Unlock()
	}
	return rows.Err()
}

func persistRun(s Status) {
	conn := currentDB()
	if conn == nil || strings.TrimSpace(s.Code) == "" {
		return
	}
	ctx := context.Background()
	if err := upsertJob(ctx, conn, s); err != nil {
		logger.Error(ctx, "任务状态落库 %s: %v", s.Code, err)
		return
	}
	if !s.RanAtLeast {
		return
	}
	if err := insertLog(ctx, conn, s); err != nil {
		logger.Error(ctx, "任务日志落库 %s: %v", s.Code, err)
	}
}

func persistTouch(s Status) {
	conn := currentDB()
	if conn == nil || strings.TrimSpace(s.Code) == "" {
		return
	}
	ctx := context.Background()
	if err := upsertJob(ctx, conn, s); err != nil {
		logger.Error(ctx, "任务状态落库 %s: %v", s.Code, err)
	}
}

func upsertJob(ctx context.Context, conn *dbx.DB, s Status) error {
	now := store.FormatTime(storeTimeNow())
	args := jobArgs(s, now)
	sql := upsertSQL(conn.Driver())
	_, err := conn.Exec(ctx, sql, args)
	return err
}

func insertLog(ctx context.Context, conn *dbx.DB, s Status) error {
	now := store.FormatTime(storeTimeNow())
	_, err := conn.Exec(ctx, `
		INSERT INTO FG_JOB_LOG (
		  logId, tenantId, jobCode, triggerName, startTime, endTime,
		  okFlag, durationMs, resultText, errorText,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @logId, @tenantId, @jobCode, @triggerName, @startTime, @endTime,
		  @okFlag, @durationMs, @resultText, @errorText,
		  @now, 'system', @now, 'system', 'RUN', 1, 'Y'
		)`, dbx.Args{
		"logId": id.New(), "tenantId": s.TenantID, "jobCode": s.Code,
		"triggerName": emptyTrigger(s.Trigger),
		"startTime":   timeArg(s.LastStart), "endTime": timeArg(s.LastEnd),
		"okFlag": yn(s.OK), "durationMs": s.DurationMs,
		"resultText": string(s.Result), "errorText": s.ErrorText, "now": now,
	})
	return err
}

func jobArgs(s Status, now string) dbx.Args {
	return dbx.Args{
		"tenantId": s.TenantID, "jobCode": s.Code, "title": s.Title,
		"enabledFlag": yn(s.Enabled), "lastTrigger": s.Trigger,
		"lastStart": timeArg(s.LastStart), "lastEnd": timeArg(s.LastEnd),
		"nextStart": nextStartArg(s),
		"lastOk":    yn(s.OK), "lastDurationMs": s.DurationMs,
		"lastResult": string(s.Result), "lastError": s.ErrorText, "ranFlag": yn(s.RanAtLeast),
		"now": now,
	}
}

func upsertSQL(driver string) string {
	cols := `
		  tenantId, jobCode, title, enabledFlag, lastTrigger, lastStart, lastEnd, nextStart,
		  lastDurationMs, lastOk, lastResult, lastError, ranFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag`
	vals := `
		  @tenantId, @jobCode, @title, @enabledFlag, @lastTrigger, @lastStart, @lastEnd, @nextStart,
		  @lastDurationMs, @lastOk, @lastResult, @lastError, @ranFlag,
		  @now, 'system', @now, 'system', 'RUN', 1, 'Y'`
	if strings.Contains(strings.ToLower(driver), "mysql") {
		return `INSERT INTO FG_JOB (` + cols + `) VALUES (` + vals + `)
			ON DUPLICATE KEY UPDATE
			  title = VALUES(title), enabledFlag = VALUES(enabledFlag),
			  lastTrigger = VALUES(lastTrigger), lastStart = VALUES(lastStart), lastEnd = VALUES(lastEnd),
			  nextStart = VALUES(nextStart),
			  lastOk = VALUES(lastOk), lastDurationMs = VALUES(lastDurationMs),
			  lastResult = VALUES(lastResult), lastError = VALUES(lastError), ranFlag = VALUES(ranFlag),
			  editTime = VALUES(editTime), editWho = 'system', oprSeqFlag = 'RUN'`
	}
	return `INSERT INTO FG_JOB (` + cols + `) VALUES (` + vals + `)
		ON CONFLICT(tenantId, jobCode) DO UPDATE SET
		  title = excluded.title, enabledFlag = excluded.enabledFlag,
		  lastTrigger = excluded.lastTrigger, lastStart = excluded.lastStart, lastEnd = excluded.lastEnd,
		  nextStart = excluded.nextStart,
		  lastOk = excluded.lastOk, lastDurationMs = excluded.lastDurationMs,
		  lastResult = excluded.lastResult, lastError = excluded.lastError, ranFlag = excluded.ranFlag,
		  editTime = excluded.editTime, editWho = 'system', oprSeqFlag = 'RUN'`
}
