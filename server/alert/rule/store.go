package rule

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/server/alert/models"
	"fluxguard/server/alert/validate"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// Store 读写通用预警规则。条件 JSON 原样保存，不解释字段。
type Store struct {
	db *dbx.DB
}

// NewStore 创建规则 Store。
func NewStore(db *dbx.DB) *Store {
	return &Store{db: db}
}

const ruleSelect = `
	SELECT ruleId, ruleName, alertType, levelCode, COALESCE(channelId, ''),
	       COALESCE(silenceStart, ''), COALESCE(silenceEnd, ''), statusFlag, conditionText,
	       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion
	FROM FG_ALERT_RULE`

// Page 按当前引擎分页返回规则。
func (st *Store) Page(tenantID string, q models.RuleQuery, page dbx.Page) ([]models.Rule, int64, error) {
	page.OrderBy = ""
	where, args := ruleWhere(tenantID, q)
	rows, total, err := st.db.QueryPage(context.Background(), ruleSelect+where+`
		ORDER BY ruleName, ruleId`, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Rule{}
	for rows.Next() {
		item, err := scanRule(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// List 返回某类型下的全部规则，供功能模块扫描。
func (st *Store) List(tenantID, alertType string) ([]models.Rule, error) {
	where, args := ruleWhere(tenantID, models.RuleQuery{AlertType: alertType})
	rows, err := st.db.Query(context.Background(), ruleSelect+where+` ORDER BY ruleName, ruleId`, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Rule{}
	for rows.Next() {
		item, err := scanRule(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Get 按编码和类型读取一条规则。
func (st *Store) Get(ctx context.Context, tenantID, ruleID, alertType string) (models.Rule, error) {
	rows, err := st.db.Query(ctx, ruleSelect+`
		WHERE tenantId = @tenantId AND ruleId = @ruleId AND alertType = @alertType`,
		dbx.Args{"tenantId": tenantID, "ruleId": ruleID, "alertType": alertType})
	if err != nil {
		return models.Rule{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return models.Rule{}, err
		}
		return models.Rule{}, validate.ErrText("规则不存在")
	}
	return scanRule(rows.Scan)
}

// Save 新增或修改规则。类型在创建时写入，修改时不能改成别的类型。
func (st *Store) Save(ctx context.Context, in models.RuleInput, creating bool) (models.Rule, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Rule{}, err
	}
	in, err = normalize(in, creating)
	if err != nil {
		return models.Rule{}, err
	}
	var n int
	if err = store.Scan(ctx, st.db, `SELECT count(*) FROM FG_ALERT_RULE WHERE tenantId = @tenantId AND ruleId = @ruleId`,
		dbx.Args{"tenantId": sess.TenantID, "ruleId": in.RuleID}, &n); err != nil {
		return models.Rule{}, err
	}
	if creating && n > 0 {
		return models.Rule{}, validate.ErrText("规则编码已存在")
	}
	if !creating && n == 0 {
		return models.Rule{}, validate.ErrText("规则不存在")
	}
	if !creating {
		if _, err = st.Get(ctx, sess.TenantID, in.RuleID, in.AlertType); err != nil {
			return models.Rule{}, err
		}
	}
	args := dbx.Args{
		"tenantId": sess.TenantID, "ruleId": in.RuleID, "ruleName": in.RuleName, "alertType": in.AlertType,
		"levelCode": in.LevelCode, "channelId": emptyNil(in.ChannelID),
		"silenceStart": emptyNil(in.SilenceStart), "silenceEnd": emptyNil(in.SilenceEnd),
		"statusFlag": in.StatusFlag, "conditionText": in.ConditionText, "activeFlag": in.ActiveFlag,
		"noteText": in.NoteText,
	}
	var query string
	if creating {
		now := store.FormatTime(sess.At)
		args["addTime"] = now
		args["addWho"] = sess.UserID
		args["editTime"] = now
		args["editWho"] = sess.UserID
		args["oprSeqFlag"] = id.New()
		query = `
			INSERT INTO FG_ALERT_RULE (
			  tenantId, ruleId, ruleName, alertType, levelCode, channelId, silenceStart, silenceEnd,
			  statusFlag, conditionText, addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
			) VALUES (
			  @tenantId, @ruleId, @ruleName, @alertType, @levelCode, @channelId, @silenceStart, @silenceEnd,
			  @statusFlag, @conditionText, @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, 1, @activeFlag, @noteText
			)`
	} else {
		args["editTime"] = store.FormatTime(sess.At)
		args["editWho"] = sess.UserID
		args["oprSeqFlag"] = id.New()
		query = `
			UPDATE FG_ALERT_RULE SET
			  ruleName = @ruleName, levelCode = @levelCode, channelId = @channelId,
			  silenceStart = @silenceStart, silenceEnd = @silenceEnd, statusFlag = @statusFlag,
			  conditionText = @conditionText, noteText = @noteText, activeFlag = @activeFlag,
			  editTime = @editTime, editWho = @editWho, oprSeqFlag = @oprSeqFlag,
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND ruleId = @ruleId AND alertType = @alertType`
	}
	if _, err = st.db.Exec(ctx, query, args); err != nil {
		return models.Rule{}, err
	}
	return st.Get(ctx, sess.TenantID, in.RuleID, in.AlertType)
}

// Delete 删除指定类型的规则。类型不符时当作不存在。
func (st *Store) Delete(ctx context.Context, ruleID, alertType string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	ruleID = strings.TrimSpace(ruleID)
	alertType = strings.TrimSpace(alertType)
	if ruleID == "" {
		return validate.ErrText("缺少规则编码")
	}
	if _, err = st.Get(ctx, sess.TenantID, ruleID, alertType); err != nil {
		return err
	}
	_, err = st.db.Exec(ctx, `DELETE FROM FG_ALERT_RULE WHERE tenantId = @tenantId AND ruleId = @ruleId AND alertType = @alertType`,
		dbx.Args{"tenantId": sess.TenantID, "ruleId": ruleID, "alertType": alertType})
	return err
}

func normalize(in models.RuleInput, creating bool) (models.RuleInput, error) {
	name, err := validate.CleanName(in.RuleName, "规则名称")
	if err != nil {
		return models.RuleInput{}, err
	}
	ruleID, err := validate.CleanID(in.RuleID, creating)
	if err != nil {
		return models.RuleInput{}, err
	}
	alertType := strings.TrimSpace(in.AlertType)
	if !validType(alertType) {
		return models.RuleInput{}, validate.ErrText("规则类型无效")
	}
	level := strings.TrimSpace(in.LevelCode)
	if !validate.ValidLevel(level) {
		level = "warn"
	}
	silenceStart, silenceEnd, err := validate.SilenceWindow(in.SilenceStart, in.SilenceEnd)
	if err != nil {
		return models.RuleInput{}, err
	}
	note, err := validate.CleanNote(in.NoteText)
	if err != nil {
		return models.RuleInput{}, err
	}
	condition := strings.TrimSpace(in.ConditionText)
	if condition == "" {
		condition = "{}"
	}
	if !json.Valid([]byte(condition)) || !strings.HasPrefix(condition, "{") {
		return models.RuleInput{}, validate.ErrText("规则条件必须是 JSON 对象")
	}
	if utf8.RuneCountInString(condition) > 8000 {
		return models.RuleInput{}, validate.ErrText("规则条件过长")
	}
	in.RuleID = ruleID
	in.RuleName = name
	in.AlertType = alertType
	in.LevelCode = level
	in.ChannelID = strings.TrimSpace(in.ChannelID)
	in.SilenceStart = silenceStart
	in.SilenceEnd = silenceEnd
	in.StatusFlag = validate.Flag(in.StatusFlag, "Y")
	in.ActiveFlag = validate.Flag(in.ActiveFlag, "Y")
	in.ConditionText = condition
	in.NoteText = note
	return in, nil
}

func validType(value string) bool {
	if value == "" || utf8.RuneCountInString(value) > 32 {
		return false
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			if i == 0 && (r < 'a' || r > 'z') {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func ruleWhere(tenantID string, q models.RuleQuery) (string, dbx.Args) {
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId"
	where += store.LikeClause(args, "ruleName", "ruleName", q.RuleName)
	where += store.EqClause(args, "alertType", "alertType", q.AlertType)
	where += store.EqClause(args, "levelCode", "levelCode", q.LevelCode)
	where += store.EqClause(args, "statusFlag", "statusFlag", q.StatusFlag)
	where += store.LikeClause(args, "conditionText", "conditionMatch", q.ConditionMatch)
	return where, args
}

func scanRule(scan func(dest ...any) error) (models.Rule, error) {
	var silenceStart, silenceEnd, addTime, editTime any
	var item models.Rule
	err := scan(&item.RuleID, &item.RuleName, &item.AlertType, &item.LevelCode, &item.ChannelID,
		&silenceStart, &silenceEnd, &item.StatusFlag, &item.ConditionText,
		&item.ActiveFlag, &item.NoteText, &addTime, &item.AddWho, &editTime, &item.EditWho, &item.OprSeqFlag, &item.CurrentVersion)
	if err != nil {
		return models.Rule{}, err
	}
	item.SilenceStart = validate.AsText(silenceStart)
	item.SilenceEnd = validate.AsText(silenceEnd)
	item.AddTime = validate.AsText(addTime)
	item.EditTime = validate.AsText(editTime)
	return item, nil
}

func emptyNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
