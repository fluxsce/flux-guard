package panel

func mongoPanels() []Panel {
	return []Panel{
		{KindCode: "mongo", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "连通、角色、版本和排队。"},
		{KindCode: "mongo", PanelCode: "connections", PanelName: "连接", SortOrder: 20, List: true, Metrics: []string{"connectionsCurrent"}, Sections: []string{"connections"}, NoteText: "当前连接、可用连接和占用。"},
		{KindCode: "mongo", PanelCode: "memory", PanelName: "内存", SortOrder: 30, List: true, Metrics: []string{"cacheUsedBytes"}, Sections: []string{"memory"}, NoteText: "常驻内存和 WiredTiger 缓存。列表看缓存已用。"},
		{KindCode: "mongo", PanelCode: "ops", PanelName: "操作", SortOrder: 40, List: true, Metrics: []string{"opsPerSec"}, Sections: []string{"ops"}, NoteText: "两次采集之间的每秒操作。"},
		{KindCode: "mongo", PanelCode: "traffic", PanelName: "流量", SortOrder: 50, Sections: []string{"traffic"}, NoteText: "入站和出站。"},
		{KindCode: "mongo", PanelCode: "replica", PanelName: "复制", SortOrder: 60, List: true, Metrics: []string{"replicaLagSec"}, Sections: []string{"replica"}, NoteText: "复制延迟和成员状态。"},
		{KindCode: "mongo", PanelCode: "storage", PanelName: "库", SortOrder: 70, Sections: []string{"storage"}, NoteText: "各库占用。"},
		{KindCode: "mongo", PanelCode: "slow", PanelName: "慢操作", SortOrder: 80, List: true, Metrics: []string{"slowOps"}, Sections: []string{"slow"}, NoteText: "正在执行且已超过 1 秒的操作。"},
		{KindCode: "mongo", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "MongoDB 命令通道。"},
	}
}
