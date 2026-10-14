-- 一次代理自更新。文件在 data/agentrelease，每一台代理对应一行命令。
CREATE TABLE IF NOT EXISTS FG_AGENT_RELEASE (
  releaseId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  agentCode TEXT NOT NULL,
  versionText TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  fileName TEXT NOT NULL,
  goos TEXT,
  goarch TEXT,
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
  PRIMARY KEY (tenantId, releaseId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_RELEASE_ADD ON FG_AGENT_RELEASE(tenantId, addTime);

CREATE TABLE IF NOT EXISTS FG_AGENT_RELEASE_ITEM (
  commandId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  releaseId TEXT NOT NULL,
  agentId TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, commandId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_RELEASE_ITEM_REL ON FG_AGENT_RELEASE_ITEM(tenantId, releaseId);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_RELEASE_ITEM_ADD ON FG_AGENT_RELEASE_ITEM(tenantId, addTime);
