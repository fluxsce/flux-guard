-- 文件句柄滚动记录。每次上报只插入，供曲线查询。
CREATE TABLE IF NOT EXISTS FG_HOST_FD (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  pointId TEXT NOT NULL,
  batchId TEXT NOT NULL,
  allocatedCount INTEGER NOT NULL DEFAULT 0,
  maximumCount INTEGER NOT NULL DEFAULT 0,
  usedPercent REAL NOT NULL DEFAULT 0,
  observedTime TEXT NOT NULL,
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
CREATE INDEX IF NOT EXISTS IDX_FG_HOST_FD_TIME ON FG_HOST_FD(tenantId, agentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_HOST_FD_ADD ON FG_HOST_FD(tenantId, addTime);
