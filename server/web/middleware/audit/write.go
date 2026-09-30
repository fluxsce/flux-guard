package audit

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/common/logger"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// opened 是本进程的库连接。Use 登记，写入时直接用它。
var opened *dbx.DB

// Use 登记审计写入用的库连接。连接为空时不再写入。
func Use(db *dbx.DB) {
	opened = db
}

// write 同步插入一条审计。缺结果时记为成功。失败只打日志，不改已经完成的业务结果。
func write(ctx context.Context, event *Event) {
	if opened == nil || event == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if event.Result != ResultOK && event.Result != ResultFail {
		event.Result = ResultOK
	}
	now := store.FormatTime(time.Now().UTC())
	userID := clip(event.UserID, 32)
	_, err := opened.Exec(ctx, `
		INSERT INTO FG_AUTH_AUDIT_LOG (
		  auditId, tenantId, userId, userName,
		  action, moduleCode, targetType, targetId, targetName, resourceCode,
		  requestPath, requestMethod, clientIP, resultFlag, detailText, failReason,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @auditId, @tenantId, @userId, @userName,
		  @action, @moduleCode, @targetType, @targetId, @targetName, @resourceCode,
		  @requestPath, @requestMethod, @clientIP, @resultFlag, @detailText, @failReason,
		  @now, @userId, @now, @userId, @oprSeqFlag, 1, 'Y'
		)`, dbx.Args{
		"auditId": id.New(), "tenantId": clip(event.TenantID, 32), "userId": userID,
		"userName": clip(event.UserName, 100), "action": clip(event.Action, 32),
		"moduleCode": clip(event.ModuleCode, 64), "targetType": clip(event.TargetType, 32),
		"targetId": clip(event.TargetID, 128), "targetName": clip(event.TargetName, 255),
		"resourceCode": clip(event.ResourceCode, 128), "requestPath": clip(event.RequestPath, 255),
		"requestMethod": clip(event.RequestMethod, 16), "clientIP": clip(event.ClientIP, 128),
		"resultFlag": event.Result, "detailText": clip(event.Detail, maxDetail),
		"failReason": clip(event.FailReason, maxFailReason),
		"now": now, "oprSeqFlag": id.New(),
	})
	if err != nil {
		logger.Warn(ctx, "审计写入失败 action=%s userId=%s err=%v", event.Action, event.UserID, err)
	}
}

func clip(value string, n int) string {
	value = strings.TrimSpace(value)
	if n <= 0 || value == "" {
		return ""
	}
	if utf8.RuneCountInString(value) <= n {
		return value
	}
	runes := []rune(value)
	return string(runes[:n])
}
