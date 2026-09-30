// Package dao 查询登录用户的模块和按钮授权。中间件只做拦截，不自己写 SQL。
package dao

import (
	"context"
	"sort"
	"strings"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// Menu 是控制台侧栏的一条模块。
// Buttons 是当前用户已授权的按钮编码。Actions 是该模块登记的按钮动作，前端工具条和右键按它判断。
type Menu struct {
	ResourceCode     string   `json:"resourceCode"`
	ResourceName     string   `json:"resourceName"`
	ResourcePath     string   `json:"resourcePath"`
	ParentResourceID string   `json:"parentResourceId"`
	SortOrder        int      `json:"sortOrder"`
	GroupName        string   `json:"groupName"`
	IconClass        string   `json:"iconClass,omitempty"`
	Buttons          []string `json:"buttons"`
	Actions          []string `json:"actions"`
}

// Access 读取角色资源表。不修改授权。
type Access struct {
	db *dbx.DB
}

// Open 使用登录服务交出的库连接。
func Open(db *dbx.DB) *Access {
	return &Access{db: db}
}

// AllowButton 判断用户是否拥有这个按钮编码。只认角色资源表里的精确授权。
func (a *Access) AllowButton(tenantID, userID, button string) bool {
	if a == nil || a.db == nil || button == "" {
		return false
	}
	ok, err := a.allow(context.Background(), tenantID, userID, button)
	return err == nil && ok
}

// AllowModule 判断用户能否进入路径上的模块。
// 目录里没有该 MODULE 时放行，调用方只保留登录校验。
// 有该模块时，拥有模块编码或任一「模块:」按钮即通过。任一条 DENY 使该编码失效。
func (a *Access) AllowModule(tenantID, userID, module string) bool {
	if a == nil || a.db == nil || module == "" {
		return false
	}
	ok, err := a.allowModule(context.Background(), tenantID, userID, module)
	return err == nil && ok
}

// Allow 判断用户是否拥有资源编码。菜单和页面内的二次判断用它，按钮拦截用 AllowButton。
func (a *Access) Allow(tenantID, userID, code string) bool {
	if a == nil || a.db == nil || code == "" {
		return false
	}
	ok, err := a.allow(context.Background(), tenantID, userID, code)
	return err == nil && ok
}

// Menus 返回用户按角色授权可见的模块，并带上该模块已授权的按钮。
// 租户管理员不在这里放行，标识由当前用户接口交给前端。
func (a *Access) Menus(tenantID, userID string) []Menu {
	if a == nil || a.db == nil {
		return nil
	}
	rows, err := a.db.Query(context.Background(), `
		SELECT res.resourceCode, res.resourceName, COALESCE(res.resourcePath, ''), COALESCE(res.parentResourceId, ''),
		       res.sortOrder, COALESCE(res.iconClass, ''), COALESCE(parent.resourceName, '')
		FROM FG_AUTH_RESOURCE res
		LEFT JOIN FG_AUTH_RESOURCE parent
		  ON parent.tenantId = res.tenantId AND parent.resourceId = res.parentResourceId
		WHERE res.tenantId = @tenantId AND res.resourceType = 'MODULE' AND res.activeFlag = 'Y' AND res.resourceStatus = 'Y'
		ORDER BY res.sortOrder, res.resourceCode`, dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil
	}
	var pending []Menu
	for rows.Next() {
		var item Menu
		if err := rows.Scan(&item.ResourceCode, &item.ResourceName, &item.ResourcePath, &item.ParentResourceID, &item.SortOrder, &item.IconClass, &item.GroupName); err != nil {
			_ = rows.Close()
			return nil
		}
		pending = append(pending, item)
	}
	if err := rows.Close(); err != nil {
		return nil
	}
	buttons, err := a.buttonCodes(tenantID)
	if err != nil {
		return nil
	}
	grants, err := a.granted(context.Background(), tenantID, userID)
	if err != nil {
		return nil
	}
	var out []Menu
	for _, item := range pending {
		if !grants[item.ResourceCode] {
			continue
		}
		item.Buttons, item.Actions = menuButtons(item.ResourceCode, buttons, grants)
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ResourceCode < out[j].ResourceCode
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func (a *Access) buttonCodes(tenantID string) ([]string, error) {
	rows, err := a.db.Query(context.Background(), `
		SELECT resourceCode FROM FG_AUTH_RESOURCE
		WHERE tenantId = @tenantId AND resourceType = 'BUTTON' AND activeFlag = 'Y' AND resourceStatus = 'Y'
		ORDER BY sortOrder, resourceCode`, dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func (a *Access) granted(ctx context.Context, tenantID, userID string) (map[string]bool, error) {
	rows, err := a.db.Query(ctx, `
		SELECT res.resourceCode, rr.permissionType
		FROM FG_AUTH_USER_ROLE ur
		JOIN FG_AUTH_ROLE r ON r.tenantId = ur.tenantId AND r.roleId = ur.roleId
		JOIN FG_AUTH_ROLE_RESOURCE rr ON rr.tenantId = ur.tenantId AND rr.roleId = ur.roleId
		JOIN FG_AUTH_RESOURCE res ON res.tenantId = rr.tenantId AND res.resourceId = rr.resourceId
		WHERE ur.tenantId = @tenantId AND ur.userId = @userId
		  AND ur.activeFlag = 'Y' AND r.activeFlag = 'Y' AND r.roleStatus = 'Y'
		  AND rr.activeFlag = 'Y' AND res.activeFlag = 'Y' AND res.resourceStatus = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	marks := map[string][2]bool{}
	for rows.Next() {
		var code, kind string
		if err := rows.Scan(&code, &kind); err != nil {
			return nil, err
		}
		item := marks[code]
		switch kind {
		case "DENY":
			item[1] = true
		case "ALLOW":
			item[0] = true
		}
		marks[code] = item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for code, item := range marks {
		if item[0] && !item[1] {
			out[code] = true
		}
	}
	return out, nil
}

func menuButtons(module string, codes []string, grants map[string]bool) (buttons, actions []string) {
	prefix := module + ":"
	buttons = []string{}
	actions = []string{}
	for _, code := range codes {
		action, ok := strings.CutPrefix(code, prefix)
		if !ok || action == "" || strings.Contains(action, ":") {
			continue
		}
		actions = append(actions, action)
		if grants[code] {
			buttons = append(buttons, code)
		}
	}
	return buttons, actions
}

func (a *Access) allowModule(ctx context.Context, tenantID, userID, module string) (bool, error) {
	var count int
	err := store.Scan(ctx, a.db, `
		SELECT count(*) FROM FG_AUTH_RESOURCE
		WHERE tenantId = @tenantId AND resourceCode = @resourceCode AND resourceType = 'MODULE'
		  AND activeFlag = 'Y' AND resourceStatus = 'Y'`,
		dbx.Args{"tenantId": tenantID, "resourceCode": module}, &count)
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}
	rows, err := a.db.Query(ctx, `
		SELECT res.resourceCode, rr.permissionType
		FROM FG_AUTH_USER_ROLE ur
		JOIN FG_AUTH_ROLE r ON r.tenantId = ur.tenantId AND r.roleId = ur.roleId
		JOIN FG_AUTH_ROLE_RESOURCE rr ON rr.tenantId = ur.tenantId AND rr.roleId = ur.roleId
		JOIN FG_AUTH_RESOURCE res ON res.tenantId = rr.tenantId AND res.resourceId = rr.resourceId
		WHERE ur.tenantId = @tenantId AND ur.userId = @userId
		  AND (res.resourceCode = @module OR res.resourceCode LIKE @prefix)
		  AND ur.activeFlag = 'Y' AND r.activeFlag = 'Y' AND r.roleStatus = 'Y'
		  AND rr.activeFlag = 'Y' AND res.activeFlag = 'Y' AND res.resourceStatus = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID, "module": module, "prefix": module + ":%"})
	if err != nil {
		return false, err
	}
	defer rows.Close()
	grants := map[string][2]bool{}
	for rows.Next() {
		var code, kind string
		if err := rows.Scan(&code, &kind); err != nil {
			return false, err
		}
		item := grants[code]
		switch kind {
		case "DENY":
			item[1] = true
		case "ALLOW":
			item[0] = true
		}
		grants[code] = item
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	for _, item := range grants {
		if item[0] && !item[1] {
			return true, nil
		}
	}
	return false, nil
}

func (a *Access) allow(ctx context.Context, tenantID, userID, code string) (bool, error) {
	rows, err := a.db.Query(ctx, `
		SELECT rr.permissionType
		FROM FG_AUTH_USER_ROLE ur
		JOIN FG_AUTH_ROLE r ON r.tenantId = ur.tenantId AND r.roleId = ur.roleId
		JOIN FG_AUTH_ROLE_RESOURCE rr ON rr.tenantId = ur.tenantId AND rr.roleId = ur.roleId
		JOIN FG_AUTH_RESOURCE res ON res.tenantId = rr.tenantId AND res.resourceId = rr.resourceId
		WHERE ur.tenantId = @tenantId AND ur.userId = @userId AND res.resourceCode = @resourceCode
		  AND ur.activeFlag = 'Y' AND r.activeFlag = 'Y' AND r.roleStatus = 'Y'
		  AND rr.activeFlag = 'Y' AND res.activeFlag = 'Y' AND res.resourceStatus = 'Y'`,
		dbx.Args{"tenantId": tenantID, "userId": userID, "resourceCode": code})
	if err != nil {
		return false, err
	}
	defer rows.Close()
	allowed, denied := false, false
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			return false, err
		}
		switch kind {
		case "DENY":
			denied = true
		case "ALLOW":
			allowed = true
		}
	}
	return allowed && !denied, rows.Err()
}
