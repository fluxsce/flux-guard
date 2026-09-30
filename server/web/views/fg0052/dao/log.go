package dao

import (
	"context"
	"strings"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// LogView 是一条代理操作日志。COMMAND 来自 FG_COMMAND，SHELL 来自 FG_SHELL_AUDIT。
// DetailText：命令行是回执；终端 INPUT 是回车确认的命令行；OPEN/CLOSE 可为空。
type LogView struct {
	Source      string `json:"source"`
	LogID       string `json:"logId"`
	AgentID     string `json:"agentId"`
	AgentName   string `json:"agentName"`
	UserID      string `json:"userId"`
	UserName    string `json:"userName"`
	EventType   string `json:"eventType"`
	StatusFlag  string `json:"statusFlag"`
	DeliverMode string `json:"deliverMode"`
	IntervalSec int    `json:"intervalSec"`
	DetailText  string `json:"detailText"`
	AddTime     string `json:"addTime"`
	AddWho      string `json:"addWho"`
	EditTime    string `json:"editTime"`
	EditWho     string `json:"editWho"`
}

// LogQuery 是代理日志的筛选条件。空字段不参与条件。
type LogQuery struct {
	AgentName  string
	UserName   string
	Source     string
	EventType  string
	StatusFlag string
	FromTime   string
	ToTime     string
}

// LogDAO 只读查询本租户的代理命令与终端账本。
type LogDAO struct {
	db *dbx.DB
}

// NewLogDAO 使用鉴权服务交出的连接。
func NewLogDAO(db *dbx.DB) *LogDAO {
	return &LogDAO{db: db}
}

const unionFrom = `
	FROM (
	  SELECT 'COMMAND' AS source, c.commandId AS logId, c.agentId,
	         COALESCE(a.agentName, '') AS agentName,
	         c.addWho AS userId, c.addWho AS userName,
	         c.actionName AS eventType, c.statusFlag,
	         COALESCE(c.deliverMode, '') AS deliverMode,
	         COALESCE(c.intervalSec, 0) AS intervalSec,
	         COALESCE(c.resultText, '') AS detailText,
	         c.addTime, c.addWho, c.editTime, c.editWho
	  FROM FG_COMMAND c
	  LEFT JOIN FG_AGENT a ON a.tenantId = c.tenantId AND a.agentId = c.agentId
	  WHERE c.tenantId = @tenantId AND c.activeFlag = 'Y'
	  UNION ALL
	  SELECT 'SHELL', s.auditId, s.agentId,
	         COALESCE(a.agentName, ''),
	         s.userId, COALESCE(NULLIF(s.userName, ''), s.userId),
	         s.eventType, '',
	         '',
	         0,
	         COALESCE(s.detailText, ''),
	         s.addTime, s.addWho, s.editTime, s.editWho
	  FROM FG_SHELL_AUDIT s
	  LEFT JOIN FG_AGENT a ON a.tenantId = s.tenantId AND a.agentId = s.agentId
	  WHERE s.tenantId = @tenantId AND s.activeFlag = 'Y'
	) t`

const logSelect = `
	SELECT source, logId, agentId, agentName, userId, userName,
	       eventType, statusFlag, deliverMode, intervalSec, detailText,
	       addTime, addWho, editTime, editWho` + unionFrom

// PageLogs 按时间倒序分页返回本租户代理日志。
func (d *LogDAO) PageLogs(tenantID string, q LogQuery, page dbx.Page) ([]LogView, int64, error) {
	if d == nil || d.db == nil {
		return []LogView{}, 0, nil
	}
	ctx := context.Background()
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE 1=1"
	where += agentClause(args, q.AgentName)
	where += userClause(args, q.UserName)
	where += sourceClause(args, q.Source)
	where += store.EqClause(args, "eventType", "eventType", q.EventType)
	where += statusClause(args, q.StatusFlag)
	where += timeClause(args, "fromTime", ">=", q.FromTime)
	where += timeClause(args, "toTime", "<=", q.ToTime)
	countSQL := "SELECT COUNT(*) " + unionFrom + where
	listSQL := logSelect + where + `
		ORDER BY addTime DESC, logId DESC`
	rows, total, err := d.db.QueryPage(ctx, countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	var out []LogView
	for rows.Next() {
		item, err := scanLog(rows)
		if err != nil {
			_ = rows.Close()
			return out, total, err
		}
		out = append(out, item)
	}
	if err := rows.Close(); err != nil {
		return out, total, err
	}
	if out == nil {
		out = []LogView{}
	}
	return out, total, nil
}

// GetLog 按来源和业务主键读取一行。
func (d *LogDAO) GetLog(ctx context.Context, tenantID, source, logID string) (LogView, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	source = strings.ToUpper(strings.TrimSpace(source))
	logID = strings.TrimSpace(logID)
	row, err := d.db.QueryRow(ctx, logSelect+`
		WHERE source = @source AND logId = @logId`,
		dbx.Args{"tenantId": tenantID, "source": source, "logId": logID})
	if err != nil {
		return LogView{}, err
	}
	return scanLog(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLog(row rowScanner) (LogView, error) {
	var item LogView
	err := row.Scan(
		&item.Source, &item.LogID, &item.AgentID, &item.AgentName,
		&item.UserID, &item.UserName, &item.EventType, &item.StatusFlag,
		&item.DeliverMode, &item.IntervalSec, &item.DetailText,
		&item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
	)
	return item, err
}

func agentClause(args dbx.Args, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.NewReplacer("%", "", "_", "").Replace(raw)
	args["agentKey"] = "%" + strings.ToLower(raw) + "%"
	return " AND (LOWER(agentName) LIKE @agentKey OR LOWER(agentId) LIKE @agentKey)"
}

func userClause(args dbx.Args, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.NewReplacer("%", "", "_", "").Replace(raw)
	args["userKey"] = "%" + strings.ToLower(raw) + "%"
	return " AND (LOWER(userName) LIKE @userKey OR LOWER(userId) LIKE @userKey)"
}

func sourceClause(args dbx.Args, raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "COMMAND", "SHELL":
		args["source"] = strings.ToUpper(strings.TrimSpace(raw))
		return " AND source = @source"
	default:
		return ""
	}
}

func statusClause(args dbx.Args, raw string) string {
	raw = strings.TrimSpace(raw)
	switch raw {
	case "D", "W", "R", "Y", "N":
		args["statusFlag"] = raw
		return " AND statusFlag = @statusFlag"
	default:
		return ""
	}
}

func timeClause(args dbx.Args, key, op, raw string) string {
	parsed, ok := store.ParseTime(strings.TrimSpace(raw))
	if !ok {
		return ""
	}
	args[key] = store.FormatTime(parsed)
	return " AND addTime " + op + " @" + key
}
