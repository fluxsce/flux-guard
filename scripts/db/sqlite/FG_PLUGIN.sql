CREATE TABLE IF NOT EXISTS FG_PLUGIN (
  pluginId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  pluginCode TEXT NOT NULL,
  pluginName TEXT NOT NULL,
  pluginKind TEXT NOT NULL,
  pluginScope TEXT NOT NULL,
  builtinFlag TEXT NOT NULL DEFAULT 'N',
  pluginStatus TEXT NOT NULL DEFAULT 'Y',
  entryPoint TEXT,
  configSchema TEXT,
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
  PRIMARY KEY (tenantId, pluginId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_PLUGIN_CODE ON FG_PLUGIN(tenantId, pluginCode);
CREATE INDEX IF NOT EXISTS IDX_FG_PLUGIN_ADD ON FG_PLUGIN(tenantId, addTime);

INSERT INTO FG_PLUGIN (
  pluginId, tenantId, pluginCode, pluginName, pluginKind, pluginScope,
  builtinFlag, pluginStatus, entryPoint,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
) VALUES
('plugin-examplealert', 'default', 'examplealert', '示例告警', 'MODULE', 'SERVER', 'Y', 'Y', 'fluxguard/server/plugin/examplealert', 'system', 'system', 'INIT', 1, 'Y'),
('plugin-recent', 'default', 'recent', '近期样本库', 'SINK', 'SERVER', 'Y', 'Y', 'fluxguard/server/ingest', 'system', 'system', 'INIT', 1, 'Y');
