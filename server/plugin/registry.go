package plugin

import (
	"fmt"
	"sync"
)

// Registry 保存本进程已加载的外置插件目录。进程由 Load 拉起。
type Registry struct {
	mu    sync.RWMutex
	infos []Info
	codes map[string]struct{}
}

// NewRegistry 返回空注册表。
func NewRegistry() *Registry {
	return &Registry{codes: map[string]struct{}{}}
}

// Default 是服务器进程的插件目录。
var Default = NewRegistry()

// Infos 返回目录副本。
func Infos() []Info { return Default.Infos() }

// addExternal 登记一份外置插件。编码已经存在时返回错误，不打断进程。
func (r *Registry) addExternal(info Info) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.codes == nil {
		r.codes = map[string]struct{}{}
	}
	if _, ok := r.codes[info.Code]; ok {
		return fmt.Errorf("插件编码重复: %s", info.Code)
	}
	r.codes[info.Code] = struct{}{}
	r.infos = append(r.infos, info)
	return nil
}

func (r *Registry) has(code string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.codes[code]
	return ok
}

func (r *Registry) remove(code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.codes, code)
	next := make([]Info, 0, len(r.infos))
	for _, info := range r.infos {
		if info.Code != code {
			next = append(next, info)
		}
	}
	r.infos = next
}

// Infos 返回目录副本。
func (r *Registry) Infos() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, len(r.infos))
	copy(out, r.infos)
	return out
}
