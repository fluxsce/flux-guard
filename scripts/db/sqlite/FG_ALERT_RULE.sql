-- 通用预警规则。公共列给各功能共用，条件 JSON 由功能模块自己拆。预警发送不读这张表。
CREATE TABLE IF NOT EXISTS FG_ALERT_RULE (
  tenantId TEXT NOT NULL,
  ruleId TEXT NOT NULL,
  ruleName TEXT NOT NULL,
  alertType TEXT NOT NULL,
  levelCode TEXT NOT NULL DEFAULT 'warn',
  channelId TEXT,
  silenceStart TEXT,
  silenceEnd TEXT,
  statusFlag TEXT NOT NULL DEFAULT 'Y',
  conditionText TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, ruleId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_RULE_TYPE ON FG_ALERT_RULE(tenantId, alertType, statusFlag);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_RULE_ADD ON FG_ALERT_RULE(tenantId, addTime);
