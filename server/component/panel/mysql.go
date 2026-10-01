package panel

func mysqlPanels() []Panel {
	return []Panel{
		{KindCode: "mysql", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "连通、角色和版本。"},
		{KindCode: "mysql", PanelCode: "connections", PanelName: "连接", SortOrder: 20, List: true, Metrics: []string{"threadsConnected"}, Sections: []string{"connections"}, NoteText: "连接数和活跃线程。"},
		{KindCode: "mysql", PanelCode: "slow", PanelName: "慢查询", SortOrder: 30, List: true, Metrics: []string{"slowQueries"}, Sections: []string{"slow"}, NoteText: "缓冲池命中和累计慢查询。"},
		{KindCode: "mysql", PanelCode: "replica", PanelName: "复制", SortOrder: 40, List: true, Metrics: []string{"replicaLagSec"}, Sections: []string{"replica"}, NoteText: "复制延迟。"},
		{KindCode: "mysql", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "MySQL SQL 客户端。"},
	}
}
