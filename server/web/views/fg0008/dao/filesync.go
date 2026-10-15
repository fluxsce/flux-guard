package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0008/models"
	dbx "github.com/fluxsce/dbx/db"
)

// SyncFilesInventory 把短拨到的 files 清单写入 FG_APP_INST，并注销本实例下本次未再出现的行（保留 runtime 站点标记）。
func SyncFilesInventory(ctx context.Context, db *dbx.DB, tenantID, agentID, instance string, items []protocol.AppArtifact) error {
	tenantID = strings.TrimSpace(tenantID)
	agentID = strings.TrimSpace(agentID)
	instance = strings.TrimSpace(instance)
	if tenantID == "" || agentID == "" || instance == "" {
		return ErrMissing
	}
	tpl, err := siteTemplate(ctx, db, tenantID, agentID, instance)
	if err != nil {
		return err
	}
	if len(items) > protocol.AppFilesListMax {
		return fmt.Errorf("文件数量超过 %d，无法列出", protocol.AppFilesListMax)
	}
	edited := store.FormatTime(time.Now().UTC())
	reported := edited
	if tpl.ReportedTime != "" {
		reported = tpl.ReportedTime
	}
	seen := map[string]bool{instance: true} // 站点标记 appCode == instanceName
	for _, item := range items {
		appCode := clip(strings.TrimSpace(item.AppName), 100)
		kind := strings.TrimSpace(item.Kind)
		if appCode == "" || kind == "" || kind == "runtime" {
			continue
		}
		seen[appCode] = true
		if err := upsertSyncedApp(ctx, db, tenantID, agentID, appCode, kind, reported, edited); err != nil {
			return err
		}
		if err := upsertSyncedInst(ctx, db, tenantID, agentID, instance, tpl, item, appCode, reported, edited); err != nil {
			return err
		}
	}
	return retireSynced(ctx, db, tenantID, agentID, instance, seen, edited)
}

func siteTemplate(ctx context.Context, db *dbx.DB, tenantID, agentID, instance string) (models.Instance, error) {
	all, err := listInst(ctx, db, tenantID, "")
	if err != nil {
		return models.Instance{}, err
	}
	var fallback models.Instance
	for _, row := range all {
		if row.AgentID != agentID || row.InstanceName != instance {
			continue
		}
		if row.ArtifactKind == "runtime" {
			return row, nil
		}
		if fallback.InstID == "" {
			fallback = row
		}
	}
	if fallback.InstID != "" {
		return fallback, nil
	}
	return models.Instance{}, ErrMissing
}

func upsertSyncedApp(ctx context.Context, db *dbx.DB, tenantID, agentID, appCode, kind, reported, edited string) error {
	var existing int
	if err := store.Scan(ctx, db, `
		SELECT count(*) FROM FG_APP WHERE tenantId = @tenantId AND appCode = @appCode`,
		dbx.Args{"tenantId": tenantID, "appCode": appCode}, &existing); err != nil {
		return err
	}
	if existing > 0 {
		_, err := db.Exec(ctx, `
			UPDATE FG_APP SET artifactKind = @artifactKind, reportedTime = @reportedTime,
			  editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND appCode = @appCode`,
			dbx.Args{
				"artifactKind": kind, "reportedTime": reported, "editTime": edited,
				"tenantId": tenantID, "appCode": appCode,
			})
		return err
	}
	_, err := db.Exec(ctx, `
		INSERT INTO FG_APP (
		  appCode, tenantId, appName, artifactKind, ignoreFlag, reportedTime,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @appCode, @tenantId, @appName, @artifactKind, 'N', @reportedTime,
		  @editTime, @addWho, @editTime, 'system', 'FILES', 1, 'Y'
		)`, dbx.Args{
		"appCode": appCode, "tenantId": tenantID, "appName": appCode,
		"artifactKind": kind, "reportedTime": reported, "editTime": edited, "addWho": agentID,
	})
	return err
}

func upsertSyncedInst(ctx context.Context, db *dbx.DB, tenantID, agentID, instance string, tpl models.Instance, item protocol.AppArtifact, appCode, reported, edited string) error {
	var instID string
	err := store.Scan(ctx, db, `
		SELECT instId FROM FG_APP_INST
		WHERE tenantId = @tenantId AND agentId = @agentId AND instanceName = @instanceName AND appCode = @appCode`,
		dbx.Args{
			"tenantId": tenantID, "agentId": agentID, "appCode": appCode, "instanceName": instance,
		}, &instID)
	args := dbx.Args{
		"tenantId": tenantID, "agentId": agentID, "appCode": appCode,
		"machineId":      nullText(clip(tpl.MachineID, 64)),
		"hostName":       nullText(clip(tpl.HostName, 100)),
		"ipAddress":      nullText(clip(tpl.IPAddress, 128)),
		"instanceName":   clip(instance, 100),
		"runtimeKind":    clip(tpl.RuntimeKind, 16),
		"modeCode":       "files",
		"artifactKind":   clip(strings.TrimSpace(item.Kind), 16),
		"artifactPath":   nullText(clip(strings.TrimSpace(item.Path), 255)),
		"versionText":    nullText(clip(strings.TrimSpace(item.Version), 64)),
		"tomcatVersion":  nullText(clip(tpl.TomcatVersion, 64)),
		"runningFlag":    "N",
		"commandAddress": nullText(clip(tpl.CommandAddress, 128)),
		"reportedTime":   reported, "editTime": edited,
	}
	if err == nil && instID != "" {
		args["instId"] = instID
		_, err = db.Exec(ctx, `
			UPDATE FG_APP_INST SET
			  machineId = @machineId, hostName = @hostName, ipAddress = @ipAddress,
			  instanceName = @instanceName, runtimeKind = @runtimeKind, modeCode = @modeCode, artifactKind = @artifactKind,
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
	args["addWho"] = agentID
	_, err = db.Exec(ctx, `
		INSERT INTO FG_APP_INST (
		  instId, tenantId, agentId, appCode, machineId, hostName, ipAddress, instanceName,
		  runtimeKind, modeCode, artifactKind, artifactPath, versionText, tomcatVersion, runningFlag,
		  commandAddress, reportedTime, addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @instId, @tenantId, @agentId, @appCode, @machineId, @hostName, @ipAddress, @instanceName,
		  @runtimeKind, @modeCode, @artifactKind, @artifactPath, @versionText, @tomcatVersion, @runningFlag,
		  @commandAddress, @reportedTime, @editTime, @addWho, @editTime, 'system', 'FILES', 1, 'Y'
		)`, args)
	return err
}

func retireSynced(ctx context.Context, db *dbx.DB, tenantID, agentID, instance string, seen map[string]bool, edited string) error {
	rows, err := db.Query(ctx, `
		SELECT instId, appCode, COALESCE(artifactKind, '') FROM FG_APP_INST
		WHERE tenantId = @tenantId AND agentId = @agentId AND instanceName = @instanceName AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID, "instanceName": instance})
	if err != nil {
		return err
	}
	var drop []string
	for rows.Next() {
		var instID, appCode, kind string
		if err := rows.Scan(&instID, &appCode, &kind); err != nil {
			rows.Close()
			return err
		}
		if kind == "runtime" {
			continue
		}
		if !seen[strings.TrimSpace(appCode)] {
			drop = append(drop, instID)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, instID := range drop {
		if _, err := db.Exec(ctx, `
			UPDATE FG_APP_INST SET activeFlag = 'N', runningFlag = 'N', editTime = @editTime, editWho = 'system',
			  currentVersion = currentVersion + 1
			WHERE tenantId = @tenantId AND instId = @instId`,
			dbx.Args{"editTime": edited, "tenantId": tenantID, "instId": instID}); err != nil {
			return err
		}
	}
	return nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if n > 0 && len(s) > n {
		return s[:n]
	}
	return s
}

func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}
