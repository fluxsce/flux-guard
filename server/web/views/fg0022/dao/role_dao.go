package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"fluxguard/common/id"
	"fluxguard/server/errcode"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	// ErrRoleMissing 表示要维护或授权的角色不存在。
	ErrRoleMissing = errcode.New("role.missing", "角色不存在")
	// ErrRoleID 表示角色编码为空或含有不允许的字符。
	ErrRoleID = errcode.New("role.id.invalid", "角色编码须以字母开头，只能含字母、数字和下划线")
	// ErrRoleName 表示角色名称为空。
	ErrRoleName = errcode.New("role.name.required", "角色名称不能为空")
	// ErrRoleExists 表示同一租户下编码已占用。
	ErrRoleExists = errcode.New("role.exists", "角色编码已存在")
	// ErrRoleBuiltIn 表示内置角色不能删除。
	ErrRoleBuiltIn = errcode.New("role.builtin", "内置角色不能删除")
)

// ResourceView 是角色授权树上的一个节点。
// Checked 只在该节点下全部叶子都已授权时为 true；只授权了部分按钮时父节点为 false，由树显示半选。
type ResourceView struct {
	ResourceID       string          `json:"resourceId"`
	ResourceName     string          `json:"resourceName"`
	ResourceCode     string          `json:"resourceCode"`
	ResourceType     string          `json:"resourceType"`
	ParentResourceID string          `json:"parentResourceId,omitempty"`
	Checked          bool            `json:"checked"`
	Children         []*ResourceView `json:"children,omitempty"`
}

// RoleGrantInput 是角色页保存的授权集合。
type RoleGrantInput struct {
	RoleID      string   `json:"roleId"`
	ResourceIDs []string `json:"resourceIds"`
}

// RoleInput 是新增或修改角色时提交的字段。内置标记由库里的原值决定，不从这里改。
type RoleInput struct {
	RoleID          string `json:"roleId"`
	RoleName        string `json:"roleName"`
	RoleDescription string `json:"roleDescription"`
	RoleStatus      string `json:"roleStatus"`
	DataScope       string `json:"dataScope"`
	ActiveFlag      string `json:"activeFlag"`
	NoteText        string `json:"noteText"`
}

// RoleView 角色列表。
type RoleView struct {
	RoleID          string `json:"roleId"`
	RoleName        string `json:"roleName"`
	RoleDescription string `json:"roleDescription"`
	RoleStatus      string `json:"roleStatus"`
	BuiltInFlag     string `json:"builtInFlag"`
	DataScope       string `json:"dataScope"`
	ActiveFlag      string `json:"activeFlag"`
	NoteText        string `json:"noteText"`
	AddTime         string `json:"addTime"`
	AddWho          string `json:"addWho"`
	EditTime        string `json:"editTime"`
	EditWho         string `json:"editWho"`
}

// RoleDAO 读写本租户角色和角色授权。
type RoleDAO struct {
	db *dbx.DB
}

// NewRoleDAO 创建角色数据访问。
func NewRoleDAO(db *dbx.DB) *RoleDAO {
	return &RoleDAO{db: db}
}

// RoleQuery 是角色列表的筛选条件。
type RoleQuery struct {
	RoleName    string
	RoleStatus  string
	BuiltInFlag string
}

// ListRoles 返回租户下的角色，最多一页上限。
func (d *RoleDAO) ListRoles(tenantID string) []RoleView {
	items, _, err := d.PageRoles(tenantID, RoleQuery{}, dbx.Page{Page: 1, PageSize: setting.PageSizeCeiling})
	if err != nil || items == nil {
		return []RoleView{}
	}
	return items
}

// PageRoles 按当前引擎分页返回角色。
func (d *RoleDAO) PageRoles(tenantID string, q RoleQuery, page dbx.Page) ([]RoleView, int64, error) {
	page.OrderBy = ""
	args := dbx.Args{"tenantId": tenantID}
	where := " WHERE tenantId = @tenantId AND activeFlag = 'Y'"
	where += store.LikeClause(args, "roleName", "roleName", q.RoleName)
	where += store.EqClause(args, "roleStatus", "roleStatus", q.RoleStatus)
	where += store.EqClause(args, "builtInFlag", "builtInFlag", q.BuiltInFlag)
	countSQL := "SELECT COUNT(*) FROM FG_AUTH_ROLE" + where
	listSQL := `
		SELECT roleId, roleName, COALESCE(roleDescription, ''), roleStatus, builtInFlag, COALESCE(dataScope, ''),
		       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho
		FROM FG_AUTH_ROLE` + where + `
		ORDER BY roleName`
	rows, total, err := d.db.QueryPage(context.Background(), countSQL, listSQL, page, args)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []RoleView
	for rows.Next() {
		var item RoleView
		if err := rows.Scan(
			&item.RoleID, &item.RoleName, &item.RoleDescription, &item.RoleStatus, &item.BuiltInFlag, &item.DataScope,
			&item.ActiveFlag, &item.NoteText, &item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
		); err != nil {
			return out, total, err
		}
		out = append(out, item)
	}
	if out == nil {
		out = []RoleView{}
	}
	return out, total, rows.Err()
}

// GetRole 按角色编码返回当前租户的一行。不存在时返回 ErrRoleMissing。
func (d *RoleDAO) GetRole(tenantID, roleID string) (RoleView, error) {
	return d.getRole(tenantID, strings.TrimSpace(roleID))
}

func (d *RoleDAO) getRole(tenantID, roleID string) (RoleView, error) {
	var item RoleView
	err := store.Scan(context.Background(), d.db, `
		SELECT roleId, roleName, COALESCE(roleDescription, ''), roleStatus, builtInFlag, COALESCE(dataScope, ''),
		       activeFlag, COALESCE(noteText, ''), addTime, addWho, editTime, editWho
		FROM FG_AUTH_ROLE
		WHERE tenantId = @tenantId AND roleId = @roleId`,
		dbx.Args{"tenantId": tenantID, "roleId": roleID},
		&item.RoleID, &item.RoleName, &item.RoleDescription, &item.RoleStatus, &item.BuiltInFlag, &item.DataScope,
		&item.ActiveFlag, &item.NoteText, &item.AddTime, &item.AddWho, &item.EditTime, &item.EditWho,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RoleView{}, ErrRoleMissing
		}
		return RoleView{}, err
	}
	return item, nil
}

func yn(value, fallback string) string {
	switch strings.TrimSpace(value) {
	case "Y", "N":
		return strings.TrimSpace(value)
	default:
		return fallback
	}
}

func validRoleID(value string) bool {
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

// CreateRole 新增自定义角色并返回落库后的这一条。编码在租户内不可重复，内置标记固定为 N。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (d *RoleDAO) CreateRole(ctx context.Context, input RoleInput) (RoleView, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return RoleView{}, err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	input.RoleID = strings.TrimSpace(input.RoleID)
	input.RoleName = strings.TrimSpace(input.RoleName)
	if !validRoleID(input.RoleID) {
		return RoleView{}, ErrRoleID
	}
	if input.RoleName == "" {
		return RoleView{}, ErrRoleName
	}
	var exists int
	err = store.Scan(ctx, d.db, `
		SELECT count(*) FROM FG_AUTH_ROLE WHERE tenantId = @tenantId AND roleId = @roleId`,
		dbx.Args{"tenantId": tenantID, "roleId": input.RoleID}, &exists)
	if err != nil {
		return RoleView{}, err
	}
	if exists > 0 {
		return RoleView{}, ErrRoleExists
	}
	now := sess.When()
	_, err = d.db.Exec(ctx, `
		INSERT INTO FG_AUTH_ROLE (
		  roleId, tenantId, roleName, roleDescription, roleStatus, builtInFlag, dataScope,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @roleId, @tenantId, @roleName, @roleDescription, @roleStatus, 'N', @dataScope,
		  @now, @actor, @now, @actor, @seq, 1, @activeFlag, @noteText
		)`, dbx.Args{
		"roleId": input.RoleID, "tenantId": tenantID, "roleName": input.RoleName,
		"roleDescription": strings.TrimSpace(input.RoleDescription), "roleStatus": yn(input.RoleStatus, "Y"),
		"dataScope": strings.TrimSpace(input.DataScope), "now": now, "actor": actor, "seq": id.New(),
		"activeFlag": "Y", "noteText": strings.TrimSpace(input.NoteText),
	})
	if err != nil {
		return RoleView{}, err
	}
	return d.getRole(tenantID, input.RoleID)
}

// UpdateRole 修改已有角色并返回落库后的这一条。编码不变，内置标记保持原值。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (d *RoleDAO) UpdateRole(ctx context.Context, input RoleInput) (RoleView, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return RoleView{}, err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	input.RoleID = strings.TrimSpace(input.RoleID)
	input.RoleName = strings.TrimSpace(input.RoleName)
	if input.RoleID == "" {
		return RoleView{}, ErrRoleMissing
	}
	if input.RoleName == "" {
		return RoleView{}, ErrRoleName
	}
	now := sess.When()
	result, err := d.db.Exec(ctx, `
		UPDATE FG_AUTH_ROLE SET
		  roleName = @roleName,
		  roleDescription = @roleDescription,
		  roleStatus = @roleStatus,
		  dataScope = @dataScope,
		  activeFlag = @activeFlag,
		  noteText = @noteText,
		  editTime = @now,
		  editWho = @actor,
		  oprSeqFlag = @seq,
		  currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND roleId = @roleId`,
		dbx.Args{
			"roleName": input.RoleName, "roleDescription": strings.TrimSpace(input.RoleDescription),
			"roleStatus": yn(input.RoleStatus, "Y"), "dataScope": strings.TrimSpace(input.DataScope),
			"activeFlag": "Y", "noteText": strings.TrimSpace(input.NoteText),
			"now": now, "actor": actor, "seq": id.New(), "tenantId": tenantID, "roleId": input.RoleID,
		})
	if err != nil {
		return RoleView{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return RoleView{}, err
	}
	if affected == 0 {
		return RoleView{}, ErrRoleMissing
	}
	return d.getRole(tenantID, input.RoleID)
}

// DeleteRole 删除自定义角色，并去掉它的授权和用户关联。内置角色保留。
// 租户取自 ctx 上的登录会话。
func (d *RoleDAO) DeleteRole(ctx context.Context, roleID string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	tenantID := sess.TenantID
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrRoleMissing
	}
	var builtIn string
	err = store.Scan(ctx, d.db, `
		SELECT builtInFlag FROM FG_AUTH_ROLE WHERE tenantId = @tenantId AND roleId = @roleId`,
		dbx.Args{"tenantId": tenantID, "roleId": roleID}, &builtIn)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRoleMissing
		}
		return err
	}
	if builtIn == "Y" {
		return ErrRoleBuiltIn
	}
	return d.db.Tx(ctx, func(tx *dbx.DB) error {
		args := dbx.Args{"tenantId": tenantID, "roleId": roleID}
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_AUTH_ROLE_RESOURCE WHERE tenantId = @tenantId AND roleId = @roleId`, args); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_AUTH_USER_ROLE WHERE tenantId = @tenantId AND roleId = @roleId`, args); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_AUTH_ROLE WHERE tenantId = @tenantId AND roleId = @roleId AND builtInFlag <> 'Y'`, args); err != nil {
			return err
		}
		return nil
	})
}

// resourceFlat 是授权树组装前的一行目录。
type resourceFlat struct {
	id      string
	name    string
	code    string
	kind    string
	parent  string
	granted bool
}

// resourceLink 只保留父子关系，供保存时补齐勾选。
type resourceLink struct {
	id     string
	parent string
}

// resourceLinks 读取本租户仍启用的资源父子关系。
func (d *RoleDAO) resourceLinks(tenantID string) ([]resourceLink, error) {
	rows, err := d.db.Query(context.Background(), `
		SELECT resourceId, COALESCE(parentResourceId, '')
		FROM FG_AUTH_RESOURCE
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND resourceStatus = 'Y'`,
		dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []resourceLink
	for rows.Next() {
		var item resourceLink
		if err := rows.Scan(&item.id, &item.parent); err != nil {
			return nil, err
		}
		links = append(links, item)
	}
	return links, rows.Err()
}

// buildRoleResourceTree 把平铺目录收成树，并按后代叶子是否全部授权标记 checked。
func buildRoleResourceTree(rows []resourceFlat) []*ResourceView {
	nodes := make(map[string]*ResourceView, len(rows))
	order := make([]string, 0, len(rows))
	children := make(map[string][]string)
	granted := make(map[string]bool, len(rows))
	parentOf := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.id == "" {
			continue
		}
		nodes[row.id] = &ResourceView{
			ResourceID:       row.id,
			ResourceName:     row.name,
			ResourceCode:     row.code,
			ResourceType:     row.kind,
			ParentResourceID: row.parent,
		}
		order = append(order, row.id)
		granted[row.id] = row.granted
		parentOf[row.id] = row.parent
		if row.parent != "" {
			children[row.parent] = append(children[row.parent], row.id)
		}
	}

	leaves := make(map[string][]string, len(nodes))
	var descendantLeaves func(id string) []string
	descendantLeaves = func(id string) []string {
		if cached, ok := leaves[id]; ok {
			return cached
		}
		kids := children[id]
		if len(kids) == 0 {
			leaves[id] = []string{id}
			return leaves[id]
		}
		var all []string
		for _, child := range kids {
			if _, ok := nodes[child]; !ok {
				continue
			}
			all = append(all, descendantLeaves(child)...)
		}
		if len(all) == 0 {
			all = []string{id}
		}
		leaves[id] = all
		return all
	}
	for _, id := range order {
		descendantLeaves(id)
	}

	var roots []*ResourceView
	for _, id := range order {
		node := nodes[id]
		allGranted := len(leaves[id]) > 0
		for _, leaf := range leaves[id] {
			if !granted[leaf] {
				allGranted = false
				break
			}
		}
		node.Checked = allGranted
		for _, childID := range children[id] {
			child, ok := nodes[childID]
			if !ok {
				continue
			}
			node.Children = append(node.Children, child)
		}
		parent := parentOf[id]
		if parent == "" || nodes[parent] == nil {
			roots = append(roots, node)
		}
	}
	return roots
}

// expandRoleResources 补齐角色授权勾选。
// 勾中父节点时写入全部子孙；勾中子节点时向上写入父节点。
func expandRoleResources(selected []string, links []resourceLink) []string {
	parentOf := make(map[string]string, len(links))
	children := make(map[string][]string)
	for _, link := range links {
		if link.id == "" {
			continue
		}
		parentOf[link.id] = link.parent
		if link.parent != "" {
			children[link.parent] = append(children[link.parent], link.id)
		}
	}

	final := make(map[string]bool, len(selected))
	for _, id := range selected {
		if id != "" {
			final[id] = true
		}
	}

	var addDescendants func(id string)
	addDescendants = func(id string) {
		for _, childID := range children[id] {
			if final[childID] {
				continue
			}
			final[childID] = true
			addDescendants(childID)
		}
	}
	origins := make([]string, 0, len(final))
	for id := range final {
		origins = append(origins, id)
	}
	for _, id := range origins {
		addDescendants(id)
	}

	selectedIDs := make([]string, 0, len(final))
	for id := range final {
		selectedIDs = append(selectedIDs, id)
	}
	for _, id := range selectedIDs {
		parent := parentOf[id]
		for parent != "" {
			final[parent] = true
			next, ok := parentOf[parent]
			if !ok {
				break
			}
			parent = next
		}
	}

	out := make([]string, 0, len(final))
	for id := range final {
		out = append(out, id)
	}
	return out
}

// Resources 返回角色授权树。父节点仅在全部后代叶子都已授权时 checked 为 true。
func (d *RoleDAO) Resources(tenantID, roleID string) []*ResourceView {
	rows, err := d.db.Query(context.Background(), `
		SELECT res.resourceId, res.resourceName, res.resourceCode, res.resourceType,
		       COALESCE(res.parentResourceId, ''),
		       CASE WHEN rr.roleResourceId IS NOT NULL THEN 'Y' ELSE 'N' END
		FROM FG_AUTH_RESOURCE res
		LEFT JOIN FG_AUTH_ROLE_RESOURCE rr
		  ON rr.tenantId = res.tenantId AND rr.resourceId = res.resourceId
		 AND rr.roleId = @roleId AND rr.activeFlag = 'Y' AND rr.permissionType = 'ALLOW'
		WHERE res.tenantId = @tenantId AND res.activeFlag = 'Y' AND res.resourceStatus = 'Y'
		ORDER BY res.resourceLevel, res.sortOrder, res.resourceCode`,
		dbx.Args{"roleId": roleID, "tenantId": tenantID})
	if err != nil {
		return nil
	}
	defer rows.Close()
	var flat []resourceFlat
	for rows.Next() {
		var item resourceFlat
		var granted string
		if err := rows.Scan(&item.id, &item.name, &item.code, &item.kind, &item.parent, &granted); err != nil {
			return nil
		}
		item.granted = granted == "Y"
		flat = append(flat, item)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return buildRoleResourceTree(flat)
}

// SaveRoleResources 用界面勾选替换该角色的授权。
// 勾中父节点时补齐全部子孙；勾中子节点时向上补齐父节点。取消单个按钮时前端不提交父节点，因此不会把已取消的按钮写回去。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (d *RoleDAO) SaveRoleResources(ctx context.Context, roleID string, resourceIDs []string) error {
	sess, err := request.Require(ctx)
	if err != nil {
		return err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	if roleID == "" {
		return ErrRoleMissing
	}
	var exists int
	err = store.Scan(ctx, d.db, `
		SELECT count(*) FROM FG_AUTH_ROLE
		WHERE tenantId = @tenantId AND roleId = @roleId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "roleId": roleID}, &exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrRoleMissing
	}
	links, err := d.resourceLinks(tenantID)
	if err != nil {
		return err
	}
	resourceIDs = expandRoleResources(resourceIDs, links)
	now := sess.When()
	return d.db.Tx(ctx, func(tx *dbx.DB) error {
		if _, err := tx.Exec(ctx, `
			DELETE FROM FG_AUTH_ROLE_RESOURCE WHERE tenantId = @tenantId AND roleId = @roleId`,
			dbx.Args{"tenantId": tenantID, "roleId": roleID}); err != nil {
			return err
		}
		seen := map[string]struct{}{}
		for _, resourceID := range resourceIDs {
			if resourceID == "" {
				continue
			}
			if _, ok := seen[resourceID]; ok {
				continue
			}
			seen[resourceID] = struct{}{}
			var ready int
			if err := store.Scan(ctx, tx, `
				SELECT count(*) FROM FG_AUTH_RESOURCE
				WHERE tenantId = @tenantId AND resourceId = @resourceId AND activeFlag = 'Y' AND resourceStatus = 'Y'`,
				dbx.Args{"tenantId": tenantID, "resourceId": resourceID}, &ready); err != nil {
				return err
			}
			if ready == 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO FG_AUTH_ROLE_RESOURCE (
				  roleResourceId, tenantId, roleId, resourceId, permissionType, grantedBy,
				  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
				) VALUES (
				  @roleResourceId, @tenantId, @roleId, @resourceId, 'ALLOW', @actor,
				  @now, @actor, @now, @actor, 'UI', 1, 'Y'
				)`,
				dbx.Args{
					"roleResourceId": id.New(), "tenantId": tenantID, "roleId": roleID, "resourceId": resourceID,
					"actor": actor, "now": now,
				}); err != nil {
				return err
			}
		}
		return nil
	})
}
