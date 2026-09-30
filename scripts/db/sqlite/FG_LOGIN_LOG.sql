CREATE TABLE IF NOT EXISTS FG_LOGIN_LOG (
  logId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  userId TEXT NOT NULL,
  userName TEXT NOT NULL,
  loginTime TEXT NOT NULL DEFAULT (datetime('now')),
  loginIp TEXT NOT NULL,
  loginType INTEGER NOT NULL DEFAULT 1,
  loginStatus TEXT NOT NULL DEFAULT 'N',
  failReason TEXT,
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
  PRIMARY KEY (tenantId, logId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_LOGIN_USER ON FG_LOGIN_LOG(tenantId, userId, loginTime);
CREATE INDEX IF NOT EXISTS IDX_FG_LOGIN_STATUS ON FG_LOGIN_LOG(loginStatus);
CREATE INDEX IF NOT EXISTS IDX_FG_LOGIN_LOG_ADD ON FG_LOGIN_LOG(tenantId, addTime);
