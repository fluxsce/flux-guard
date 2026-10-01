package panel

func mongoPanels() []Panel {
	return []Panel{
		{KindCode: "mongo", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "连通、角色和版本。"},
		{KindCode: "mongo", PanelCode: "connections", PanelName: "连接", SortOrder: 20, List: true, Metrics: []string{"connectionsCurrent"}, Sections: []string{"connections"}, NoteText: "当前连接和可用连接。"},
		{KindCode: "mongo", PanelCode: "ops", PanelName: "操作", SortOrder: 30, List: true, Metrics: []string{"opQuery"}, Sections: []string{"ops"}, NoteText: "插入、查询、更新和删除的累计次数。"},
		{KindCode: "mongo", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "MongoDB 命令通道。"},
	}
}
