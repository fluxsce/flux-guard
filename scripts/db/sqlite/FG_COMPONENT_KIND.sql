-- 组件目录。初始行在本脚本，启动时不再补。
CREATE TABLE IF NOT EXISTS FG_COMPONENT_KIND (
  kindCode TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  kindName TEXT NOT NULL,
  defaultPort INTEGER NOT NULL DEFAULT 0,
  metricTable TEXT NOT NULL,
  sortOrder INTEGER NOT NULL DEFAULT 0,
  statusFlag TEXT NOT NULL DEFAULT 'Y',
  addTime TEXT NOT NULL DEFAULT (datetime('now')),
  addWho TEXT NOT NULL,
  editTime TEXT NOT NULL DEFAULT (datetime('now')),
  editWho TEXT NOT NULL,
  oprSeqFlag TEXT NOT NULL,
  currentVersion INTEGER NOT NULL DEFAULT 1,
  activeFlag TEXT NOT NULL DEFAULT 'Y',
  noteText TEXT,
  extProperty TEXT,
  reserved1 TEXT,
  reserved2 TEXT,
  reserved3 TEXT,
  reserved4 TEXT,
  reserved5 TEXT,
  PRIMARY KEY (tenantId, kindCode)
);
CREATE INDEX IF NOT EXISTS IDX_FG_COMPONENT_KIND_ADD ON FG_COMPONENT_KIND(tenantId, addTime);

INSERT INTO FG_COMPONENT_KIND (
  tenantId, kindCode, kindName, defaultPort, metricTable, sortOrder, statusFlag,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
) VALUES
('default', 'redis', 'Redis', 6379, 'FG_COMPONENT_REDIS_METRIC', 10, 'Y', 'system', 'system', 'INIT', 1, 'Y', '地址填 host:port。有口令时走 AUTH。'),
('default', 'mysql', 'MySQL', 3306, 'FG_COMPONENT_MYSQL_METRIC', 20, 'Y', 'system', 'system', 'INIT', 1, 'Y', '只读账号即可。库名可空。复制延迟看 Seconds_Behind_Source。'),
('default', 'mongo', 'MongoDB', 27017, 'FG_COMPONENT_MONGO_METRIC', 30, 'Y', 'system', 'system', 'INIT', 1, 'Y', '当前拨号不走 SCRAM。需要口令的实例会记为连不上。'),
('default', 'kafka', 'Kafka', 9092, 'FG_COMPONENT_KAFKA_METRIC', 40, 'Y', 'system', 'system', 'INIT', 1, 'Y', '地址可写多个 broker，逗号分隔。传输可选明文或 TLS。认证可以不填，或使用 SASL PLAIN、SCRAM-SHA-256、SCRAM-SHA-512。'),
('default', 'zk', 'ZooKeeper', 2181, 'FG_COMPONENT_ZK_METRIC', 50, 'Y', 'system', 'system', 'INIT', 1, 'Y', '使用四字命令 mntr，服务端需要打开这项。'),
('default', 'nacos', 'Nacos', 8848, 'FG_COMPONENT_NACOS_METRIC', 60, 'Y', 'system', 'system', 'INIT', 1, 'Y', '地址填 host:port 或 http 地址。有用户名时先登录再看健康检查。');
