// Package panel 是组件监控的页签目录。每种组件的页签不一样，终端也按种类分开。
// 初始行在 FG_COMPONENT_PANEL.sql，启动时不再写入。
package panel

// Panel 是详情里的一页。概览收走其它页签没有点名的分节。
type Panel struct {
	KindCode  string
	PanelCode string
	PanelName string
	SortOrder int
	Terminal  bool
	List      bool
	Metrics   []string
	Sections  []string
	NoteText  string
}

// ID 是页签主键，种类和页签编码用冒号接起来。
func (p Panel) ID() string {
	return p.KindCode + ":" + p.PanelCode
}

// All 返回固定页签，顺序即详情里的页签顺序。
func All() []Panel {
	out := make([]Panel, 0, 32)
	out = append(out, redisPanels()...)
	out = append(out, mysqlPanels()...)
	out = append(out, mongoPanels()...)
	out = append(out, kafkaPanels()...)
	out = append(out, zkPanels()...)
	out = append(out, nacosPanels()...)
	return out
}

// Of 返回一种组件的页签。
func Of(kind string) []Panel {
	var out []Panel
	for _, item := range All() {
		if item.KindCode == kind {
			out = append(out, item)
		}
	}
	return out
}

// ListMetrics 是实例列表上要直接显示的当前值，列名必须在该种类的当前值表里。
func ListMetrics(kind string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, item := range Of(kind) {
		if !item.List {
			continue
		}
		for _, code := range item.Metrics {
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			out = append(out, code)
		}
	}
	return out
}
