// Package release 接收一次应用更新，保存制品，并向选中的应用侧代理分批下发命令。
package release

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/command"
	"fluxguard/server/command/models"
	"fluxguard/server/helper/apprelease"
	"fluxguard/server/store"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/web/views/fg0008/dao"
	appmodels "fluxguard/server/web/views/fg0008/models"
	dbx "github.com/fluxsce/dbx/db"
)

// Input 是控制台提交的一次更新。文件由调用方打开。
type Input struct {
	AppCode     string
	Kind        string
	Version     string
	File        string
	Body        io.Reader
	InstIDs     []string
	CanaryCount int // 首批台数；0 表示用默认金丝雀 1 台
	BatchSize   int // 每批最多台数；0 表示用 AppReleaseBatchMax
}

type releaseExt struct {
	PendingInstIDs  []string `json:"pendingInstIds"`
	BatchSize       int      `json:"batchSize"`
	Purpose         string   `json:"purpose,omitempty"` // ""=update，rollback=回滚
	SourceReleaseID string   `json:"sourceReleaseId,omitempty"`
}

// Submit 校验所选实例属于同一个应用，保存制品，并按金丝雀/批次下发首批命令。
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
	batchSize, canary := normalizeBatch(in.BatchSize, in.CanaryCount)
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
	first, pending := splitInstBatch(inst, canary)
	extBody, _ := json.Marshal(releaseExt{PendingInstIDs: pendingIDs(pending), BatchSize: batchSize})
	now := store.FormatTime(time.Now().UTC())
	if _, err := db.Exec(ctx, `
		INSERT INTO FG_APP_RELEASE (
		  releaseId, tenantId, appCode, artifactKind, versionText, sha256, fileName, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @releaseId, @tenantId, @appCode, @artifactKind, @versionText, @sha256, @fileName, @extProperty,
		  @now, @user, @now, @user, 'APP', 1, 'Y'
		)`, dbx.Args{
		"releaseId": releaseID, "tenantId": tenantID, "appCode": inst[0].AppCode,
		"artifactKind": kind, "versionText": version, "sha256": sum, "fileName": fileName,
		"extProperty": string(extBody), "now": now, "user": user,
	}); err != nil {
		return appmodels.Release{}, err
	}
	action := protocol.ActionAppReplace
	if kind == "runtime" {
		action = protocol.ActionAppUpgrade
	}
	out := appmodels.Release{
		ReleaseID: releaseID, AppCode: inst[0].AppCode, ArtifactKind: kind, VersionText: version,
		FileName: fileName, PendingCount: len(pending), BatchSize: batchSize,
	}
	if err := dispatchUpdate(ctx, db, svc, tenantID, user, releaseID, action, kind, version, sum, fileName, first, &out); err != nil {
		return out, err
	}
	return out, nil
}

// Continue 把同一发布里尚未下发的实例按批次继续下发。
func Continue(ctx context.Context, db *dbx.DB, tenantID, user, releaseID string) (appmodels.Release, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return appmodels.Release{}, errors.New("缺少发布")
	}
	var appCode, kind, version, sum, fileName, raw string
	err := store.Scan(ctx, db, `
		SELECT appCode, artifactKind, versionText, sha256, COALESCE(fileName, ''), COALESCE(extProperty, '')
		FROM FG_APP_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID},
		&appCode, &kind, &version, &sum, &fileName, &raw)
	if err != nil {
		return appmodels.Release{}, dao.ErrMissing
	}
	var ext releaseExt
	_ = json.Unmarshal([]byte(raw), &ext)
	if ext.Purpose == "rollback" {
		return continueRollback(ctx, db, tenantID, user, releaseID, appCode, kind, version, ext)
	}
	if len(ext.PendingInstIDs) == 0 {
		return Progress(ctx, db, tenantID, releaseID)
	}
	batchSize := ext.BatchSize
	if batchSize <= 0 || batchSize > limits.AppReleaseBatchMax {
		batchSize = limits.AppReleaseBatchMax
	}
	pending, err := selected(ctx, db, tenantID, appCode, kind, ext.PendingInstIDs)
	if err != nil {
		return appmodels.Release{}, err
	}
	if kind == "runtime" {
		pending = onePerAgent(pending)
	}
	first, rest := splitInstBatch(pending, batchSize)
	ext.PendingInstIDs = pendingIDs(rest)
	ext.BatchSize = batchSize
	if err := saveExt(ctx, db, tenantID, user, releaseID, ext); err != nil {
		return appmodels.Release{}, err
	}
	svc := command.Current()
	if svc == nil {
		return appmodels.Release{}, errors.New("命令通道未启用")
	}
	action := protocol.ActionAppReplace
	if kind == "runtime" {
		action = protocol.ActionAppUpgrade
	}
	out := appmodels.Release{
		ReleaseID: releaseID, AppCode: appCode, ArtifactKind: kind, VersionText: version,
		FileName: fileName, PendingCount: len(rest), BatchSize: batchSize,
	}
	if err := dispatchUpdate(ctx, db, svc, tenantID, user, releaseID, action, kind, version, sum, fileName, first, &out); err != nil {
		return out, err
	}
	full, err := Progress(ctx, db, tenantID, releaseID)
	if err != nil {
		return out, nil
	}
	return full, nil
}

// Rollback 对一次已成功换包的发布，按实例备份路径回滚；默认金丝雀先回 1 台。
func Rollback(ctx context.Context, db *dbx.DB, tenantID, user, sourceReleaseID string, instIDs []string, canaryCount int) (appmodels.Release, error) {
	sourceReleaseID = strings.TrimSpace(sourceReleaseID)
	if sourceReleaseID == "" {
		return appmodels.Release{}, errors.New("缺少发布")
	}
	source, err := Progress(ctx, db, tenantID, sourceReleaseID)
	if err != nil {
		return appmodels.Release{}, err
	}
	if source.Purpose == "rollback" {
		return appmodels.Release{}, errors.New("不能对回滚发布再回滚，请选择正向更新记录")
	}
	targets, err := rollbackTargets(ctx, db, tenantID, source, instIDs)
	if err != nil {
		return appmodels.Release{}, err
	}
	batchSize, canary := normalizeBatch(0, canaryCount)
	first, pending := splitRollbackBatch(targets, canary)
	svc := command.Current()
	if svc == nil {
		return appmodels.Release{}, errors.New("命令通道未启用")
	}
	releaseID := id.New()
	version := "rollback:" + source.VersionText
	if len(version) > 64 {
		version = version[:64]
	}
	extBody, _ := json.Marshal(releaseExt{
		PendingInstIDs: pendingRollbackIDs(pending), BatchSize: batchSize,
		Purpose: "rollback", SourceReleaseID: sourceReleaseID,
	})
	now := store.FormatTime(time.Now().UTC())
	if _, err := db.Exec(ctx, `
		INSERT INTO FG_APP_RELEASE (
		  releaseId, tenantId, appCode, artifactKind, versionText, sha256, fileName, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @releaseId, @tenantId, @appCode, @artifactKind, @versionText, @sha256, @fileName, @extProperty,
		  @now, @user, @now, @user, 'APP', 1, 'Y'
		)`, dbx.Args{
		"releaseId": releaseID, "tenantId": tenantID, "appCode": source.AppCode,
		"artifactKind": source.ArtifactKind, "versionText": version, "sha256": "rollback",
		"fileName": source.FileName, "extProperty": string(extBody), "now": now, "user": user,
	}); err != nil {
		return appmodels.Release{}, err
	}
	out := appmodels.Release{
		ReleaseID: releaseID, AppCode: source.AppCode, ArtifactKind: source.ArtifactKind,
		VersionText: version, FileName: source.FileName, Purpose: "rollback",
		SourceReleaseID: sourceReleaseID, PendingCount: len(pending), BatchSize: batchSize,
	}
	if err := dispatchRollback(ctx, db, svc, tenantID, user, releaseID, source.ArtifactKind, version, first, &out); err != nil {
		return out, err
	}
	return out, nil
}

func continueRollback(ctx context.Context, db *dbx.DB, tenantID, user, releaseID, appCode, kind, version string, ext releaseExt) (appmodels.Release, error) {
	if len(ext.PendingInstIDs) == 0 {
		return Progress(ctx, db, tenantID, releaseID)
	}
	source, err := Progress(ctx, db, tenantID, ext.SourceReleaseID)
	if err != nil {
		return appmodels.Release{}, err
	}
	batchSize := ext.BatchSize
	if batchSize <= 0 || batchSize > limits.AppReleaseBatchMax {
		batchSize = limits.AppReleaseBatchMax
	}
	targets, err := rollbackTargets(ctx, db, tenantID, source, ext.PendingInstIDs)
	if err != nil {
		return appmodels.Release{}, err
	}
	first, rest := splitRollbackBatch(targets, batchSize)
	ext.PendingInstIDs = pendingRollbackIDs(rest)
	ext.BatchSize = batchSize
	if err := saveExt(ctx, db, tenantID, user, releaseID, ext); err != nil {
		return appmodels.Release{}, err
	}
	svc := command.Current()
	if svc == nil {
		return appmodels.Release{}, errors.New("命令通道未启用")
	}
	out := appmodels.Release{
		ReleaseID: releaseID, AppCode: appCode, ArtifactKind: kind, VersionText: version,
		Purpose: "rollback", SourceReleaseID: ext.SourceReleaseID,
		PendingCount: len(rest), BatchSize: batchSize,
	}
	if err := dispatchRollback(ctx, db, svc, tenantID, user, releaseID, kind, version, first, &out); err != nil {
		return out, err
	}
	full, err := Progress(ctx, db, tenantID, releaseID)
	if err != nil {
		return out, nil
	}
	return full, nil
}

// Progress 读一次发布里各条命令的当前状态。
func Progress(ctx context.Context, db *dbx.DB, tenantID, releaseID string) (appmodels.Release, error) {
	var out appmodels.Release
	var raw string
	err := store.Scan(ctx, db, `
		SELECT releaseId, appCode, artifactKind, versionText, COALESCE(fileName, ''), COALESCE(extProperty, '')
		FROM FG_APP_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID},
		&out.ReleaseID, &out.AppCode, &out.ArtifactKind, &out.VersionText, &out.FileName, &raw)
	if err != nil {
		return appmodels.Release{}, dao.ErrMissing
	}
	var ext releaseExt
	_ = json.Unmarshal([]byte(raw), &ext)
	out.PendingCount = len(ext.PendingInstIDs)
	out.BatchSize = ext.BatchSize
	out.Purpose = ext.Purpose
	out.SourceReleaseID = ext.SourceReleaseID
	rows, err := db.Query(ctx, `
		SELECT item.commandId, item.instId, item.agentId, cmd.statusFlag,
		       COALESCE(cmd.resultText, ''), COALESCE(cmd.noteText, '')
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
		var flag, note string
		if err := rows.Scan(&item.CommandID, &item.InstID, &item.AgentID, &flag, &item.Message, &note); err != nil {
			return appmodels.Release{}, err
		}
		item.Status = models.StatusName(flag)
		item.Steps = decodeSteps(note)
		out.Items = append(out.Items, item)
	}
	if out.Items == nil {
		out.Items = []appmodels.ReleaseItem{}
	}
	return out, rows.Err()
}

type rollbackTarget struct {
	InstID       string
	AgentID      string
	InstanceName string
	AppCode      string
	TargetPath   string
	BackupPath   string
	Online       bool
}

func rollbackTargets(ctx context.Context, db *dbx.DB, tenantID string, source appmodels.Release, instIDs []string) ([]rollbackTarget, error) {
	want := map[string]bool{}
	for _, id := range instIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = true
		}
	}
	filter := len(want) > 0
	instRows, err := dao.ListInstances(ctx, db, tenantID, source.AppCode)
	if err != nil {
		return nil, err
	}
	online := map[string]appmodels.Instance{}
	for _, row := range instRows {
		online[row.InstID] = row
	}
	var out []rollbackTarget
	for _, item := range source.Items {
		if filter && !want[item.InstID] {
			continue
		}
		if item.Status != "done" {
			continue
		}
		target, backup, ok := pathsFromSteps(item.Steps)
		if !ok {
			continue
		}
		row, ok := online[item.InstID]
		if !ok {
			return nil, errors.New("有实例已不在清单中")
		}
		if row.OnlineFlag != "Y" {
			return nil, errors.New("离线实例不能回滚")
		}
		out = append(out, rollbackTarget{
			InstID: item.InstID, AgentID: item.AgentID, InstanceName: row.InstanceName,
			AppCode: source.AppCode, TargetPath: target, BackupPath: backup, Online: true,
		})
	}
	if filter && len(out) != len(want) {
		return nil, errors.New("所选实例没有可回滚的备份（须已成功换包且带回备份路径）")
	}
	if len(out) == 0 {
		return nil, errors.New("没有可回滚的实例")
	}
	return out, nil
}

func pathsFromSteps(steps []appmodels.ReleaseStep) (target, backup string, ok bool) {
	for _, step := range steps {
		if step.Name == "replace" && step.Status == "done" &&
			strings.TrimSpace(step.TargetPath) != "" && strings.TrimSpace(step.BackupPath) != "" {
			return step.TargetPath, step.BackupPath, true
		}
	}
	return "", "", false
}

func dispatchUpdate(
	ctx context.Context, db *dbx.DB, svc *command.Service,
	tenantID, user, releaseID, action, kind, version, sum, fileName string,
	inst []appmodels.Instance, out *appmodels.Release,
) error {
	now := store.FormatTime(time.Now().UTC())
	for _, item := range inst {
		view, err := svc.Dispatch(ctx, tenantID, user, command.DispatchInput{
			AgentID: item.AgentID, Action: action, ReleaseID: releaseID,
			AppName: item.AppCode, Kind: item.ArtifactKind, Version: version,
			SHA256: sum, FileName: fileName, Instance: item.InstanceName,
		})
		if err != nil {
			return err
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
			return err
		}
		out.Items = append(out.Items, appmodels.ReleaseItem{
			CommandID: view.CommandID, InstID: item.InstID, AgentID: item.AgentID, Status: view.Status, Message: view.Message,
		})
	}
	return nil
}

func dispatchRollback(
	ctx context.Context, db *dbx.DB, svc *command.Service,
	tenantID, user, releaseID, kind, version string,
	targets []rollbackTarget, out *appmodels.Release,
) error {
	now := store.FormatTime(time.Now().UTC())
	for _, item := range targets {
		view, err := svc.Dispatch(ctx, tenantID, user, command.DispatchInput{
			AgentID: item.AgentID, Action: protocol.ActionAppRollback, ReleaseID: releaseID,
			AppName: item.AppCode, Kind: kind, Version: version,
			Instance: item.InstanceName, TargetPath: item.TargetPath, BackupPath: item.BackupPath,
		})
		if err != nil {
			return err
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
			return err
		}
		out.Items = append(out.Items, appmodels.ReleaseItem{
			CommandID: view.CommandID, InstID: item.InstID, AgentID: item.AgentID, Status: view.Status, Message: view.Message,
		})
	}
	return nil
}

func saveExt(ctx context.Context, db *dbx.DB, tenantID, user, releaseID string, ext releaseExt) error {
	extBody, _ := json.Marshal(ext)
	now := store.FormatTime(time.Now().UTC())
	_, err := db.Exec(ctx, `
		UPDATE FG_APP_RELEASE
		SET extProperty = @extProperty, editTime = @now, editWho = @user, currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND releaseId = @releaseId`,
		dbx.Args{"extProperty": string(extBody), "now": now, "user": user, "tenantId": tenantID, "releaseId": releaseID})
	return err
}

// decodeSteps 从命令 noteText 还原步骤；旧回执没有步骤时返回空。
func decodeSteps(raw string) []appmodels.ReleaseStep {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var steps []protocol.CommandStep
	if err := json.Unmarshal([]byte(raw), &steps); err != nil || len(steps) == 0 {
		return nil
	}
	out := make([]appmodels.ReleaseStep, 0, len(steps))
	for _, step := range steps {
		out = append(out, appmodels.ReleaseStep{
			Name: step.Name, Status: step.Status,
			TargetPath: step.TargetPath, BackupPath: step.BackupPath, Detail: step.Detail,
		})
	}
	return out
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
	order := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || want[id] {
			continue
		}
		want[id] = true
		order = append(order, id)
	}
	byID := map[string]appmodels.Instance{}
	for _, item := range all {
		byID[item.InstID] = item
	}
	var out []appmodels.Instance
	for _, id := range order {
		item, ok := byID[id]
		if !ok {
			return nil, errors.New("有实例不属于这个应用")
		}
		if item.OnlineFlag != "Y" {
			return nil, errors.New("离线实例不能更新")
		}
		if kind == "runtime" {
			mode := strings.TrimSpace(item.ModeCode)
			if mode == "" && item.ArtifactKind == "war" {
				mode = "tomcat"
			}
			if mode != "tomcat" {
				return nil, errors.New("版本升级只支持 Tomcat 实例")
			}
		} else if item.ArtifactKind != kind {
			return nil, errors.New("请选择同一种制品")
		}
		out = append(out, item)
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

func normalizeBatch(batchSize, canary int) (int, int) {
	if batchSize <= 0 || batchSize > limits.AppReleaseBatchMax {
		batchSize = limits.AppReleaseBatchMax
	}
	if canary <= 0 {
		canary = limits.AppReleaseCanaryDefault
	}
	if canary > batchSize {
		canary = batchSize
	}
	return batchSize, canary
}

func splitInstBatch(inst []appmodels.Instance, n int) (first, rest []appmodels.Instance) {
	if n <= 0 || n >= len(inst) {
		return inst, nil
	}
	first = append([]appmodels.Instance{}, inst[:n]...)
	rest = append([]appmodels.Instance{}, inst[n:]...)
	return first, rest
}

func splitRollbackBatch(in []rollbackTarget, n int) (first, rest []rollbackTarget) {
	if n <= 0 || n >= len(in) {
		return in, nil
	}
	first = append([]rollbackTarget{}, in[:n]...)
	rest = append([]rollbackTarget{}, in[n:]...)
	return first, rest
}

func pendingIDs(inst []appmodels.Instance) []string {
	out := make([]string, 0, len(inst))
	for _, item := range inst {
		out = append(out, item.InstID)
	}
	return out
}

func pendingRollbackIDs(in []rollbackTarget) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		out = append(out, item.InstID)
	}
	return out
}

// ParseCanary 解析表单里的金丝雀台数。
func ParseCanary(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
