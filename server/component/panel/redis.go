package panel

func redisPanels() []Panel {
	return []Panel{
		{KindCode: "redis", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "连通、访问和持久化。集群或哨兵的状态也在这一页。"},
		{KindCode: "redis", PanelCode: "memory", PanelName: "内存", SortOrder: 20, List: true, Metrics: []string{"usedMemoryBytes", "memUsagePercent", "aofOk"}, Sections: []string{"memory"}, NoteText: "已用、上限占用、峰值和碎片率。列表同时看 AOF 是否开启。"},
		{KindCode: "redis", PanelCode: "traffic", PanelName: "流量", SortOrder: 30, Sections: []string{"traffic"}, NoteText: "入站和出站。"},
		{KindCode: "redis", PanelCode: "slow", PanelName: "慢操作", SortOrder: 40, List: true, Metrics: []string{"slowlogLen"}, Sections: []string{"slow"}, NoteText: "慢操作条数和最近命令。"},
		{KindCode: "redis", PanelCode: "replica", PanelName: "复制", SortOrder: 50, Sections: []string{"replica"}, NoteText: "从节点、复制延迟和主链路。"},
		{KindCode: "redis", PanelCode: "keys", PanelName: "键", SortOrder: 60, Sections: []string{"keys"}, NoteText: "键数量、淘汰和各库键数量。"},
		{KindCode: "redis", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "Redis 命令通道，和主机终端不是同一路。"},
	}
}
