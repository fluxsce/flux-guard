-- 环境设置。每个租户每个分组一行，content 为该分组的 JSON。
CREATE TABLE IF NOT EXISTS FG_SYS_SETTING (
  tenantId TEXT NOT NULL,
  groupCode TEXT NOT NULL,
  content TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, groupCode)
);
CREATE INDEX IF NOT EXISTS IDX_FG_SYS_SETTING_ADD ON FG_SYS_SETTING(tenantId, addTime);

-- 开发态上报令牌明文是 agt_dev_local，与 configs 里的 agent_token 相同。摘要只存这一份，各代理复用。
INSERT INTO FG_SYS_SETTING (
  tenantId, groupCode, content, addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
) VALUES (
  'default', 'ingestToken',
  '{"tokenHash":"4a1209434b00b83b89b56e53bf652d9aa3d63c7c0bd9492724cfd2ca38030360","statusFlag":"Y","hint":"ocal"}',
  'system', 'system', 'INIT', 1, 'Y',
  '开发态上报令牌，生产环境在环境设置中轮换'
);
