package plugin

// 控制台看到的进程状态。idle 是已登记、尚未拉起，下一次调用会启动。
// held 是已经明确停下，调用不会再自动拉起。通道地址、可执行文件和口令不在这里。
const (
	CatalogIdle    = "idle"
	CatalogRunning = "running"
	CatalogExited  = "exited"
	CatalogHeld    = "held"
)

// CatalogMethod 是清单里登记的一个方法。Ops 只有维护方法才有。
type CatalogMethod struct {
	Name string   `json:"name"`
	Ops  []string `json:"ops,omitempty"`
}

// CatalogItem 是控制台插件目录的一行。
type CatalogItem struct {
	Code        string          `json:"pluginCode"`
	Name        string          `json:"pluginName"`
	Version     string          `json:"version"`
	Kind        string          `json:"pluginKind"`
	Scope       string          `json:"pluginScope"`
	BuiltinFlag string          `json:"builtinFlag"`
	Component   string          `json:"component"`
	Lang        string          `json:"lang"`
	Startup     string          `json:"startup"`
	Status      string          `json:"status"`
	PID         int             `json:"pid,omitempty"`
	Methods     []CatalogMethod `json:"methods"`
}

// Catalog 返回已登记插件的目录副本。顺序不稳定，调用方自己排序。
func Catalog() []CatalogItem {
	activeMu.Lock()
	defer activeMu.Unlock()
	out := make([]CatalogItem, 0, len(active))
	for _, item := range active {
		if item == nil {
			continue
		}
		status, pid, hold := statusStopped, 0, false
		if item.svc != nil {
			status, pid, hold = item.svc.snapshot()
		}
		out = append(out, catalogItem(item.manifest, status, pid, hold))
	}
	return out
}

func catalogItem(m Manifest, status string, pid int, hold bool) CatalogItem {
	methods := make([]CatalogMethod, 0, len(m.Methods))
	for _, method := range m.Methods {
		row := CatalogMethod{Name: method.Name}
		for _, op := range method.Ops {
			if op.Name == "" {
				continue
			}
			row.Ops = append(row.Ops, op.Name)
		}
		methods = append(methods, row)
	}
	item := CatalogItem{
		Code:        m.ID,
		Name:        m.Name,
		Version:     m.Version,
		Kind:        KindService,
		Scope:       ScopeServer,
		BuiltinFlag: "N",
		Component:   m.Component,
		Lang:        m.Runtime.Lang,
		Startup:     m.Lifecycle.Startup,
		Status:      catalogStatus(status, hold),
		Methods:     methods,
	}
	if item.Status == CatalogRunning && pid > 0 {
		item.PID = pid
	}
	return item
}

func catalogStatus(status string, hold bool) string {
	switch status {
	case statusRunning:
		return CatalogRunning
	case statusExited:
		return CatalogExited
	default:
		if hold {
			return CatalogHeld
		}
		return CatalogIdle
	}
}
