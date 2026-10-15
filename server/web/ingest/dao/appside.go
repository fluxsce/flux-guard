package dao

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/helper/agent"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// SaveAppside 按应用侧代理的清单更新应用和实例。其它代理什么都不写。
// 已有应用的显示名和忽略标记保持不动。这次没再报上来的实例标成不活动。
// 同时把主机名、地址、命令端口写回 FG_AGENT，供采集列表只读代理表。
func SaveAppside(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	sides := batch.Sides()
	if strings.TrimSpace(batch.AgentCode) != protocol.AgentAppside || len(sides) == 0 {
		return nil
	}
	reported := store.FormatTime(batch.SentAt)
	edited := store.FormatTime(time.Now().UTC())
	if err := syncAppsideAgent(ctx, tx, batch, sides, edited); err != nil {
		return err
	}
	seen := map[string]bool{}
	// files 心跳只报站点标记时，不要把上次短拨同步进来的 war/日志行误注销。
	filesHeartbeatOnly := map[string]bool{}
	for i := range sides {
		side := sides[i]
		if strings.TrimSpace(side.Mode) == "files" && filesSiteHeartbeat(side) {
			filesHeartbeatOnly[strings.TrimSpace(side.InstanceName)] = true
		}
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
	return retireMissing(ctx, tx, batch.TenantID, batch.AgentID, seen, filesHeartbeatOnly, edited)
}

// filesSiteHeartbeat 判断这一套 files 上报是否仅为站点标记（非全量清单）。
func filesSiteHeartbeat(side protocol.AppSide) bool {
	if len(side.Artifacts) != 1 {
		return false
	}
	item := side.Artifacts[0]
	kind := strings.TrimSpace(item.Kind)
	name := strings.TrimSpace(item.AppName)
	inst := strings.TrimSpace(side.InstanceName)
	return kind == "runtime" || (name != "" && name == inst)
}

// syncAppsideAgent 用本批上报补全 FG_AGENT 的展示字段。空值不覆盖已有主机资料。
// 显示名仅在系统底名时改成 appside@主机名，手工名不冲掉；同机其它种类代理另有 agentCode 分行。
func syncAppsideAgent(ctx context.Context, tx *dbx.DB, batch protocol.Batch, sides []protocol.AppSide, edited string) error {
	hostName, ip, cmd := "", "", ""
	for _, side := range sides {
		if hostName == "" {
			hostName = strings.TrimSpace(side.HostName)
		}
		if ip == "" {
			ip = strings.TrimSpace(side.IPAddress)
		}
		if cmd == "" {
			cmd = strings.TrimSpace(side.CommandAddress)
		}
	}
	var keptAgent string
	err := store.Scan(ctx, tx, `
		SELECT COALESCE(agentName, '') FROM FG_AGENT WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": batch.TenantID, "agentId": batch.AgentID}, &keptAgent)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	display := agent.ResolveDisplayName(keptAgent, batch.AgentCode, "", hostName)
	if hostName == "" && ip == "" && cmd == "" && display == keptAgent {
		return nil
	}
	_, err = tx.Exec(ctx, `
		UPDATE FG_AGENT
		SET hostName = CASE WHEN @hostName = '' THEN hostName ELSE @hostName END,
		    ipAddress = CASE WHEN @ipAddress = '' THEN ipAddress ELSE @ipAddress END,
		    commandAddress = CASE WHEN @commandAddress = '' THEN commandAddress ELSE @commandAddress END,
		    agentName = @agentName,
		    editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{
			"hostName": hostName, "ipAddress": ip, "commandAddress": cmd, "agentName": display,
			"editTime": edited, "tenantId": batch.TenantID, "agentId": batch.AgentID,
		})
	return err
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
		"modeCode":       clip(strings.TrimSpace(side.Mode), 16),
		"artifactKind":   clip(strings.TrimSpace(item.Kind), 16),
		"artifactPath":   nullText(clip(strings.TrimSpace(item.Path), 255)),
		"versionText":    nullText(clip(strings.TrimSpace(item.Version), 64)),
		"tomcatVersion":  nullText(clip(strings.TrimSpace(side.TomcatVersion), 64)),
		"runningFlag":    side.RunningFlag,
		"commandAddress": nullText(clip(strings.TrimSpace(side.CommandAddress), 128)),
		"noteText":       clip(strings.TrimSpace(side.Description), 500),
		"reportedTime":   reported, "editTime": edited,
	}
	if err == nil && instID != "" {
		args["instId"] = instID
		_, err = tx.Exec(ctx, `
			UPDATE FG_APP_INST SET
			  machineId = @machineId, hostName = @hostName, ipAddress = @ipAddress,
			  instanceName = @instanceName, runtimeKind = @runtimeKind, modeCode = @modeCode, artifactKind = @artifactKind,
			  artifactPath = @artifactPath, versionText = @versionText, tomcatVersion = @tomcatVersion,
			  runningFlag = @runningFlag, commandAddress = @commandAddress, reportedTime = @reportedTime,
			  noteText = CASE WHEN @noteText = '' THEN noteText ELSE @noteText END,
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
		  runtimeKind, modeCode, artifactKind, artifactPath, versionText, tomcatVersion, runningFlag,
		  commandAddress, noteText, reportedTime, addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @instId, @tenantId, @agentId, @appCode, @machineId, @hostName, @ipAddress, @instanceName,
		  @runtimeKind, @modeCode, @artifactKind, @artifactPath, @versionText, @tomcatVersion, @runningFlag,
		  @commandAddress, @noteText, @reportedTime, @editTime, @addWho, @editTime, 'system', 'INGEST', 1, 'Y'
		)`, args)
	return err
}

func retireMissing(ctx context.Context, tx *dbx.DB, tenantID, agentID string, seen map[string]bool, filesHeartbeatOnly map[string]bool, edited string) error {
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
		if filesHeartbeatOnly[strings.TrimSpace(instanceName)] {
			// 心跳站点标记：只允许注销「站点行本身」；子文件等短拨清单保留到下次刷新。
			if strings.TrimSpace(appCode) != strings.TrimSpace(instanceName) {
				continue
			}
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
