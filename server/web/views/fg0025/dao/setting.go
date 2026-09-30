package dao

import (
	"context"
	"database/sql"
	"errors"

	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

// ErrConflict 表示版本与库中不一致。
var ErrConflict = errors.New("设置已被修改")

// SettingDAO 按分组读写 FG_SYS_SETTING（任务策略）。
type SettingDAO struct {
	db *dbx.DB
}

// NewSettingDAO 创建任务策略数据访问。
func NewSettingDAO(db *dbx.DB) *SettingDAO {
	return &SettingDAO{db: db}
}

// Get 读取分组。没有行时 ok 为 false，版本为 0。
func (d *SettingDAO) Get(ctx context.Context, tenantID, groupCode string) (string, int, bool, error) {
	var content string
	var version int
	err := store.Scan(ctx, d.db, `
		SELECT content, currentVersion
		FROM FG_SYS_SETTING
		WHERE tenantId = @tenantId AND groupCode = @groupCode AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "groupCode": groupCode},
		&content, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, err
	}
	return content, version, true, nil
}

// Save 按版本写入分组。版本不符时返回 ErrConflict。
func (d *SettingDAO) Save(ctx context.Context, groupCode string, version int, content string) (int, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return 0, err
	}
	tenantID, who := sess.TenantID, sess.UserID
	_, current, ok, err := d.Get(ctx, tenantID, groupCode)
	if err != nil {
		return 0, err
	}
	if ok && current != version {
		return 0, ErrConflict
	}
	now := sess.When()
	if !ok {
		revived, err := d.db.Exec(ctx, `
			UPDATE FG_SYS_SETTING
			SET content = @content, activeFlag = 'Y', editTime = @now, editWho = @who,
			    oprSeqFlag = 'SAVE', currentVersion = 1
			WHERE tenantId = @tenantId AND groupCode = @groupCode AND activeFlag = 'N'`,
			dbx.Args{
				"tenantId": tenantID, "groupCode": groupCode,
				"content": content, "who": who, "now": now,
			})
		if err != nil {
			return 0, err
		}
		if n, _ := revived.RowsAffected(); n > 0 {
			return 1, nil
		}
		_, err = d.db.Exec(ctx, `
			INSERT INTO FG_SYS_SETTING (
			  tenantId, groupCode, content,
			  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @tenantId, @groupCode, @content,
			  @now, @who, @now, @who, 'SAVE', 1, 'Y'
			)`, dbx.Args{
			"tenantId": tenantID, "groupCode": groupCode,
			"content": content, "who": who, "now": now,
		})
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	next := current + 1
	_, err = d.db.Exec(ctx, `
		UPDATE FG_SYS_SETTING
		SET content = @content, editTime = @editTime, editWho = @who,
		    oprSeqFlag = 'SAVE', currentVersion = @next
		WHERE tenantId = @tenantId AND groupCode = @groupCode AND currentVersion = @version`,
		dbx.Args{
			"content": content, "editTime": now, "who": who, "next": next,
			"tenantId": tenantID, "groupCode": groupCode, "version": version,
		})
	if err != nil {
		return 0, err
	}
	return next, nil
}

// Delete 停用分组。没有行时不算错误。
func (d *SettingDAO) Delete(ctx context.Context, groupCode string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(ctx, `
		UPDATE FG_SYS_SETTING
		SET activeFlag = 'N', editTime = @editTime, editWho = @who,
		    oprSeqFlag = 'DEL', currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND groupCode = @groupCode AND activeFlag = 'Y'`,
		dbx.Args{
			"editTime": sess.When(), "who": sess.UserID,
			"tenantId": sess.TenantID, "groupCode": groupCode,
		})
	return err
}

// ActiveCount 仍启用的分组行数。用来判断其它租户是否还挂着同一任务。
func (d *SettingDAO) ActiveCount(ctx context.Context, groupCode string) (int, error) {
	var n int
	err := store.Scan(ctx, d.db, `
		SELECT count(*) FROM FG_SYS_SETTING
		WHERE groupCode = @groupCode AND activeFlag = 'Y'`,
		dbx.Args{"groupCode": groupCode}, &n)
	return n, err
}
