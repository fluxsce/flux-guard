// Package catalog 是组件目录。采集器按这里的种类编码注册，指标名给页面和曲线共用。
package catalog

// 种类编码与采集包、采集表一一对应。
const (
	Redis = "redis"
	MySQL = "mysql"
	Mongo = "mongo"
	Kafka = "kafka"
	ZK    = "zk"
	Nacos = "nacos"
)

// Kind 与 models.Kind 字段一致，避免 catalog 再依赖 models 造成循环。
type Kind struct {
	KindCode    string
	KindName    string
	DefaultPort int
	MetricTable string
	PaneTable   string
	SortOrder   int
	NoteText    string
	Metrics     []Metric
}

// Metric 是一项要入库的数值。
type Metric struct {
	Code string
	Name string
	Unit string
}

// All 返回固定目录，顺序即控制台展示顺序。
func All() []Kind {
	return []Kind{
		{
			KindCode: Redis, KindName: "Redis", DefaultPort: 6379, MetricTable: "FG_COMPONENT_REDIS_METRIC", PaneTable: "FG_COMPONENT_REDIS", SortOrder: 10,
			NoteText: "单机、哨兵或集群。认证可以不填、只填口令，或使用 ACL 用户名。",
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
		{
			KindCode: MySQL, KindName: "MySQL", DefaultPort: 3306, MetricTable: "FG_COMPONENT_MYSQL_METRIC", PaneTable: "FG_COMPONENT_MYSQL", SortOrder: 20,
			NoteText: "只读账号即可。库名可空。复制延迟看 Seconds_Behind_Source。",
			Metrics: []Metric{
				{"threadsConnected", "连接数", ""},
				{"threadsRunning", "活跃线程", ""},
				{"questions", "累计查询", ""},
				{"slowQueries", "累计慢查询", ""},
				{"bufferPoolHitRate", "缓冲池命中率", "%"},
				{"replicaLagSec", "复制延迟", "s"},
				{"uptimeSec", "运行秒数", "s"},
			},
		},
		{
			KindCode: Mongo, KindName: "MongoDB", DefaultPort: 27017, MetricTable: "FG_COMPONENT_MONGO_METRIC", PaneTable: "FG_COMPONENT_MONGO", SortOrder: 30,
			NoteText: "当前拨号不走 SCRAM。需要口令的实例会记为连不上。",
			Metrics: []Metric{
				{"connectionsCurrent", "当前连接", ""},
				{"connectionsAvailable", "可用连接", ""},
				{"opInsert", "累计插入", ""},
				{"opQuery", "累计查询", ""},
				{"opUpdate", "累计更新", ""},
				{"opDelete", "累计删除", ""},
			},
		},
		{
			KindCode: Kafka, KindName: "Kafka", DefaultPort: 9092, MetricTable: "FG_COMPONENT_KAFKA_METRIC", PaneTable: "FG_COMPONENT_KAFKA", SortOrder: 40,
			NoteText: "地址可写多个 broker，逗号分隔。传输可选明文或 TLS。认证可以不填，或使用 SASL PLAIN、SCRAM-SHA-256、SCRAM-SHA-512。",
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
		{
			KindCode: ZK, KindName: "ZooKeeper", DefaultPort: 2181, MetricTable: "FG_COMPONENT_ZK_METRIC", PaneTable: "FG_COMPONENT_ZK", SortOrder: 50,
			NoteText: "使用四字命令 mntr，服务端需要打开这项。",
			Metrics: []Metric{
				{"avgLatency", "平均延迟", "ms"},
				{"outstandingRequests", "堆积请求", ""},
				{"znodeCount", "节点数", ""},
				{"followers", "跟随者", ""},
				{"watchCount", "Watch 数", ""},
			},
		},
		{
			KindCode: Nacos, KindName: "Nacos", DefaultPort: 8848, MetricTable: "FG_COMPONENT_NACOS_METRIC", PaneTable: "FG_COMPONENT_NACOS", SortOrder: 60,
			NoteText: "地址填 host:port 或 http 地址。有用户名时先登录再看健康检查。",
			Metrics: []Metric{
				{"configCount", "配置数", ""},
				{"serviceCount", "服务数", ""},
			},
		},
	}
}

// Get 按编码取目录项。
func Get(code string) (Kind, bool) {
	for _, item := range All() {
		if item.KindCode == code {
			return item, true
		}
	}
	return Kind{}, false
}

// Known 判断种类是否在目录里。
func Known(code string) bool {
	_, ok := Get(code)
	return ok
}

// Columns 返回该种类要写入曲线的数值列，顺序固定。
func Columns(code string) []string {
	item, ok := Get(code)
	if !ok {
		return nil
	}
	out := make([]string, len(item.Metrics))
	for i, metric := range item.Metrics {
		out[i] = metric.Code
	}
	return out
}
