package panel

func nacosPanels() []Panel {
	return []Panel{
		{KindCode: "nacos", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "连通情况。"},
		{KindCode: "nacos", PanelCode: "registry", PanelName: "配置与服务", SortOrder: 20, List: true, Metrics: []string{"configCount", "serviceCount"}, Sections: []string{"registry"}, NoteText: "配置数和服务数。"},
		{KindCode: "nacos", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "Nacos 控制台操作。"},
	}
}
