package service

import (
	"context"
	"database/sql"
	"errors"

	"fluxguard/protocol"
	"fluxguard/server/command/models"
	"fluxguard/server/store"
	dbx "github.com/fluxsce/dbx/db"
)

// ClaimNext 认领该代理下一条待取命令。
func (s *Service) ClaimNext(ctx context.Context, tenantID, agentID string) (protocol.CommandCall, bool, error) {
	for range 3 {
		var commandID, action string
		var interval int
		query, args, err := store.PageQuery(s.db, `
			SELECT commandId, actionName, COALESCE(intervalSec, 0)
			FROM FG_COMMAND
			WHERE tenantId = @tenantId AND agentId = @agentId AND statusFlag = 'W' AND activeFlag = 'Y'
			ORDER BY addTime`,
			dbx.Args{"tenantId": tenantID, "agentId": agentID}, 1)
		if err != nil {
			return protocol.CommandCall{}, false, err
		}
		err = store.Scan(ctx, s.db, query, args, &commandID, &action, &interval)
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
			return protocol.CommandCall{CommandID: commandID, Action: action, IntervalSec: interval}, true, nil
		}
	}
	return protocol.CommandCall{}, false, nil
}

// FinishOwned 把代理回执写回本代理名下的执行中命令。
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
	return s.finish(ctx, result.CommandID, status, result.Message)
}
