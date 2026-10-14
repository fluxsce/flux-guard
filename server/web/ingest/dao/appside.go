package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// SaveAppside 按应用侧代理的清单更新应用和实例。其它代理什么都不写。
// 已有应用的显示名和忽略标记保持不动。这次没再报上来的实例标成不活动。
func SaveAppside(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	sides := batch.Sides()
	if strings.TrimSpace(batch.AgentCode) != protocol.AgentAppside || len(sides) == 0 {
		return nil
	}
	reported := store.FormatTime(batch.SentAt)
	edited := store.FormatTime(time.Now().UTC())
	seen := map[string]bool{}
	for i := range sides {
		side := sides[i]
		for _, item := range side.Artifacts {
			appCode := clip(strings.TrimSpace(item.AppName), 100)
			if appCode == "" {
				continue
			}
			seen[slotKey(side.InstanceName, appCode)] = true
			if err := upsertApp(ctx, tx, batch, appCode, strings.TrimSpace(item.Kind), reported, edited); err != nil {
				return err
			}
			if err := upsertInst(ctx, tx, batch, &side, item, appCode, reported, edited); err != nil {
				return err
			}
		}
	}
	return retireMissing(ctx, tx, batch.TenantID, batch.AgentID, seen, edited)
}

func slotKey(instance, appCode string) string {
	return strings.TrimSpace(instance) + "\n" + appCode
}

func upsertApp(ctx context.Context, tx *dbx.DB, batch protocol.Batch, appCode, kind, reported, edited string) error {
	var existing int
	if err := store.Scan(ctx, tx, `
		SELECT count(*) FROM FG_APP WHERE tenantId = @tenantId AND appCode = @appCode`,
		dbx.Args{"tenantId": batch.TenantID, "appCode": appCode}, &existing); err != nil {
		return err
	}
	if existing > 0 {
		_, err := tx.Exec(ctx, `
			UPDATE FG_APP SET artifactKind = @artifactKind, reportedTime = @reportedTime,
			  editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND appCode = @appCode`,
			dbx.Args{
				"artifactKind": kind, "reportedTime": reported, "editTime": edited,
				"tenantId": batch.TenantID, "appCode": appCode,
			})
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO FG_APP (
		  appCode, tenantId, appName, artifactKind, ignoreFlag, reportedTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @appCode, @tenantId, @appName, @artifactKind, 'N', @reportedTime,
		  @editTime, @addWho, @editTime, 'system', 'INGEST', 1, 'Y'
		)`, dbx.Args{
		"appCode": appCode, "tenantId": batch.TenantID, "appName": appCode,
		"artifactKind": kind, "reportedTime": reported, "editTime": edited, "addWho": batch.AgentID,
	})
	return err
}

func upsertInst(ctx context.Context, tx *dbx.DB, batch protocol.Batch, side *protocol.AppSide, item protocol.AppArtifact, appCode, reported, edited string) error {
	var instID string
	err := store.Scan(ctx, tx, `
		SELECT instId FROM FG_APP_INST
		WHERE tenantId = @tenantId AND agentId = @agentId AND instanceName = @instanceName AND appCode = @appCode`,
		dbx.Args{
			"tenantId": batch.TenantID, "agentId": batch.AgentID, "appCode": appCode,
			"instanceName": clip(strings.TrimSpace(side.InstanceName), 100),
		}, &instID)
	args := dbx.Args{
		"tenantId": batch.TenantID, "agentId": batch.AgentID, "appCode": appCode,
		"machineId":      nullText(clip(strings.TrimSpace(side.MachineID), 64)),
		"hostName":       nullText(clip(strings.TrimSpace(side.HostName), 100)),
		"ipAddress":      nullText(clip(strings.TrimSpace(side.IPAddress), 128)),
		"instanceName":   clip(strings.TrimSpace(side.InstanceName), 100),
		"runtimeKind":    clip(strings.TrimSpace(side.RuntimeKind), 16),
		"artifactKind":   clip(strings.TrimSpace(item.Kind), 16),
		"artifactPath":   nullText(clip(strings.TrimSpace(item.Path), 255)),
		"versionText":    nullText(clip(strings.TrimSpace(item.Version), 64)),
		"tomcatVersion":  nullText(clip(strings.TrimSpace(side.TomcatVersion), 64)),
		"runningFlag":    side.RunningFlag,
		"commandAddress": nullText(clip(strings.TrimSpace(side.CommandAddress), 128)),
		"reportedTime":   reported, "editTime": edited,
	}
	if err == nil && instID != "" {
		args["instId"] = instID
		_, err = tx.Exec(ctx, `
			UPDATE FG_APP_INST SET
			  machineId = @machineId, hostName = @hostName, ipAddress = @ipAddress,
			  instanceName = @instanceName, runtimeKind = @runtimeKind, artifactKind = @artifactKind,
			  artifactPath = @artifactPath, versionText = @versionText, tomcatVersion = @tomcatVersion,
			  runningFlag = @runningFlag, commandAddress = @commandAddress, reportedTime = @reportedTime,
			  activeFlag = 'Y', editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND instId = @instId`, args)
		return err
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	args["instId"] = id.New()
	args["addWho"] = batch.AgentID
	_, err = tx.Exec(ctx, `
		INSERT INTO FG_APP_INST (
		  instId, tenantId, agentId, appCode, machineId, hostName, ipAddress, instanceName,
		  runtimeKind, artifactKind, artifactPath, versionText, tomcatVersion, runningFlag,
		  commandAddress, reportedTime, addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @instId, @tenantId, @agentId, @appCode, @machineId, @hostName, @ipAddress, @instanceName,
		  @runtimeKind, @artifactKind, @artifactPath, @versionText, @tomcatVersion, @runningFlag,
		  @commandAddress, @reportedTime, @editTime, @addWho, @editTime, 'system', 'INGEST', 1, 'Y'
		)`, args)
	return err
}

func retireMissing(ctx context.Context, tx *dbx.DB, tenantID, agentID string, seen map[string]bool, edited string) error {
	rows, err := tx.Query(ctx, `
		SELECT instId, instanceName, appCode FROM FG_APP_INST
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID})
	if err != nil {
		return err
	}
	var drop []string
	for rows.Next() {
		var instID, instanceName, appCode string
		if err := rows.Scan(&instID, &instanceName, &appCode); err != nil {
			rows.Close()
			return err
		}
		if !seen[slotKey(instanceName, appCode)] {
			drop = append(drop, instID)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, instID := range drop {
		if _, err := tx.Exec(ctx, `
			UPDATE FG_APP_INST SET activeFlag = 'N', runningFlag = 'N', editTime = @editTime, editWho = 'system',
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND instId = @instId`,
			dbx.Args{"editTime": edited, "tenantId": tenantID, "instId": instID}); err != nil {
			return err
		}
	}
	return nil
}
