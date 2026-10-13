-- 一个 Java 进程的一代。启动时间变化时收口上一行，再插入新行。
-- 次数和耗时是这一代结束前最后一次上报的累计值。当前代的 closedTime 为空。
CREATE TABLE IF NOT EXISTS FG_JAVA_RUN (
  runId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  processId TEXT,
  startTime TEXT NOT NULL,
  openedTime TEXT NOT NULL,
  closedTime TEXT,
  collectionCount INTEGER NOT NULL DEFAULT 0,
  collectionTimeMs INTEGER NOT NULL DEFAULT 0,
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
  PRIMARY KEY (tenantId, runId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_RUN_AGENT ON FG_JAVA_RUN(tenantId, agentId, openedTime);
