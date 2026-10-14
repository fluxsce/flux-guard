-- 语句级执行历史，对齐 gateway 的 HUB_STATEMENT_EXECUTION_HISTORY。
-- 按语句内容哈希去重：已 SUCCESS / ALREADY_EXISTS 的下次启动跳过；脚本里新增或改过的语句会再跑。
CREATE TABLE IF NOT EXISTS FG_SCRIPT_STATEMENT (
  statementId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  scriptName TEXT NOT NULL,
  statementHash TEXT NOT NULL,
  statementType TEXT NOT NULL,
  statementContent TEXT NOT NULL,
  databaseDriver TEXT NOT NULL,
  executionStatus TEXT NOT NULL,
  executionTime TEXT NOT NULL,
  executionDuration INTEGER NOT NULL DEFAULT 0,
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
  PRIMARY KEY (tenantId, statementId)
);
CREATE UNIQUE INDEX IF NOT EXISTS UK_FG_SCRIPT_STATEMENT ON FG_SCRIPT_STATEMENT(tenantId, scriptName, statementHash, databaseDriver);
CREATE INDEX IF NOT EXISTS IDX_FG_SCRIPT_STATEMENT_HASH ON FG_SCRIPT_STATEMENT(statementHash);
CREATE INDEX IF NOT EXISTS IDX_FG_SCRIPT_STATEMENT_STATUS ON FG_SCRIPT_STATEMENT(executionStatus);
CREATE INDEX IF NOT EXISTS IDX_FG_SCRIPT_STATEMENT_ADD ON FG_SCRIPT_STATEMENT(tenantId, addTime);
