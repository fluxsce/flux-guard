package service

import (
	"context"

	"fluxguard/protocol"
	"fluxguard/server/web/ingest/dao"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	RegisterWriter(appsideWriter{})
}

// appsideWriter 写入应用侧代理报上来的应用和实例。
type appsideWriter struct{}

func (appsideWriter) Save(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	return dao.SaveAppside(ctx, tx, batch)
}
