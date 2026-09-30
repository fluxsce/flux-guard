package shell

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/common/logger"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

const (
	eventOpen  = "OPEN"
	eventClose = "CLOSE"
	eventInput = "INPUT"

	maxAuditDetail = 200
	defaultAuditLimit = 100
)

// AuditView 是终端会话账本里的一行，供控制台按代理回看。
type AuditView struct {
	AuditID    string `json:"auditId"`
	ShellID    string `json:"shellId"`
	AgentID    string `json:"agentId"`
	UserID     string `json:"userId"`
	UserName   string `json:"userName"`
	EventType  string `json:"eventType"`
	DetailText string `json:"detailText,omitempty"`
	AddTime    string `json:"addTime"`
}

// ListAudits 按代理列出最近的终端审计，新的在前。
func (s *Store) ListAudits(ctx context.Context, tenantID, agentID string, limit int) ([]AuditView, error) {
	agentID = strings.TrimSpace(agentID)
	if limit <= 0 || limit > 200 {
		limit = defaultAuditLimit
	}
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT auditId, shellId, agentId, userId, COALESCE(userName, ''), eventType,
		       COALESCE(detailText, ''), addTime
		FROM FG_SHELL_AUDIT
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY addTime DESC, auditId DESC
		LIMIT %d`, limit),
		dbx.Args{"tenantId": tenantID, "agentId": agentID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuditView, 0)
	for rows.Next() {
		var item AuditView
		if err := rows.Scan(
			&item.AuditID, &item.ShellID, &item.AgentID, &item.UserID, &item.UserName,
			&item.EventType, &item.DetailText, &item.AddTime,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) recordAudit(ctx context.Context, tenantID, shellID, agentID, userID, userName, eventType, detail string) {
	if s == nil || s.db == nil {
		return
	}
	shellID = strings.TrimSpace(shellID)
	agentID = strings.TrimSpace(agentID)
	userID = strings.TrimSpace(userID)
	eventType = strings.TrimSpace(eventType)
	if tenantID == "" || shellID == "" || agentID == "" || userID == "" || eventType == "" {
		return
	}
	name := strings.TrimSpace(userName)
	if name == "" {
		name = userID
	}
	now := store.FormatTime(time.Now().UTC())
	_, err := s.db.Exec(ctx, `
		INSERT INTO FG_SHELL_AUDIT (
		  auditId, tenantId, shellId, agentId, userId, userName, eventType, detailText,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @auditId, @tenantId, @shellId, @agentId, @userId, @userName, @eventType, @detailText,
		  @now, @userId, @now, @userId, 'SHELL', 1, 'Y'
		)`, dbx.Args{
		"auditId": id.New(), "tenantId": tenantID, "shellId": shellID, "agentId": agentID,
		"userId": userID, "userName": name, "eventType": eventType,
		"detailText": clipDetail(detail), "now": now,
	})
	if err != nil {
		logger.Warn(ctx, "终端审计写入失败 shell=%s event=%s err=%v", shellID, eventType, err)
	}
}

func clipDetail(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if utf8.RuneCountInString(text) <= maxAuditDetail {
		return text
	}
	return string([]rune(text)[:maxAuditDetail])
}
