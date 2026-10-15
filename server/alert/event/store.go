package event

import (
	"context"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/alert/models"
	"fluxguard/server/alert/validate"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// Store 读写预警日志。发送结果写在同一行。
type Store struct {
	db *dbx.DB
}

// NewStore 创建日志 Store。
func NewStore(db *dbx.DB) *Store {
	return &Store{db: db}
}

// Pending 是一轮投递要发送的日志。
type Pending struct {
	TenantID string
	Log      models.Log
}

const logSelect = `
	SELECT alertLogId, alertLevel, alertType, sceneCode, alertTitle, COALESCE(alertContent, ''),
	       alertTimestamp, COALESCE(channelId, ''), sendStatus, sendTime, COALESCE(sendResult, ''),
	       COALESCE(sendError, ''), failCount, COALESCE(alertExtra, ''),
	       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho
	FROM FG_ALERT_LOG`

// List 返回最近日志（最多 200 条）。
func (st *Store) List(tenantID string) ([]models.Log, error) {
	items, _, err := st.Page(tenantID, models.LogQuery{}, dbx.Page{Page: 1, PageSize: 200})
	return items, err
}

// Page 按当前引擎分页返回预警日志。
func (st *Store) Page(tenantID string, q models.LogQuery, page dbx.Page) ([]models.Log, int64, error) {
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y'"
	where += store.LikeClause(args, "alertTitle", "alertTitle", q.AlertTitle)
	where += store.EqClause(args, "alertLevel", "alertLevel", q.AlertLevel)
	where += store.EqClause(args, "alertType", "alertType", q.AlertType)
	where += store.EqClause(args, "sendStatus", "sendStatus", q.SendStatus)
	listSQL := logSelect + where + ` ORDER BY alertTimestamp DESC, alertLogId DESC`
	rows, total, err := st.db.QueryPage(context.Background(), listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Log{}
	for rows.Next() {
		item, err := scanLog(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// Get 按编码读取一条日志。
func (st *Store) Get(ctx context.Context, tenantID, alertLogID string) (models.Log, error) {
	rows, err := st.db.Query(ctx, logSelect+` WHERE tenantId = @tenantId AND alertLogId = @alertLogId`,
		dbx.Args{"tenantId": tenantID, "alertLogId": alertLogID})
	if err != nil {
		return models.Log{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return models.Log{}, err
		}
		return models.Log{}, errMissing
	}
	return scanLog(rows.Scan)
}

// ListPending 返回尚未发送成功、且失败未满 3 次的日志。
func (st *Store) ListPending(ctx context.Context) ([]Pending, error) {
	rows, err := st.db.Query(ctx, `
		SELECT tenantId, alertLogId, alertLevel, alertType, sceneCode, alertTitle, COALESCE(alertContent, ''),
		       alertTimestamp, COALESCE(channelId, ''), sendStatus, sendTime, COALESCE(sendResult, ''),
		       COALESCE(sendError, ''), failCount, COALESCE(alertExtra, ''),
		       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho
		FROM FG_ALERT_LOG
		WHERE activeFlag = 'Y' AND sendStatus IN ('pending', 'fail') AND failCount < 3
		ORDER BY alertTimestamp`, nil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pending
	for rows.Next() {
		var row Pending
		item, err := scanPending(rows.Scan)
		if err != nil {
			return nil, err
		}
		row.TenantID = item.tenantID
		row.Log = item.log
		out = append(out, row)
	}
	return out, rows.Err()
}

// Insert 写入一条待发送日志。渠道留空时不写。
func (st *Store) Insert(ctx context.Context, tenantID string, notice models.Notice, now time.Time) (models.Log, error) {
	when := store.FormatTime(now)
	item := models.Log{
		AlertLogID: id.New(), AlertLevel: strings.TrimSpace(notice.Level), AlertType: strings.TrimSpace(notice.Type),
		SceneCode: strings.TrimSpace(notice.Scene), AlertTitle: validate.Clip(notice.Title, 500),
		AlertContent: validate.Clip(notice.Content, 4000), AlertTimestamp: when,
		ChannelID: strings.TrimSpace(notice.ChannelID), SendStatus: "pending", AlertExtra: notice.Extra,
		ActiveFlag: "Y", AddTime: when, AddWho: "system", EditTime: when, EditWho: "system",
	}
	if item.AlertLevel == "" {
		item.AlertLevel = "warn"
	}
	if item.AlertType == "" {
		item.AlertType = "alert"
	}
	if item.SceneCode == "" {
		item.SceneCode = "fire"
	}
	_, err := st.db.Exec(ctx, `
		INSERT INTO FG_ALERT_LOG (
		  tenantId, alertLogId, alertLevel, alertType, sceneCode, alertTitle, alertContent, alertTimestamp,
		  channelId, sendStatus, failCount, alertExtra,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @tenantId, @alertLogId, @alertLevel, @alertType, @sceneCode, @alertTitle, @alertContent, @alertTimestamp,
		  @channelId, @sendStatus, 0, @alertExtra,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, 1, 'Y'
		)`, dbx.Args{
		"tenantId": tenantID, "alertLogId": item.AlertLogID, "alertLevel": item.AlertLevel,
		"alertType": item.AlertType, "sceneCode": item.SceneCode, "alertTitle": item.AlertTitle,
		"alertContent": item.AlertContent, "alertTimestamp": when, "channelId": emptyNil(item.ChannelID),
		"sendStatus": "pending", "alertExtra": emptyNil(item.AlertExtra),
		"addTime": when, "addWho": "system", "editTime": when, "editWho": "system", "oprSeqFlag": id.New(),
	})
	if err != nil {
		return models.Log{}, err
	}
	return item, nil
}

// MarkSent 回写发送结果。失败时累加失败次数，满 3 次后不再重试。
func (st *Store) MarkSent(ctx context.Context, tenantID, alertLogID, status, sentAt, result, errText string, failCount int) error {
	_, err := st.db.Exec(ctx, `
		UPDATE FG_ALERT_LOG SET
		  sendStatus = @sendStatus, sendTime = @sendTime, sendResult = @sendResult, sendError = @sendError,
		  failCount = @failCount, editTime = @editTime, editWho = 'system', oprSeqFlag = @oprSeqFlag,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND alertLogId = @alertLogId`,
		dbx.Args{
			"sendStatus": status, "sendTime": emptyNil(sentAt), "sendResult": emptyNil(result),
			"sendError": emptyNil(errText), "failCount": failCount, "editTime": sentAt, "oprSeqFlag": id.New(),
			"tenantId": tenantID, "alertLogId": alertLogID,
		})
	return err
}

type scanner func(dest ...any) error

func scanLog(scan scanner) (models.Log, error) {
	var sendTime, addTime, editTime, failCount any
	var item models.Log
	err := scan(
		&item.AlertLogID, &item.AlertLevel, &item.AlertType, &item.SceneCode, &item.AlertTitle, &item.AlertContent,
		&item.AlertTimestamp, &item.ChannelID, &item.SendStatus, &sendTime, &item.SendResult,
		&item.SendError, &failCount, &item.AlertExtra,
		&item.ActiveFlag, &item.NoteText, &addTime, &item.AddWho, &editTime, &item.EditWho)
	if err != nil {
		return models.Log{}, err
	}
	item.SendTime = validate.AsText(sendTime)
	item.AddTime = validate.AsText(addTime)
	item.EditTime = validate.AsText(editTime)
	n, _ := validate.AsFloat(failCount)
	item.FailCount = int(n)
	return item, nil
}

type pendingScan struct {
	tenantID string
	log      models.Log
}

func scanPending(scan scanner) (pendingScan, error) {
	var sendTime, addTime, editTime, failCount any
	var item pendingScan
	err := scan(
		&item.tenantID,
		&item.log.AlertLogID, &item.log.AlertLevel, &item.log.AlertType, &item.log.SceneCode, &item.log.AlertTitle, &item.log.AlertContent,
		&item.log.AlertTimestamp, &item.log.ChannelID, &item.log.SendStatus, &sendTime, &item.log.SendResult,
		&item.log.SendError, &failCount, &item.log.AlertExtra,
		&item.log.ActiveFlag, &item.log.NoteText, &addTime, &item.log.AddWho, &editTime, &item.log.EditWho)
	if err != nil {
		return pendingScan{}, err
	}
	item.log.SendTime = validate.AsText(sendTime)
	item.log.AddTime = validate.AsText(addTime)
	item.log.EditTime = validate.AsText(editTime)
	n, _ := validate.AsFloat(failCount)
	item.log.FailCount = int(n)
	return item, nil
}

func emptyNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

var errMissing = errNoRow{}

type errNoRow struct{}

func (errNoRow) Error() string { return "预警日志不存在" }
