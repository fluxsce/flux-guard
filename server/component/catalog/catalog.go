// Package catalog 是本进程能采集的种类。
// 种类名称、端口、备注和页签在 FG_COMPONENT_KIND、FG_COMPONENT_PANEL，不在这里再写一份。
// 指标编码和单位是采集列，种类表里没有这些字段。
package catalog

// 种类编码与采集包一一对应。
const (
	Redis = "redis"
	MySQL = "mysql"
	Mongo = "mongo"
	Kafka = "kafka"
	ZK    = "zk"
	Nacos = "nacos"
)

// Metric 是一项要入库的数值。
type Metric struct {
	Code string
	Name string
	Unit string
}

// binding 是一种组件的采集表和指标列。
type binding struct {
	MetricTable string
	PaneTable   string
	Metrics     []Metric
}

var bindings = map[string]binding{
	Redis: {
		MetricTable: "FG_COMPONENT_REDIS_METRIC", PaneTable: "FG_COMPONENT_REDIS",
		Metrics: []Metric{
			{"usedMemoryBytes", "已用内存", "byte"},
			{"connectedClients", "连接数", ""},
			{"opsPerSec", "每秒操作", ""},
			{"hitRate", "命中率", "%"},
			{"connectedSlaves", "从节点", ""},
			{"knownNodes", "节点数", ""},
			{"failSlots", "失败槽", ""},
			{"memFragmentation", "碎片率", ""},
			{"keyCount", "键数量", ""},
			{"slowlogLen", "慢操作", ""},
			{"uptimeSec", "运行秒数", "s"},
			{"maxMemoryBytes", "内存上限", "byte"},
			{"memUsagePercent", "内存占用", "%"},
			{"usedMemoryPeakBytes", "内存峰值", "byte"},
			{"blockedClients", "阻塞客户端", ""},
			{"netInKbps", "入站", "KB/s"},
			{"netOutKbps", "出站", "KB/s"},
			{"replicaLagSec", "复制延迟", "s"},
			{"masterLinkDown", "主链路断开", ""},
			{"rdbOk", "RDB", ""},
			{"aofOk", "AOF", ""},
			{"evictedPerSec", "淘汰", "/s"},
			{"expiredPerSec", "过期", "/s"},
			{"rejectedPerSec", "拒绝连接", "/s"},
		},
	},
	MySQL: {
		MetricTable: "FG_COMPONENT_MYSQL_METRIC", PaneTable: "FG_COMPONENT_MYSQL",
		Metrics: []Metric{
			{"threadsConnected", "连接数", ""},
			{"threadsRunning", "活跃线程", ""},
			{"questions", "累计查询", ""},
			{"slowQueries", "累计慢查询", ""},
			{"bufferPoolHitRate", "缓冲池命中率", "%"},
			{"replicaLagSec", "复制延迟", "s"},
			{"uptimeSec", "运行秒数", "s"},
			{"questionsPerSec", "每秒语句", "/s"},
			{"slowPerSec", "每秒慢查询", "/s"},
			{"selectPerSec", "每秒查询", "/s"},
			{"insertPerSec", "每秒插入", "/s"},
			{"updatePerSec", "每秒更新", "/s"},
			{"deletePerSec", "每秒删除", "/s"},
			{"netInKbps", "入站", "KB/s"},
			{"netOutKbps", "出站", "KB/s"},
			{"connUsagePercent", "连接占用", "%"},
			{"maxConnections", "连接上限", ""},
			{"abortedPerSec", "失败连接", "/s"},
			{"lockWaitsPerSec", "行锁等待", "/s"},
			{"bufferPoolDirtyPercent", "脏页占比", "%"},
			{"tmpDiskPercent", "磁盘临时表", "%"},
			{"replicaIo", "IO 线程", ""},
			{"replicaSql", "SQL 线程", ""},
		},
	},
	Mongo: {
		MetricTable: "FG_COMPONENT_MONGO_METRIC", PaneTable: "FG_COMPONENT_MONGO",
		Metrics: []Metric{
			{"connectionsCurrent", "当前连接", ""},
			{"connectionsAvailable", "可用连接", ""},
			{"connectionsActive", "活跃连接", ""},
			{"connUsagePercent", "连接占用", "%"},
			{"residentBytes", "常驻内存", "byte"},
			{"cacheUsedBytes", "缓存已用", "byte"},
			{"cacheMaxBytes", "缓存上限", "byte"},
			{"cacheDirtyBytes", "脏数据", "byte"},
			{"cacheUsagePercent", "缓存占用", "%"},
			{"cacheDirtyPercent", "脏数据占比", "%"},
			{"opsPerSec", "每秒操作", "/s"},
			{"opInsertPerSec", "每秒插入", "/s"},
			{"opQueryPerSec", "每秒查询", "/s"},
			{"opUpdatePerSec", "每秒更新", "/s"},
			{"opDeletePerSec", "每秒删除", "/s"},
			{"opCommandPerSec", "每秒命令", "/s"},
			{"opGetmorePerSec", "每秒取更多", "/s"},
			{"replicaLagSec", "复制延迟", "s"},
			{"memberCount", "成员", ""},
			{"unhealthyMembers", "异常成员", ""},
			{"netInKbps", "入站", "KB/s"},
			{"netOutKbps", "出站", "KB/s"},
			{"queuedReaders", "读排队", ""},
			{"queuedWriters", "写排队", ""},
			{"readTickets", "读票据", ""},
			{"writeTickets", "写票据", ""},
			{"assertPerSec", "断言", "/s"},
			{"slowOps", "慢操作", ""},
			{"uptimeSec", "运行秒数", "s"},
			{"dataBytes", "数据占用", "byte"},
		},
	},
	Kafka: {
		MetricTable: "FG_COMPONENT_KAFKA_METRIC", PaneTable: "FG_COMPONENT_KAFKA",
		Metrics: []Metric{
			{"brokerCount", "Broker 数", ""},
			{"onlineBrokers", "可达 Broker", ""},
			{"offlineBrokers", "不可达 Broker", ""},
			{"offlinePartitions", "离线分区", ""},
			{"underReplicated", "未同步分区", ""},
			{"preferredReplicas", "首选副本偏离", ""},
			{"underMinIsr", "低于最小 ISR", ""},
			{"topicCount", "Topic 数", ""},
			{"groupCount", "消费组数", ""},
			{"rebalanceGroups", "再均衡", ""},
			{"maxLag", "最大分区延迟", ""},
			{"totalLag", "总延迟", ""},
			{"logBytes", "日志占用", "byte"},
		},
	},
	ZK: {
		MetricTable: "FG_COMPONENT_ZK_METRIC", PaneTable: "FG_COMPONENT_ZK",
		Metrics: []Metric{
			{"avgLatency", "平均延迟", "ms"},
			{"outstandingRequests", "堆积请求", ""},
			{"znodeCount", "节点数", ""},
			{"followers", "跟随者", ""},
			{"watchCount", "Watch 数", ""},
		},
	},
	Nacos: {
		MetricTable: "FG_COMPONENT_NACOS_METRIC", PaneTable: "FG_COMPONENT_NACOS",
		Metrics: []Metric{
			{"configCount", "配置数", ""},
			{"serviceCount", "服务数", ""},
		},
	},
}

var codes = []string{Redis, MySQL, Mongo, Kafka, ZK, Nacos}

// Codes 返回本进程登记的种类，顺序固定。
func Codes() []string {
	out := make([]string, len(codes))
	copy(out, codes)
	return out
}

// Known 判断本进程是否采集这种组件。
func Known(code string) bool {
	_, ok := bindings[code]
	return ok
}

// Metrics 返回该种类的指标名，供页面给曲线列命名。
func Metrics(code string) []Metric {
	item, ok := bindings[code]
	if !ok {
		return nil
	}
	out := make([]Metric, len(item.Metrics))
	copy(out, item.Metrics)
	return out
}

// Columns 返回该种类要写入曲线的数值列，顺序固定。
func Columns(code string) []string {
	item, ok := bindings[code]
	if !ok {
		return nil
	}
	out := make([]string, len(item.Metrics))
	for i, metric := range item.Metrics {
		out[i] = metric.Code
	}
	return out
}

// MetricTable 返回曲线表。表名与采集包里的插入语句一致。
func MetricTable(code string) (string, bool) {
	item, ok := bindings[code]
	if !ok || item.MetricTable == "" {
		return "", false
	}
	return item.MetricTable, true
}

// PaneTable 返回当前值表。每个连接一行。
func PaneTable(code string) (string, bool) {
	item, ok := bindings[code]
	if !ok || item.PaneTable == "" {
		return "", false
	}
	return item.PaneTable, true
}
