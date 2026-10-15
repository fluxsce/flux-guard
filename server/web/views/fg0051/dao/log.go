package dao

import (
	"context"
	"strings"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// LogView 是一条系统操作日志，对应 FG_AUTH_AUDIT_LOG 的一行。
// 列表和详情共用这一结构。查询本身不写入这张表。
type LogView struct {
	AuditID       string `json:"auditId"`
	UserID        string `json:"userId"`
	UserName      string `json:"userName"`
	Action        string `json:"action"`
	ModuleCode    string `json:"moduleCode"`
	TargetType    string `json:"targetType"`
	TargetID      string `json:"targetId"`
	TargetName    string `json:"targetName"`
	ResourceCode  string `json:"resourceCode"`
	RequestPath   string `json:"requestPath"`
	RequestMethod string `json:"requestMethod"`
	ClientIP      string `json:"clientIP"`
	ResultFlag    string `json:"resultFlag"`
	DetailText    string `json:"detailText"`
	FailReason    string `json:"failReason"`
	AddTime       string `json:"addTime"`
	AddWho        string `json:"addWho"`
	EditTime      string `json:"editTime"`
	EditWho       string `json:"editWho"`
}

// LogQuery 是操作日志的筛选条件。空字段不参与条件。
type LogQuery struct {
	UserName   string
	Action     string
	ModuleCode string
	ResultFlag string
	FromTime   string
	ToTime     string
}

// LogDAO 只读查询本租户的系统操作日志。
type LogDAO struct {
	db *dbx.DB
}

// NewLogDAO 使用鉴权服务交出的连接。
func NewLogDAO(db *dbx.DB) *LogDAO {
	return &LogDAO{db: db}
}

const logSelect = `
	SELECT auditId, userId, COALESCE(userName, ''),
	       action, COALESCE(moduleCode, ''), COALESCE(targetType, ''),
	       COALESCE(targetId, ''), COALESCE(targetName, ''), COALESCE(resourceCode, ''),
	       COALESCE(requestPath, ''), COALESCE(requestMethod, ''), COALESCE(clientIP, ''),
	       resultFlag, COALESCE(detailText, ''), COALESCE(failReason, ''),
	       addTime, addWho, editTime, editWho
	FROM FG_AUTH_AUDIT_LOG`

// PageLogs 按时间倒序分页返回本租户操作日志。
func (d *LogDAO) PageLogs(tenantID string, q LogQuery, page dbx.Page) ([]LogView, int64, error) {
	if d == nil || d.db == nil {
		return []LogView{}, 0, nil
	}
	ctx := context.Background()
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y'"
	where += userClause(args, q.UserName)
	where += store.EqClause(args, "action", "action", q.Action)
	where += store.LikeClause(args, "moduleCode", "moduleCode", q.ModuleCode)
	where += resultClause(args, q.ResultFlag)
	where += timeClause(args, "fromTime", ">=", q.FromTime)
	where += timeClause(args, "toTime", "<=", q.ToTime)
	listSQL := logSelect + where + `
		ORDER BY addTime DESC, auditId DESC`
	rows, total, err := d.db.QueryPage(ctx, listSQL, page, args)
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

// GetLog 按审计主键读取本租户的一行。不存在时返回 sql.ErrNoRows。
func (d *LogDAO) GetLog(ctx context.Context, tenantID, auditID string) (LogView, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	row, err := d.db.QueryRow(ctx, logSelect+`
		WHERE tenantId = @tenantId AND auditId = @auditId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "auditId": auditID})
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
		&item.AuditID, &item.UserID, &item.UserName,
		&item.Action, &item.ModuleCode, &item.TargetType,
		&item.TargetID, &item.TargetName, &item.ResourceCode,
		&item.RequestPath, &item.RequestMethod, &item.ClientIP,
		&item.ResultFlag, &item.DetailText, &item.FailReason,
		&item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
	)
	return item, err
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

func resultClause(args dbx.Args, raw string) string {
	switch strings.TrimSpace(raw) {
	case "Y", "N":
		args["resultFlag"] = strings.TrimSpace(raw)
		return " AND resultFlag = @resultFlag"
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
