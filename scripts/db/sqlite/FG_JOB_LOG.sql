-- 系统任务执行日志。每次跑完追加一行，重启后历史仍在。
CREATE TABLE IF NOT EXISTS FG_JOB_LOG (
  logId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  jobCode TEXT NOT NULL,
  triggerName TEXT NOT NULL,
  startTime TEXT,
  endTime TEXT,
  okFlag TEXT NOT NULL DEFAULT 'N',
  durationMs INTEGER NOT NULL DEFAULT 0,
  resultText TEXT,
  errorText TEXT,
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
CREATE INDEX IF NOT EXISTS IDX_FG_JOB_LOG_JOB ON FG_JOB_LOG(tenantId, jobCode, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_JOB_LOG_ADD ON FG_JOB_LOG(tenantId, addTime);
