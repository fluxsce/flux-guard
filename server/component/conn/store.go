package conn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/crypto"
	"fluxguard/common/id"
	"fluxguard/server/component/catalog"
	"fluxguard/server/component/models"
	"fluxguard/server/component/profile"
	"fluxguard/server/component/row"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

const (
	minInterval = 15
	maxInterval = 86400
)

// Open 是拨号时用的连接，含已解密口令。不进入查询接口。
type Open struct {
	ComponentID  string
	TenantID     string
	KindCode     string
	Address      string
	UserName     string
	Secret       string
	DatabaseName string
	OptionText   string
	IntervalSec  int
	ProbedTime   string
}

// Filter 是连接列表的筛选。
type Filter struct {
	KindCode      string
	ComponentName string
	StatusFlag    string
}

var orderCols = map[string]struct{}{
	"componentName": {}, "kindCode": {}, "probedTime": {}, "upFlag": {}, "intervalSec": {},
}

// Save 新增或修改一条连接。口令留空时修改操作保留原密文。
func Save(ctx context.Context, db *dbx.DB, in models.Input, creating bool) (models.Conn, error) {
	if db == nil {
		return models.Conn{}, fmt.Errorf("数据库未打开")
	}
	sess, err := request.Require(ctx)
	if err != nil {
		return models.Conn{}, err
	}
	kind := strings.TrimSpace(in.KindCode)
	if !catalog.Known(kind) {
		return models.Conn{}, fmt.Errorf("不支持的组件种类")
	}
	name := strings.TrimSpace(in.ComponentName)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return models.Conn{}, fmt.Errorf("请填写不超过 100 字的名称")
	}
	address := strings.TrimSpace(in.Address)
	if address == "" || len(address) > 255 {
		return models.Conn{}, fmt.Errorf("请填写不超过 255 字的地址")
	}
	userName := strings.TrimSpace(in.UserName)
	if len(userName) > 64 {
		return models.Conn{}, fmt.Errorf("用户名过长")
	}
	databaseName := strings.TrimSpace(in.DatabaseName)
	if len(databaseName) > 64 {
		return models.Conn{}, fmt.Errorf("库名过长")
	}
	optionText := strings.TrimSpace(in.OptionText)
	if len(optionText) > 100 {
		return models.Conn{}, fmt.Errorf("附加选项过长")
	}
	spec, err := profile.Normalize(kind, optionText, userName, in.Secret, databaseName, creating)
	if err != nil {
		return models.Conn{}, err
	}
	optionText = spec.Option
	userName = spec.User
	databaseName = spec.Database
	note := strings.TrimSpace(in.NoteText)
	if utf8.RuneCountInString(note) > 500 {
		return models.Conn{}, fmt.Errorf("备注过长")
	}
	interval := in.IntervalSec
	if interval == 0 {
		interval = 30
	}
	if interval < minInterval || interval > maxInterval {
		return models.Conn{}, fmt.Errorf("采集间隔须在 %d 到 %d 秒之间", minInterval, maxInterval)
	}
	status := strings.ToUpper(strings.TrimSpace(in.StatusFlag))
	if status != "Y" && status != "N" {
		status = "Y"
	}
	secret := in.Secret
	if utf8.RuneCountInString(secret) > 200 {
		return models.Conn{}, fmt.Errorf("口令过长")
	}
	componentID := strings.TrimSpace(in.ComponentID)
	if creating {
		componentID = id.New()
	} else if componentID == "" {
		return models.Conn{}, fmt.Errorf("缺少连接编号")
	}
	var sealed string
	if creating {
		if secret != "" && !spec.ClearSecret {
			sealed, err = crypto.Encrypt("", secret)
			if err != nil {
				return models.Conn{}, err
			}
		}
	} else {
		existing, err := loadSecret(ctx, db, sess.TenantID, componentID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return models.Conn{}, fmt.Errorf("连接不存在")
			}
			return models.Conn{}, err
		}
		sealed = existing
		if spec.ClearSecret {
			sealed = ""
		} else if strings.TrimSpace(secret) != "" {
			sealed, err = crypto.Encrypt("", secret)
			if err != nil {
				return models.Conn{}, err
			}
		}
	}
	args := dbx.Args{
		"tenantId": sess.TenantID, "componentId": componentID, "kindCode": kind,
		"componentName": name, "address": address, "userName": userName, "secretEnc": sealed,
		"databaseName": databaseName, "optionText": optionText, "intervalSec": interval,
		"statusFlag": status, "noteText": note,
	}
	if creating {
		for key, value := range row.Stamp(sess.UserID, note) {
			args[key] = value
		}
		args["upFlag"] = "N"
		_, err = db.Exec(ctx, `
			INSERT INTO FG_COMPONENT_CONN (
			  tenantId, componentId, kindCode, componentName, address, userName, secretEnc,
			  databaseName, optionText, intervalSec, statusFlag, upFlag,
			  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
			) VALUES (
			  @tenantId, @componentId, @kindCode, @componentName, @address, @userName, @secretEnc,
			  @databaseName, @optionText, @intervalSec, @statusFlag, @upFlag,
			  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
			)`, args)
	} else {
		for key, value := range row.Touch(sess.UserID) {
			args[key] = value
		}
		_, err = db.Exec(ctx, `
			UPDATE FG_COMPONENT_CONN SET
			  kindCode = @kindCode, componentName = @componentName, address = @address,
			  userName = @userName, secretEnc = @secretEnc, databaseName = @databaseName,
			  optionText = @optionText, intervalSec = @intervalSec, statusFlag = @statusFlag,
			  noteText = @noteText, editTime = @editTime, editWho = @editWho, oprSeqFlag = @oprSeqFlag,
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND componentId = @componentId AND activeFlag = 'Y'`, args)
	}
	if err != nil {
		return models.Conn{}, err
	}
	return Get(ctx, db, sess.TenantID, componentID)
}

// Delete 停用一条连接，历史曲线保留。
func Delete(ctx context.Context, db *dbx.DB, componentID string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	componentID = strings.TrimSpace(componentID)
	if componentID == "" {
		return fmt.Errorf("缺少连接编号")
	}
	touch := row.Touch(sess.UserID)
	res, err := db.Exec(ctx, `
		UPDATE FG_COMPONENT_CONN SET
		  activeFlag = 'N', editTime = @editTime, editWho = @editWho, oprSeqFlag = @oprSeqFlag,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND componentId = @componentId AND activeFlag = 'Y'`,
		dbx.Args{
			"tenantId": sess.TenantID, "componentId": componentID,
			"editTime": touch["editTime"], "editWho": touch["editWho"], "oprSeqFlag": touch["oprSeqFlag"],
		})
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("连接不存在")
	}
	return nil
}

// Get 返回一条连接，不含口令。
func Get(ctx context.Context, db *dbx.DB, tenantID, componentID string) (models.Conn, error) {
	var item models.Conn
	var probed, add, edit any
	err := store.Scan(ctx, db, `
		SELECT componentId, kindCode, componentName, address, COALESCE(userName, ''),
		       CASE WHEN secretEnc IS NULL OR secretEnc = '' THEN 'N' ELSE 'Y' END,
		       COALESCE(databaseName, ''), COALESCE(optionText, ''), intervalSec, statusFlag,
		       upFlag, COALESCE(versionText, ''), COALESCE(roleName, ''), COALESCE(errorText, ''),
		       COALESCE(summaryText, ''), probedTime, COALESCE(noteText, ''), addTime, editTime
		FROM FG_COMPONENT_CONN
		WHERE tenantId = @tenantId AND componentId = @componentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "componentId": componentID},
		&item.ComponentID, &item.KindCode, &item.ComponentName, &item.Address, &item.UserName, &item.SecretSet,
		&item.DatabaseName, &item.OptionText, &item.IntervalSec, &item.StatusFlag, &item.UpFlag,
		&item.VersionText, &item.RoleName, &item.ErrorText, &item.SummaryText,
		&probed, &item.NoteText, &add, &edit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Conn{}, fmt.Errorf("连接不存在")
		}
		return models.Conn{}, err
	}
	item.ProbedTime = row.Text(probed)
	item.AddTime = row.Text(add)
	item.EditTime = row.Text(edit)
	if kind, ok := catalog.Get(item.KindCode); ok {
		item.KindName = kind.KindName
	}
	return item, nil
}

// List 分页返回本租户的连接。
func List(ctx context.Context, db *dbx.DB, tenantID string, filter Filter, page dbx.Page) ([]models.Conn, int64, error) {
	if db == nil {
		return []models.Conn{}, 0, nil
	}
	if _, ok := orderCols[page.OrderBy]; !ok {
		page.OrderBy = ""
	}
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y'"
	where += store.EqClause(args, "kindCode", "kindCode", filter.KindCode)
	where += store.LikeClause(args, "componentName", "nameLike", filter.ComponentName)
	where += store.EqClause(args, "statusFlag", "statusFlag", filter.StatusFlag)
	countSQL := "SELECT COUNT(*) FROM FG_COMPONENT_CONN" + where
	listSQL := `
		SELECT componentId, kindCode, componentName, address, COALESCE(userName, ''),
		       CASE WHEN secretEnc IS NULL OR secretEnc = '' THEN 'N' ELSE 'Y' END,
		       COALESCE(databaseName, ''), COALESCE(optionText, ''), intervalSec, statusFlag,
		       upFlag, COALESCE(versionText, ''), COALESCE(roleName, ''), COALESCE(errorText, ''),
		       COALESCE(summaryText, ''), probedTime, COALESCE(noteText, ''), addTime, editTime
		FROM FG_COMPONENT_CONN` + where + `
		ORDER BY componentName`
	rows, total, err := db.QueryPage(ctx, countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []models.Conn
	for rows.Next() {
		item, err := scanConn(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		if kind, ok := catalog.Get(item.KindCode); ok {
			item.KindName = kind.KindName
		}
		out = append(out, item)
	}
	if out == nil {
		out = []models.Conn{}
	}
	return out, total, rows.Err()
}

// Due 返回到达采集时间、且处于启用状态的连接。口令已解密。
func Due(ctx context.Context, db *dbx.DB) ([]Open, error) {
	if db == nil {
		return nil, nil
	}
	rows, err := db.Query(ctx, `
		SELECT tenantId, componentId, kindCode, address, COALESCE(userName, ''), COALESCE(secretEnc, ''),
		       COALESCE(databaseName, ''), COALESCE(optionText, ''), intervalSec, probedTime
		FROM FG_COMPONENT_CONN
		WHERE activeFlag = 'Y' AND statusFlag = 'Y'`, nil)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Open
	for rows.Next() {
		var item Open
		var secret, probed any
		if err := rows.Scan(&item.TenantID, &item.ComponentID, &item.KindCode, &item.Address, &item.UserName,
			&secret, &item.DatabaseName, &item.OptionText, &item.IntervalSec, &probed); err != nil {
			return nil, err
		}
		item.ProbedTime = row.Text(probed)
		if !due(item.ProbedTime, item.IntervalSec) {
			continue
		}
		plain, err := crypto.Decrypt("", row.Text(secret))
		if err != nil {
			plain = ""
		}
		item.Secret = plain
		out = append(out, item)
	}
	return out, rows.Err()
}

// Draft 按弹窗里的填写拼出一次试连。不写连接，也不写采集表。口令留空且已有连接时沿用已存口令。
func Draft(ctx context.Context, db *dbx.DB, in models.Input) (Open, error) {
	if db == nil {
		return Open{}, fmt.Errorf("数据库未打开")
	}
	sess, err := request.Require(ctx)
	if err != nil {
		return Open{}, err
	}
	kind := strings.TrimSpace(in.KindCode)
	if !catalog.Known(kind) {
		return Open{}, fmt.Errorf("不支持的组件种类")
	}
	address := strings.TrimSpace(in.Address)
	if address == "" || len(address) > 255 {
		return Open{}, fmt.Errorf("请填写不超过 255 字的地址")
	}
	userName := strings.TrimSpace(in.UserName)
	if len(userName) > 64 {
		return Open{}, fmt.Errorf("用户名过长")
	}
	databaseName := strings.TrimSpace(in.DatabaseName)
	if len(databaseName) > 64 {
		return Open{}, fmt.Errorf("库名过长")
	}
	optionText := strings.TrimSpace(in.OptionText)
	if len(optionText) > 100 {
		return Open{}, fmt.Errorf("附加选项过长")
	}
	componentID := strings.TrimSpace(in.ComponentID)
	spec, err := profile.Normalize(kind, optionText, userName, in.Secret, databaseName, componentID == "")
	if err != nil {
		return Open{}, err
	}
	secret := strings.TrimSpace(in.Secret)
	if spec.ClearSecret {
		secret = ""
	} else if secret == "" && componentID != "" {
		existing, err := LoadOpen(ctx, db, sess.TenantID, componentID)
		if err != nil {
			return Open{}, err
		}
		secret = existing.Secret
	}
	if utf8.RuneCountInString(secret) > 200 {
		return Open{}, fmt.Errorf("口令过长")
	}
	return Open{
		ComponentID: componentID, TenantID: sess.TenantID, KindCode: kind, Address: address,
		UserName: spec.User, Secret: secret, DatabaseName: spec.Database, OptionText: spec.Option,
	}, nil
}

// LoadOpen 读出一条可拨号的连接。
func LoadOpen(ctx context.Context, db *dbx.DB, tenantID, componentID string) (Open, error) {
	var item Open
	var secret, probed any
	err := store.Scan(ctx, db, `
		SELECT tenantId, componentId, kindCode, address, COALESCE(userName, ''), COALESCE(secretEnc, ''),
		       COALESCE(databaseName, ''), COALESCE(optionText, ''), intervalSec, probedTime
		FROM FG_COMPONENT_CONN
		WHERE tenantId = @tenantId AND componentId = @componentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "componentId": componentID},
		&item.TenantID, &item.ComponentID, &item.KindCode, &item.Address, &item.UserName,
		&secret, &item.DatabaseName, &item.OptionText, &item.IntervalSec, &probed)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Open{}, fmt.Errorf("连接不存在")
		}
		return Open{}, err
	}
	item.ProbedTime = row.Text(probed)
	plain, err := crypto.Decrypt("", row.Text(secret))
	if err != nil {
		return Open{}, fmt.Errorf("口令无法读取")
	}
	item.Secret = plain
	return item, nil
}

// Mark 把最近一次拨号写回连接行。
func Mark(ctx context.Context, db *dbx.DB, tenantID, componentID string, status models.Status) error {
	touch := row.Touch("system")
	_, err := db.Exec(ctx, `
		UPDATE FG_COMPONENT_CONN SET
		  upFlag = @upFlag, versionText = @versionText, roleName = @roleName,
		  errorText = @errorText, summaryText = @summaryText, probedTime = @probedTime,
		  editTime = @editTime, editWho = @editWho, oprSeqFlag = @oprSeqFlag,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND componentId = @componentId AND activeFlag = 'Y'`,
		dbx.Args{
			"tenantId": tenantID, "componentId": componentID,
			"upFlag": row.Flag(status.Up), "versionText": row.Cut(status.Version, 64),
			"roleName": row.Cut(status.Role, 32), "errorText": row.Cut(status.Error, 100),
			"summaryText": row.Cut(status.Summary, 100), "probedTime": touch["editTime"],
			"editTime": touch["editTime"], "editWho": touch["editWho"], "oprSeqFlag": touch["oprSeqFlag"],
		})
	return err
}

func loadSecret(ctx context.Context, db *dbx.DB, tenantID, componentID string) (string, error) {
	var secret string
	err := store.Scan(ctx, db, `
		SELECT COALESCE(secretEnc, '') FROM FG_COMPONENT_CONN
		WHERE tenantId = @tenantId AND componentId = @componentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "componentId": componentID}, &secret)
	return secret, err
}

func due(probed string, interval int) bool {
	if interval < minInterval {
		interval = minInterval
	}
	when, ok := store.ParseTime(probed)
	if !ok {
		return true
	}
	return time.Now().UTC().Sub(when) >= time.Duration(interval)*time.Second
}

type scanner func(dest ...any) error

func scanConn(scan scanner) (models.Conn, error) {
	var item models.Conn
	var probed, add, edit any
	err := scan(&item.ComponentID, &item.KindCode, &item.ComponentName, &item.Address, &item.UserName, &item.SecretSet,
		&item.DatabaseName, &item.OptionText, &item.IntervalSec, &item.StatusFlag, &item.UpFlag,
		&item.VersionText, &item.RoleName, &item.ErrorText, &item.SummaryText, &probed, &item.NoteText, &add, &edit)
	if err != nil {
		return models.Conn{}, err
	}
	item.ProbedTime = row.Text(probed)
	item.AddTime = row.Text(add)
	item.EditTime = row.Text(edit)
	return item, nil
}
