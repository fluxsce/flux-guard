CREATE TABLE IF NOT EXISTS FG_TENANT (
  tenantId TEXT NOT NULL,
  tenantName TEXT NOT NULL,
  tenantStatus TEXT NOT NULL DEFAULT 'Y',
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
  PRIMARY KEY (tenantId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_TENANT_ADD ON FG_TENANT(tenantId, addTime);

INSERT INTO FG_TENANT (
  tenantId, tenantName, tenantStatus, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
) VALUES (
  'default', '默认租户', 'Y', 'system', 'system', 'INIT', 1, 'Y'
);
