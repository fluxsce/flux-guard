package dao

import (
	"context"
	"strings"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// MenuDAO 补本模块在权限目录里的菜单。已有编码不覆盖。
type MenuDAO struct {
	db *dbx.DB
}

// NewMenuDAO 使用鉴权服务交出的连接。
func NewMenuDAO(db *dbx.DB) *MenuDAO {
	return &MenuDAO{db: db}
}

type menuItem struct {
	id, name, code, kind, path, parent, icon string
	level, sort                              int
	viewer                                   bool
}

// 预警分组由本页补上。维护按钮只授给超级管理员。
var ruleMenu = []menuItem{
	{id: "group:alert", name: "预警管理", code: "group:alert", kind: "GROUP", icon: "bell", level: 1, sort: 15, viewer: true},
	{id: "fg0041", name: "预警模板", code: "fg0041", kind: "MODULE", path: "/fg0041", parent: "group:alert", level: 2, sort: 10, viewer: true},
	{id: "fg0041:view", name: "查看", code: "fg0041:view", kind: "BUTTON", parent: "fg0041", level: 3, sort: 10, viewer: true},
	{id: "fg0041:edit", name: "维护", code: "fg0041:edit", kind: "BUTTON", parent: "fg0041", level: 3, sort: 20},
}

// Ensure 补上预警分组、模板页和按钮。
func (d *MenuDAO) Ensure(tenantID string) {
	if d == nil {
		return
	}
	ensureMenu(d.db, tenantID, ruleMenu)
}

func ensureMenu(db *dbx.DB, tenantID string, items []menuItem) {
	if db == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	for _, item := range items {
		ensureItem(db, tenantID, item)
	}
}

func ensureItem(db *dbx.DB, tenantID string, item menuItem) {
	var exists int
	err := store.Scan(context.Background(), db, `
		SELECT count(*) FROM FG_AUTH_RESOURCE
		WHERE tenantId = @tenantId AND resourceCode = @resourceCode`,
		dbx.Args{"tenantId": tenantID, "resourceCode": item.code}, &exists)
	if err != nil {
		return
	}
	if exists == 0 {
		_, _ = db.Exec(context.Background(), `
			INSERT INTO FG_AUTH_RESOURCE (
			  resourceId, tenantId, resourceName, resourceCode, resourceType,
			  resourcePath, parentResourceId, resourceLevel, sortOrder, iconClass, language,
			  resourceStatus, builtInFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @resourceId, @tenantId, @resourceName, @resourceCode, @resourceType,
			  @resourcePath, @parentResourceId, @resourceLevel, @sortOrder, @iconClass, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'
			)`, dbx.Args{
			"resourceId": item.id, "tenantId": tenantID, "resourceName": item.name,
			"resourceCode": item.code, "resourceType": item.kind,
			"resourcePath": nullIfEmpty(item.path), "parentResourceId": nullIfEmpty(item.parent),
			"resourceLevel": item.level, "sortOrder": item.sort, "iconClass": nullIfEmpty(item.icon),
		})
	}
	grant(db, tenantID, "ROLE_SUPER_ADMIN", "RR_SA_", item.id)
	if item.viewer {
		grant(db, tenantID, "ROLE_VIEWER", "RR_VW_", item.id)
	}
}

func grant(db *dbx.DB, tenantID, roleID, prefix, resourceID string) {
	var roleN int
	if err := store.Scan(context.Background(), db, `
		SELECT count(*) FROM FG_AUTH_ROLE
		WHERE tenantId = @tenantId AND roleId = @roleId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "roleId": roleID}, &roleN); err != nil || roleN == 0 {
		return
	}
	linkID := prefix + strings.ReplaceAll(resourceID, ":", "_")
	var exists int
	if err := store.Scan(context.Background(), db, `
		SELECT count(*) FROM FG_AUTH_ROLE_RESOURCE
		WHERE tenantId = @tenantId AND roleResourceId = @roleResourceId`,
		dbx.Args{"tenantId": tenantID, "roleResourceId": linkID}, &exists); err != nil || exists > 0 {
		return
	}
	_, _ = db.Exec(context.Background(), `
		INSERT INTO FG_AUTH_ROLE_RESOURCE (
		  roleResourceId, tenantId, roleId, resourceId, permissionType, grantedBy,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @roleResourceId, @tenantId, @roleId, @resourceId, 'ALLOW', 'system',
		  'system', 'system', 'INIT', 1, 'Y'
		)`, dbx.Args{
		"roleResourceId": linkID, "tenantId": tenantID, "roleId": roleID, "resourceId": resourceID,
	})
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
