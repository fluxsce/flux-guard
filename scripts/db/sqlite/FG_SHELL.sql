-- 一个终端一行。同一代理、同一用户可以同时有多行打开，互不覆盖。
-- statusFlag：O 打开，C 关闭。不限制一台代理只有一个终端。
CREATE TABLE IF NOT EXISTS FG_SHELL (
  shellId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  userId TEXT NOT NULL,
  userName TEXT,
  shellTitle TEXT NOT NULL,
  shellNo INTEGER NOT NULL,
  statusFlag TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, shellId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_SHELL_AGENT ON FG_SHELL(tenantId, agentId, statusFlag);
CREATE INDEX IF NOT EXISTS IDX_FG_SHELL_USER ON FG_SHELL(tenantId, userId, agentId, statusFlag);
CREATE INDEX IF NOT EXISTS IDX_FG_SHELL_ADD ON FG_SHELL(tenantId, addTime);
