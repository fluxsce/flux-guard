package service

import (
	"context"

	"fluxguard/protocol"
	"fluxguard/server/web/ingest/dao"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	RegisterWriter(hostWriter{})
}

// hostWriter 写入主机指标和主机资料。没有主机样本或主机块时由 dao 直接跳过。
type hostWriter struct{}

func (hostWriter) Save(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	if err := dao.SaveHost(ctx, tx, batch); err != nil {
		return err
	}
	return dao.SaveHostMetrics(ctx, tx, batch)
}
