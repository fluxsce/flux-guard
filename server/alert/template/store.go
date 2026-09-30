package template

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/server/alert/models"
	"fluxguard/server/alert/validate"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

const templateSelect = `
	SELECT templateId, templateName, channelKind, contentType, subjectText, bodyText,
	       builtInFlag, statusFlag, activeFlag, COALESCE(noteText, ''),
	       addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion
	FROM FG_ALERT_TEMPLATE`

// Store 读写预警模板。
type Store struct {
	db *dbx.DB
}

// NewStore 创建模板 Store。
func NewStore(db *dbx.DB) *Store {
	return &Store{db: db}
}

// Page 按当前引擎分页返回模板。
func (st *Store) Page(tenantID string, q models.TemplateQuery, page dbx.Page) ([]models.Template, int64, error) {
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId"
	where += store.LikeClause(args, "templateName", "templateName", q.TemplateName)
	where += store.EqClause(args, "channelKind", "channelKind", q.ChannelKind)
	where += store.EqClause(args, "statusFlag", "statusFlag", q.StatusFlag)
	countSQL := "SELECT COUNT(*) FROM FG_ALERT_TEMPLATE" + where
	listSQL := templateSelect + where + `
		ORDER BY channelKind, templateName, templateId`
	rows, total, err := st.db.QueryPage(context.Background(), countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Template{}
	for rows.Next() {
		item, err := scanTemplate(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// Save 新增或修改模板。内置模板可以改文案，不能改种类。
func (st *Store) Save(ctx context.Context, in models.TemplateInput, creating bool) (models.Template, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Template{}, err
	}
	tenantID, who := sess.TenantID, sess.UserID
	name, err := validate.CleanName(in.TemplateName, "模板名称")
	if err != nil {
		return models.Template{}, err
	}
	kind := strings.TrimSpace(in.ChannelKind)
	if !validate.ValidKind(kind) {
		return models.Template{}, validate.ErrText("渠道种类无效")
	}
	content := strings.TrimSpace(in.ContentType)
	if content == "" {
		content = "text"
	}
	if !validate.ValidContent(content) {
		return models.Template{}, validate.ErrText("正文类型无效")
	}
	subject := strings.TrimSpace(in.SubjectText)
	body := strings.TrimSpace(in.BodyText)
	if subject == "" || body == "" {
		return models.Template{}, validate.ErrText("请填写标题和正文")
	}
	if utf8.RuneCountInString(subject) > 255 {
		return models.Template{}, validate.ErrText("标题过长")
	}
	if utf8.RuneCountInString(body) > 8000 {
		return models.Template{}, validate.ErrText("正文过长")
	}
	note, err := validate.CleanNote(in.NoteText)
	if err != nil {
		return models.Template{}, err
	}
	templateID, err := validate.CleanID(in.TemplateID, creating)
	if err != nil {
		return models.Template{}, err
	}
	existing, err := st.Get(ctx, tenantID, templateID)
	missingRow := errors.Is(err, sql.ErrNoRows)
	if err != nil && !missingRow {
		return models.Template{}, err
	}
	if creating && !missingRow {
		return models.Template{}, validate.ErrText("模板编码已存在")
	}
	if !creating && missingRow {
		return models.Template{}, validate.ErrText("模板不存在")
	}
	builtIn := "N"
	if !creating && existing.BuiltInFlag == "Y" {
		builtIn = "Y"
		kind = existing.ChannelKind
	}
	args := dbx.Args{
		"tenantId": tenantID, "templateId": templateID, "templateName": name,
		"channelKind": kind, "contentType": content, "subjectText": subject, "bodyText": body,
		"builtInFlag": builtIn, "statusFlag": validate.Flag(in.StatusFlag, "Y"),
	}
	if creating {
		for key, value := range stampArgs(who, note, "Y", sess.At) {
			args[key] = value
		}
		_, err = st.db.Exec(ctx, `
			INSERT INTO FG_ALERT_TEMPLATE (
			  tenantId, templateId, templateName, channelKind, contentType, subjectText, bodyText,
			  builtInFlag, statusFlag,
			  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
			) VALUES (
			  @tenantId, @templateId, @templateName, @channelKind, @contentType, @subjectText, @bodyText,
			  @builtInFlag, @statusFlag,
			  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
			)`, args)
	} else {
		for key, value := range touchArgs(who, note, "Y", sess.At) {
			args[key] = value
		}
		_, err = st.db.Exec(ctx, `
			UPDATE FG_ALERT_TEMPLATE SET
			  templateName = @templateName, channelKind = @channelKind, contentType = @contentType,
			  subjectText = @subjectText, bodyText = @bodyText, statusFlag = @statusFlag,
			  noteText = @noteText, activeFlag = @activeFlag,
			  editTime = @editTime, editWho = @editWho, oprSeqFlag = @oprSeqFlag,
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND templateId = @templateId`, args)
	}
	if err != nil {
		return models.Template{}, err
	}
	return st.Get(ctx, tenantID, templateID)
}

// Delete 删除自定义模板。内置模板保留。
func (st *Store) Delete(ctx context.Context, templateID string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return validate.ErrText("缺少模板编码")
	}
	item, err := st.Get(ctx, sess.TenantID, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return validate.ErrText("模板不存在")
		}
		return err
	}
	if item.BuiltInFlag == "Y" {
		return validate.ErrText("内置模板不能删除")
	}
	_, err = st.db.Exec(ctx, `DELETE FROM FG_ALERT_TEMPLATE WHERE tenantId = @tenantId AND templateId = @templateId`,
		dbx.Args{"tenantId": sess.TenantID, "templateId": templateID})
	return err
}

// Get 按编码读取模板。
func (st *Store) Get(ctx context.Context, tenantID, templateID string) (models.Template, error) {
	var addTime, editTime any
	var item models.Template
	err := store.Scan(ctx, st.db, templateSelect+` WHERE tenantId = @tenantId AND templateId = @templateId`,
		dbx.Args{"tenantId": tenantID, "templateId": templateID},
		&item.TemplateID, &item.TemplateName, &item.ChannelKind, &item.ContentType, &item.SubjectText, &item.BodyText,
		&item.BuiltInFlag, &item.StatusFlag, &item.ActiveFlag, &item.NoteText,
		&addTime, &item.AddWho, &editTime, &item.EditWho, &item.OprSeqFlag, &item.CurrentVersion)
	if err != nil {
		return models.Template{}, err
	}
	item.AddTime = validate.AsText(addTime)
	item.EditTime = validate.AsText(editTime)
	return item, nil
}

type scanner func(dest ...any) error

func scanTemplate(scan scanner) (models.Template, error) {
	var addTime, editTime any
	var item models.Template
	err := scan(&item.TemplateID, &item.TemplateName, &item.ChannelKind, &item.ContentType, &item.SubjectText, &item.BodyText,
		&item.BuiltInFlag, &item.StatusFlag, &item.ActiveFlag, &item.NoteText,
		&addTime, &item.AddWho, &editTime, &item.EditWho, &item.OprSeqFlag, &item.CurrentVersion)
	if err != nil {
		return models.Template{}, err
	}
	item.AddTime = validate.AsText(addTime)
	item.EditTime = validate.AsText(editTime)
	return item, nil
}

// Require 确认模板存在且与渠道种类匹配。
func (st *Store) Require(ctx context.Context, tenantID, templateID, kind string) error {
	item, err := st.Get(ctx, tenantID, templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return validate.ErrText("模板不存在")
		}
		return err
	}
	if item.ChannelKind != kind && item.ChannelKind != "*" {
		return validate.ErrText("模板与渠道种类不一致")
	}
	if item.StatusFlag != "Y" || item.ActiveFlag != "Y" {
		return validate.ErrText("模板已停用")
	}
	return nil
}

// Attach 填充渠道上的模板名称。
func (st *Store) Attach(ctx context.Context, tenantID string, item *models.Channel) {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.TemplateID) == "" {
		item.TemplateID = BuiltinID(item.ChannelKind)
	}
	tpl, err := st.Get(ctx, tenantID, item.TemplateID)
	if err != nil {
		return
	}
	item.TemplateName = tpl.TemplateName
}

// AttachAll 批量填充渠道模板名称。
func (st *Store) AttachAll(ctx context.Context, tenantID string, rows []models.Channel) {
	for i := range rows {
		st.Attach(ctx, tenantID, &rows[i])
	}
}

// ForChannel 返回渠道应使用的模板。
func (st *Store) ForChannel(ctx context.Context, tenantID string, view models.Channel, _ string) models.Template {
	id := strings.TrimSpace(view.TemplateID)
	if id == "" {
		id = BuiltinID(view.ChannelKind)
	}
	tpl, err := st.Get(ctx, tenantID, id)
	if err != nil || strings.TrimSpace(tpl.BodyText) == "" || tpl.StatusFlag != "Y" ||
		(tpl.ChannelKind != view.ChannelKind && tpl.ChannelKind != "*") {
		tpl = FallbackTemplate(view.ChannelKind)
	}
	if tpl.TemplateID == "" {
		tpl.TemplateID = id
	}
	return tpl
}

// Preview 用样例数据渲染标题和正文，不发送。
func (st *Store) Preview(tenantID string, in models.PreviewInput) (models.Preview, error) {
	scene := strings.TrimSpace(in.SceneCode)
	if scene == "" {
		scene = "fire"
	}
	vars := SampleVars()
	vars["scene"] = validate.SceneName(scene)
	if id := strings.TrimSpace(in.TemplateID); id != "" && st != nil {
		tpl, err := st.Get(context.Background(), tenantID, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return models.Preview{}, validate.ErrText("模板不存在")
			}
			return models.Preview{}, err
		}
		return models.Preview{
			SubjectText: Render(tpl.SubjectText, vars),
			BodyText:    Render(tpl.BodyText, vars),
			ContentType: tpl.ContentType,
		}, nil
	}
	kind := strings.TrimSpace(in.ChannelKind)
	content := strings.TrimSpace(in.ContentType)
	subject := strings.TrimSpace(in.SubjectText)
	body := strings.TrimSpace(in.BodyText)
	if body == "" {
		tpl := FallbackTemplate(kind)
		if subject == "" {
			subject = tpl.SubjectText
		}
		body = tpl.BodyText
		if content == "" {
			content = tpl.ContentType
		}
	}
	if content == "" {
		content = "text"
	}
	return models.Preview{SubjectText: Render(subject, vars), BodyText: Render(body, vars), ContentType: content}, nil
}

// Missing 判断是否为无行错误。
func (st *Store) Missing(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
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
