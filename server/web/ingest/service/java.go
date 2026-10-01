package service

import (
	"context"

	"fluxguard/protocol"
	"fluxguard/server/web/ingest/dao"
	dbx "github.com/fluxsce/dbx/db"
)

func init() {
	RegisterWriter(javaWriter{})
}

// javaWriter 写入 Java 应用资料，以及堆、内存池、运行时、GC、停顿、线程和线程状态曲线。
type javaWriter struct{}

func (javaWriter) Save(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error {
	if err := dao.SaveJava(ctx, tx, batch); err != nil {
		return err
	}
	return dao.SaveJavaMetrics(ctx, tx, batch)
}
