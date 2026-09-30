// Package registry 登记业务实现的 Job。调度与手动执行都从这里按编码取出。
package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"fluxguard/server/job/catalog"
	"fluxguard/server/job/contract"
)

type store struct {
	mu    sync.RWMutex
	items map[string]contract.Job
}

var jobs = &store{items: map[string]contract.Job{}}

// Register 登记一个任务，并写入目录。同编码后登记覆盖先登记。
func Register(job contract.Job) {
	if job == nil {
		panic("job/registry: Register nil job")
	}
	meta := job.Meta()
	meta.Code = strings.TrimSpace(meta.Code)
	if meta.Code == "" {
		panic("job/registry: Register empty code")
	}
	catalog.Register(catalog.Spec{
		Code:        meta.Code,
		Title:       meta.Title,
		Description: meta.Description,
		Kind:        meta.Kind,
		Source:      meta.Source,
		Scope:       meta.Scope,
	})
	jobs.mu.Lock()
	jobs.items[meta.Code] = job
	jobs.mu.Unlock()
}

// Has 是否已登记可执行实现。只有目录、没有实现时为 false，调度会跳过。
func Has(code string) bool {
	_, ok := Get(code)
	return ok
}

// Codes 已登记可执行任务的编码，排序后返回。
func Codes() []string {
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	out := make([]string, 0, len(jobs.items))
	for code := range jobs.items {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// Get 按编码取已登记任务。
func Get(code string) (contract.Job, bool) {
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	job, ok := jobs.items[strings.TrimSpace(code)]
	return job, ok
}

// Must 取已登记任务；不存在则 panic（启动期接线用）。
func Must(code string) contract.Job {
	job, ok := Get(code)
	if !ok {
		panic(fmt.Sprintf("job/registry: unknown code %q", code))
	}
	return job
}
