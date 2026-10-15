package channel

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/alert/models"
	"fluxguard/server/alert/template"
	"fluxguard/server/alert/validate"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// Store 读写通知渠道。
type Store struct {
	db  *dbx.DB
	tpl *template.Store
}

// NewStore 创建渠道 Store。
func NewStore(db *dbx.DB, tpl *template.Store) *Store {
	return &Store{db: db, tpl: tpl}
}

// Page 按当前引擎分页返回渠道。
func (st *Store) Page(tenantID string, q models.ChannelQuery, page dbx.Page) ([]models.Channel, int64, error) {
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId"
	where += store.LikeClause(args, "channelName", "channelName", q.ChannelName)
	where += store.EqClause(args, "channelKind", "channelKind", q.ChannelKind)
	where += store.EqClause(args, "statusFlag", "statusFlag", q.StatusFlag)
	listSQL := `
		SELECT channelId, channelName, channelKind, configText, statusFlag, activeFlag, COALESCE(noteText, ''),
		       addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion
		FROM FG_ALERT_CHANNEL` + where + `
		ORDER BY channelName, channelId`
	rows, total, err := st.db.QueryPage(context.Background(), listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Channel{}
	for rows.Next() {
		item, _, err := scanChannel(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	st.tpl.AttachAll(context.Background(), tenantID, out)
	return out, total, nil
}

// Save 新增或修改渠道。
func (st *Store) Save(ctx context.Context, in models.ChannelInput, creating bool) (models.Channel, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Channel{}, err
	}
	tenantID, who := sess.TenantID, sess.UserID
	name, err := validate.CleanName(in.ChannelName, "渠道名称")
	if err != nil {
		return models.Channel{}, err
	}
	in.ChannelKind = strings.TrimSpace(in.ChannelKind)
	if !validate.ValidKind(in.ChannelKind) || in.ChannelKind == "*" {
		return models.Channel{}, validate.ErrText("渠道种类无效")
	}
	in.TemplateID = strings.TrimSpace(in.TemplateID)
	if in.TemplateID == "" {
		in.TemplateID = template.BuiltinID(in.ChannelKind)
	}
	if err = st.tpl.Require(ctx, tenantID, in.TemplateID, in.ChannelKind); err != nil {
		return models.Channel{}, err
	}
	note, err := validate.CleanNote(in.NoteText)
	if err != nil {
		return models.Channel{}, err
	}
	channelID, err := validate.CleanID(in.ChannelID, creating)
	if err != nil {
		return models.Channel{}, err
	}
	old := map[string]any{}
	var n int
	err = store.Scan(ctx, st.db, `SELECT count(*) FROM FG_ALERT_CHANNEL WHERE tenantId = @tenantId AND channelId = @channelId`,
		dbx.Args{"tenantId": tenantID, "channelId": channelID}, &n)
	if err != nil {
		return models.Channel{}, err
	}
	if creating && n > 0 {
		return models.Channel{}, validate.ErrText("渠道编码已存在")
	}
	if !creating && n == 0 {
		return models.Channel{}, validate.ErrText("渠道不存在")
	}
	if !creating {
		_, old, err = st.Load(ctx, tenantID, channelID)
		if err != nil {
			return models.Channel{}, err
		}
	}
	cfg, err := applyChannel(old, in)
	if err != nil {
		return models.Channel{}, err
	}
	text, err := writeConfig(cfg)
	if err != nil {
		return models.Channel{}, err
	}
	args := dbx.Args{
		"tenantId": tenantID, "channelId": channelID, "channelName": name,
		"channelKind": in.ChannelKind, "configText": text, "statusFlag": validate.Flag(in.StatusFlag, "Y"),
	}
	if creating {
		for key, value := range stampArgs(who, note, "Y", sess.At) {
			args[key] = value
		}
		_, err = st.db.Exec(ctx, `
			INSERT INTO FG_ALERT_CHANNEL (
			  tenantId, channelId, channelName, channelKind, configText, statusFlag,
			  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
			) VALUES (
			  @tenantId, @channelId, @channelName, @channelKind, @configText, @statusFlag,
			  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
			)`, args)
	} else {
		for key, value := range touchArgs(who, note, "Y", sess.At) {
			args[key] = value
		}
		_, err = st.db.Exec(ctx, `
			UPDATE FG_ALERT_CHANNEL SET
			  channelName = @channelName, channelKind = @channelKind, configText = @configText,
			  statusFlag = @statusFlag, noteText = @noteText, activeFlag = @activeFlag,
			  editTime = @editTime, editWho = @editWho, oprSeqFlag = @oprSeqFlag,
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND channelId = @channelId`, args)
	}
	if err != nil {
		return models.Channel{}, err
	}
	view, _, err := st.Load(ctx, tenantID, channelID)
	return view, err
}

// Delete 删除渠道。
func (st *Store) Delete(ctx context.Context, channelID string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	return st.deleteBy(ctx, sess.TenantID, "FG_ALERT_CHANNEL", "channelId", channelID, "渠道")
}

// Load 读取渠道及原始配置。
func (st *Store) Load(ctx context.Context, tenantID, channelID string) (models.Channel, map[string]any, error) {
	var configText string
	var addTime, editTime any
	var item models.Channel
	err := store.Scan(ctx, st.db, `
		SELECT channelId, channelName, channelKind, configText, statusFlag, activeFlag, COALESCE(noteText, ''),
		       addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion
		FROM FG_ALERT_CHANNEL WHERE tenantId = @tenantId AND channelId = @channelId`,
		dbx.Args{"tenantId": tenantID, "channelId": channelID},
		&item.ChannelID, &item.ChannelName, &item.ChannelKind, &configText, &item.StatusFlag, &item.ActiveFlag, &item.NoteText,
		&addTime, &item.AddWho, &editTime, &item.EditWho, &item.OprSeqFlag, &item.CurrentVersion)
	if err != nil {
		return models.Channel{}, nil, err
	}
	item.AddTime = validate.AsText(addTime)
	item.EditTime = validate.AsText(editTime)
	cfg := ParseConfig(configText)
	view := channelFromConfig(item, cfg)
	st.tpl.Attach(ctx, tenantID, &view)
	return view, cfg, nil
}

// Require 确认渠道编码属于该租户。空编码表示不发送。
func (st *Store) Require(ctx context.Context, tenantID, channelID string) error {
	if strings.TrimSpace(channelID) == "" {
		return nil
	}
	var n int
	err := store.Scan(ctx, st.db, `SELECT count(*) FROM FG_ALERT_CHANNEL WHERE tenantId = @tenantId AND channelId = @channelId`,
		dbx.Args{"tenantId": tenantID, "channelId": channelID}, &n)
	if err != nil {
		return err
	}
	if n == 0 {
		return validate.ErrText("渠道不存在")
	}
	return nil
}

// Missing 判断是否为无行错误。
func (st *Store) Missing(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

type scanner func(dest ...any) error

func scanChannel(scan scanner) (models.Channel, map[string]any, error) {
	var configText string
	var addTime, editTime any
	var item models.Channel
	err := scan(&item.ChannelID, &item.ChannelName, &item.ChannelKind, &configText, &item.StatusFlag, &item.ActiveFlag, &item.NoteText,
		&addTime, &item.AddWho, &editTime, &item.EditWho, &item.OprSeqFlag, &item.CurrentVersion)
	if err != nil {
		return models.Channel{}, nil, err
	}
	item.AddTime = validate.AsText(addTime)
	item.EditTime = validate.AsText(editTime)
	cfg := ParseConfig(configText)
	return channelFromConfig(item, cfg), cfg, nil
}

func (st *Store) deleteBy(ctx context.Context, tenantID, table, column, id, label string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return validate.ErrText("缺少" + label + "编码")
	}
	queryCount := "SELECT count(*) FROM " + table + " WHERE tenantId = @tenantId AND " + column + " = @id"
	var n int
	err := store.Scan(ctx, st.db, queryCount, dbx.Args{"tenantId": tenantID, "id": id}, &n)
	if err != nil {
		return err
	}
	if n == 0 {
		return validate.ErrText(label + "不存在")
	}
	query := "DELETE FROM " + table + " WHERE tenantId = @tenantId AND " + column + " = @id"
	_, err = st.db.Exec(ctx, query, dbx.Args{"tenantId": tenantID, "id": id})
	return err
}

func stampArgs(who, note, active string, at time.Time) dbx.Args {
	now := store.FormatTime(at)
	return dbx.Args{
		"addTime": now, "addWho": who, "editTime": now, "editWho": who,
		"oprSeqFlag": id.New(), "currentVersion": 1, "activeFlag": validate.Flag(active, "Y"),
		"noteText": note,
	}
}

func touchArgs(who, note, active string, at time.Time) dbx.Args {
	return dbx.Args{
		"editTime": store.FormatTime(at), "editWho": who,
		"oprSeqFlag": id.New(), "activeFlag": validate.Flag(active, "Y"), "noteText": note,
	}
}
