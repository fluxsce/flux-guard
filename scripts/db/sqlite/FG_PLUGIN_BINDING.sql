CREATE TABLE IF NOT EXISTS FG_PLUGIN_BINDING (
  bindingId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  pluginId TEXT NOT NULL,
  targetType TEXT NOT NULL,
  targetId TEXT NOT NULL,
  enabledFlag TEXT NOT NULL DEFAULT 'Y',
  pluginConfig TEXT,
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
  PRIMARY KEY (tenantId, bindingId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_BIND_TARGET ON FG_PLUGIN_BINDING(tenantId, targetType, targetId);
CREATE INDEX IF NOT EXISTS IDX_FG_BIND_PLUGIN ON FG_PLUGIN_BINDING(tenantId, pluginId);
CREATE INDEX IF NOT EXISTS IDX_FG_PLUGIN_BINDING_ADD ON FG_PLUGIN_BINDING(tenantId, addTime);
