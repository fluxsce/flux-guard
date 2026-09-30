CREATE TABLE IF NOT EXISTS FG_AUTH_AUDIT_LOG (
  auditId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  userId TEXT NOT NULL,
  userName TEXT,
  action TEXT NOT NULL,
  moduleCode TEXT,
  targetType TEXT,
  targetId TEXT,
  targetName TEXT,
  resourceCode TEXT,
  requestPath TEXT,
  requestMethod TEXT,
  clientIP TEXT,
  resultFlag TEXT NOT NULL DEFAULT 'Y',
  detailText TEXT,
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
  PRIMARY KEY (tenantId, auditId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_AUDIT_TIME ON FG_AUTH_AUDIT_LOG(tenantId, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_AUDIT_USER ON FG_AUTH_AUDIT_LOG(tenantId, userId, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_AUDIT_MODULE ON FG_AUTH_AUDIT_LOG(tenantId, moduleCode, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_AUDIT_ACTION ON FG_AUTH_AUDIT_LOG(tenantId, action, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_AUDIT_RESULT ON FG_AUTH_AUDIT_LOG(tenantId, resultFlag, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_AUDIT_TARGET ON FG_AUTH_AUDIT_LOG(tenantId, targetType, targetId);
-- 已有库补列；列已存在时由 store 忽略错误。
ALTER TABLE FG_AUTH_AUDIT_LOG ADD COLUMN failReason TEXT;
