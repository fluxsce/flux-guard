-- 一次应用更新。文件在 data/apprelease，每一台实例对应一行命令。
CREATE TABLE IF NOT EXISTS FG_APP_RELEASE (
  releaseId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  appCode TEXT NOT NULL,
  artifactKind TEXT NOT NULL,
  versionText TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  fileName TEXT NOT NULL,
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
CREATE INDEX IF NOT EXISTS IDX_FG_APP_RELEASE_ADD ON FG_APP_RELEASE(tenantId, addTime);

CREATE TABLE IF NOT EXISTS FG_APP_RELEASE_ITEM (
  commandId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  releaseId TEXT NOT NULL,
  instId TEXT NOT NULL,
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
CREATE INDEX IF NOT EXISTS IDX_FG_APP_RELEASE_ITEM_REL ON FG_APP_RELEASE_ITEM(tenantId, releaseId);
CREATE INDEX IF NOT EXISTS IDX_FG_APP_RELEASE_ITEM_ADD ON FG_APP_RELEASE_ITEM(tenantId, addTime);
