package service

import (
	"context"

	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

const commandButton = "fg0002:command"

// ensureButton 给已有库补上下发按钮，并授给超级管理员。只读角色不授予。
func ensureButton(ctx context.Context, db *dbx.DB, tenantID string) error {
	var resources int
	if err := store.Scan(ctx, db, `
		SELECT count(*) FROM FG_AUTH_RESOURCE
		WHERE tenantId = @tenantId AND resourceId = @resourceId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "resourceId": commandButton}, &resources); err != nil {
		return err
	}
	if resources == 0 {
		if _, err := db.Exec(ctx, `
			INSERT INTO FG_AUTH_RESOURCE (
			  resourceId, tenantId, resourceName, resourceCode, resourceType,
			  parentResourceId, resourceLevel, sortOrder, language,
			  resourceStatus, builtInFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @resourceId, @tenantId, '下发命令', @resourceId, 'BUTTON',
			  'fg0002', 3, 30, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'
			)`, dbx.Args{"tenantId": tenantID, "resourceId": commandButton}); err != nil {
			return err
		}
	}
	var grants int
	if err := store.Scan(ctx, db, `
		SELECT count(*) FROM FG_AUTH_ROLE_RESOURCE
		WHERE tenantId = @tenantId AND roleId = 'ROLE_SUPER_ADMIN' AND resourceId = @resourceId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "resourceId": commandButton}, &grants); err != nil {
		return err
	}
	if grants > 0 {
		return nil
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_AUTH_ROLE_RESOURCE (
		  roleResourceId, tenantId, roleId, resourceId, permissionType, grantedBy,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  'RR_SA_fg0002_command', @tenantId, 'ROLE_SUPER_ADMIN', @resourceId, 'ALLOW', 'system',
		  'system', 'system', 'INIT', 1, 'Y'
		)`, dbx.Args{"tenantId": tenantID, "resourceId": commandButton})
	return err
}
