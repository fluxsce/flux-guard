// Package upgrade 从中央 agents/ 包装下发代理自更新，并向选中的代理分批下发命令。
package upgrade

import (
	"context"
	"database/sql"
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

const (
	tableRelease = "FG_AGENT_RELEASE"
	tableItem    = "FG_AGENT_RELEASE_ITEM"
	oprSeq       = "AGT"
)

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
	row := fgmodels.AgentRelease{
		ReleaseID: releaseID, TenantID: tenantID, AgentCode: code, VersionText: version,
		SHA256: sum, FileName: fileName, GoOS: goos, GoArch: goarch, ExtProperty: string(extBody),
		AddTime: now, AddWho: user, EditTime: now, EditWho: user,
		OprSeqFlag: oprSeq, CurrentVersion: 1, ActiveFlag: "Y",
		PendingCount: len(pending), BatchSize: batchSize,
	}
	if err := db.Insert(ctx, tableRelease, &row); err != nil {
		return fgmodels.AgentRelease{}, err
	}
	if err := dispatchBatch(ctx, db, svc, tenantID, user, releaseID, code, version, sum, fileName, goos, goarch, first, &row); err != nil {
		return row, err
	}
	return row, nil
}

// Continue 把同一发布里尚未下发的代理按批次继续下发。
func Continue(ctx context.Context, db *dbx.DB, tenantID, user, releaseID string) (fgmodels.AgentRelease, error) {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return fgmodels.AgentRelease{}, errors.New("缺少发布")
	}
	row, err := loadRelease(ctx, db, tenantID, releaseID)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	ext := applyReleaseMeta(&row)
	if len(ext.PendingAgentIDs) == 0 {
		return Progress(ctx, db, tenantID, releaseID)
	}
	batchSize := ext.BatchSize
	if batchSize <= 0 || batchSize > limits.AgentUpgradeBatchMax {
		batchSize = limits.AgentUpgradeBatchMax
	}
	// 继续前再校验仍在线且平台匹配。
	pending, err := selected(ctx, db, tenantID, row.AgentCode, row.GoOS, row.GoArch, ext.PendingAgentIDs)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	first, rest := splitBatch(pending, batchSize)
	ext.PendingAgentIDs = rest
	ext.BatchSize = batchSize
	extBody, _ := json.Marshal(ext)
	now := store.FormatTime(time.Now().UTC())
	row.ExtProperty = string(extBody)
	row.EditTime = now
	row.EditWho = user
	row.CurrentVersion++
	pk, err := db.PK(&row)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	if err := db.Update(ctx, tableRelease, &row, pk); err != nil {
		return fgmodels.AgentRelease{}, err
	}
	svc := command.Current()
	if svc == nil {
		return fgmodels.AgentRelease{}, errors.New("命令通道未启用")
	}
	out := fgmodels.AgentRelease{
		ReleaseID: releaseID, AgentCode: row.AgentCode, VersionText: row.VersionText, FileName: row.FileName,
		GoOS: row.GoOS, GoArch: row.GoArch, PendingCount: len(rest), BatchSize: batchSize,
	}
	if err := dispatchBatch(ctx, db, svc, tenantID, user, releaseID, row.AgentCode, row.VersionText, row.SHA256, row.FileName, row.GoOS, row.GoArch, first, &out); err != nil {
		return out, err
	}
	full, err := Progress(ctx, db, tenantID, releaseID)
	if err != nil {
		return out, nil
	}
	return full, nil
}

// Progress 读一次发布里各条命令的当前状态，并对照代理上报版本。
func Progress(ctx context.Context, db *dbx.DB, tenantID, releaseID string) (fgmodels.AgentRelease, error) {
	out, err := loadRelease(ctx, db, tenantID, releaseID)
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	applyReleaseMeta(&out)
	var items []fgmodels.AgentReleaseItem
	if err := db.Select(ctx, &items, `
		SELECT item.commandId AS commandId, item.agentId AS agentId, a.agentName AS agentName,
		       cmd.statusFlag AS statusFlag, cmd.resultText AS resultText, a.agentVersion AS agentVersion
		FROM FG_AGENT_RELEASE_ITEM item
		JOIN FG_COMMAND cmd ON cmd.tenantId = item.tenantId AND cmd.commandId = item.commandId
		LEFT JOIN FG_AGENT a ON a.tenantId = item.tenantId AND a.agentId = item.agentId
		WHERE item.tenantId = @tenantId AND item.releaseId = @releaseId AND item.activeFlag = 'Y'
		ORDER BY item.addTime, item.commandId`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID}); err != nil {
		return fgmodels.AgentRelease{}, err
	}
	open := out.PendingCount > 0
	out.Items = make([]fgmodels.AgentReleaseItem, 0, len(items))
	for i := range items {
		finalizeItem(&items[i], out.VersionText)
		if items[i].Status == "waiting" || items[i].Status == "dialing" || items[i].Status == "running" {
			open = true
		}
		out.Items = append(out.Items, items[i])
	}
	if open {
		out.OpenFlag = "Y"
	} else {
		out.OpenFlag = "N"
	}
	return out, nil
}

func loadRelease(ctx context.Context, db *dbx.DB, tenantID, releaseID string) (fgmodels.AgentRelease, error) {
	cols, err := dbx.Columns(&fgmodels.AgentRelease{})
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	var row fgmodels.AgentRelease
	err = db.Get(ctx, &row, `
		SELECT `+cols+`
		FROM FG_AGENT_RELEASE
		WHERE tenantId = @tenantId AND releaseId = @releaseId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "releaseId": releaseID})
	if errors.Is(err, sql.ErrNoRows) {
		return fgmodels.AgentRelease{}, errors.New("找不到这次升级")
	}
	if err != nil {
		return fgmodels.AgentRelease{}, err
	}
	row.TenantID = tenantID
	return row, nil
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
		item := fgmodels.AgentReleaseItem{
			CommandID: view.CommandID, TenantID: tenantID, ReleaseID: releaseID, AgentID: agentID,
			AddTime: now, AddWho: user, EditTime: now, EditWho: user,
			OprSeqFlag: oprSeq, CurrentVersion: 1, ActiveFlag: "Y",
		}
		if err := db.Insert(ctx, tableItem, &item); err != nil {
			return err
		}
		item.Status = view.Status
		item.Message = view.Message
		out.Items = append(out.Items, item)
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
	var rows []agentPickRow
	if err := db.Select(ctx, &rows, `
		SELECT agentId, agentCode, statusFlag, lastSeenTime, goos, goarch
		FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID}); err != nil {
		return nil, err
	}
	now := time.Now()
	found := map[string]bool{}
	for _, row := range rows {
		if !want[row.AgentID] {
			continue
		}
		if row.AgentCode != code {
			return nil, errors.New("请选择同一种类的代理")
		}
		if row.StatusFlag != "Y" {
			return nil, errors.New("已停用的代理不能升级")
		}
		at, ok := parseSeen(row.LastSeenTime)
		if !ok || now.Sub(at) > limits.OnlineWindow {
			return nil, errors.New("离线代理不能升级")
		}
		agentOS := strings.ToLower(strings.TrimSpace(row.GoOS))
		agentArch := strings.ToLower(strings.TrimSpace(row.GoArch))
		if agentOS == "" || agentArch == "" {
			return nil, fmt.Errorf("代理 %s 尚未上报平台，请等一次心跳后再升级", row.AgentID)
		}
		if agentOS != goos || agentArch != goarch {
			return nil, fmt.Errorf("代理 %s 平台为 %s/%s，与安装包 %s/%s 不一致", row.AgentID, agentOS, agentArch, goos, goarch)
		}
		found[row.AgentID] = true
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
