package agent

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/store"
	"fluxguard/server/utils/const/errcode"
	"fluxguard/server/utils/const/limits"
	"fluxguard/server/utils/secret"
	"fluxguard/server/web/request"
	dbx "github.com/fluxsce/dbx/db"
)

var (
	// ErrAgentNameEmpty 表示登记时没有名称。
	ErrAgentNameEmpty = errcode.New("agent.name.required", "代理名称不能为空")
	// ErrAgentStopped 表示这台采集已停用，不再接收上报。
	ErrAgentStopped = errcode.New("agent.stopped", "该采集已停用")
	// ErrAgentIdentity 表示机器编号或代理种类不能用来区分一台机器。
	ErrAgentIdentity = errcode.New("agent.identity.invalid", "机器编号或代理种类无效")
	// ErrAgentNameTaken 表示同一租户下名称重复。
	ErrAgentNameTaken = errcode.New("agent.name.taken", "代理名称已存在")
	// ErrAgentIDsRequired 表示删除时没有传入代理编号。
	ErrAgentIDsRequired = errcode.New("agent.ids.required", "请选择要删除的代理")
	// ErrAgentMissing 表示改名或操作的代理不存在。
	ErrAgentMissing = errcode.New("agent.missing", "代理不存在")
)

// onlineWindow 内没有心跳则视为离线。
const onlineWindow = limits.OnlineWindow

// AgentView 采集代理列表。令牌摘要不出现在这里；json 给 API，db 给 Select/Get。
type AgentView struct {
	AgentID        string     `json:"agentId" db:"agentId"`
	AgentName      string     `json:"agentName" db:"agentName"`
	AgentCode      string     `json:"agentCode,omitempty" db:"agentCode"`
	MachineID      string     `json:"machineId,omitempty" db:"machineId"`
	AgentVersion   string     `json:"agentVersion,omitempty" db:"agentVersion"`
	GoOS           string     `json:"goos,omitempty" db:"goos"`
	GoArch         string     `json:"goarch,omitempty" db:"goarch"`
	HostName       string     `json:"hostName" db:"hostName"`
	IPAddress      string     `json:"ipAddress" db:"ipAddress"`
	CommandAddress string     `json:"commandAddress,omitempty" db:"commandAddress"`
	NoteText       string     `json:"noteText,omitempty" db:"noteText,omitempty"`
	OnlineFlag     string     `json:"onlineFlag" db:"-"`
	StatusFlag     string     `json:"statusFlag" db:"statusFlag"`
	LastSeenTime   *time.Time `json:"lastSeenTime,omitempty" db:"lastSeenTime"`
}

// RegisterInput 登记一台采集代理。
type RegisterInput struct {
	AgentName string `json:"agentName"`
	HostName  string `json:"hostName"`
	IPAddress string `json:"ipAddress"`
}

type agent struct {
	AgentID        string     `db:"agentId,pk"`
	TenantID       string     `db:"tenantId,pk"`
	AgentName      string     `db:"agentName"`
	AgentCode      string     `db:"agentCode,omitempty"`
	MachineID      string     `db:"machineId,omitempty"`
	AgentVersion   string     `db:"agentVersion,omitempty"`
	GoOS           string     `db:"goos,omitempty"`
	GoArch         string     `db:"goarch,omitempty"`
	HostName       string     `db:"hostName,omitempty"`
	IPAddress      string     `db:"ipAddress,omitempty"`
	CommandAddress string     `db:"commandAddress,omitempty"`
	TokenHash      string     `db:"agentTokenHash"`
	StatusFlag     string     `db:"statusFlag"`
	ActiveFlag     string     `db:"activeFlag"`
	LastSeen       *time.Time `db:"lastSeenTime,omitempty"`
	NoteText       string     `db:"noteText,omitempty"`
	AddTime        string     `db:"addTime,noupdate,omitempty"`
	AddWho         string     `db:"addWho,noupdate,omitempty"`
	EditTime       string     `db:"editTime,omitempty"`
	EditWho        string     `db:"editWho,omitempty"`
	OprSeqFlag     string     `db:"oprSeqFlag,omitempty"`
	CurrentVersion int        `db:"currentVersion,omitempty"`
}

// agentIDStatus 是按身份查找时的主键与停用标记。
type agentIDStatus struct {
	AgentID    string `db:"agentId"`
	StatusFlag string `db:"statusFlag"`
}

// agentTokenRow 是令牌换租户与代理编号。
type agentTokenRow struct {
	TenantID string `db:"tenantId"`
	AgentID  string `db:"agentId"`
}

// Store 读写 FG_AGENT。上报和命令通道用它认出代理，管理页用它登记。
type Store struct {
	db *dbx.DB
}

// Open 使用已经灌好脚本的库。
func Open(conn *dbx.DB) *Store {
	return &Store{db: conn}
}

// ListAgents 返回采集代理。在线状态按最近心跳推导。
// 列表只读 FG_AGENT；主机名、地址、命令端口由各类代理上报时写回本表。
func (a *Store) ListAgents(tenantID string) []AgentView {
	cols, err := dbx.Columns(&AgentView{})
	if err != nil {
		return nil
	}
	var rows []AgentView
	if err := a.db.Select(context.Background(), &rows, `
		SELECT `+cols+`
		FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y'`, dbx.Args{"tenantId": tenantID}); err != nil {
		return nil
	}
	now := time.Now()
	for i := range rows {
		rows[i].OnlineFlag = onlineOf(rows[i].StatusFlag, rows[i].LastSeenTime, now)
	}
	if rows == nil {
		rows = []AgentView{}
	}
	return rows
}

// RegisterAgent 登记代理并返回只显示一次的明文令牌。
// 租户、编辑人和时间取自 ctx 上的登录会话。
func (a *Store) RegisterAgent(ctx context.Context, in RegisterInput) (AgentView, string, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return AgentView{}, "", err
	}
	tenantID, actor := sess.TenantID, sess.UserID
	name := strings.TrimSpace(in.AgentName)
	if name == "" {
		return AgentView{}, "", ErrAgentNameEmpty
	}
	taken, err := a.db.Count(ctx, `
		SELECT count(*) FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentName = @agentName`,
		dbx.Args{"tenantId": tenantID, "agentName": name})
	if err != nil {
		return AgentView{}, "", err
	}
	if taken > 0 {
		return AgentView{}, "", ErrAgentNameTaken
	}
	plain := "agt_" + id.New()
	now := sess.When()
	item := agent{
		AgentID: id.New(), TenantID: tenantID, AgentName: name,
		HostName: strings.TrimSpace(in.HostName), IPAddress: strings.TrimSpace(in.IPAddress),
		TokenHash: secret.HashToken(plain), StatusFlag: "Y", ActiveFlag: "Y",
		AddTime: now, AddWho: actor, EditTime: now, EditWho: actor,
		OprSeqFlag: "REG", CurrentVersion: 1,
	}
	if err := a.db.Insert(ctx, "FG_AGENT", &item); err != nil {
		return AgentView{}, "", err
	}
	return toAgentView(item, time.Now()), plain, nil
}

// EnsureMachineAgent 按租户、种类和机器编号找到或建出一台代理。
// 上报令牌只证明租户。同一台机器上不同种类（host / appside / go）各占一行；
// 同种类靠 machineId 区分（Go/Java 实例编号里已带进程名）。显示名先用系统底名，可在控制台改。
func (a *Store) EnsureMachineAgent(tenantID, agentCode, machineID string) (string, error) {
	agentCode = strings.TrimSpace(agentCode)
	machineID = strings.TrimSpace(machineID)
	if !protocol.KnownAgent(agentCode) || machineID == "" || len(machineID) > 64 {
		return "", ErrAgentIdentity
	}
	ctx := context.Background()
	var found agentIDStatus
	err := a.db.Get(ctx, &found, `
		SELECT agentId, statusFlag FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentCode = @agentCode AND machineId = @machineId`,
		dbx.Args{"tenantId": tenantID, "agentCode": agentCode, "machineId": machineID})
	if err == nil {
		if found.StatusFlag != "Y" {
			return "", ErrAgentStopped
		}
		return found.AgentID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	row := agent{
		AgentID: id.New(), TenantID: tenantID, AgentName: agentCode + "-" + shortMachine(machineID),
		AgentCode: agentCode, MachineID: machineID,
		TokenHash: secret.HashToken("machine:" + tenantID + ":" + agentCode + ":" + machineID),
		StatusFlag: "Y", ActiveFlag: "Y", AddWho: "system", EditWho: "system",
		OprSeqFlag: "INGEST", CurrentVersion: 1,
	}
	if err := a.db.Insert(ctx, "FG_AGENT", &row); err != nil {
		return "", err
	}
	return row.AgentID, nil
}

// RenameAgent 改显示名与备注。身份仍按 agentId / machineId，上报不会冲掉这里改过的名字。
func (a *Store) RenameAgent(ctx context.Context, agentID, agentName, noteText string) (AgentView, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return AgentView{}, err
	}
	agentID = strings.TrimSpace(agentID)
	name := strings.TrimSpace(agentName)
	if agentID == "" {
		return AgentView{}, ErrAgentIDsRequired
	}
	if name == "" {
		return AgentView{}, ErrAgentNameEmpty
	}
	name = clipName(name, 100)
	note := clipName(strings.TrimSpace(noteText), 500)
	taken, err := a.db.Count(ctx, `
		SELECT count(*) FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentName = @agentName AND agentId <> @agentId`,
		dbx.Args{"tenantId": sess.TenantID, "agentName": name, "agentId": agentID})
	if err != nil {
		return AgentView{}, err
	}
	if taken > 0 {
		return AgentView{}, ErrAgentNameTaken
	}
	now := sess.When()
	res, err := a.db.Exec(ctx, `
		UPDATE FG_AGENT
		SET agentName = @agentName, noteText = @noteText, editTime = @editTime, editWho = @editWho,
		    currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{
			"agentName": name, "noteText": note, "editTime": now, "editWho": sess.UserID,
			"tenantId": sess.TenantID, "agentId": agentID,
		})
	if err != nil {
		return AgentView{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return AgentView{}, err
	}
	if n != 1 {
		return AgentView{}, ErrAgentMissing
	}
	view, err := a.agentView(ctx, sess.TenantID, agentID)
	if err != nil {
		return AgentView{}, err
	}
	return view, nil
}

// agentView 读一台代理的当前展示行。
func (a *Store) agentView(ctx context.Context, tenantID, agentID string) (AgentView, error) {
	cols, err := dbx.Columns(&AgentView{})
	if err != nil {
		return AgentView{}, err
	}
	var view AgentView
	err = a.db.Get(ctx, &view, `
		SELECT `+cols+`
		FROM FG_AGENT
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID})
	if errors.Is(err, sql.ErrNoRows) {
		return AgentView{}, ErrAgentMissing
	}
	if err != nil {
		return AgentView{}, err
	}
	view.OnlineFlag = onlineOf(view.StatusFlag, view.LastSeenTime, time.Now())
	return view, nil
}

// EnsureReportingAgent 按种类找到或建出共用令牌下的代理行。
// 没有机器编号的上报仍走这里，同一种类先共用一行。
func (a *Store) EnsureReportingAgent(tenantID, agentCode string) (string, error) {
	name := strings.TrimSpace(agentCode)
	if !protocol.KnownAgent(name) {
		return "", ErrAgentIdentity
	}
	ctx := context.Background()
	var found agentIDStatus
	err := a.db.Get(ctx, &found, `
		SELECT agentId, statusFlag FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentName = @agentName`,
		dbx.Args{"tenantId": tenantID, "agentName": name})
	if err == nil {
		if found.StatusFlag != "Y" {
			return "", ErrAgentStopped
		}
		return found.AgentID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	row := agent{
		AgentID: id.New(), TenantID: tenantID, AgentName: name,
		TokenHash: secret.HashToken("shared:" + tenantID + ":" + name),
		StatusFlag: "Y", ActiveFlag: "Y", AddWho: "system", EditWho: "system",
		OprSeqFlag: "INGEST", CurrentVersion: 1,
	}
	if err := a.db.Insert(ctx, "FG_AGENT", &row); err != nil {
		return "", err
	}
	return row.AgentID, nil
}

// TouchInfo 是心跳时顺带更新的产品版本与平台。
type TouchInfo struct {
	Version     string
	GoOS        string
	GoArch      string
	Description string // 非空时覆盖 FG_AGENT.noteText
}

// TouchAgent 记下代理心跳，并在有值时更新产品版本、平台与描述。
func (a *Store) TouchAgent(tenantID, agentID string, info TouchInfo) {
	now := store.FormatTime(time.Now().UTC())
	version := strings.TrimSpace(info.Version)
	goos := strings.ToLower(strings.TrimSpace(info.GoOS))
	goarch := strings.ToLower(strings.TrimSpace(info.GoArch))
	note := clipName(strings.TrimSpace(info.Description), 500)
	if len(version) > 64 {
		version = version[:64]
	}
	if len(goos) > 32 {
		goos = goos[:32]
	}
	if len(goarch) > 32 {
		goarch = goarch[:32]
	}
	if version == "" && goos == "" && goarch == "" && note == "" {
		_, _ = a.db.Exec(context.Background(), `
			UPDATE FG_AGENT SET lastSeenTime = @lastSeenTime, editTime = @editTime, editWho = 'system'
			WHERE tenantId = @tenantId AND agentId = @agentId`,
			dbx.Args{"lastSeenTime": now, "editTime": now, "tenantId": tenantID, "agentId": agentID})
		return
	}
	_, _ = a.db.Exec(context.Background(), `
		UPDATE FG_AGENT
		SET lastSeenTime = @lastSeenTime,
		    agentVersion = CASE WHEN @agentVersion = '' THEN agentVersion ELSE @agentVersion END,
		    goos = CASE WHEN @goos = '' THEN goos ELSE @goos END,
		    goarch = CASE WHEN @goarch = '' THEN goarch ELSE @goarch END,
		    noteText = CASE WHEN @noteText = '' THEN noteText ELSE @noteText END,
		    editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
		WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{
			"lastSeenTime": now, "agentVersion": version, "goos": goos, "goarch": goarch,
			"noteText": note, "editTime": now, "tenantId": tenantID, "agentId": agentID,
		})
}

// agentDeleteTables 删除代理时一并清掉的关联表（先子后主）。历史样本表留给归档任务按保留期清理。
var agentDeleteTables = []string{
	"FG_SHELL_AUDIT",
	"FG_SHELL",
	"FG_COMMAND",
	"FG_AGENT_RELEASE_ITEM",
	"FG_APP_RELEASE_ITEM",
	"FG_APP_INST",
	"FG_HOST_INCIDENT",
	"FG_AGENT_HOST",
	"FG_AGENT_JAVA",
	"FG_AGENT_GO",
	"FG_AGENT",
}

// DeleteAgents 物理删除本租户下的代理及身份、终端、命令等关联行。
// 再次上报同机器编号时会新建一行。返回实际删掉的代理行数。
func (a *Store) DeleteAgents(ctx context.Context, agentIDs []string) (int, error) {
	sess, err := request.Require(ctx)
	if err != nil {
		return 0, err
	}
	ids := uniqueIDs(agentIDs)
	if len(ids) == 0 {
		return 0, ErrAgentIDsRequired
	}
	n := 0
	err = a.db.Tx(ctx, func(tx *dbx.DB) error {
		for _, agentID := range ids {
			args := dbx.Args{"tenantId": sess.TenantID, "agentId": agentID}
			for _, table := range agentDeleteTables {
				result, err := tx.Exec(ctx,
					"DELETE FROM "+table+" WHERE tenantId = @tenantId AND agentId = @agentId", args)
				if err != nil {
					return err
				}
				if table == "FG_AGENT" {
					affected, _ := result.RowsAffected()
					n += int(affected)
				}
			}
		}
		return nil
	})
	return n, err
}

// AgentByToken 用明文令牌换租户和代理。
func (a *Store) AgentByToken(token string) (string, string, bool) {
	if token == "" {
		return "", "", false
	}
	var row agentTokenRow
	err := a.db.Get(context.Background(), &row, `
		SELECT tenantId, agentId FROM FG_AGENT
		WHERE agentTokenHash = @agentTokenHash AND activeFlag = 'Y' AND statusFlag = 'Y'`,
		dbx.Args{"agentTokenHash": secret.HashToken(token)})
	if err != nil {
		return "", "", false
	}
	return row.TenantID, row.AgentID, true
}

// toAgentView 登记后立刻返回展示行。
func toAgentView(item agent, now time.Time) AgentView {
	return AgentView{
		AgentID: item.AgentID, AgentName: item.AgentName, AgentCode: item.AgentCode, MachineID: item.MachineID,
		AgentVersion: item.AgentVersion, GoOS: item.GoOS, GoArch: item.GoArch,
		HostName: item.HostName, IPAddress: item.IPAddress, CommandAddress: item.CommandAddress,
		NoteText: item.NoteText,
		OnlineFlag: onlineOf(item.StatusFlag, item.LastSeen, now), StatusFlag: item.StatusFlag,
		LastSeenTime: item.LastSeen,
	}
}

// onlineOf 两分钟内有心跳且状态为 Y 时记为在线。
func onlineOf(statusFlag string, lastSeen *time.Time, now time.Time) string {
	if statusFlag == "Y" && lastSeen != nil && now.Sub(*lastSeen) <= onlineWindow {
		return "Y"
	}
	return "N"
}

func shortMachine(machineID string) string {
	machineID = strings.ReplaceAll(machineID, "-", "")
	if len(machineID) > 8 {
		return machineID[:8]
	}
	return machineID
}

// uniqueIDs 去掉空白与重复的代理编号。
func uniqueIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
