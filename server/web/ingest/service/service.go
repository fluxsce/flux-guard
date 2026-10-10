package service

import (
	"context"
	"errors"
	"time"

	"fluxguard/protocol"
	"fluxguard/server/web/ingest/dao"
	"fluxguard/server/web/ingest/models"
	dbx "github.com/fluxsce/dbx/db"
)

// Service 接收代理批次，先按 batchId 去重，再交给已登记的写入器。
type Service struct {
	db *dbx.DB
}

// Open 使用已经灌好脚本的库。
func Open(conn *dbx.DB) *Service {
	return &Service{db: conn}
}

// Accept 写入一批上报。租户和代理必须已由调用方根据令牌填好。
// 批次、事件和各代理自己的表在同一个事务里提交。告警不在这里判断。
func (s *Service) Accept(ctx context.Context, batch protocol.Batch) (protocol.Result, error) {
	if err := ctx.Err(); err != nil {
		return protocol.Result{}, err
	}
	if err := validate(batch); err != nil {
		return protocol.Result{}, err
	}
	now := time.Now().UTC()
	if batch.SentAt.IsZero() {
		batch.SentAt = now
	}
	batch.Samples = normalizeSamples(batch.Samples, now)
	batch.Events = normalizeEvents(batch.Events, now)

	err := s.db.Tx(ctx, func(tx *dbx.DB) error {
		if err := dao.InsertBatch(ctx, tx, batch); err != nil {
			return err
		}
		for _, writer := range writers {
			if err := writer.Save(ctx, tx, batch); err != nil {
				return err
			}
		}
		return dao.InsertEvents(ctx, tx, batch)
	})
	if errors.Is(err, dao.ErrDuplicate) {
		return protocol.Result{Duplicate: true, SampleCount: len(batch.Samples), EventCount: len(batch.Events)}, nil
	}
	if err != nil {
		return protocol.Result{}, err
	}
	return protocol.Result{SampleCount: len(batch.Samples), EventCount: len(batch.Events)}, nil
}

// HostOf 返回这台代理最近上报的机器资料。
func (s *Service) HostOf(tenantID, agentID string) (protocol.Host, bool) {
	return dao.HostOf(context.Background(), s.db, tenantID, agentID)
}

// LatestEvents 返回该租户最新的事件，新的在前。
func (s *Service) LatestEvents(tenantID string, limit int) []models.Event {
	return dao.LatestEvents(context.Background(), s.db, tenantID, limit)
}

// Stats 统计库里的事件和最近一次接收时间。
func (s *Service) Stats(tenantID string) models.Stats {
	return dao.Stats(context.Background(), s.db, tenantID)
}
