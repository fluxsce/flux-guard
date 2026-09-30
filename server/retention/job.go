package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"fluxguard/common/logger"
	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	"fluxguard/server/job/ext/archive"
	dbx "github.com/fluxsce/dbx/db"
)

// archiveJob 数据归档的 Job 实现。调度器只看到 contract.Job，按表删除留在本包。
type archiveJob struct {
	db *dbx.DB
}

// NewJob 数据归档任务。调度器只调用 Job.Run，删除逻辑留在本包。
func NewJob(db *dbx.DB) contract.Job {
	return &archiveJob{db: db}
}

// Meta 返回数据归档在目录中的静态说明。
func (j *archiveJob) Meta() contract.Meta {
	spec := catalog.Must(archive.Code)
	return contract.Meta{
		Code:        spec.Code,
		Title:       spec.Title,
		Description: spec.Description,
		Kind:        spec.Kind,
		Source:      spec.Source,
		Scope:       spec.Scope,
	}
}

// Run 把本次参数放进上下文后，只清理 call.TenantID 这一个租户。
func (j *archiveJob) Run(ctx context.Context, call contract.Call) (contract.Result, error) {
	ctx = archive.WithParams(ctx, archive.ParseParams(call.Params))
	return purge(ctx, j.db, call.TenantID)
}

// purge 按数据集逐表删除。某一表失败时继续其余表，最后把该错误交给调度器记为失败。
func purge(ctx context.Context, db *dbx.DB, tenantID string) (contract.Result, error) {
	if db == nil {
		return dataResult(map[string]string{"detail": "无数据库"}), nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return dataResult(map[string]string{"detail": "无租户"}), nil
	}
	var deleted int64
	var failed error
	parts := make([]string, 0, 8)
	for _, tableJob := range NewJobs(db) {
		if tableJob == nil {
			continue
		}
		n, runErr := tableJob.Run(ctx, []string{tenantID})
		deleted += n
		if runErr != nil && ctx.Err() == nil {
			failed = runErr
			continue
		}
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", tableJob.Name(), n))
		}
	}
	summary := "无过期数据"
	if len(parts) > 0 {
		summary = strings.Join(parts, "，")
	}
	if failed == nil && deleted > 0 {
		logger.Info(ctx, "数据归档完成 deleted=%d %s", deleted, summary)
	}
	return dataResult(map[string]any{"deleted": deleted, "detail": summary}), failed
}

// dataResult 把结果对象编成 Result.Data。编码失败时留下空对象，避免调度器缺结果。
func dataResult(v any) contract.Result {
	raw, err := json.Marshal(v)
	if err != nil {
		return contract.Result{Data: json.RawMessage(`{}`)}
	}
	return contract.Result{Data: raw}
}
