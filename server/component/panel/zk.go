package panel

func zkPanels() []Panel {
	return []Panel{
		{KindCode: "zk", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "连通、角色和版本。"},
		{KindCode: "zk", PanelCode: "latency", PanelName: "延迟", SortOrder: 20, List: true, Metrics: []string{"avgLatency"}, Sections: []string{"latency"}, NoteText: "平均延迟和堆积请求。"},
		{KindCode: "zk", PanelCode: "scale", PanelName: "规模", SortOrder: 30, List: true, Metrics: []string{"znodeCount"}, Sections: []string{"scale"}, NoteText: "节点、跟随者和 Watch。"},
		{KindCode: "zk", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "ZooKeeper 四字命令和节点浏览。"},
	}
}
