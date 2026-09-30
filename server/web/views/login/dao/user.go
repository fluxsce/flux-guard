package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"fluxguard/server/store"
	"fluxguard/server/web/views/login/models"
	dbx "github.com/fluxsce/dbx/db"
)

// Account 是登录时读到的账号。
type Account struct {
	UserID       string
	TenantID     string
	UserName     string
	PasswordHash string
	RealName     string
	StatusFlag      string
	ActiveFlag      string
	TenantAdminFlag string
	Expire          time.Time
}

// UserDAO 读写登录用户本人相关的账号行。
type UserDAO struct {
	db *dbx.DB
}

// Open 使用登录服务交出的库连接。
func Open(db *dbx.DB) *UserDAO {
	return &UserDAO{db: db}
}

// FindByUserID 按主键读取仍有效的用户。userId 与本进程租户组成主键，userName 是自定义名称。不存在时返回 sql.ErrNoRows。
func (d *UserDAO) FindByUserID(ctx context.Context, tenantID, userID string) (Account, error) {
	var found Account
	if d == nil || d.db == nil {
		return found, sql.ErrNoRows
	}
	var expire any
	err := store.Scan(ctx, d.db, `
		SELECT userId, tenantId, userName, realName, password, statusFlag, activeFlag, tenantAdminFlag, userExpireDate
		FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID},
		&found.UserID, &found.TenantID, &found.UserName, &found.RealName, &found.PasswordHash,
		&found.StatusFlag, &found.ActiveFlag, &found.TenantAdminFlag, &expire,
	)
	found.Expire, _ = store.TimeValue(expire)
	return found, err
}

// FindActive 读取仍启用的用户，供会话还原当前操作者。
func (d *UserDAO) FindActive(ctx context.Context, tenantID, userID string) (Account, error) {
	var found Account
	if d == nil || d.db == nil {
		return found, sql.ErrNoRows
	}
	var expire any
	err := store.Scan(ctx, d.db, `
		SELECT userId, tenantId, userName, realName, statusFlag, activeFlag, userExpireDate
		FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId`,
		dbx.Args{"tenantId": tenantID, "userId": userID},
		&found.UserID, &found.TenantID, &found.UserName, &found.RealName, &found.StatusFlag, &found.ActiveFlag, &expire,
	)
	found.Expire, _ = store.TimeValue(expire)
	return found, err
}

// PasswordHash 读取本人当前口令哈希。
func (d *UserDAO) PasswordHash(ctx context.Context, tenantID, userID string) (string, error) {
	if d == nil || d.db == nil {
		return "", sql.ErrNoRows
	}
	var hash string
	err := store.Scan(ctx, d.db, `
		SELECT password FROM FG_USER
		WHERE tenantId = @tenantId AND userId = @userId AND activeFlag = 'Y' AND statusFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID}, &hash)
	return hash, err
}

// LoadSelf 读取本人资料。
func (d *UserDAO) LoadSelf(ctx context.Context, tenantID, userID string) (models.SelfProfile, error) {
	var out models.SelfProfile
	if d == nil || d.db == nil {
		return out, sql.ErrNoRows
	}
	err := store.Scan(ctx, d.db, `
		SELECT userId, userName, realName, COALESCE(email, ''), COALESCE(mobile, ''), gender
		FROM FG_USER
		WHERE tenantId = @tenantId AND userId = @userId AND activeFlag = 'Y' AND statusFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID},
		&out.UserID, &out.UserName, &out.RealName, &out.Email, &out.Mobile, &out.Gender)
	return out, err
}

// ProfileUpdate 是本人资料更新。口令相关字段仅在改口令时写入。
type ProfileUpdate struct {
	RealName      string
	Email         any
	Mobile        any
	Gender        int
	ChangePwd     string
	Password      any
	PwdUpdateTime any
	EditWho       string
	EditTime      string
	OprSeqFlag    string
	TenantID      string
	UserID        string
}

// UpdateSelf 只更新本人姓名、邮箱、手机、性别，以及可选的新口令。
func (d *UserDAO) UpdateSelf(ctx context.Context, in ProfileUpdate) error {
	if d == nil || d.db == nil {
		return errors.New("login db missing")
	}
	_, err := d.db.Exec(ctx, `
		UPDATE FG_USER SET
		  realName = @realName,
		  email = @email,
		  mobile = @mobile,
		  gender = @gender,
		  password = CASE WHEN @changePwd = 'Y' THEN @password ELSE password END,
		  pwdUpdateTime = CASE WHEN @changePwd = 'Y' THEN @pwdUpdateTime ELSE pwdUpdateTime END,
		  mustChangePwd = CASE WHEN @changePwd = 'Y' THEN 'N' ELSE mustChangePwd END,
		  pwdErrorCount = CASE WHEN @changePwd = 'Y' THEN 0 ELSE pwdErrorCount END,
		  lockTime = CASE WHEN @changePwd = 'Y' THEN NULL ELSE lockTime END,
		  editWho = @editWho,
		  editTime = @editTime,
		  oprSeqFlag = @oprSeqFlag,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND userId = @userId AND activeFlag = 'Y' AND statusFlag = 'Y'`,
		dbx.Args{
			"realName": in.RealName, "email": in.Email, "mobile": in.Mobile,
			"gender": in.Gender, "changePwd": in.ChangePwd, "password": in.Password, "pwdUpdateTime": in.PwdUpdateTime,
			"editWho": in.EditWho, "editTime": in.EditTime, "oprSeqFlag": in.OprSeqFlag,
			"tenantId": in.TenantID, "userId": in.UserID,
		})
	return err
}
