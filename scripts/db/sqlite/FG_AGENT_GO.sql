-- 一个 Go 进程的当前身份和最近一次运行时。每次上报更新同一行。
-- 这不是应用清单。应用换包在 FG_APP。
CREATE TABLE IF NOT EXISTS FG_AGENT_GO (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  appName TEXT NOT NULL,
  goVersion TEXT,
  processId TEXT,
  startTime TEXT,
  hostName TEXT,
  numCpu INTEGER NOT NULL DEFAULT 0,
  goroutineCount INTEGER NOT NULL DEFAULT 0,
  heapAllocBytes INTEGER NOT NULL DEFAULT 0,
  heapSysBytes INTEGER NOT NULL DEFAULT 0,
  heapObjects INTEGER NOT NULL DEFAULT 0,
  gcCount INTEGER NOT NULL DEFAULT 0,
  gcPauseMs REAL NOT NULL DEFAULT 0,
  reportedTime TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, agentId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_GO_NAME ON FG_AGENT_GO(tenantId, appName);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_GO_ADD ON FG_AGENT_GO(tenantId, addTime);
