// Package catalog 是任务目录。不包含任何具体任务编码。
// 新任务在自己的包里 Register，调度核只按已登记项运行。
package catalog

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	// SourceBuiltin 进程内实现的任务。
	SourceBuiltin = "builtin"
	// SourceExternal 调用外部系统的任务。仍须 registry.Register，否则不会被调度。
	SourceExternal = "external"

	// ScopeTenant 策略与状态按租户隔离。
	ScopeTenant = "tenant"
	// ScopeSystem 全站一份策略（租户键用 SystemTenant）。
	ScopeSystem = "system"

	// SystemTenant ScopeSystem 任务使用的租户键。
	SystemTenant = "_"
)

// Spec 目录中一条任务的静态说明。Code / Kind 由登记方给出，目录不预置业务名。
type Spec struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Source      string `json:"source"`
	Scope       string `json:"scope"`
}

type store struct {
	mu    sync.RWMutex
	order []string
	items map[string]Spec
}

var global = &store{items: map[string]Spec{}}

// Register 登记任务。同 code 后登记覆盖先登记。
func Register(spec Spec) {
	spec.Code = strings.TrimSpace(spec.Code)
	if spec.Code == "" {
		panic("job/catalog: Register empty code")
	}
	if spec.Source == "" {
		spec.Source = SourceBuiltin
	}
	if spec.Scope == "" {
		spec.Scope = ScopeTenant
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, ok := global.items[spec.Code]; !ok {
		global.order = append(global.order, spec.Code)
	}
	global.items[spec.Code] = spec
}

// Specs 返回已登记任务（登记顺序）。
func Specs() []Spec {
	global.mu.RLock()
	defer global.mu.RUnlock()
	out := make([]Spec, 0, len(global.order))
	for _, code := range global.order {
		if s, ok := global.items[code]; ok {
			out = append(out, s)
		}
	}
	return out
}

// ByCode 按编码取目录项。
func ByCode(code string) (Spec, bool) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	s, ok := global.items[strings.TrimSpace(code)]
	return s, ok
}

// Must 取目录项；不存在则 panic（启动期接线用）。
func Must(code string) Spec {
	s, ok := ByCode(code)
	if !ok {
		panic(fmt.Sprintf("job/catalog: unknown code %q", code))
	}
	return s
}

// Remove 从目录卸下一条任务。没有该编码时不做任何事。
func Remove(code string) {
	code = strings.TrimSpace(code)
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, ok := global.items[code]; !ok {
		return
	}
	delete(global.items, code)
	next := make([]string, 0, len(global.order))
	for _, item := range global.order {
		if item != code {
			next = append(next, item)
		}
	}
	global.order = next
}

// Codes 已登记编码，排序后返回。
func Codes() []string {
	codes := make([]string, 0, len(Specs()))
	for _, s := range Specs() {
		codes = append(codes, s.Code)
	}
	sort.Strings(codes)
	return codes
}
