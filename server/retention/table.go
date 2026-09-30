package retention

import (
	"context"
	"strings"
	"time"

	"fluxguard/common/logger"
	"fluxguard/server/job/ext/archive"
	"fluxguard/server/limits"
	dbx "github.com/fluxsce/dbx/db"
)

// TableJob 按 Dataset 硬删过期行。
type TableJob struct {
	db *dbx.DB
	ds Dataset
}

// NewTableJob 创建按表删除的执行器。Cutoff 返回 skip 时该租户本轮不删。
func NewTableJob(db *dbx.DB, ds Dataset) *TableJob {
	return &TableJob{db: db, ds: ds}
}

// NewJobs 按当前 Dataset 清单生成删除任务。
func NewJobs(db *dbx.DB) []*TableJob {
	datasets := Datasets()
	jobs := make([]*TableJob, 0, len(datasets))
	for i := range datasets {
		jobs = append(jobs, NewTableJob(db, datasets[i]))
	}
	return jobs
}

// Name 返回任务名。
func (j *TableJob) Name() string {
	if j == nil {
		return ""
	}
	return j.ds.Name
}

// Run 按已知租户分批删除早于截止时间的行。返回本任务删掉的总行数。
func (j *TableJob) Run(ctx context.Context, tenants []string) (int64, error) {
	if j == nil || j.db == nil || j.ds.Cutoff == nil || j.ds.Table == "" || j.ds.TimeCol == "" {
		return 0, nil
	}
	sql, err := limitedDelete(j.db.Driver(), j.ds.Table, j.ds.TimeCol, j.ds.ExtraWhere, limits.RetentionDeleteBatch)
	if err != nil {
		logger.Error(ctx, "数据归档SQL无效 %s: %v", j.ds.Name, err)
		return 0, err
	}
	var total int64
	for _, tenantID := range tenants {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		tenantID = strings.TrimSpace(tenantID)
		if tenantID == "" {
			continue
		}
		// 调度器已经按本任务的策略决定要不要调用。这里只拦直接跑表、且没有本次参数的情况。
		if !archive.HasParams(ctx) && !archive.Get(tenantID).Enabled {
			continue
		}
		before, ok := j.ds.Cutoff(ctx, tenantID)
		if !ok {
			continue
		}
		n, truncated, err := j.deleteTenant(ctx, sql, tenantID, formatBefore(before))
		if err != nil {
			logger.Error(ctx, "数据归档失败 %s tenant=%s: %v", j.ds.Name, tenantID, err)
			return total, err
		}
		total += n
		if n > 0 {
			logger.Info(ctx, "数据归档 %s tenant=%s count=%d before=%s truncated=%v",
				j.ds.Name, tenantID, n, formatBefore(before), truncated)
		}
	}
	return total, nil
}

// deleteTenant 按批删除同一租户早于 before 的行。本批未满表示已经删完；达到批数上限则 truncated 为 true，剩余留到下次。
func (j *TableJob) deleteTenant(ctx context.Context, sql, tenantID, before string) (int64, bool, error) {
	var total int64
	for i := 0; i < limits.RetentionDeleteMaxBatches; i++ {
		if err := ctx.Err(); err != nil {
			return total, false, err
		}
		res, err := j.db.Exec(ctx, sql, dbx.Args{"tenantId": tenantID, "before": before})
		if err != nil {
			return total, false, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, false, err
		}
		total += n
		if n < int64(limits.RetentionDeleteBatch) {
			return total, false, nil
		}
		time.Sleep(limits.RetentionDeletePause)
	}
	return total, true, nil
}
