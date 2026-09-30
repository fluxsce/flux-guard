-- 同一租户下 batchId 只收一次。代理在网络失败时必须重发同一个批次号。
CREATE TABLE IF NOT EXISTS FG_INGEST_BATCH (
  batchId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  agentCode TEXT NOT NULL,
  sampleCount INTEGER NOT NULL DEFAULT 0,
  eventCount INTEGER NOT NULL DEFAULT 0,
  acceptedFlag TEXT NOT NULL DEFAULT 'Y',
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
  PRIMARY KEY (tenantId, batchId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_BATCH_AGENT ON FG_INGEST_BATCH(tenantId, agentId, addTime);
CREATE INDEX IF NOT EXISTS IDX_FG_INGEST_BATCH_ADD ON FG_INGEST_BATCH(tenantId, addTime);
