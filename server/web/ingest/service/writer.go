package service

import (
	"context"

	"fluxguard/protocol"
	dbx "github.com/fluxsce/dbx/db"
)

// Writer 把一批上报写入该代理自己的表。
// 每批都会依次调用已登记的写入器；写入器自己判断要不要落库。
// 新增代理时在本目录实现 Writer，SQL 放 dao，并在 init 里 RegisterWriter。
type Writer interface {
	Save(ctx context.Context, tx *dbx.DB, batch protocol.Batch) error
}

var writers []Writer

// RegisterWriter 登记一种代理的写入器。重复登记同一种实现没有意义，按登记顺序执行。
func RegisterWriter(w Writer) {
	if w == nil {
		return
	}
	writers = append(writers, w)
}
