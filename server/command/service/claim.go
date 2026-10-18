package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"fluxguard/protocol"
	"fluxguard/server/command/models"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// ClaimNext 认领该代理下一条待取命令。
func (s *Service) ClaimNext(ctx context.Context, tenantID, agentID string) (protocol.CommandCall, bool, error) {
	for range 3 {
		var commandID, action, raw string
		var interval int
		query, args, err := store.PageQuery(s.db, `
			SELECT commandId, actionName, COALESCE(intervalSec, 0), COALESCE(extProperty, '')
			FROM FG_COMMAND
			WHERE tenantId = @tenantId AND agentId = @agentId AND statusFlag = 'W' AND activeFlag = 'Y'
			ORDER BY addTime`,
			dbx.Args{"tenantId": tenantID, "agentId": agentID}, 1)
		if err != nil {
			return protocol.CommandCall{}, false, err
		}
		err = store.Scan(ctx, s.db, query, args, &commandID, &action, &interval, &raw)
		if errors.Is(err, sql.ErrNoRows) {
			return protocol.CommandCall{}, false, nil
		}
		if err != nil {
			return protocol.CommandCall{}, false, err
		}
		claimed, err := s.claim(ctx, commandID, models.StatusWaiting, models.DeliverPull)
		if err != nil {
			return protocol.CommandCall{}, false, err
		}
		if claimed {
			return callFromExt(commandID, action, raw, interval), true, nil
		}
	}
	return protocol.CommandCall{}, false, nil
}

// ListCommandMeta 校验「列目录」命令归属并返回实例名（代理推清单时调用）。
func (s *Service) ListCommandMeta(ctx context.Context, tenantID, agentID, commandID string) (instance string, err error) {
	var owner, action, flag, raw string
	err = store.Scan(ctx, s.db, `
		SELECT agentId, actionName, statusFlag, COALESCE(extProperty, '')
		FROM FG_COMMAND
		WHERE tenantId = @tenantId AND commandId = @commandId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "commandId": commandID}, &owner, &action, &flag, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", models.ErrCommandGone
	}
	if err != nil {
		return "", err
	}
	if owner != agentID || action != protocol.ActionAppFileList || flag != models.StatusRunning {
		return "", models.ErrCommandGone
	}
	call := callFromExt(commandID, action, raw, 0)
	instance = strings.TrimSpace(call.Instance)
	if instance == "" {
		return "", models.ErrBadRelease
	}
	return instance, nil
}

// FinishOwned 把代理回执写回本代理名下仍在执行的命令。状态只由这一份回执决定。
func (s *Service) FinishOwned(ctx context.Context, tenantID, agentID string, result protocol.CommandResult) error {
	var owner, flag string
	err := store.Scan(ctx, s.db, `
		SELECT agentId, statusFlag FROM FG_COMMAND
		WHERE tenantId = @tenantId AND commandId = @commandId AND activeFlag = 'Y'`,
		dbx.Args{"tenantId": tenantID, "commandId": result.CommandID}, &owner, &flag)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (owner != agentID || flag != models.StatusRunning)) {
		return models.ErrCommandGone
	}
	if err != nil {
		return err
	}
	status := models.StatusDone
	if !result.OK {
		status = models.StatusFailed
	}
	return s.finish(ctx, result.CommandID, status, result.Message, result.Steps)
}
