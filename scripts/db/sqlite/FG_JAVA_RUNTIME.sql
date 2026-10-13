-- JVM 进程 CPU 和类加载。每次上报只插入。
CREATE TABLE IF NOT EXISTS FG_JAVA_RUNTIME (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  pointId TEXT NOT NULL,
  batchId TEXT NOT NULL,
  processCpuPercent REAL NOT NULL DEFAULT 0,
  loadedClassCount INTEGER NOT NULL DEFAULT 0,
  unloadedClassCount INTEGER NOT NULL DEFAULT 0,
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
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_RUNTIME_TIME ON FG_JAVA_RUNTIME(tenantId, agentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_RUNTIME_ADD ON FG_JAVA_RUNTIME(tenantId, addTime);
-- 第一拍没有上一拍，开销为 -1，不得写成 0。
ALTER TABLE FG_JAVA_RUNTIME ADD COLUMN pauseOverheadPercent REAL NOT NULL DEFAULT -1;
