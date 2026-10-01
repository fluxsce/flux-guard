// Package runtime 按种类编码调用已登记的采集器。采集包在 init 里登记，调度不引用具体种类。
package runtime

import (
	"context"
	"fmt"

	"fluxguard/server/component/catalog"
	"fluxguard/server/component/conn"
	"fluxguard/server/component/models"
	dbx "github.com/fluxsce/dbx/db"
)

// Runner 采集一种组件，并把点写入该种类的采集表。
type Runner interface {
	Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error)
	Series(ctx context.Context, db *dbx.DB, tenantID, componentID string, limit int) ([]models.Point, error)
}

var runners = map[string]Runner{}

// Register 登记一种采集器。重复登记时保留先登记的。
func Register(kind string, runner Runner) {
	if kind == "" || runner == nil {
		return
	}
	if _, ok := runners[kind]; ok {
		return
	}
	runners[kind] = runner
}

// Probe 拨号并落一条采集记录，再把摘要写回连接。
func Probe(ctx context.Context, db *dbx.DB, target conn.Open) (models.Status, error) {
	runner := runners[target.KindCode]
	if runner == nil {
		status := models.Status{Error: "这种组件还没有采集器"}
		if db != nil {
			_ = conn.Mark(ctx, db, target.TenantID, target.ComponentID, status)
		}
		return status, fmt.Errorf("%s", status.Error)
	}
	status, err := runner.Probe(ctx, db, target)
	if db != nil {
		if markErr := conn.Mark(ctx, db, target.TenantID, target.ComponentID, status); markErr != nil && err == nil {
			err = markErr
		}
	}
	return status, err
}

// Series 读取该种类的历史点。
func Series(ctx context.Context, db *dbx.DB, kind, tenantID, componentID string, limit int) ([]models.Point, error) {
	runner := runners[kind]
	if runner == nil || !catalog.Known(kind) {
		return nil, fmt.Errorf("不支持的组件种类")
	}
	return runner.Series(ctx, db, tenantID, componentID, limit)
}
