-- 两次上报之间的 GC 停顿。每次上报只插入，一个回收器一行。不是累计计数。
CREATE TABLE IF NOT EXISTS FG_JAVA_GC_PAUSE (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  pointId TEXT NOT NULL,
  batchId TEXT NOT NULL,
  collectorName TEXT NOT NULL,
  pauseCount INTEGER NOT NULL DEFAULT 0,
  pauseMaxMs INTEGER NOT NULL DEFAULT 0,
  pauseTotalMs INTEGER NOT NULL DEFAULT 0,
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
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_GC_PAUSE_TIME ON FG_JAVA_GC_PAUSE(tenantId, agentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_GC_PAUSE_ADD ON FG_JAVA_GC_PAUSE(tenantId, addTime);
