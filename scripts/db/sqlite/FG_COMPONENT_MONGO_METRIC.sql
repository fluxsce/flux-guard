-- 已有旧表时改名。新库没有旧表，重复启动时忽略这次失败。
ALTER TABLE FG_MONGO_METRIC RENAME TO FG_COMPONENT_MONGO_METRIC;
-- MongoDB 采集记录。每次拨号只插入。
CREATE TABLE IF NOT EXISTS FG_COMPONENT_MONGO_METRIC (
  tenantId TEXT NOT NULL,
  pointId TEXT NOT NULL,
  componentId TEXT NOT NULL,
  upFlag TEXT NOT NULL DEFAULT 'N',
  versionText TEXT,
  roleName TEXT,
  errorText TEXT,
  summaryText TEXT,
  observedTime TEXT NOT NULL,
  connectionsCurrent REAL NOT NULL DEFAULT 0,
  connectionsAvailable REAL NOT NULL DEFAULT 0,
  opInsert REAL NOT NULL DEFAULT 0,
  opQuery REAL NOT NULL DEFAULT 0,
  opUpdate REAL NOT NULL DEFAULT 0,
  opDelete REAL NOT NULL DEFAULT 0,
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
CREATE INDEX IF NOT EXISTS IDX_FG_COMPONENT_MONGO_METRIC_TIME ON FG_COMPONENT_MONGO_METRIC(tenantId, componentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_COMPONENT_MONGO_METRIC_ADD ON FG_COMPONENT_MONGO_METRIC(tenantId, addTime);
UPDATE FG_COMPONENT_KIND SET metricTable = 'FG_COMPONENT_MONGO_METRIC' WHERE kindCode = 'mongo' AND metricTable = 'FG_MONGO_METRIC';
