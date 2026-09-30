CREATE TABLE IF NOT EXISTS FG_EVENT_POINT (
  eventId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  batchId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  agentCode TEXT NOT NULL,
  eventName TEXT NOT NULL,
  severity TEXT NOT NULL DEFAULT 'INFO',
  messageText TEXT,
  labelJson TEXT,
  observedTime TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, eventId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_EVENT_AGENT ON FG_EVENT_POINT(tenantId, agentId, observedTime);
CREATE INDEX IF NOT EXISTS IDX_FG_EVENT_POINT_ADD ON FG_EVENT_POINT(tenantId, addTime);
