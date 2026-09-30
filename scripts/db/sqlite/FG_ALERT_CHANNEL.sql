-- 通知渠道。configText 为 JSON，密钥以 ENCY_ 密文保存。
CREATE TABLE IF NOT EXISTS FG_ALERT_CHANNEL (
  tenantId TEXT NOT NULL,
  channelId TEXT NOT NULL,
  channelName TEXT NOT NULL,
  channelKind TEXT NOT NULL,
  configText TEXT NOT NULL,
  statusFlag TEXT NOT NULL DEFAULT 'Y',
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
  PRIMARY KEY (tenantId, channelId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_CHANNEL_KIND ON FG_ALERT_CHANNEL(tenantId, channelKind);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_CHANNEL_ADD ON FG_ALERT_CHANNEL(tenantId, addTime);
