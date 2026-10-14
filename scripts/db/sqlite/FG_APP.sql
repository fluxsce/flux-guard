-- 应用由应用侧代理上报生成。显示名和忽略标记由控制台改，上报不覆盖。
CREATE TABLE IF NOT EXISTS FG_APP (
  appCode TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  appName TEXT NOT NULL,
  artifactKind TEXT NOT NULL,
  ignoreFlag TEXT NOT NULL DEFAULT 'N',
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
  PRIMARY KEY (tenantId, appCode)
);
CREATE INDEX IF NOT EXISTS IDX_FG_APP_ADD ON FG_APP(tenantId, addTime);
