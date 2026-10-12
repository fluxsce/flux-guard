-- 组件目录。初始行在本脚本，启动时不再补。
CREATE TABLE IF NOT EXISTS `FG_COMPONENT_KIND` (
  `kindCode` VARCHAR(32) NOT NULL COMMENT '种类编码，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `kindName` VARCHAR(100) NOT NULL COMMENT '种类名称',
  `defaultPort` INT NOT NULL DEFAULT 0 COMMENT '默认端口',
  `metricTable` VARCHAR(64) NOT NULL COMMENT '采集表名',
  `sortOrder` INT NOT NULL DEFAULT 0 COMMENT '排序',
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
  PRIMARY KEY (`tenantId`, `kindCode`),
  KEY `IDX_FG_COMPONENT_KIND_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='组件目录表';

INSERT INTO `FG_COMPONENT_KIND` (
  `tenantId`, `kindCode`, `kindName`, `defaultPort`, `metricTable`, `sortOrder`, `statusFlag`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES
('default', 'redis', 'Redis', 6379, 'FG_COMPONENT_REDIS_METRIC', 10, 'Y', 'system', 'system', 'INIT', 1, 'Y', '地址填 host:port。有口令时走 AUTH。'),
('default', 'mysql', 'MySQL', 3306, 'FG_COMPONENT_MYSQL_METRIC', 20, 'Y', 'system', 'system', 'INIT', 1, 'Y', '只读账号即可。库名可空。复制延迟看 Seconds_Behind_Source。'),
('default', 'mongo', 'MongoDB', 27017, 'FG_COMPONENT_MONGO_METRIC', 30, 'Y', 'system', 'system', 'INIT', 1, 'Y', '传输可选明文或 TLS。认证可以不填，或使用 SCRAM-SHA-256、SCRAM-SHA-1、LDAP PLAIN、X.509、AWS IAM、OIDC。'),
('default', 'kafka', 'Kafka', 9092, 'FG_COMPONENT_KAFKA_METRIC', 40, 'Y', 'system', 'system', 'INIT', 1, 'Y', '地址可写多个 broker，逗号分隔。传输可选明文或 TLS。认证可以不填，或使用 SASL PLAIN、SCRAM-SHA-256、SCRAM-SHA-512。'),
('default', 'zk', 'ZooKeeper', 2181, 'FG_COMPONENT_ZK_METRIC', 50, 'Y', 'system', 'system', 'INIT', 1, 'Y', '使用四字命令 mntr，服务端需要打开这项。'),
('default', 'nacos', 'Nacos', 8848, 'FG_COMPONENT_NACOS_METRIC', 60, 'Y', 'system', 'system', 'INIT', 1, 'Y', '地址填 host:port 或 http 地址。有用户名时先登录再看健康检查。');

UPDATE `FG_COMPONENT_KIND`
SET `noteText` = '传输可选明文或 TLS。认证可以不填，或使用 SCRAM-SHA-256、SCRAM-SHA-1、LDAP PLAIN、X.509、AWS IAM、OIDC。'
WHERE `tenantId` = 'default' AND `kindCode` = 'mongo';
