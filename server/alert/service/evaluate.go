package service

import (
	"context"
	"strings"
	"time"

	"fluxguard/common/logger"
	"fluxguard/server/alert/models"
)

// Evaluate 轮询尚未发送完成的预警日志，按渠道和模板发送。
func (s *Service) Evaluate(ctx context.Context) {
	if s == nil || s.db == nil {
		return
	}
	if !s.eval.TryLock() {
		return
	}
	defer s.eval.Unlock()
	rows, err := s.events.ListPending(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.Error(ctx, "预警投递: %v", err)
		}
		return
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		s.sendLog(ctx, row.TenantID, row.Log)
	}
}

// Submit 把功能模块已经写好的一条预警记入日志。发送由后台轮询完成。渠道留空时不记。
func (s *Service) Submit(ctx context.Context, tenantID string, notice models.Notice, now time.Time) error {
	if s == nil || strings.TrimSpace(notice.ChannelID) == "" {
		return nil
	}
	_, err := s.events.Insert(ctx, tenantID, notice, now)
	return err
}
