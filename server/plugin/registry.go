package plugin

import "sync"

// Registry 保存本服务器进程加载的扩展。
type Registry struct {
	mu      sync.RWMutex
	sinks   []Sink
	modules []Module
	infos   []Info
	codes   map[string]struct{}
}

// NewRegistry 返回空注册表。
func NewRegistry() *Registry {
	return &Registry{codes: map[string]struct{}{}}
}

// Default 是服务器进程的扩展表。扩展在 init 里登记。
var Default = NewRegistry()

// RegisterSink 登记转发扩展。
func RegisterSink(s Sink) { Default.RegisterSink(s) }

// RegisterModule 登记菜单扩展。
func RegisterModule(m Module) { Default.RegisterModule(m) }

// Sinks 返回已登记转发扩展的副本。
func Sinks() []Sink { return Default.Sinks() }

// Modules 返回已登记菜单扩展的副本。
func Modules() []Module { return Default.Modules() }

// Infos 返回目录副本。
func Infos() []Info { return Default.Infos() }

// RegisterSink 登记转发扩展。编码重复时 panic。
func (r *Registry) RegisterSink(s Sink) {
	r.add(s.Code(), Info{Code: s.Code(), Name: s.Name(), Kind: "SINK", Scope: "SERVER", BuiltinFlag: "Y"})
	r.mu.Lock()
	r.sinks = append(r.sinks, s)
	r.mu.Unlock()
}

// RegisterModule 登记菜单扩展。编码重复时 panic。
func (r *Registry) RegisterModule(m Module) {
	r.add(m.Code(), Info{Code: m.Code(), Name: m.Name(), Kind: "MODULE", Scope: "SERVER", BuiltinFlag: "Y"})
	r.mu.Lock()
	r.modules = append(r.modules, m)
	r.mu.Unlock()
}

func (r *Registry) add(code string, info Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.codes == nil {
		r.codes = map[string]struct{}{}
	}
	if _, ok := r.codes[code]; ok {
		panic("服务器扩展编码重复: " + code)
	}
	r.codes[code] = struct{}{}
	r.infos = append(r.infos, info)
}

// Sinks 返回转发扩展副本。
func (r *Registry) Sinks() []Sink {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Sink, len(r.sinks))
	copy(out, r.sinks)
	return out
}

// Modules 返回菜单扩展副本。
func (r *Registry) Modules() []Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Module, len(r.modules))
	copy(out, r.modules)
	return out
}

// Infos 返回目录副本。
func (r *Registry) Infos() []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, len(r.infos))
	copy(out, r.infos)
	return out
}
