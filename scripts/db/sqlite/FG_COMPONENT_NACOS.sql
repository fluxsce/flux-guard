-- Nacos 当前值。每个连接一行，采集时覆盖。列表只读这张表，不扫 FG_COMPONENT_NACOS_METRIC。
CREATE TABLE IF NOT EXISTS FG_COMPONENT_NACOS (
  tenantId TEXT NOT NULL,
  componentId TEXT NOT NULL,
  observedTime TEXT NOT NULL,
  configCount REAL NOT NULL DEFAULT 0,
  serviceCount REAL NOT NULL DEFAULT 0,
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
  PRIMARY KEY (tenantId, componentId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_COMPONENT_NACOS_ADD ON FG_COMPONENT_NACOS(tenantId, addTime);
