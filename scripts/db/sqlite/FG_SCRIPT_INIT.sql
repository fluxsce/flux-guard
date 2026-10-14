-- 脚本级执行摘要（可选）。逐条是否再跑看 FG_SCRIPT_STATEMENT。
-- 字段对齐 gateway 的 HUB_SCRIPT_EXECUTION_HISTORY。
CREATE TABLE IF NOT EXISTS FG_SCRIPT_INIT (
  initId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  scriptName TEXT NOT NULL,
  scriptPath TEXT NOT NULL,
  scriptVersion TEXT NOT NULL,
  databaseDriver TEXT NOT NULL,
  executionStatus TEXT NOT NULL,
  executionTime TEXT NOT NULL,
  executionDuration INTEGER NOT NULL DEFAULT 0,
  statementsExecuted INTEGER NOT NULL DEFAULT 0,
  errorMessage TEXT,
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
  PRIMARY KEY (tenantId, initId)
);
CREATE UNIQUE INDEX IF NOT EXISTS UK_FG_SCRIPT_INIT ON FG_SCRIPT_INIT(tenantId, scriptName, scriptVersion, databaseDriver);
CREATE INDEX IF NOT EXISTS IDX_FG_SCRIPT_INIT_STATUS ON FG_SCRIPT_INIT(executionStatus);
CREATE INDEX IF NOT EXISTS IDX_FG_SCRIPT_INIT_ADD ON FG_SCRIPT_INIT(tenantId, addTime);
