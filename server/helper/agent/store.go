package agent

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/errcode"
	"fluxguard/server/limits"
	"fluxguard/server/secret"
	"fluxguard/server/store"
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
)

// onlineWindow 内没有心跳则视为离线。
const onlineWindow = limits.OnlineWindow

// AgentView 采集代理列表。令牌摘要不出现在这里。
type AgentView struct {
	AgentID        string     `json:"agentId"`
	AgentName      string     `json:"agentName"`
	AgentCode      string     `json:"agentCode,omitempty"`
	MachineID      string     `json:"machineId,omitempty"`
	HostName       string     `json:"hostName"`
	IPAddress      string     `json:"ipAddress"`
	CommandAddress string     `json:"commandAddress,omitempty"`
	OnlineFlag     string     `json:"onlineFlag"`
	StatusFlag     string     `json:"statusFlag"`
	LastSeenTime   *time.Time `json:"lastSeenTime,omitempty"`
}

// RegisterInput 登记一台采集代理。
type RegisterInput struct {
	AgentName string `json:"agentName"`
	HostName  string `json:"hostName"`
	IPAddress string `json:"ipAddress"`
}

type agent struct {
	AgentID        string
	TenantID       string
	AgentName      string
	AgentCode      string
	MachineID      string
	HostName       string
	IPAddress      string
	CommandAddress string
	TokenHash      string
	StatusFlag     string
	ActiveFlag     string
	LastSeen       *time.Time
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
func (a *Store) ListAgents(tenantID string) []AgentView {
	rows, err := a.db.Query(context.Background(), `
		SELECT a.agentId, a.agentName, COALESCE(a.agentCode, ''), COALESCE(a.machineId, ''),
		       COALESCE(a.hostName, ''), COALESCE(a.ipAddress, ''), COALESCE(h.commandAddress, ''),
		       a.statusFlag, a.lastSeenTime
		FROM FG_AGENT a
		LEFT JOIN FG_AGENT_HOST h ON h.tenantId = a.tenantId AND h.agentId = a.agentId
		WHERE a.tenantId = @tenantId AND a.activeFlag = 'Y'`, dbx.Args{"tenantId": tenantID})
	if err != nil {
		return nil
	}
	defer rows.Close()
	now := time.Now()
	var out []AgentView
	for rows.Next() {
		item, err := scanAgent(rows, now)
		if err != nil {
			return out
		}
		out = append(out, item)
	}
	return out
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
	var taken int
	err = store.Scan(ctx, a.db, `
		SELECT count(*) FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentName = @agentName`,
		dbx.Args{"tenantId": tenantID, "agentName": name}, &taken)
	if err != nil {
		return AgentView{}, "", err
	}
	if taken > 0 {
		return AgentView{}, "", ErrAgentNameTaken
	}
	plain := "agt_" + id.New()
	item := agent{
		AgentID: id.New(), TenantID: tenantID, AgentName: name,
		HostName: strings.TrimSpace(in.HostName), IPAddress: strings.TrimSpace(in.IPAddress),
		TokenHash: secret.HashToken(plain), StatusFlag: "Y", ActiveFlag: "Y",
	}
	now := sess.When()
	_, err = a.db.Exec(ctx, `
		INSERT INTO FG_AGENT (
		  agentId, tenantId, agentName, hostName, ipAddress, agentTokenHash, statusFlag,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (@agentId, @tenantId, @agentName, @hostName, @ipAddress, @agentTokenHash, 'Y', @now, @actor, @now, @actor, 'REG', 1, 'Y')`,
		dbx.Args{
			"agentId": item.AgentID, "tenantId": item.TenantID, "agentName": item.AgentName,
			"hostName": nullIfEmpty(item.HostName), "ipAddress": nullIfEmpty(item.IPAddress), "agentTokenHash": item.TokenHash,
			"now": now, "actor": actor,
		},
	)
	if err != nil {
		return AgentView{}, "", err
	}
	return toAgentView(item, time.Now()), plain, nil
}

// EnsureMachineAgent 按租户、种类和机器编号找到或建出一台代理。
// 上报令牌只证明租户。同一台机器每次上报落在同一行。
func (a *Store) EnsureMachineAgent(tenantID, agentCode, machineID string) (string, error) {
	agentCode = strings.TrimSpace(agentCode)
	machineID = strings.TrimSpace(machineID)
	if !protocol.KnownAgent(agentCode) || machineID == "" || len(machineID) > 64 {
		return "", ErrAgentIdentity
	}
	var agentID, status string
	err := store.Scan(context.Background(), a.db, `
		SELECT agentId, statusFlag FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentCode = @agentCode AND machineId = @machineId`,
		dbx.Args{"tenantId": tenantID, "agentCode": agentCode, "machineId": machineID}, &agentID, &status)
	if err == nil {
		if status != "Y" {
			return "", ErrAgentStopped
		}
		return agentID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	agentID = id.New()
	name := agentCode + "-" + shortMachine(machineID)
	_, err = a.db.Exec(context.Background(), `
		INSERT INTO FG_AGENT (
		  agentId, tenantId, agentName, agentCode, machineId, agentTokenHash, statusFlag,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, @tenantId, @agentName, @agentCode, @machineId, @agentTokenHash, 'Y',
		  'system', 'system', 'INGEST', 1, 'Y'
		)`, dbx.Args{
		"agentId": agentID, "tenantId": tenantID, "agentName": name,
		"agentCode": agentCode, "machineId": machineID,
		"agentTokenHash": secret.HashToken("machine:" + tenantID + ":" + agentCode + ":" + machineID),
	})
	if err != nil {
		return "", err
	}
	return agentID, nil
}

// EnsureReportingAgent 按种类找到或建出共用令牌下的代理行。
// 没有机器编号的上报仍走这里，同一种类先共用一行。
func (a *Store) EnsureReportingAgent(tenantID, agentCode string) (string, error) {
	name := strings.TrimSpace(agentCode)
	if !protocol.KnownAgent(name) {
		return "", ErrAgentIdentity
	}
	var agentID, status string
	err := store.Scan(context.Background(), a.db, `
		SELECT agentId, statusFlag FROM FG_AGENT
		WHERE tenantId = @tenantId AND activeFlag = 'Y' AND agentName = @agentName`,
		dbx.Args{"tenantId": tenantID, "agentName": name}, &agentID, &status)
	if err == nil {
		if status != "Y" {
			return "", ErrAgentStopped
		}
		return agentID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	agentID = id.New()
	_, err = a.db.Exec(context.Background(), `
		INSERT INTO FG_AGENT (
		  agentId, tenantId, agentName, agentTokenHash, statusFlag,
		  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @agentId, @tenantId, @agentName, @agentTokenHash, 'Y',
		  'system', 'system', 'INGEST', 1, 'Y'
		)`, dbx.Args{
		"agentId": agentID, "tenantId": tenantID, "agentName": name,
		"agentTokenHash": secret.HashToken("shared:" + tenantID + ":" + name),
	})
	if err != nil {
		return "", err
	}
	return agentID, nil
}

// TouchAgent 记下代理心跳。
func (a *Store) TouchAgent(tenantID, agentID string) {
	now := store.FormatTime(time.Now().UTC())
	_, _ = a.db.Exec(context.Background(), `
		UPDATE FG_AGENT SET lastSeenTime = @lastSeenTime, editTime = @editTime, editWho = 'system'
		WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"lastSeenTime": now, "editTime": now, "tenantId": tenantID, "agentId": agentID})
}

// AgentByToken 用明文令牌换租户和代理。
func (a *Store) AgentByToken(token string) (string, string, bool) {
	if token == "" {
		return "", "", false
	}
	var tenantID, agentID string
	err := store.Scan(context.Background(), a.db, `
		SELECT tenantId, agentId FROM FG_AGENT
		WHERE agentTokenHash = @agentTokenHash AND activeFlag = 'Y' AND statusFlag = 'Y'`,
		dbx.Args{"agentTokenHash": secret.HashToken(token)}, &tenantID, &agentID)
	if err != nil {
		return "", "", false
	}
	return tenantID, agentID, true
}

type agentRows interface {
	Scan(dest ...any) error
}

// scanAgent 读一行代理，并把最近心跳换算成在线标记。
func scanAgent(rows agentRows, now time.Time) (AgentView, error) {
	var item agent
	var seen any
	if err := rows.Scan(&item.AgentID, &item.AgentName, &item.AgentCode, &item.MachineID, &item.HostName, &item.IPAddress, &item.CommandAddress, &item.StatusFlag, &seen); err != nil {
		return AgentView{}, err
	}
	if t, ok := store.TimeValue(seen); ok {
		item.LastSeen = &t
	}
	item.ActiveFlag = "Y"
	return toAgentView(item, now), nil
}

// toAgentView 两分钟内有心跳且状态为 Y 时记为在线。
func toAgentView(item agent, now time.Time) AgentView {
	online := "N"
	if item.StatusFlag == "Y" && item.LastSeen != nil && now.Sub(*item.LastSeen) <= onlineWindow {
		online = "Y"
	}
	return AgentView{
		AgentID: item.AgentID, AgentName: item.AgentName, AgentCode: item.AgentCode, MachineID: item.MachineID,
		HostName: item.HostName, IPAddress: item.IPAddress, CommandAddress: item.CommandAddress,
		OnlineFlag: online, StatusFlag: item.StatusFlag,
		LastSeenTime: item.LastSeen,
	}
}

func shortMachine(machineID string) string {
	machineID = strings.ReplaceAll(machineID, "-", "")
	if len(machineID) > 8 {
		return machineID[:8]
	}
	return machineID
}

// nullIfEmpty 把空字符串写成 SQL NULL。
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
