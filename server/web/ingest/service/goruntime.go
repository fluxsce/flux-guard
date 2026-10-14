package service

import (
	"context"

	"fluxguard/protocol"
	"fluxguard/server/web/ingest/dao"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	RegisterWriter(goWriter{})
}

// goWriter 写入 Go 进程自己的身份和最近一次运行时。
type goWriter struct{}

func (goWriter) Save(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	return dao.SaveGo(ctx, tx, batch)
}
