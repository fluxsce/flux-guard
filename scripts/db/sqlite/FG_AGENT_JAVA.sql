-- 一个 Java 代理的当前身份。每次上报更新同一行。堆、GC 等曲线另表只插入。
CREATE TABLE IF NOT EXISTS FG_AGENT_JAVA (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  appName TEXT NOT NULL,
  mainClass TEXT,
  processId TEXT,
  javaVersion TEXT,
  startTime TEXT,
  hostName TEXT,
  ipAddress TEXT,
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
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_JAVA_NAME ON FG_AGENT_JAVA(tenantId, appName);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_JAVA_ADD ON FG_AGENT_JAVA(tenantId, addTime);
