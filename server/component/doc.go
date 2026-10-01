// Package component 是中央对组件的直连采集。
//
// 主机和 Java 仍由代理上报。数据库、Redis、Kafka 这类组件在控制台填写连接，由本进程拨号。
//
//	catalog   组件目录。种类、默认端口和指标名以代码为准，并写入 FG_COMPONENT_KIND
//	conn      连接表 FG_COMPONENT_CONN。口令加密存放，查询不返回明文
//	sched     按每条连接的间隔拨号。一台拨不通只记下这一条
//	redis mysql mongo kafka zk nacos
//	          各自采集，写入对应的 FG_*_METRIC，只插入
//
// 控制台模块是 fg0007。
package component
