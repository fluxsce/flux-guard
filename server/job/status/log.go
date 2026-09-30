package status

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fluxguard/server/job/policy"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// Log 一次执行日志，对应 FG_JOB_LOG 的一行。
type Log struct {
	LogID      string
	TenantID   string
	Code       string
	Trigger    string
	Start      time.Time
	End        time.Time
	DurationMs int64
	OK         bool
	Result     json.RawMessage
	ErrorText  string
}

// Recent 按时间倒序读取某任务的执行日志。未绑定库时返回空。limit 限制在 1 到 100，缺省 20。
func Recent(tenantID, code string, limit int) ([]Log, error) {
	conn := currentDB()
	code = strings.TrimSpace(code)
	if conn == nil || code == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	tenantID = policy.TenantKey(tenantID)
	rows, err := conn.Query(context.Background(), fmt.Sprintf(`
		SELECT logId, triggerName, startTime, endTime, durationMs, okFlag, resultText, errorText
		FROM FG_JOB_LOG
		WHERE activeFlag = 'Y' AND tenantId = @tenantId AND jobCode = @jobCode
		ORDER BY addTime DESC
		LIMIT %d`, limit), dbx.Args{"tenantId": tenantID, "jobCode": code})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Log
	for rows.Next() {
		var (
			item                          Log
			triggerRaw, resultRaw, errRaw any
			startRaw, endRaw              any
			okFlag                        string
		)
		item.TenantID = tenantID
		item.Code = code
		if err := rows.Scan(&item.LogID, &triggerRaw, &startRaw, &endRaw, &item.DurationMs, &okFlag, &resultRaw, &errRaw); err != nil {
			return nil, err
		}
		item.Trigger = textOf(triggerRaw)
		item.OK = okFlag == "Y"
		item.Result = json.RawMessage(textOf(resultRaw))
		item.ErrorText = textOf(errRaw)
		if t, ok := store.TimeValue(startRaw); ok {
			item.Start = t
		}
		if t, ok := store.TimeValue(endRaw); ok {
			item.End = t
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
