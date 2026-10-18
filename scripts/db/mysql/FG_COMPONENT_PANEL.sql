-- 组件监控页签。初始行在本脚本，启动时不再补。
-- 概览收走其它页签没有点名的分节。listFlag=Y 的指标出现在该种类的实例列表上。
-- terminalFlag=Y 的页签按种类接入各自的终端，按钮码是 fg0007:shell。
CREATE TABLE IF NOT EXISTS `FG_COMPONENT_PANEL` (
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `panelId` VARCHAR(64) NOT NULL COMMENT '页签ID，种类:编码，联合主键',
  `kindCode` VARCHAR(32) NOT NULL COMMENT '种类编码',
  `panelCode` VARCHAR(32) NOT NULL COMMENT '页签编码',
  `panelName` VARCHAR(100) NOT NULL COMMENT '页签名称',
  `sortOrder` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `terminalFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '终端页(Y是,N否)',
  `listFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '列表当前值(Y显示,N不显示)',
  `metricText` VARCHAR(255) DEFAULT NULL COMMENT '列表指标编码，逗号分隔',
  `sectionText` VARCHAR(255) DEFAULT NULL COMMENT '分节编码，逗号分隔。概览为空，表示收走未被点名的分节',
  `statusFlag` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '状态(Y启用,N停用)',
  `addTime` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `addWho` VARCHAR(32) NOT NULL COMMENT '创建人ID',
  `editTime` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后修改时间',
  `editWho` VARCHAR(32) NOT NULL COMMENT '最后修改人ID',
  `oprSeqFlag` VARCHAR(32) NOT NULL COMMENT '操作序列标识',
  `currentVersion` INT NOT NULL DEFAULT 1 COMMENT '当前版本号',
  `activeFlag` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '活动状态标记(N非活动,Y活动)',
  `noteText` VARCHAR(500) DEFAULT NULL COMMENT '备注信息',
  `extProperty` TEXT DEFAULT NULL COMMENT '扩展属性，JSON格式',
  `reserved1` VARCHAR(64) DEFAULT NULL COMMENT '预留字段1',
  `reserved2` VARCHAR(64) DEFAULT NULL COMMENT '预留字段2',
  `reserved3` VARCHAR(100) DEFAULT NULL COMMENT '预留字段3',
  `reserved4` VARCHAR(100) DEFAULT NULL COMMENT '预留字段4',
  `reserved5` VARCHAR(255) DEFAULT NULL COMMENT '预留字段5',
  PRIMARY KEY (`tenantId`, `panelId`),
  KEY `IDX_FG_COMPONENT_PANEL_KIND` (`tenantId`, `kindCode`, `sortOrder`),
  KEY `IDX_FG_COMPONENT_PANEL_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='组件监控页签表';

INSERT INTO `FG_COMPONENT_PANEL` (
  `tenantId`, `panelId`, `kindCode`, `panelCode`, `panelName`, `sortOrder`,
  `terminalFlag`, `listFlag`, `metricText`, `sectionText`, `statusFlag`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES
('default', 'redis:overview', 'redis', 'overview', '概览', 10, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '连通、访问和持久化。集群或哨兵的状态也在这一页。'),
('default', 'redis:memory', 'redis', 'memory', '内存', 20, 'N', 'Y', 'usedMemoryBytes,memUsagePercent,aofOk', 'memory', 'Y', 'system', 'system', 'INIT', 1, 'Y', '已用、上限占用、峰值和碎片率。列表同时看 AOF 是否开启。'),
('default', 'redis:traffic', 'redis', 'traffic', '流量', 30, 'N', 'N', NULL, 'traffic', 'Y', 'system', 'system', 'INIT', 1, 'Y', '入站和出站。'),
('default', 'redis:slow', 'redis', 'slow', '慢操作', 40, 'N', 'Y', 'slowlogLen', 'slow', 'Y', 'system', 'system', 'INIT', 1, 'Y', '慢操作条数和最近命令。'),
('default', 'redis:replica', 'redis', 'replica', '复制', 50, 'N', 'N', NULL, 'replica', 'Y', 'system', 'system', 'INIT', 1, 'Y', '从节点、复制延迟和主链路。'),
('default', 'redis:keys', 'redis', 'keys', '键', 60, 'N', 'N', NULL, 'keys', 'Y', 'system', 'system', 'INIT', 1, 'Y', '键数量、淘汰和各库键数量。'),
('default', 'redis:terminal', 'redis', 'terminal', '终端', 90, 'Y', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '按库浏览键。可按类型查看，删除，或设置过期时间。'),
('default', 'mysql:overview', 'mysql', 'overview', '概览', 10, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '连通、角色和版本。'),
('default', 'mysql:connections', 'mysql', 'connections', '连接', 20, 'N', 'Y', 'threadsConnected,connUsagePercent', 'connections', 'Y', 'system', 'system', 'INIT', 1, 'Y', '连接数、上限占用和活跃线程。'),
('default', 'mysql:ops', 'mysql', 'ops', '吞吐', 30, 'N', 'Y', 'questionsPerSec', 'throughput', 'Y', 'system', 'system', 'INIT', 1, 'Y', '两次采集之间的每秒语句。'),
('default', 'mysql:traffic', 'mysql', 'traffic', '流量', 35, 'N', 'N', NULL, 'traffic', 'Y', 'system', 'system', 'INIT', 1, 'Y', '入站和出站。'),
('default', 'mysql:innodb', 'mysql', 'innodb', '缓冲池', 40, 'N', 'N', NULL, 'innodb', 'Y', 'system', 'system', 'INIT', 1, 'Y', '两次采集之间的缓冲池命中、脏页和磁盘临时表。'),
('default', 'mysql:slow', 'mysql', 'slow', '慢查询', 50, 'N', 'Y', 'slowPerSec', 'slow', 'Y', 'system', 'system', 'INIT', 1, 'Y', '两次采集之间的慢查询，以及正在执行的语句。'),
('default', 'mysql:replica', 'mysql', 'replica', '复制', 60, 'N', 'Y', 'replicaLagSec', 'replica', 'Y', 'system', 'system', 'INIT', 1, 'Y', '复制延迟和复制线程。'),
('default', 'mysql:terminal', 'mysql', 'terminal', '终端', 90, 'Y', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', 'MySQL SQL 客户端。'),
('default', 'mongo:overview', 'mongo', 'overview', '概览', 10, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '连通、角色、版本和排队。'),
('default', 'mongo:connections', 'mongo', 'connections', '连接', 20, 'N', 'Y', 'connectionsCurrent', 'connections', 'Y', 'system', 'system', 'INIT', 1, 'Y', '当前连接、可用连接和占用。'),
('default', 'mongo:memory', 'mongo', 'memory', '内存', 30, 'N', 'Y', 'cacheUsedBytes', 'memory', 'Y', 'system', 'system', 'INIT', 1, 'Y', '常驻内存和 WiredTiger 缓存。列表看缓存已用。'),
('default', 'mongo:ops', 'mongo', 'ops', '操作', 40, 'N', 'Y', 'opsPerSec', 'ops', 'Y', 'system', 'system', 'INIT', 1, 'Y', '两次采集之间的每秒操作。'),
('default', 'mongo:traffic', 'mongo', 'traffic', '流量', 50, 'N', 'N', NULL, 'traffic', 'Y', 'system', 'system', 'INIT', 1, 'Y', '入站和出站。'),
('default', 'mongo:replica', 'mongo', 'replica', '复制', 60, 'N', 'Y', 'replicaLagSec', 'replica', 'Y', 'system', 'system', 'INIT', 1, 'Y', '复制延迟和成员状态。'),
('default', 'mongo:storage', 'mongo', 'storage', '库', 70, 'N', 'N', NULL, 'storage', 'Y', 'system', 'system', 'INIT', 1, 'Y', '各库占用。'),
('default', 'mongo:slow', 'mongo', 'slow', '慢操作', 80, 'N', 'Y', 'slowOps', 'slow', 'Y', 'system', 'system', 'INIT', 1, 'Y', '正在执行且已超过 1 秒的操作。'),
('default', 'mongo:terminal', 'mongo', 'terminal', '终端', 90, 'Y', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '库、集合和索引。可查询文档，并看历史慢日志。'),
('default', 'kafka:overview', 'kafka', 'overview', '概览', 10, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', 'Broker、离线分区、未同步分区和日志占用。'),
('default', 'kafka:broker', 'kafka', 'broker', 'Broker', 15, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '每台 Broker 的地址、控制器、连通、负责的分区、副本和日志占用。'),
('default', 'kafka:topic', 'kafka', 'topic', 'Topic', 20, 'N', 'Y', 'topicCount', NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '分区数、副本因子、离线分区、未同步副本、首选副本偏离、低于最小 ISR，异常分区写在同一行。'),
('default', 'kafka:group', 'kafka', 'group', '消费组', 30, 'N', 'Y', 'maxLag', 'consume', 'Y', 'system', 'system', 'INIT', 1, 'Y', '与 kafka-consumer-groups --describe 相同：主题、分区、当前位点、日志末端、延迟、消费者、主机和客户端。'),
('default', 'kafka:terminal', 'kafka', 'terminal', '终端', 90, 'Y', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '主题配置、新建、删除、增加分区，以及消费组删除和重置位点。'),
('default', 'zk:overview', 'zk', 'overview', '概览', 10, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '连通、角色和版本。'),
('default', 'zk:latency', 'zk', 'latency', '延迟', 20, 'N', 'Y', 'avgLatency', 'latency', 'Y', 'system', 'system', 'INIT', 1, 'Y', '平均延迟和堆积请求。'),
('default', 'zk:scale', 'zk', 'scale', '规模', 30, 'N', 'Y', 'znodeCount', 'scale', 'Y', 'system', 'system', 'INIT', 1, 'Y', '节点、跟随者和 Watch。'),
('default', 'zk:terminal', 'zk', 'terminal', '终端', 90, 'Y', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', 'ZooKeeper 四字命令和节点浏览。'),
('default', 'nacos:overview', 'nacos', 'overview', '概览', 10, 'N', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', '连通情况。'),
('default', 'nacos:registry', 'nacos', 'registry', '配置与服务', 20, 'N', 'Y', 'configCount,serviceCount', 'registry', 'Y', 'system', 'system', 'INIT', 1, 'Y', '配置数和服务数。'),
('default', 'nacos:terminal', 'nacos', 'terminal', '终端', 90, 'Y', 'N', NULL, NULL, 'Y', 'system', 'system', 'INIT', 1, 'Y', 'Nacos 控制台操作。');
UPDATE `FG_COMPONENT_PANEL` SET `noteText` = '连通、角色、版本和排队。' WHERE `tenantId` = 'default' AND `panelId` = 'mongo:overview';
UPDATE `FG_COMPONENT_PANEL` SET `metricText` = 'connectionsCurrent', `noteText` = '当前连接、可用连接和占用。' WHERE `tenantId` = 'default' AND `panelId` = 'mongo:connections';
UPDATE `FG_COMPONENT_PANEL` SET `panelName` = '操作', `sortOrder` = 40, `metricText` = 'opsPerSec', `sectionText` = 'ops', `noteText` = '两次采集之间的每秒操作。' WHERE `tenantId` = 'default' AND `panelId` = 'mongo:ops';
UPDATE `FG_COMPONENT_PANEL` SET `noteText` = '库、集合和索引。可查询文档，并看历史慢日志。' WHERE `tenantId` = 'default' AND `panelId` = 'mongo:terminal';
UPDATE `FG_COMPONENT_PANEL` SET `noteText` = '主题配置、新建、删除、增加分区，以及消费组删除和重置位点。' WHERE `tenantId` = 'default' AND `panelId` = 'kafka:terminal';
UPDATE `FG_COMPONENT_PANEL` SET `noteText` = '按库浏览键。可按类型查看，删除，或设置过期时间。' WHERE `tenantId` = 'default' AND `panelId` = 'redis:terminal';
UPDATE `FG_COMPONENT_PANEL` SET `metricText` = 'threadsConnected,connUsagePercent', `noteText` = '连接数、上限占用和活跃线程。' WHERE `tenantId` = 'default' AND `panelId` = 'mysql:connections';
UPDATE `FG_COMPONENT_PANEL` SET `sortOrder` = 50, `metricText` = 'slowPerSec', `noteText` = '两次采集之间的慢查询，以及正在执行的语句。' WHERE `tenantId` = 'default' AND `panelId` = 'mysql:slow';
UPDATE `FG_COMPONENT_PANEL` SET `sortOrder` = 60, `noteText` = '复制延迟和复制线程。' WHERE `tenantId` = 'default' AND `panelId` = 'mysql:replica';
INSERT INTO `FG_COMPONENT_PANEL` (
  `tenantId`, `panelId`, `kindCode`, `panelCode`, `panelName`, `sortOrder`,
  `terminalFlag`, `listFlag`, `metricText`, `sectionText`, `statusFlag`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES
('default', 'mysql:ops', 'mysql', 'ops', '吞吐', 30, 'N', 'Y', 'questionsPerSec', 'throughput', 'Y', 'system', 'system', 'INIT', 1, 'Y', '两次采集之间的每秒语句。'),
('default', 'mysql:traffic', 'mysql', 'traffic', '流量', 35, 'N', 'N', NULL, 'traffic', 'Y', 'system', 'system', 'INIT', 1, 'Y', '入站和出站。'),
('default', 'mysql:innodb', 'mysql', 'innodb', '缓冲池', 40, 'N', 'N', NULL, 'innodb', 'Y', 'system', 'system', 'INIT', 1, 'Y', '两次采集之间的缓冲池命中、脏页和磁盘临时表。');
