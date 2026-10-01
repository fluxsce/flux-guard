// Package component 是中央对组件的直连采集。
//
// 主机和 Java 仍由代理上报。数据库、Redis、Kafka 这类组件在控制台填写连接，由本进程拨号。
//
//	catalog   组件目录。种类、默认端口和指标名以代码为准。初始行在 FG_COMPONENT_KIND.sql
//	panel     每种组件的监控页签。终端按种类分开。初始行在 FG_COMPONENT_PANEL.sql
//	conn      连接表 FG_COMPONENT_CONN。口令加密存放，查询不返回明文
//	sched     按每条连接的间隔拨号。一台拨不通只记下这一条
//	redis mysql mongo kafka zk nacos
//	          各自采集。曲线写入对应的 FG_COMPONENT_*_METRIC，只插入。
//	          列表要看的当前值覆盖到 FG_COMPONENT_*，每个连接一行。
//	pane      当前值表的写入和单表读取。不看监控时不扫采集表。
//
// 控制台模块是 fg0007。
package component
