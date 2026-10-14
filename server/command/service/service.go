// Package service 读写 FG_COMMAND：下发、拨号、认领与回执落库。
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"fluxguard/common/id"
	"fluxguard/protocol"
	"fluxguard/server/command/models"
	"fluxguard/server/setting"
	"fluxguard/server/store"
	"fluxguard/server/utils/const/limits"
	dbx "github.com/fluxsce/dbx/db"
)

// Service 读写 FG_COMMAND，并在能拨通时代理执行。
type Service struct {
	db  *dbx.DB
	hub hub
}

var current *Service

// Current 返回本进程命令服务。尚未 Open 时为 nil。
func Current() *Service {
	return current
}

// Open 打开命令服务。下发按钮在 FG_AUTH_RESOURCE.sql，启动时不再补行。
// 同一条库连接再次打开时返回已有实例，下发和长轮询共用唤醒。
func Open(db *dbx.DB) (*Service, error) {
	if db == nil {
		return nil, errors.New("命令服务缺少库连接")
	}
	if current != nil && current.db == db {
		return current, nil
	}
	current = &Service{db: db}
	return current, nil
}

// Dispatch 写入一条命令。能拨通则同步带回结果，否则留下待取并唤醒长轮询。
func (s *Service) Dispatch(ctx context.Context, tenantID, user string, in models.DispatchInput) (models.View, error) {
	if !protocol.KnownAction(in.Action) {
		return models.View{}, models.ErrUnknownAction
	}
	if in.Action == protocol.ActionIntervalSet && (in.IntervalSec < protocol.IntervalMin || in.IntervalSec > protocol.IntervalMax) {
		return models.View{}, models.ErrBadInterval
	}
	if in.Action == protocol.ActionAppRollback {
		if strings.TrimSpace(in.ReleaseID) == "" || strings.TrimSpace(in.Version) == "" ||
			strings.TrimSpace(in.Instance) == "" || strings.TrimSpace(in.TargetPath) == "" || strings.TrimSpace(in.BackupPath) == "" {
			return models.View{}, models.ErrBadRelease
		}
	} else if in.Action == protocol.ActionAppFilePush {
		if strings.TrimSpace(in.Instance) == "" || strings.TrimSpace(in.TargetPath) == "" {
			return models.View{}, models.ErrBadRelease
		}
	} else if in.Action == protocol.ActionAppFileList {
		if strings.TrimSpace(in.Instance) == "" {
			return models.View{}, models.ErrBadRelease
		}
	} else if (protocol.AppAction(in.Action) || protocol.AgentAction(in.Action)) &&
		(strings.TrimSpace(in.ReleaseID) == "" || strings.TrimSpace(in.SHA256) == "" || strings.TrimSpace(in.Version) == "") {
		return models.View{}, models.ErrBadRelease
	}
	if in.Action == protocol.ActionAppReplace && (strings.TrimSpace(in.AppName) == "" || strings.TrimSpace(in.Kind) == "") {
		return models.View{}, models.ErrBadRelease
	}
	agentID := strings.TrimSpace(in.AgentID)
	var code, status string
	err := store.Scan(ctx, s.db, `
		SELECT COALESCE(agentCode, ''), statusFlag FROM FG_AGENT
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID}, &code, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return models.View{}, models.ErrNotFound
	}
	if err != nil {
		return models.View{}, err
	}
	if status != "Y" {
		return models.View{}, models.ErrStopped
	}
	if protocol.HostAction(in.Action) && code != "" && code != protocol.AgentHost {
		return models.View{}, models.ErrNotHost
	}
	if protocol.AppAction(in.Action) && code != protocol.AgentAppside {
		return models.View{}, models.ErrNotAppside
	}
	if protocol.AgentAction(in.Action) {
		if !protocol.UpdatableAgent(code) {
			return models.View{}, models.ErrNotUpdatable
		}
		if kind := strings.TrimSpace(in.Kind); kind != "" && kind != code {
			return models.View{}, models.ErrNotUpdatable
		}
	}
	// 自更新与出站推文件超过拨号时限或专供隔离网；只走长轮询。
	primary, list, addr := s.targetAddress(ctx, tenantID, agentID, in.Action)
	token := setting.IngestPlain(tenantID)
	canDial := !protocol.AgentAction(in.Action) && !protocol.FilePushAction(in.Action) &&
		addr != "" && token != "" && allowedTarget(addr, primary, list)
	commandID := id.New()
	statusFlag := models.StatusWaiting
	if canDial {
		statusFlag = models.StatusDialing
	}
	var interval any
	if in.Action == protocol.ActionIntervalSet {
		interval = in.IntervalSec
	}
	now := store.FormatTime(time.Now().UTC())
	if _, err := s.db.Exec(ctx, `
		INSERT INTO FG_COMMAND (
		  commandId, tenantId, agentId, actionName, intervalSec, statusFlag, extProperty,
		  addTime, addWho, editTime, editWho, oprSeqFlag, currentVersion, activeFlag
		) VALUES (
		  @commandId, @tenantId, @agentId, @actionName, @intervalSec, @statusFlag, @extProperty,
		  @now, @user, @now, @user, 'CMD', 1, 'Y'
		)`, dbx.Args{
		"commandId": commandID, "tenantId": tenantID, "agentId": agentID, "actionName": in.Action,
		"intervalSec": interval, "statusFlag": statusFlag, "extProperty": commandExt(in),
		"now": now, "user": user,
	}); err != nil {
		return models.View{}, err
	}
	if canDial {
		if view, pushed := s.push(ctx, tenantID, commandID, addr, token, in); pushed {
			return view, nil
		}
		s.markWaiting(ctx, commandID)
		s.hub.wake(agentID)
	} else {
		s.hub.wake(agentID)
	}
	return s.Get(ctx, tenantID, commandID)
}

// Get 读取一条命令。
func (s *Service) Get(ctx context.Context, tenantID, commandID string) (models.View, error) {
	var view models.View
	var flag string
	err := store.Scan(ctx, s.db, `
		SELECT commandId, agentId, actionName, statusFlag, COALESCE(deliverMode, ''), COALESCE(resultText, '')
		FROM FG_COMMAND
		WHERE tenantId = @tenantId AND commandId = @commandId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "commandId": commandID},
		&view.CommandID, &view.AgentID, &view.Action, &flag, &view.DeliverMode, &view.Message)
	if errors.Is(err, sql.ErrNoRows) {
		return models.View{}, models.ErrCommandGone
	}
	if err != nil {
		return models.View{}, err
	}
	view.Status = models.StatusName(flag)
	return view, nil
}

// ListByAgent 按代理列出最近的命令执行记录，新的在前。
func (s *Service) ListByAgent(ctx context.Context, tenantID, agentID string, limit int) ([]models.View, error) {
	agentID = strings.TrimSpace(agentID)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT commandId, agentId, actionName, statusFlag, COALESCE(deliverMode, ''), COALESCE(resultText, ''),
		       COALESCE(intervalSec, 0), addTime, addWho
		FROM FG_COMMAND
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		ORDER BY addTime DESC, commandId DESC
		LIMIT %d`, limit),
		dbx.Args{"tenantId": tenantID, "agentId": agentID})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.View, 0)
	for rows.Next() {
		var view models.View
		var flag string
		if err := rows.Scan(
			&view.CommandID, &view.AgentID, &view.Action, &flag, &view.DeliverMode, &view.Message,
			&view.IntervalSec, &view.AddTime, &view.AddWho,
		); err != nil {
			return nil, err
		}
		view.Status = models.StatusName(flag)
		out = append(out, view)
	}
	return out, rows.Err()
}

// OwnsRelease 判断这件制品是否发给了这台应用侧代理。
func (s *Service) OwnsRelease(ctx context.Context, tenantID, agentID, releaseID string) bool {
	if s == nil || strings.TrimSpace(releaseID) == "" {
		return false
	}
	var count int
	err := store.Scan(ctx, s.db, `
		SELECT count(*) FROM FG_COMMAND
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND extProperty LIKE @marker`,
		dbx.Args{
			"tenantId": tenantID, "agentId": agentID,
			"marker": "%\"releaseId\":\"" + releaseID + "\"%",
		}, &count)
	return err == nil && count > 0
}

// Wait 挂起直到该代理有待取命令或超时。
func (s *Service) Wait(agentID string, ctx context.Context, hold time.Duration) {
	s.hub.wait(agentID, ctx, hold)
}

func commandCall(commandID string, in models.DispatchInput) protocol.CommandCall {
	return protocol.CommandCall{
		CommandID: commandID, Action: in.Action, IntervalSec: in.IntervalSec,
		ReleaseID: in.ReleaseID, AppName: in.AppName, Kind: in.Kind,
		Version: in.Version, SHA256: in.SHA256, FileName: in.FileName, Instance: in.Instance,
		GoOS: in.GoOS, GoArch: in.GoArch, TargetPath: in.TargetPath, BackupPath: in.BackupPath,
	}
}

func commandExt(in models.DispatchInput) any {
	if !protocol.AppAction(in.Action) && !protocol.AgentAction(in.Action) {
		return nil
	}
	body, err := json.Marshal(commandCall("", in))
	if err != nil {
		return nil
	}
	return string(body)
}

func callFromExt(commandID, action, raw string, interval int) protocol.CommandCall {
	call := protocol.CommandCall{CommandID: commandID, Action: action, IntervalSec: interval}
	if raw == "" {
		return call
	}
	_ = json.Unmarshal([]byte(raw), &call)
	call.CommandID = commandID
	call.Action = action
	call.IntervalSec = interval
	return call
}

func (s *Service) targetAddress(ctx context.Context, tenantID, agentID, action string) (primary, list, addr string) {
	if protocol.AppAction(action) {
		return s.appsideAddress(ctx, tenantID, agentID)
	}
	return s.hostAddress(ctx, tenantID, agentID)
}

func (s *Service) appsideAddress(ctx context.Context, tenantID, agentID string) (primary, list, addr string) {
	err := store.Scan(ctx, s.db, `
		SELECT COALESCE(ipAddress, ''), COALESCE(commandAddress, '')
		FROM FG_APP_INST
		WHERE tenantId = @tenantId AND agentId = @agentId AND activeFlag = 'Y'
		  AND commandAddress IS NOT NULL AND commandAddress <> ''`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID}, &primary, &addr)
	if err != nil {
		return "", "", ""
	}
	return strings.TrimSpace(primary), strings.TrimSpace(primary), strings.TrimSpace(addr)
}

func (s *Service) hostAddress(ctx context.Context, tenantID, agentID string) (primary, list, addr string) {
	err := store.Scan(ctx, s.db, `
		SELECT COALESCE(ipAddress, ''), COALESCE(ipList, ''), COALESCE(commandAddress, '')
		FROM FG_AGENT_HOST WHERE tenantId = @tenantId AND agentId = @agentId`,
		dbx.Args{"tenantId": tenantID, "agentId": agentID}, &primary, &list, &addr)
	if err != nil {
		return "", "", ""
	}
	return strings.TrimSpace(primary), strings.TrimSpace(list), strings.TrimSpace(addr)
}

func (s *Service) push(ctx context.Context, tenantID, commandID, addr, key string, in models.DispatchInput) (models.View, bool) {
	claimed, err := s.claim(ctx, commandID, models.StatusDialing, models.DeliverPush)
	if err != nil || !claimed {
		return models.View{}, false
	}
	endpoint, _, err := dialEndpoint(addr)
	if err != nil {
		s.release(ctx, commandID)
		return models.View{}, false
	}
	reply, err := postExec(ctx, endpoint, key, commandCall(commandID, in))
	if err != nil {
		s.release(ctx, commandID)
		return models.View{}, false
	}
	flag := models.StatusDone
	if !reply.OK {
		flag = models.StatusFailed
	}
	if err := s.finish(ctx, commandID, flag, reply.Message, reply.Steps); err != nil {
		return models.View{}, false
	}
	view, err := s.Get(ctx, tenantID, commandID)
	if err != nil {
		return models.View{}, false
	}
	return view, true
}

func (s *Service) claim(ctx context.Context, commandID, from, mode string) (bool, error) {
	res, err := s.db.Exec(ctx, `
		UPDATE FG_COMMAND
		SET statusFlag = 'R', deliverMode = @mode, editTime = @editTime, editWho = 'system',
		    currentVersion = currentVersion + 1
		WHERE commandId = @commandId AND statusFlag = @from AND activeFlag = 'Y'`,
		dbx.Args{
			"mode": mode, "editTime": store.FormatTime(time.Now().UTC()),
			"commandId": commandID, "from": from,
		})
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Service) release(ctx context.Context, commandID string) {
	_, _ = s.db.Exec(ctx, `
		UPDATE FG_COMMAND
		SET statusFlag = 'W', deliverMode = NULL, editTime = @editTime, editWho = 'system',
		    currentVersion = currentVersion + 1
		WHERE commandId = @commandId AND statusFlag = 'R' AND activeFlag = 'Y'`,
		dbx.Args{"editTime": store.FormatTime(time.Now().UTC()), "commandId": commandID})
}

func (s *Service) markWaiting(ctx context.Context, commandID string) {
	_, _ = s.db.Exec(ctx, `
		UPDATE FG_COMMAND
		SET statusFlag = 'W', editTime = @editTime, editWho = 'system',
		    currentVersion = currentVersion + 1
		WHERE commandId = @commandId AND statusFlag = 'D' AND activeFlag = 'Y'`,
		dbx.Args{"editTime": store.FormatTime(time.Now().UTC()), "commandId": commandID})
}

func (s *Service) finish(ctx context.Context, commandID, flag, message string, steps []protocol.CommandStep) error {
	note, err := encodeSteps(steps)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(ctx, `
		UPDATE FG_COMMAND
		SET statusFlag = @statusFlag, resultText = @resultText, noteText = @noteText,
		    editTime = @editTime, editWho = 'system', currentVersion = currentVersion + 1
		WHERE commandId = @commandId AND statusFlag = 'R' AND activeFlag = 'Y'`,
		dbx.Args{
			"statusFlag": flag, "resultText": clip(message, 200), "noteText": note,
			"editTime": store.FormatTime(time.Now().UTC()), "commandId": commandID,
		})
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return models.ErrCommandGone
	}
	return nil
}

// encodeSteps 把步骤写成 noteText。空步骤清掉旧值。
func encodeSteps(steps []protocol.CommandStep) (any, error) {
	if len(steps) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return nil, err
	}
	text := string(raw)
	if len(text) > limits.CommandResultNoteMax {
		text = text[:limits.CommandResultNoteMax]
	}
	return text, nil
}

func clip(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	runes := []rune(text)
	return string(runes[:n])
}
