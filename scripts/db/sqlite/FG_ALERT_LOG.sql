-- 预警日志。功能模块写入一行，发送结果回写在同一行。不保存各功能的指标和阈值。
CREATE TABLE IF NOT EXISTS FG_ALERT_LOG (
  tenantId TEXT NOT NULL,
  alertLogId TEXT NOT NULL,
  alertLevel TEXT NOT NULL,
  alertType TEXT NOT NULL,
  sceneCode TEXT NOT NULL,
  alertTitle TEXT NOT NULL,
  alertContent TEXT,
  alertTimestamp TEXT NOT NULL,
  channelId TEXT,
  sendStatus TEXT NOT NULL DEFAULT 'pending',
  sendTime TEXT,
  sendResult TEXT,
  sendError TEXT,
  failCount INTEGER NOT NULL DEFAULT 0,
  alertExtra TEXT,
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
  PRIMARY KEY (tenantId, alertLogId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_LOG_SEND ON FG_ALERT_LOG(tenantId, sendStatus, alertTimestamp);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_LOG_TIME ON FG_ALERT_LOG(tenantId, alertTimestamp);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_LOG_ADD ON FG_ALERT_LOG(tenantId, addTime);
