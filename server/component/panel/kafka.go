package panel

func kafkaPanels() []Panel {
	return []Panel{
		{KindCode: "kafka", PanelCode: "overview", PanelName: "概览", SortOrder: 10, NoteText: "Broker、离线分区、未同步分区和日志占用。"},
		{KindCode: "kafka", PanelCode: "broker", PanelName: "Broker", SortOrder: 15, NoteText: "每台 Broker 的地址、控制器、连通和日志占用。"},
		{KindCode: "kafka", PanelCode: "topic", PanelName: "Topic", SortOrder: 20, List: true, Metrics: []string{"topicCount"}, NoteText: "主题、副本、离线、未同步和最小 ISR。"},
		{KindCode: "kafka", PanelCode: "group", PanelName: "消费组", SortOrder: 30, List: true, Metrics: []string{"maxLag"}, Sections: []string{"consume"}, NoteText: "消费组状态、成员和分区延迟。"},
		{KindCode: "kafka", PanelCode: "terminal", PanelName: "终端", SortOrder: 90, Terminal: true, NoteText: "Kafka 主题和消费组操作，不是系统终端。"},
	}
}
