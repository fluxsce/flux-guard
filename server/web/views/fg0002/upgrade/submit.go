// Package upgrade 从中央 agents/ 包装下发代理自更新，并向选中的代理分批下发命令。
package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fluxguard/common/buildinfo"
	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/command"
	"fluxguard/server/command/models"
	"fluxguard/server/helper/agentpack"
	"fluxguard/server/helper/agentrelease"
	"fluxguard/server/store"
	"fluxguard/server/utils/const/limits"
	fgmodels "fluxguard/server/web/views/fg0002/models"

	dbx "github.com/fluxsce/dbx/db"
)

// Input 是控制台提交的一次代理升级。安装包取自中央 agents/，不要求用户上传。
type Input struct {
	AgentCode   string
	Version     string
	GoOS        string
	GoArch      string
	AgentIDs    []string
	CanaryCount int // 首批台数；0 表示用默认金丝雀 1 台
	BatchSize   int // 每批最多台数；0 表示用 AgentUpgradeBatchMax
}

type releaseExt struct {
	PendingAgentIDs []string `json:"pendingAgentIds"`
	BatchSize       int      `json:"batchSize"`
}

// Submit 校验平台与在线状态，从中央 agents/ 复制制品，并按金丝雀/批次下发首批命令。
func Submit(ctx context.Context, db *dbx.DB, tenantID, user string, in Input) (fgmodels.AgentRelease, error) {
	code := strings.TrimSpace(in.AgentCode)
	if !protocol.UpdatableAgent(code) {
		return fgmodels.AgentRelease{}, errors.New("只支持升级 host、go、appside 代理")
	}
	version := strings.TrimSpace(in.Version)
	if version == "" {
		version = strings.TrimSpace(buildinfo.Version)
	}
	if version == "" || len(version) > 64 {
		return fgmodels.AgentRelease{}, errors.New("请填写版本")
	}
	// 发行包里 agents/ 与中央同平台；只允许升级匹配该平台的在线代理。
	packOS, packArch := runtime.GOOS, runtime.GOARCH
	goos, goarch, err := normalizePlatform(in.GoOS, in.GoArch)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	if goos != packOS || goarch != packArch {
		return fgmodels.AgentRelease{}, fmt.Errorf(
			"中央 agents/ 安装包为 %s/%s，不能下发到 %s/%s", packOS, packArch, goos, goarch)
	}
	pack, fileName, err := agentpack.Open(code)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	defer pack.Close()
	batchSize := in.BatchSize
	if batchSize <= 0 || batchSize > limits.AgentUpgradeBatchMax {
		batchSize = limits.AgentUpgradeBatchMax
	}
	canary := in.CanaryCount
	if canary <= 0 {
		canary = limits.AgentUpgradeCanaryDefault
	}
	if canary > batchSize {
		canary = batchSize
	}
	targets, err := selected(ctx, db, tenantID, code, goos, goarch, in.AgentIDs)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	svc := command.Current()
	if svc == nil {
		return fgmodels.AgentRelease{}, errors.New("命令通道未启用")
	}
	releaseID := id.New()
	sum, err := agentrelease.Save(tenantID, releaseID, pack)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	first, pending := splitBatch(targets, canary)
	extBody, _ := json.Marshal(releaseExt{PendingAgentIDs: pending, BatchSize: batchSize})
	now := store.FormatTime(time.Now().UTC())
	if _, err := db.Exec(ctx, `
		INSERT INTO FG_AGENT_RELEASE (
		  releaseId, tenantId, agentCode, versionText, sha256, fileName, goos, goarch, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @releaseId, @tenantId, @agentCode, @versionText, @sha256, @fileName, @goos, @goarch, @extProperty,
		  @now, @user, @now, @user, 'AGT', 1, 'Y'
		)`, dbx.Args{
		"releaseId": releaseID, "tenantId": tenantID, "agentCode": code,
		"versionText": version, "sha256": sum, "fileName": fileName, "goos": goos, "goarch": goarch,
		"extProperty": string(extBody), "now": now, "user": user,
	}); err != nil {
		return fgmodels.AgentRelease{}, err
	}
	out := fgmodels.AgentRelease{
		ReleaseID: releaseID, AgentCode: code, VersionText: version, FileName: fileName,
		GoOS: goos, GoArch: goarch, PendingCount: len(pending), BatchSize: batchSize,
	}
	if err := dispatchBatch(ctx, db, svc, tenantID, user, releaseID, code, version, sum, fileName, goos, goarch, first, &out); err != nil {
		return out, err
	}
	return out, nil
}

// Continue 把同一发布里尚未下发的代理按批次继续下发。
func Continue(ctx context.Context, db *dbx.DB, tenantID, user, releaseID string) (fgmodels.AgentRelease, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return fgmodels.AgentRelease{}, errors.New("缺少发布")
	}
	var code, version, sum, fileName, goos, goarch, raw string
	err := store.Scan(ctx, db, `
		SELECT agentCode, versionText, sha256, COALESCE(fileName, ''), COALESCE(goos, ''), COALESCE(goarch, ''), COALESCE(extProperty, '')
		FROM FG_AGENT_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID},
		&code, &version, &sum, &fileName, &goos, &goarch, &raw)
	if err != nil {
		return fgmodels.AgentRelease{}, errors.New("找不到这次升级")
	}
	var ext releaseExt
	_ = json.Unmarshal([]byte(raw), &ext)
	if len(ext.PendingAgentIDs) == 0 {
		return Progress(ctx, db, tenantID, releaseID)
	}
	batchSize := ext.BatchSize
	if batchSize <= 0 || batchSize > limits.AgentUpgradeBatchMax {
		batchSize = limits.AgentUpgradeBatchMax
	}
	// 继续前再校验仍在线且平台匹配。
	pending, err := selected(ctx, db, tenantID, code, goos, goarch, ext.PendingAgentIDs)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	first, rest := splitBatch(pending, batchSize)
	ext.PendingAgentIDs = rest
	ext.BatchSize = batchSize
	extBody, _ := json.Marshal(ext)
	now := store.FormatTime(time.Now().UTC())
	if _, err := db.Exec(ctx, `
		UPDATE FG_AGENT_RELEASE
		SET extProperty = @extProperty, editTime = @now, editWho = @user, currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND releaseId = @releaseId`,
		dbx.Args{"extProperty": string(extBody), "now": now, "user": user, "tenantId": tenantID, "releaseId": releaseID}); err != nil {
		return fgmodels.AgentRelease{}, err
	}
	svc := command.Current()
	if svc == nil {
		return fgmodels.AgentRelease{}, errors.New("命令通道未启用")
	}
	out := fgmodels.AgentRelease{
		ReleaseID: releaseID, AgentCode: code, VersionText: version, FileName: fileName,
		GoOS: goos, GoArch: goarch, PendingCount: len(rest), BatchSize: batchSize,
	}
	if err := dispatchBatch(ctx, db, svc, tenantID, user, releaseID, code, version, sum, fileName, goos, goarch, first, &out); err != nil {
		return out, err
	}
	// 补上已有条目，便于页面一次看到全部。
	full, err := Progress(ctx, db, tenantID, releaseID)
	if err != nil {
		return out, nil
	}
	return full, nil
}

// Progress 读一次发布里各条命令的当前状态，并对照代理上报版本。
func Progress(ctx context.Context, db *dbx.DB, tenantID, releaseID string) (fgmodels.AgentRelease, error) {
	var out fgmodels.AgentRelease
	var raw string
	err := store.Scan(ctx, db, `
		SELECT releaseId, agentCode, versionText, COALESCE(fileName, ''), COALESCE(goos, ''), COALESCE(goarch, ''), COALESCE(extProperty, '')
		FROM FG_AGENT_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID},
		&out.ReleaseID, &out.AgentCode, &out.VersionText, &out.FileName, &out.GoOS, &out.GoArch, &raw)
	if err != nil {
		return fgmodels.AgentRelease{}, errors.New("找不到这次升级")
	}
	var ext releaseExt
	_ = json.Unmarshal([]byte(raw), &ext)
	out.PendingCount = len(ext.PendingAgentIDs)
	out.BatchSize = ext.BatchSize
	rows, err := db.Query(ctx, `
		SELECT item.commandId, item.agentId, COALESCE(a.agentName, ''), cmd.statusFlag, COALESCE(cmd.resultText, ''),
		       COALESCE(a.agentVersion, '')
		FROM FG_AGENT_RELEASE_ITEM item
		JOIN FG_COMMAND cmd ON cmd.tenantId = item.tenantId AND cmd.commandId = item.commandId
		LEFT JOIN FG_AGENT a ON a.tenantId = item.tenantId AND a.agentId = item.agentId
		WHERE item.tenantId = @tenantId AND item.releaseId = @releaseId AND item.activeFlag = 'Y'
		ORDER BY item.addTime, item.commandId`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID})
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item fgmodels.AgentReleaseItem
		var flag string
		if err := rows.Scan(&item.CommandID, &item.AgentID, &item.AgentName, &flag, &item.Message, &item.AgentVersion); err != nil {
			return fgmodels.AgentRelease{}, err
		}
		item.Status = models.StatusName(flag)
		if item.AgentVersion != "" {
			if item.AgentVersion == out.VersionText {
				item.VersionMatched = "Y"
			} else {
				item.VersionMatched = "N"
			}
		}
		out.Items = append(out.Items, item)
	}
	if out.Items == nil {
		out.Items = []fgmodels.AgentReleaseItem{}
	}
	return out, rows.Err()
}

func dispatchBatch(
	ctx context.Context, db *dbx.DB, svc *command.Service,
	tenantID, user, releaseID, code, version, sum, fileName, goos, goarch string,
	agentIDs []string, out *fgmodels.AgentRelease,
) error {
	now := store.FormatTime(time.Now().UTC())
	for _, agentID := range agentIDs {
		view, err := svc.Dispatch(ctx, tenantID, user, command.DispatchInput{
			AgentID: agentID, Action: protocol.ActionAgentUpdate, ReleaseID: releaseID,
			Kind: code, Version: version, SHA256: sum, FileName: fileName, GoOS: goos, GoArch: goarch,
		})
		if err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO FG_AGENT_RELEASE_ITEM (
			  commandId, tenantId, releaseId, agentId,
			  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
			) VALUES (
			  @commandId, @tenantId, @releaseId, @agentId,
			  @now, @user, @now, @user, 'AGT', 1, 'Y'
			)`, dbx.Args{
			"commandId": view.CommandID, "tenantId": tenantID, "releaseId": releaseID,
			"agentId": agentID, "now": now, "user": user,
		}); err != nil {
			return err
		}
		out.Items = append(out.Items, fgmodels.AgentReleaseItem{
			CommandID: view.CommandID, AgentID: agentID, Status: view.Status, Message: view.Message,
		})
	}
	return nil
}

func selected(ctx context.Context, db *dbx.DB, tenantID, code, goos, goarch string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, errors.New("请选择要升级的代理")
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
	if len(want) == 0 {
		return nil, errors.New("请选择要升级的代理")
	}
	rows, err := db.Query(ctx, `
		SELECT agentId, COALESCE(agentCode, ''), statusFlag, lastSeenTime, COALESCE(goos, ''), COALESCE(goarch, '')
		FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	found := map[string]bool{}
	for rows.Next() {
		var agentID, agentCode, status, agentOS, agentArch string
		var seen any
		if err := rows.Scan(&agentID, &agentCode, &status, &seen, &agentOS, &agentArch); err != nil {
			return nil, err
		}
		if !want[agentID] {
			continue
		}
		if agentCode != code {
			return nil, errors.New("请选择同一种类的代理")
		}
		if status != "Y" {
			return nil, errors.New("已停用的代理不能升级")
		}
		at, ok := store.TimeValue(seen)
		if !ok || now.Sub(at) > limits.OnlineWindow {
			return nil, errors.New("离线代理不能升级")
		}
		agentOS = strings.ToLower(strings.TrimSpace(agentOS))
		agentArch = strings.ToLower(strings.TrimSpace(agentArch))
		if agentOS == "" || agentArch == "" {
			return nil, fmt.Errorf("代理 %s 尚未上报平台，请等一次心跳后再升级", agentID)
		}
		if agentOS != goos || agentArch != goarch {
			return nil, fmt.Errorf("代理 %s 平台为 %s/%s，与安装包 %s/%s 不一致", agentID, agentOS, agentArch, goos, goarch)
		}
		found[agentID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, id := range order {
		if !found[id] {
			return nil, errors.New("有代理不属于本租户")
		}
		out = append(out, id)
	}
	return out, nil
}

func splitBatch(ids []string, n int) (first, rest []string) {
	if n <= 0 || n >= len(ids) {
		return ids, nil
	}
	first = append([]string{}, ids[:n]...)
	rest = append([]string{}, ids[n:]...)
	return first, rest
}

func normalizePlatform(goos, goarch string) (string, string, error) {
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	switch goos {
	case "linux", "windows", "darwin":
	default:
		return "", "", errors.New("请选择安装包操作系统（linux / windows / darwin）")
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", "", errors.New("请选择安装包架构（amd64 / arm64）")
	}
	return goos, goarch, nil
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
