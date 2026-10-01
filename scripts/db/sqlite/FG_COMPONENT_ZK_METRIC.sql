-- 已有旧表时改名。新库没有旧表，重复启动时忽略这次失败。
ALTER TABLE FG_ZK_METRIC RENAME TO FG_COMPONENT_ZK_METRIC;
-- ZooKeeper 采集记录。每次拨号只插入。
CREATE TABLE IF NOT EXISTS FG_COMPONENT_ZK_METRIC (
  tenantId TEXT NOT NULL,
  pointId TEXT NOT NULL,
  componentId TEXT NOT NULL,
  upFlag TEXT NOT NULL DEFAULT 'N',
  versionText TEXT,
  roleName TEXT,
  errorText TEXT,
  summaryText TEXT,
  observedTime TEXT NOT NULL,
  avgLatency REAL NOT NULL DEFAULT 0,
  outstandingRequests REAL NOT NULL DEFAULT 0,
  znodeCount REAL NOT NULL DEFAULT 0,
  followers REAL NOT NULL DEFAULT 0,
  watchCount REAL NOT NULL DEFAULT 0,
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
  PRIMARY KEY (tenantId, pointId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_COMPONENT_ZK_METRIC_TIME ON FG_COMPONENT_ZK_METRIC(tenantId, componentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_COMPONENT_ZK_METRIC_ADD ON FG_COMPONENT_ZK_METRIC(tenantId, addTime);
UPDATE FG_COMPONENT_KIND SET metricTable = 'FG_COMPONENT_ZK_METRIC' WHERE kindCode = 'zk' AND metricTable = 'FG_ZK_METRIC';
