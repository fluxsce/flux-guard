// Package release 接收一次应用更新，保存制品，并向选中的应用侧代理各发一条命令。
package release

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/apprelease"
	"fluxguard/server/command"
	"fluxguard/server/command/models"
	"fluxguard/server/store"
	"fluxguard/server/web/views/fg0071/dao"
	appmodels "fluxguard/server/web/views/fg0071/models"
	dbx "github.com/fluxsce/dbx/db"
)

// Input 是控制台提交的一次更新。文件由调用方打开。
type Input struct {
	AppCode string
	Kind    string
	Version string
	File    string
	Body    io.Reader
	InstIDs []string
}

// Submit 校验所选实例属于同一个应用，然后按实例下发命令。
func Submit(ctx context.Context, db *dbx.DB, tenantID, user string, in Input) (appmodels.Release, error) {
	kind := strings.TrimSpace(in.Kind)
	switch kind {
	case "war", "jar", "static", "node", "script", "runtime":
	default:
		return appmodels.Release{}, errors.New("更新种类无效")
	}
	version := strings.TrimSpace(in.Version)
	if version == "" || len(version) > 64 {
		return appmodels.Release{}, errors.New("请填写版本")
	}
	fileName := filepath.Base(strings.TrimSpace(in.File))
	if fileName == "" || fileName == "." || strings.Contains(fileName, "..") {
		return appmodels.Release{}, errors.New("文件名无效")
	}
	inst, err := selected(ctx, db, tenantID, strings.TrimSpace(in.AppCode), kind, in.InstIDs)
	if err != nil {
		return appmodels.Release{}, err
	}
	if kind == "runtime" {
		inst = onePerAgent(inst)
	}
	svc := command.Current()
	if svc == nil {
		return appmodels.Release{}, errors.New("命令通道未启用")
	}
	releaseID := id.New()
	sum, err := apprelease.Save(tenantID, releaseID, in.Body)
	if err != nil {
		return appmodels.Release{}, err
	}
	now := store.FormatTime(time.Now().UTC())
	if _, err := db.Exec(ctx, `
		INSERT INTO FG_APP_RELEASE (
		  releaseId, tenantId, appCode, artifactKind, versionText, sha256, fileName,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @releaseId, @tenantId, @appCode, @artifactKind, @versionText, @sha256, @fileName,
		  @now, @user, @now, @user, 'APP', 1, 'Y'
		)`, dbx.Args{
		"releaseId": releaseID, "tenantId": tenantID, "appCode": inst[0].AppCode,
		"artifactKind": kind, "versionText": version, "sha256": sum, "fileName": fileName,
		"now": now, "user": user,
	}); err != nil {
		return appmodels.Release{}, err
	}
	action := protocol.ActionAppReplace
	if kind == "runtime" {
		action = protocol.ActionAppUpgrade
	}
	out := appmodels.Release{
		ReleaseID: releaseID, AppCode: inst[0].AppCode, ArtifactKind: kind, VersionText: version,
	}
	for _, item := range inst {
		view, err := svc.Dispatch(ctx, tenantID, user, command.DispatchInput{
			AgentID: item.AgentID, Action: action, ReleaseID: releaseID,
			AppName: item.AppCode, Kind: item.ArtifactKind, Version: version,
			SHA256: sum, FileName: fileName, Instance: item.InstanceName,
		})
		if err != nil {
			return out, err
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO FG_APP_RELEASE_ITEM (
			  commandId, tenantId, releaseId, instId, agentId,
			  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @commandId, @tenantId, @releaseId, @instId, @agentId,
			  @now, @user, @now, @user, 'APP', 1, 'Y'
			)`, dbx.Args{
			"commandId": view.CommandID, "tenantId": tenantID, "releaseId": releaseID,
			"instId": item.InstID, "agentId": item.AgentID, "now": now, "user": user,
		}); err != nil {
			return out, err
		}
		out.Items = append(out.Items, appmodels.ReleaseItem{
			CommandID: view.CommandID, InstID: item.InstID, AgentID: item.AgentID, Status: view.Status, Message: view.Message,
		})
	}
	return out, nil
}

// Progress 读一次发布里各条命令的当前状态。
func Progress(ctx context.Context, db *dbx.DB, tenantID, releaseID string) (appmodels.Release, error) {
	var out appmodels.Release
	err := store.Scan(ctx, db, `
		SELECT releaseId, appCode, artifactKind, versionText
		FROM FG_APP_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID},
		&out.ReleaseID, &out.AppCode, &out.ArtifactKind, &out.VersionText)
	if err != nil {
		return appmodels.Release{}, dao.ErrMissing
	}
	rows, err := db.Query(ctx, `
		SELECT item.commandId, item.instId, item.agentId, cmd.statusFlag, COALESCE(cmd.resultText, '')
		FROM FG_APP_RELEASE_ITEM item
		JOIN FG_COMMAND cmd ON cmd.tenantId = item.tenantId AND cmd.commandId = item.commandId
		WHERE item.tenantId = @tenantId AND item.releaseId = @releaseId AND item.activeFlag = 'Y'
		ORDER BY item.addTime, item.commandId`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID})
	if err != nil {
		return appmodels.Release{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item appmodels.ReleaseItem
		var flag string
		if err := rows.Scan(&item.CommandID, &item.InstID, &item.AgentID, &flag, &item.Message); err != nil {
			return appmodels.Release{}, err
		}
		item.Status = models.StatusName(flag)
		out.Items = append(out.Items, item)
	}
	if out.Items == nil {
		out.Items = []appmodels.ReleaseItem{}
	}
	return out, rows.Err()
}

func selected(ctx context.Context, db *dbx.DB, tenantID, appCode, kind string, ids []string) ([]appmodels.Instance, error) {
	if appCode == "" || len(ids) == 0 {
		return nil, errors.New("请选择应用实例")
	}
	all, err := dao.ListInstances(ctx, db, tenantID, appCode)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = true
		}
	}
	var out []appmodels.Instance
	for _, item := range all {
		if !want[item.InstID] {
			continue
		}
		if item.OnlineFlag != "Y" {
			return nil, errors.New("离线实例不能更新")
		}
		if kind != "runtime" && item.ArtifactKind != kind {
			return nil, errors.New("请选择同一种制品")
		}
		out = append(out, item)
	}
	if len(out) != len(want) {
		return nil, errors.New("有实例不属于这个应用")
	}
	return out, nil
}

func onePerAgent(in []appmodels.Instance) []appmodels.Instance {
	seen := map[string]bool{}
	var out []appmodels.Instance
	for _, item := range in {
		key := item.AgentID + "\n" + item.InstanceName
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
