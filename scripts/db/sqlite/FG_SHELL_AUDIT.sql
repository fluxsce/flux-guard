-- 终端会话账本。开闭会话与回车确认的一行命令各记一条，不记方向键与整段屏幕输出。
-- eventType：OPEN 打开，CLOSE 关闭，INPUT 回车命令行。
CREATE TABLE IF NOT EXISTS FG_SHELL_AUDIT (
  auditId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  shellId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  userId TEXT NOT NULL,
  userName TEXT,
  eventType TEXT NOT NULL,
  detailText TEXT,
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
CREATE INDEX IF NOT EXISTS IDX_FG_SHELL_AUDIT_AGENT ON FG_SHELL_AUDIT(tenantId, agentId, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_SHELL_AUDIT_SHELL ON FG_SHELL_AUDIT(tenantId, shellId, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_SHELL_AUDIT_USER ON FG_SHELL_AUDIT(tenantId, userId, addTime);
