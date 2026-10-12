// Package component 是中央对组件的直连采集。
//
// 主机和 Java 仍由代理上报。数据库、Redis、Kafka 这类组件在控制台填写连接，由本进程拨号。
//
//	catalog   本进程的采集列。指标编码在代码里。种类名称、端口和备注读 FG_COMPONENT_KIND
//	panel     监控页签读 FG_COMPONENT_PANEL。终端按种类分开。启动时不另写页签
//	conn      连接表 FG_COMPONENT_CONN。口令加密存放，查询不返回明文
//	sched     按每条连接的间隔拨号。一台拨不通只记下这一条
//	redis mysql mongo zk nacos
//	          编进本进程的采集。Kafka 由 plugins/kafka 里的官方客户端插件采集。
//	          曲线写入对应的 FG_COMPONENT_*_METRIC，只插入。
//	          列表要看的当前值覆盖到 FG_COMPONENT_*，每个连接一行。
//	          换成独立进程时，server/plugin 只拉起进程并交回结果，bridge 按结果写入采集表。
//	pane      当前值表的写入和单表读取。不看监控时不扫采集表。
//
// 控制台模块是 fg0007。
package component
