package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// loaded 是已经按 plugins/manifests 登记的一份外置插件。
type loaded struct {
	manifest    Manifest
	svc         *service
	methods     map[string]struct{}
	ops         map[string]map[string]struct{}
	credentials map[string]struct{}
}

var (
	activeMu sync.Mutex
	active   = map[string]*loaded{}
)

// Load 读取 plugins/manifests 下的 yaml 并登记外置插件。
// 启动方式是 lazy，进程在第一次调用时再拉起。目录不存在时不加载外置插件。
// 某一份清单无效时，本次已经登记的外置插件会停掉。
func Load(dir string) error {
	entries, err := os.ReadDir(filepath.Join(dir, "manifests"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var added []string
	fail := func(cause error) error {
		for i := len(added) - 1; i >= 0; i-- {
			removeOne(added[i])
		}
		return cause
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "manifests", entry.Name()))
		if err != nil {
			return fail(err)
		}
		m, methods, ops, creds, err := decodeManifest(dir, raw)
		if err != nil {
			return fail(err)
		}
		if Default.has(m.ID) || componentTaken(m.Component) || addressTaken(m.Addr) {
			if Default.has(m.ID) {
				return fail(fmt.Errorf("插件编码重复: %s", m.ID))
			}
			if addressTaken(m.Addr) {
				return fail(fmt.Errorf("插件通道重复"))
			}
			return fail(fmt.Errorf("这种组件已经有插件"))
		}
		if err := install(m, methods, ops, creds); err != nil {
			return fail(err)
		}
		added = append(added, m.ID)
	}
	return nil
}

// Unload 停掉全部外置插件，并从目录去掉这些项。
// 被换掉的内置探针由 component/bridge 交还，这里不碰采集表。
func Unload() {
	activeMu.Lock()
	codes := make([]string, 0, len(active))
	for code := range active {
		codes = append(codes, code)
	}
	activeMu.Unlock()
	for _, code := range codes {
		removeOne(code)
	}
}

// Start 启动这个外置插件。已经在跑则直接返回。已停止或已退出的可以再拉起。
func Start(code string) error {
	activeMu.Lock()
	item := active[code]
	activeMu.Unlock()
	if item == nil || item.svc == nil {
		return fmt.Errorf("没有这个插件")
	}
	item.svc.stateMu.Lock()
	item.svc.hold = false
	if item.svc.status == statusExited {
		item.svc.status = statusStopped
	}
	item.svc.stateMu.Unlock()
	return item.svc.start()
}

// Stop 停止这个外置插件。停止之后调用不会自动再拉起。
func Stop(code string) {
	activeMu.Lock()
	item := active[code]
	activeMu.Unlock()
	if item == nil || item.svc == nil {
		return
	}
	item.svc.stop()
}

// StopAll 停止全部外置插件。中央退出时调用，目录仍保留到进程结束。
func StopAll() {
	activeMu.Lock()
	items := make([]*loaded, 0, len(active))
	for _, item := range active {
		items = append(items, item)
	}
	activeMu.Unlock()
	for _, item := range items {
		if item.svc != nil {
			item.svc.stop()
		}
	}
}

// ProcState 是一个外置插件进程的当前状态。
type ProcState struct {
	Code      string
	Component string
	Status    string
	PID       int
}

// States 返回已登记外置插件的状态。
func States() []ProcState {
	activeMu.Lock()
	defer activeMu.Unlock()
	out := make([]ProcState, 0, len(active))
	for _, item := range active {
		if item.svc == nil {
			continue
		}
		status, pid, _ := item.svc.snapshot()
		out = append(out, ProcState{Code: item.manifest.ID, Component: item.manifest.Component, Status: status, PID: pid})
	}
	return out
}

func install(m Manifest, methods map[string]struct{}, ops map[string]map[string]struct{}, creds map[string]struct{}) error {
	info := Info{Code: m.ID, Name: m.Name, Kind: KindService, Scope: ScopeServer, BuiltinFlag: "N"}
	if err := Default.addExternal(info); err != nil {
		return err
	}
	svc := newService(m.ID, filepath.Dir(m.Argv[0]), m.Argv, m.Addr)
	item := &loaded{manifest: m, svc: svc, methods: methods, ops: ops, credentials: creds}
	activeMu.Lock()
	active[m.ID] = item
	activeMu.Unlock()
	return nil
}

func removeOne(code string) {
	activeMu.Lock()
	item := active[code]
	delete(active, code)
	activeMu.Unlock()
	if item == nil {
		Default.remove(code)
		return
	}
	if item.svc != nil {
		item.svc.stop()
	}
	Default.remove(code)
}

// Slot 是已加载插件声明要服务的组件种类。Component 为空表示这份插件不替换内置探针。
type Slot struct {
	ID        string
	Component string
}

// Slots 返回当前已加载插件的组件对应关系。调用方据此使用 Call 的结果。
func Slots() []Slot {
	activeMu.Lock()
	defer activeMu.Unlock()
	out := make([]Slot, 0, len(active))
	for _, item := range active {
		out = append(out, Slot{ID: item.manifest.ID, Component: item.manifest.Component})
	}
	return out
}

func addressTaken(addr string) bool {
	if addr == "" {
		return false
	}
	activeMu.Lock()
	defer activeMu.Unlock()
	for _, item := range active {
		if item.manifest.Addr == addr {
			return true
		}
	}
	return false
}

func componentTaken(kind string) bool {
	if kind == "" {
		return false
	}
	activeMu.Lock()
	defer activeMu.Unlock()
	for _, item := range active {
		if item.manifest.Component == kind {
			return true
		}
	}
	return false
}
