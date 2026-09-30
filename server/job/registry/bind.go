package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
	dbx "github.com/fluxsce/dbx/db"
)

// aliasJob 把已有实现挂到另一个编码上。Run 仍走原来的业务，目录上是新的任务。
type aliasJob struct {
	inner contract.Job
	meta  contract.Meta
}

func (a aliasJob) Meta() contract.Meta { return a.meta }

func (a aliasJob) Run(ctx context.Context, call contract.Call) (contract.Result, error) {
	return a.inner.Run(ctx, call)
}

// Kind 控制台可用来新建任务的一种执行器。
type Kind struct {
	Kind        string
	Title       string
	Description string
}

// Kinds 返回进程内已登记、可供新建的任务种类。控制台新建的任务不在此列。
func Kinds() []Kind {
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	seen := map[string]struct{}{}
	out := make([]Kind, 0)
	for _, job := range jobs.items {
		meta := job.Meta()
		if meta.Kind == "" || meta.Source == catalog.SourceExternal {
			continue
		}
		if _, ok := seen[meta.Kind]; ok {
			continue
		}
		seen[meta.Kind] = struct{}{}
		out = append(out, Kind{Kind: meta.Kind, Title: meta.Title, Description: meta.Description})
	}
	return out
}

// Runner 按种类取内置实现。同种类有多份时取任意一份内置实现。
func Runner(kind string) (contract.Job, bool) {
	kind = strings.TrimSpace(kind)
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	for _, job := range jobs.items {
		meta := job.Meta()
		if meta.Kind == kind && meta.Source != catalog.SourceExternal {
			return job, true
		}
	}
	return nil, false
}

// Bind 把某种已登记实现挂到新编码上，供控制台新建的任务调用。
func Bind(meta contract.Meta) error {
	meta.Code = strings.TrimSpace(meta.Code)
	meta.Kind = strings.TrimSpace(meta.Kind)
	meta.Title = strings.TrimSpace(meta.Title)
	if meta.Code == "" || meta.Kind == "" {
		return fmt.Errorf("任务编码和种类不能为空")
	}
	runner, ok := Runner(meta.Kind)
	if !ok {
		return fmt.Errorf("未知任务种类 %s", meta.Kind)
	}
	if meta.Source == "" {
		meta.Source = catalog.SourceExternal
	}
	if meta.Scope == "" {
		meta.Scope = catalog.ScopeTenant
	}
	Register(aliasJob{inner: runner, meta: meta})
	return nil
}

// Unregister 卸下控制台新建的任务。内置任务保留。
func Unregister(code string) error {
	spec, ok := catalog.ByCode(code)
	if !ok {
		return fmt.Errorf("未知任务 %s", code)
	}
	if spec.Source != catalog.SourceExternal {
		return fmt.Errorf("内置任务不能删除")
	}
	catalog.Remove(spec.Code)
	jobs.mu.Lock()
	delete(jobs.items, spec.Code)
	jobs.mu.Unlock()
	return nil
}

// Restore 把已落库的外部任务重新挂上。须在内置任务 Register 之后调用。
func Restore(ctx context.Context, db *dbx.DB) error {
	if db == nil {
		return nil
	}
	rows, err := db.Query(ctx, `
		SELECT groupCode, content
		FROM FG_SYS_SETTING
		WHERE activeFlag = 'Y' AND groupCode LIKE 'job:%'`, nil)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := map[string]struct{}{}
	for rows.Next() {
		var groupCode, content string
		if err := rows.Scan(&groupCode, &content); err != nil {
			return err
		}
		code := strings.TrimPrefix(groupCode, "job:")
		if code == "" || code == groupCode {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		if _, ok := catalog.ByCode(code); ok {
			continue
		}
		var doc struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Kind        string `json:"kind"`
			Source      string `json:"source"`
			Scope       string `json:"scope"`
		}
		if json.Unmarshal([]byte(content), &doc) != nil || doc.Source != catalog.SourceExternal || doc.Kind == "" {
			continue
		}
		if err := Bind(contract.Meta{
			Code:        code,
			Title:       doc.Title,
			Description: doc.Description,
			Kind:        doc.Kind,
			Source:      catalog.SourceExternal,
			Scope:       doc.Scope,
		}); err != nil {
			return err
		}
		seen[code] = struct{}{}
	}
	return rows.Err()
}
