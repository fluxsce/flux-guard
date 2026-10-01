// Package access 给已有库补上组件监控菜单。资源已存在时不再授权，避免改回管理员后来收回的权限。
package access

import (
	"context"

	"fluxguard/server/component/row"
	dbx "github.com/fluxsce/dbx/db"
)

type resource struct {
	id     string
	name   string
	code   string
	kind   string
	path   string
	parent string
	level  int
	sort   int
	viewer bool
}

var menu = []resource{
	{id: "fg0007", name: "组件监控", code: "fg0007", kind: "MODULE", path: "/fg0007", parent: "group:observe", level: 2, sort: 47, viewer: true},
	{id: "fg0007:search", name: "查询", code: "fg0007:search", kind: "BUTTON", parent: "fg0007", level: 3, sort: 10, viewer: true},
	{id: "fg0007:view", name: "监控", code: "fg0007:view", kind: "BUTTON", parent: "fg0007", level: 3, sort: 20, viewer: true},
	{id: "fg0007:edit", name: "维护连接", code: "fg0007:edit", kind: "BUTTON", parent: "fg0007", level: 3, sort: 30},
}

// EnsureMenu 在模块资源缺失时写入菜单和内置角色授权。
func EnsureMenu(ctx context.Context, db *dbx.DB) error {
	if db == nil {
		return nil
	}
	tenants, err := tenantIDs(ctx, db)
	if err != nil {
		return err
	}
	for _, tenantID := range tenants {
		var n int
		rowq, err := db.QueryRow(ctx, `
			SELECT COUNT(*) FROM FG_AUTH_RESOURCE
			WHERE tenantId = @tenantId AND resourceId = 'fg0007'`,
			dbx.Args{"tenantId": tenantID})
		if err != nil {
			return err
		}
		if err := rowq.Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		for _, item := range menu {
			if err := insertResource(ctx, db, tenantID, item); err != nil {
				return err
			}
			if err := grant(ctx, db, tenantID, "ROLE_SUPER_ADMIN", "RR_SA_", item.id); err != nil {
				return err
			}
			if item.viewer {
				if err := grant(ctx, db, tenantID, "ROLE_VIEWER", "RR_VW_", item.id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func insertResource(ctx context.Context, db *dbx.DB, tenantID string, item resource) error {
	args := dbx.Args{
		"tenantId": tenantID, "resourceId": item.id, "resourceName": item.name, "resourceCode": item.code,
		"resourceType": item.kind, "resourcePath": item.path, "parentResourceId": item.parent,
		"resourceLevel": item.level, "sortOrder": item.sort, "language": "zh-CN",
		"resourceStatus": "Y", "builtInFlag": "Y",
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_AUTH_RESOURCE (
		  tenantId, resourceId, resourceName, resourceCode, resourceType, resourcePath, parentResourceId,
		  resourceLevel, sortOrder, language, resourceStatus, builtInFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @resourceId, @resourceName, @resourceCode, @resourceType, @resourcePath, @parentResourceId,
		  @resourceLevel, @sortOrder, @language, @resourceStatus, @builtInFlag,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	return err
}

func grant(ctx context.Context, db *dbx.DB, tenantID, roleID, prefix, resourceID string) error {
	var n int
	rowq, err := db.QueryRow(ctx, `
		SELECT COUNT(*) FROM FG_AUTH_ROLE
		WHERE tenantId = @tenantId AND roleId = @roleId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "roleId": roleID})
	if err != nil {
		return err
	}
	if err := rowq.Scan(&n); err != nil || n == 0 {
		return err
	}
	id := prefix + replaceColon(resourceID)
	args := dbx.Args{
		"tenantId": tenantID, "roleResourceId": id, "roleId": roleID, "resourceId": resourceID,
		"permissionType": "ALLOW", "grantedBy": "system",
	}
	for key, value := range row.Stamp("system", "") {
		args[key] = value
	}
	args["grantedTime"] = args["addTime"]
	_, err = db.Exec(ctx, `
		INSERT INTO FG_AUTH_ROLE_RESOURCE (
		  tenantId, roleResourceId, roleId, resourceId, permissionType, grantedBy, grantedTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
		) VALUES (
		  @tenantId, @roleResourceId, @roleId, @resourceId, @permissionType, @grantedBy, @grantedTime,
		  @addTime, @addWho, @editTime, @editWho, @oprSeqFlag, @currentVersion, @activeFlag, @noteText
		)`, args)
	return err
}

func replaceColon(value string) string {
	out := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == ':' {
			out = append(out, '_')
			continue
		}
		out = append(out, value[i])
	}
	return string(out)
}

func tenantIDs(ctx context.Context, db *dbx.DB) ([]string, error) {
	rows, err := db.Query(ctx, `SELECT tenantId FROM FG_TENANT WHERE activeFlag = 'Y'`, nil)
	if err != nil {
		return []string{"default"}, nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		out = []string{"default"}
	}
	return out, rows.Err()
}
