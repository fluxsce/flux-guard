package dao

import (
	"context"
	"strings"

	"fluxguard/server/plugin/examplealert"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// Ensure 把本扩展声明的资源写入权限目录。已有编码不覆盖，也不自动授权。
func Ensure(db *dbx.DB, tenantID string) {
	if db == nil {
		return
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	for _, decl := range examplealert.Declared() {
		var exists int
		err := store.Scan(context.Background(), db, `
			SELECT count(*) FROM FG_AUTH_RESOURCE
			WHERE tenantId = @tenantId AND resourceCode = @resourceCode`,
			dbx.Args{"tenantId": tenantID, "resourceCode": decl.ResourceCode}, &exists)
		if err != nil || exists > 0 {
			continue
		}
		resourceID := decl.ResourceID
		if resourceID == "" {
			resourceID = decl.ResourceCode
		}
		_, _ = db.Exec(context.Background(), `
			INSERT INTO FG_AUTH_RESOURCE (
			  resourceId, tenantId, resourceName, resourceCode, resourceType,
			  resourcePath, parentResourceId, resourceLevel, sortOrder, iconClass, language,
			  resourceStatus, builtInFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @resourceId, @tenantId, @resourceName, @resourceCode, @resourceType,
			  @resourcePath, @parentResourceId, @resourceLevel, @sortOrder, @iconClass, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'
			)`, dbx.Args{
			"resourceId": resourceID, "tenantId": tenantID, "resourceName": decl.ResourceName,
			"resourceCode": decl.ResourceCode, "resourceType": decl.ResourceType,
			"resourcePath": nullIfEmpty(decl.ResourcePath), "parentResourceId": nullIfEmpty(decl.ParentResourceID),
			"resourceLevel": decl.ResourceLevel, "sortOrder": decl.SortOrder, "iconClass": nullIfEmpty(decl.IconClass),
		})
	}
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
