-- 一条命令一行。拨入和拉取共用这一行，控制台只看状态和结果。
CREATE TABLE IF NOT EXISTS FG_COMMAND (
  commandId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  actionName TEXT NOT NULL,
  intervalSec INTEGER,
  statusFlag TEXT NOT NULL,
  deliverMode TEXT,
  resultText TEXT,
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
CREATE INDEX IF NOT EXISTS IDX_FG_COMMAND_AGENT ON FG_COMMAND(tenantId, agentId, statusFlag);
CREATE INDEX IF NOT EXISTS IDX_FG_COMMAND_ADD ON FG_COMMAND(tenantId, addTime);
