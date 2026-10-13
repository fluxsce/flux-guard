-- JVM 线程状态。每次上报只插入。不采集栈，只区分运行、阻塞和等待。
CREATE TABLE IF NOT EXISTS FG_JAVA_THREAD_STATE (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  pointId TEXT NOT NULL,
  batchId TEXT NOT NULL,
  runnableCount INTEGER NOT NULL DEFAULT 0,
  blockedCount INTEGER NOT NULL DEFAULT 0,
  waitingCount INTEGER NOT NULL DEFAULT 0,
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
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_THREAD_STATE_TIME ON FG_JAVA_THREAD_STATE(tenantId, agentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_JAVA_THREAD_STATE_ADD ON FG_JAVA_THREAD_STATE(tenantId, addTime);
-- 检测失败或旧数据为 -1，不能当成没有死锁。
ALTER TABLE FG_JAVA_THREAD_STATE ADD COLUMN deadlockCount INTEGER NOT NULL DEFAULT -1;
ALTER TABLE FG_JAVA_THREAD_STATE ADD COLUMN deadlockText TEXT;
