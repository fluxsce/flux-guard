package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/server/utils/const/errcode"
	"fluxguard/server/utils/secret"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	roledao "fluxguard/server/web/views/fg0022/dao"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	// ErrUserID 表示用户编码为空或含有不允许的字符。
	ErrUserID = errcode.New("user.id.invalid", "用户编码须以字母开头，只能含字母、数字和下划线")
	// ErrUserName 表示登录账号为空或含空白。
	ErrUserName = errcode.New("user.name.required", "登录账号不能为空，且不能含空白")
	// ErrUserNameTaken 表示登录账号已被占用。登录按账号查找，因此全库唯一。
	ErrUserNameTaken = errcode.New("user.name.taken", "登录账号已存在")
	// ErrUserRealName 表示姓名为空。
	ErrUserRealName = errcode.New("user.real_name.required", "姓名不能为空")
	// ErrUserPassword 表示口令为空或短于 6 位。
	ErrUserPassword = errcode.New("user.password.short", "口令至少 6 位")
	// ErrUserExists 表示同一租户下用户编码已占用。
	ErrUserExists = errcode.New("user.exists", "用户编码已存在")
	// ErrUserMissing 表示要维护的用户不存在。
	ErrUserMissing = errcode.New("user.missing", "用户不存在")
	// ErrUserSelf 表示不能删除当前登录用户。
	ErrUserSelf = errcode.New("user.self", "不能删除当前登录用户")
	// ErrUserAdmin 表示租户管理员不能删除。
	ErrUserAdmin = errcode.New("user.admin", "不能删除管理员用户")
	// ErrUserExpire 表示过期时间无法解析。
	ErrUserExpire = errcode.New("user.expire.invalid", "过期时间格式不正确")
)

// UserView 用户列表，不含口令。RoleIDs 与 RoleNames 按同一顺序对应已分配角色。
type UserView struct {
	UserID          string   `json:"userId"`
	UserName        string   `json:"userName"`
	RealName        string   `json:"realName"`
	Email           string   `json:"email"`
	Mobile          string   `json:"mobile"`
	Gender          int      `json:"gender"`
	RoleIDs         []string `json:"roleIds"`
	RoleNames       string   `json:"roleNames"`
	StatusFlag      string   `json:"statusFlag"`
	TenantAdminFlag string   `json:"tenantAdminFlag"`
	UserExpireDate  string   `json:"userExpireDate"`
	LastLoginTime   string   `json:"lastLoginTime"`
	LastLoginIP     string   `json:"lastLoginIp"`
	ActiveFlag      string   `json:"activeFlag"`
	NoteText        string   `json:"noteText"`
	AddTime         string   `json:"addTime"`
	AddWho          string   `json:"addWho"`
	EditTime        string   `json:"editTime"`
	EditWho         string   `json:"editWho"`
}

// UserInput 是新增或修改用户时提交的字段。口令只在新增时必填；修改时留空表示不改。
type UserInput struct {
	UserID          string `json:"userId"`
	UserName        string `json:"userName"`
	Password        string `json:"password"`
	RealName        string `json:"realName"`
	Email           string `json:"email"`
	Mobile          string `json:"mobile"`
	Gender          int    `json:"gender"`
	StatusFlag      string `json:"statusFlag"`
	TenantAdminFlag string `json:"tenantAdminFlag"`
	UserExpireDate  string `json:"userExpireDate"`
	ActiveFlag      string `json:"activeFlag"`
	NoteText        string `json:"noteText"`
}

// UserRoleView 是给某个用户授权时的一条角色。Checked 表示当前已分配。
type UserRoleView struct {
	RoleID          string `json:"roleId"`
	RoleName        string `json:"roleName"`
	RoleDescription string `json:"roleDescription"`
	Checked         bool   `json:"checked"`
}

// UserRoleInput 用勾选结果替换用户的全部角色，可以同时分配多个。
type UserRoleInput struct {
	UserID  string   `json:"userId"`
	RoleIDs []string `json:"roleIds"`
}

// UserDAO 读写本租户用户和用户角色。
type UserDAO struct {
	db   *dbx.DB
	drop func(userID string)
}

// NewUserDAO 创建用户数据访问。drop 在改口令或删除后作废该用户的登录会话。
func NewUserDAO(db *dbx.DB, drop func(userID string)) *UserDAO {
	if drop == nil {
		drop = func(string) {}
	}
	return &UserDAO{db: db, drop: drop}
}

func yn(value, fallback string) string {
	switch strings.TrimSpace(value) {
	case "Y", "N":
		return strings.TrimSpace(value)
	default:
		return fallback
	}
}

// UserQuery 是用户列表的筛选条件。空字段不参与条件。
type UserQuery struct {
	UserName   string
	RealName   string
	StatusFlag string
}

// ListUsers 返回租户下的用户，最多一页上限。扫描和测试用。
func (d *UserDAO) ListUsers(tenantID string) []UserView {
	items, _, err := d.PageUsers(tenantID, UserQuery{}, dbx.Page{Page: 1, PageSize: setting.PageSizeCeiling})
	if err != nil || items == nil {
		return []UserView{}
	}
	return items
}

// PageUsers 按当前引擎分页返回用户。
func (d *UserDAO) PageUsers(tenantID string, q UserQuery, page dbx.Page) ([]UserView, int64, error) {
	ctx := context.Background()
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y'"
	where += store.LikeClause(args, "userName", "userName", q.UserName)
	where += store.LikeClause(args, "realName", "realName", q.RealName)
	where += store.EqClause(args, "statusFlag", "statusFlag", q.StatusFlag)
	countSQL := "SELECT COUNT(*) FROM FG_USER" + where
	listSQL := `
		SELECT userId, userName, realName, COALESCE(email, ''), COALESCE(mobile, ''), gender,
		       statusFlag, tenantAdminFlag, userExpireDate,
		       COALESCE(lastLoginTime, ''), COALESCE(lastLoginIp, ''),
		       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho
		FROM FG_USER` + where + `
		ORDER BY userName`
	rows, total, err := d.db.QueryPage(ctx, countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	var out []UserView
	for rows.Next() {
		var item UserView
		if err := rows.Scan(
			&item.UserID, &item.UserName, &item.RealName, &item.Email, &item.Mobile, &item.Gender,
			&item.StatusFlag, &item.TenantAdminFlag, &item.UserExpireDate,
			&item.LastLoginTime, &item.LastLoginIP,
			&item.ActiveFlag, &item.NoteText, &item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
		); err != nil {
			_ = rows.Close()
			return out, total, err
		}
		item.RoleIDs = []string{}
		out = append(out, item)
	}
	if err := rows.Close(); err != nil {
		return out, total, err
	}
	roleRows, err := d.db.Query(ctx, `
		SELECT ur.userId, ur.roleId, r.roleName
		FROM FG_AUTH_USER_ROLE ur
		JOIN FG_AUTH_ROLE r ON r.tenantId = ur.tenantId AND r.roleId = ur.roleId
		WHERE ur.tenantId = @tenantId AND ur.activeFlag = 'Y' AND r.activeFlag = 'Y'
		ORDER BY r.roleName`, dbx.Args{"tenantId": tenantID})
	if err != nil {
		return out, total, err
	}
	defer roleRows.Close()
	names := map[string][]string{}
	ids := map[string][]string{}
	for roleRows.Next() {
		var userID, roleID, roleName string
		if err := roleRows.Scan(&userID, &roleID, &roleName); err != nil {
			return out, total, err
		}
		ids[userID] = append(ids[userID], roleID)
		names[userID] = append(names[userID], roleName)
	}
	for i := range out {
		if roleIDs := ids[out[i].UserID]; roleIDs != nil {
			out[i].RoleIDs = roleIDs
		}
		out[i].RoleNames = strings.Join(names[out[i].UserID], "、")
	}
	if out == nil {
		out = []UserView{}
	}
	return out, total, nil
}

func validUserID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for i, r := range value {
		switch {
		case r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func validUserName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return false
		}
	}
	return true
}

func userGender(value int) int {
	if value < 0 || value > 2 {
		return 0
	}
	return value
}

func userExpire(value string, now time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return store.FormatTime(now.AddDate(5, 0, 0)), nil
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return store.FormatTime(parsed), nil
		}
	}
	if parsed, ok := store.ParseTime(value); ok {
		return store.FormatTime(parsed), nil
	}
	return "", ErrUserExpire
}

func (d *UserDAO) userNameTaken(tenantID, userID, userName string) (bool, error) {
	var count int
	err := store.Scan(context.Background(), d.db, `
		SELECT count(*) FROM FG_USER
		WHERE userName = @userName AND NOT (tenantId = @tenantId AND userId = @userId)`,
		dbx.Args{"userName": userName, "tenantId": tenantID, "userId": userID}, &count)
	return count > 0, err
}

// getUser 读回一名用户及其角色，不过滤活动标记，供保存后回显。
func (d *UserDAO) getUser(tenantID, userID string) (UserView, error) {
	var item UserView
	err := store.Scan(context.Background(), d.db, `
		SELECT userId, userName, realName, COALESCE(email, ''), COALESCE(mobile, ''), gender,
		       statusFlag, tenantAdminFlag, userExpireDate,
		       COALESCE(lastLoginTime, ''), COALESCE(lastLoginIp, ''),
		       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho
		FROM FG_USER
		WHERE tenantId = @tenantId AND userId = @userId`,
		dbx.Args{"tenantId": tenantID, "userId": userID},
		&item.UserID, &item.UserName, &item.RealName, &item.Email, &item.Mobile, &item.Gender,
		&item.StatusFlag, &item.TenantAdminFlag, &item.UserExpireDate,
		&item.LastLoginTime, &item.LastLoginIP,
		&item.ActiveFlag, &item.NoteText, &item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserView{}, ErrUserMissing
		}
		return UserView{}, err
	}
	item.RoleIDs = []string{}
	rows, err := d.db.Query(context.Background(), `
		SELECT ur.roleId, r.roleName
		FROM FG_AUTH_USER_ROLE ur
		JOIN FG_AUTH_ROLE r ON r.tenantId = ur.tenantId AND r.roleId = ur.roleId
		WHERE ur.tenantId = @tenantId AND ur.userId = @userId
		  AND ur.activeFlag = 'Y' AND r.activeFlag = 'Y'
		ORDER BY r.roleName`, dbx.Args{"tenantId": tenantID, "userId": userID})
	if err != nil {
		return UserView{}, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var roleID, roleName string
		if err := rows.Scan(&roleID, &roleName); err != nil {
			return UserView{}, err
		}
		item.RoleIDs = append(item.RoleIDs, roleID)
		names = append(names, roleName)
	}
	if err := rows.Err(); err != nil {
		return UserView{}, err
	}
	item.RoleNames = strings.Join(names, "、")
	return item, nil
}

// CreateUser 新增用户。编码在租户内唯一，登录账号全库唯一。成功时返回落库后的这一行，与列表项相同。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (d *UserDAO) CreateUser(ctx context.Context, input UserInput) (UserView, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return UserView{}, err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	input.UserID = strings.TrimSpace(input.UserID)
	input.UserName = strings.TrimSpace(input.UserName)
	input.RealName = strings.TrimSpace(input.RealName)
	if !validUserID(input.UserID) {
		return UserView{}, ErrUserID
	}
	if !validUserName(input.UserName) {
		return UserView{}, ErrUserName
	}
	if input.RealName == "" {
		return UserView{}, ErrUserRealName
	}
	if strings.TrimSpace(input.Password) == "" || len(input.Password) < 6 {
		return UserView{}, ErrUserPassword
	}
	var exists int
	err = store.Scan(ctx, d.db, `
		SELECT count(*) FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId`,
		dbx.Args{"tenantId": tenantID, "userId": input.UserID}, &exists)
	if err != nil {
		return UserView{}, err
	}
	if exists > 0 {
		return UserView{}, ErrUserExists
	}
	taken, err := d.userNameTaken(tenantID, input.UserID, input.UserName)
	if err != nil {
		return UserView{}, err
	}
	if taken {
		return UserView{}, ErrUserNameTaken
	}
	expire, err := userExpire(input.UserExpireDate, sess.At)
	if err != nil {
		return UserView{}, err
	}
	hash, err := secret.HashPassword(input.Password)
	if err != nil {
		return UserView{}, ErrUserPassword
	}
	now := sess.When()
	_, err = d.db.Exec(ctx, `
		INSERT INTO FG_USER (
		  userId, tenantId, userName, password, realName, email, mobile, gender,
		  statusFlag, tenantAdminFlag, userExpireDate, mustChangePwd,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @userId, @tenantId, @userName, @password, @realName, @email, @mobile, @gender,
		  @statusFlag, @tenantAdminFlag, @expire, 'N',
		  @now, @actor, @now, @actor, @seq, 1, @activeFlag, @noteText
		)`, dbx.Args{
		"userId": input.UserID, "tenantId": tenantID, "userName": input.UserName, "password": hash,
		"realName": input.RealName, "email": strings.TrimSpace(input.Email), "mobile": strings.TrimSpace(input.Mobile),
		"gender": userGender(input.Gender), "statusFlag": yn(input.StatusFlag, "Y"),
		"tenantAdminFlag": yn(input.TenantAdminFlag, "N"), "expire": expire,
		"now": now, "actor": actor, "seq": id.New(),
		"activeFlag": "Y", "noteText": strings.TrimSpace(input.NoteText),
	})
	if err != nil {
		return UserView{}, err
	}
	return d.getUser(tenantID, input.UserID)
}

// UpdateUser 修改已有用户。编码不变；口令留空则保持原哈希。改口令后该用户的会话失效。成功时返回落库后的这一行。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (d *UserDAO) UpdateUser(ctx context.Context, input UserInput) (UserView, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return UserView{}, err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	input.UserID = strings.TrimSpace(input.UserID)
	input.UserName = strings.TrimSpace(input.UserName)
	input.RealName = strings.TrimSpace(input.RealName)
	if input.UserID == "" {
		return UserView{}, ErrUserMissing
	}
	if !validUserName(input.UserName) {
		return UserView{}, ErrUserName
	}
	if input.RealName == "" {
		return UserView{}, ErrUserRealName
	}
	changePwd := "N"
	hash := ""
	if strings.TrimSpace(input.Password) != "" {
		if len(input.Password) < 6 {
			return UserView{}, ErrUserPassword
		}
		sum, err := secret.HashPassword(input.Password)
		if err != nil {
			return UserView{}, ErrUserPassword
		}
		changePwd = "Y"
		hash = sum
	}
	taken, err := d.userNameTaken(tenantID, input.UserID, input.UserName)
	if err != nil {
		return UserView{}, err
	}
	if taken {
		return UserView{}, ErrUserNameTaken
	}
	expire, err := userExpire(input.UserExpireDate, sess.At)
	if err != nil {
		return UserView{}, err
	}
	now := sess.When()
	result, err := d.db.Exec(ctx, `
		UPDATE FG_USER SET
		  userName = @userName,
		  realName = @realName,
		  email = @email,
		  mobile = @mobile,
		  gender = @gender,
		  statusFlag = @statusFlag,
		  tenantAdminFlag = @tenantAdminFlag,
		  userExpireDate = @expire,
		  activeFlag = @activeFlag,
		  noteText = @noteText,
		  password = CASE WHEN @changePwd = 'Y' THEN @password ELSE password END,
		  pwdUpdateTime = CASE WHEN @changePwd = 'Y' THEN @now ELSE pwdUpdateTime END,
		  mustChangePwd = CASE WHEN @changePwd = 'Y' THEN 'N' ELSE mustChangePwd END,
		  pwdErrorCount = CASE WHEN @changePwd = 'Y' THEN 0 ELSE pwdErrorCount END,
		  editTime = @now,
		  editWho = @actor,
		  oprSeqFlag = @seq,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND userId = @userId`,
		dbx.Args{
			"userName": input.UserName, "realName": input.RealName,
			"email": strings.TrimSpace(input.Email), "mobile": strings.TrimSpace(input.Mobile),
			"gender": userGender(input.Gender), "statusFlag": yn(input.StatusFlag, "Y"),
			"tenantAdminFlag": yn(input.TenantAdminFlag, "N"), "expire": expire,
			"activeFlag": "Y", "noteText": strings.TrimSpace(input.NoteText),
			"changePwd": changePwd, "password": hash,
			"now": now, "actor": actor, "seq": id.New(), "tenantId": tenantID, "userId": input.UserID,
		})
	if err != nil {
		return UserView{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return UserView{}, err
	}
	if affected == 0 {
		return UserView{}, ErrUserMissing
	}
	if changePwd == "Y" {
		d.drop(input.UserID)
	}
	return d.getUser(tenantID, input.UserID)
}

// DeleteUser 删除用户及其角色分配。当前登录用户和租户管理员不能删除。
// 租户和操作者取自 ctx 上的登录会话。
func (d *UserDAO) DeleteUser(ctx context.Context, userID string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrUserMissing
	}
	if userID == strings.TrimSpace(actor) {
		return ErrUserSelf
	}
	var adminFlag string
	err = store.Scan(ctx, d.db, `
		SELECT tenantAdminFlag FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId`,
		dbx.Args{"tenantId": tenantID, "userId": userID}, &adminFlag)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserMissing
		}
		return err
	}
	if adminFlag == "Y" {
		return ErrUserAdmin
	}
	err = d.db.Tx(ctx, func(tx *dbx.DB) error {
		args := dbx.Args{"tenantId": tenantID, "userId": userID}
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_AUTH_USER_ROLE WHERE tenantId = @tenantId AND userId = @userId`, args); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId AND tenantAdminFlag <> 'Y'`, args); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	d.drop(userID)
	return nil
}

// UserRoles 返回可分配角色，并标出该用户已经拥有的角色。
func (d *UserDAO) UserRoles(tenantID, userID string) ([]UserRoleView, error) {
	userID = strings.TrimSpace(userID)
	var exists int
	err := store.Scan(context.Background(), d.db, `
		SELECT count(*) FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID}, &exists)
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrUserMissing
	}
	rows, err := d.db.Query(context.Background(), `
		SELECT r.roleId, r.roleName, COALESCE(r.roleDescription, ''),
		       CASE WHEN ur.userRoleId IS NOT NULL THEN 'Y' ELSE 'N' END
		FROM FG_AUTH_ROLE r
		LEFT JOIN FG_AUTH_USER_ROLE ur
		  ON ur.tenantId = r.tenantId AND ur.roleId = r.roleId
		 AND ur.userId = @userId AND ur.activeFlag = 'Y'
		WHERE r.tenantId = @tenantId AND r.activeFlag = 'Y'
		ORDER BY r.roleName`, dbx.Args{"tenantId": tenantID, "userId": userID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserRoleView{}
	for rows.Next() {
		var item UserRoleView
		var checked string
		if err := rows.Scan(&item.RoleID, &item.RoleName, &item.RoleDescription, &checked); err != nil {
			return out, err
		}
		item.Checked = checked == "Y"
		out = append(out, item)
	}
	return out, rows.Err()
}

// SaveUserRoles 用勾选结果替换用户角色，允许多个角色同时生效。成功时返回带角色名的这一行。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (d *UserDAO) SaveUserRoles(ctx context.Context, userID string, roleIDs []string) (UserView, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return UserView{}, err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return UserView{}, ErrUserMissing
	}
	var exists int
	err = store.Scan(ctx, d.db, `
		SELECT count(*) FROM FG_USER WHERE tenantId = @tenantId AND userId = @userId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID}, &exists)
	if err != nil {
		return UserView{}, err
	}
	if exists == 0 {
		return UserView{}, ErrUserMissing
	}
	clean := uniqueIDs(roleIDs)
	for _, roleID := range clean {
		var ready int
		err := store.Scan(ctx, d.db, `
			SELECT count(*) FROM FG_AUTH_ROLE
			WHERE tenantId = @tenantId AND roleId = @roleId AND activeFlag = 'Y'`,
			dbx.Args{"tenantId": tenantID, "roleId": roleID}, &ready)
		if err != nil {
			return UserView{}, err
		}
		if ready == 0 {
			return UserView{}, roledao.ErrRoleMissing
		}
	}
	var previous string
	err = store.Scan(ctx, d.db, `
		SELECT roleId FROM FG_AUTH_USER_ROLE
		WHERE tenantId = @tenantId AND userId = @userId AND primaryRoleFlag = 'Y' AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID}, &previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return UserView{}, err
	}
	primary := primaryRole(previous, clean)
	now := sess.When()
	err = d.db.Tx(ctx, func(tx *dbx.DB) error {
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_AUTH_USER_ROLE WHERE tenantId = @tenantId AND userId = @userId`,
			dbx.Args{"tenantId": tenantID, "userId": userID}); err != nil {
			return err
		}
		for _, roleID := range clean {
			flag := "N"
			if roleID == primary {
				flag = "Y"
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO FG_AUTH_USER_ROLE (
				  userRoleId, tenantId, userId, roleId, grantedBy, primaryRoleFlag,
				  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
				) VALUES (
				  @userRoleId, @tenantId, @userId, @roleId, @actor, @primary,
				  @now, @actor, @now, @actor, @seq, 1, 'Y'
				)`, dbx.Args{
				"userRoleId": id.New(), "tenantId": tenantID, "userId": userID, "roleId": roleID,
				"actor": actor, "primary": flag, "now": now, "seq": id.New(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return UserView{}, err
	}
	return d.getUser(tenantID, userID)
}

func uniqueIDs(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func primaryRole(previous string, ids []string) string {
	for _, roleID := range ids {
		if roleID == previous {
			return previous
		}
	}
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}
