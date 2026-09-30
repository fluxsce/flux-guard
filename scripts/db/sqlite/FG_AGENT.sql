-- agentTokenHash 是令牌的 SHA-256。明文只在登记接口返回一次。
-- 开发代理的明文令牌是 agt_dev_local，只用于本机骨架。
CREATE TABLE IF NOT EXISTS FG_AGENT (
  agentId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  agentName TEXT NOT NULL,
  hostName TEXT,
  ipAddress TEXT,
  osName TEXT,
  agentCode TEXT,
  machineId TEXT,
  agentTokenHash TEXT NOT NULL,
  statusFlag TEXT NOT NULL DEFAULT 'Y',
  lastSeenTime TEXT,
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
  PRIMARY KEY (tenantId, agentId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_NAME ON FG_AGENT(tenantId, agentName);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_SEEN ON FG_AGENT(tenantId, lastSeenTime);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_MACHINE ON FG_AGENT(tenantId, agentCode, machineId);
CREATE INDEX IF NOT EXISTS IDX_FG_AGENT_ADD ON FG_AGENT(tenantId, addTime);

INSERT INTO FG_AGENT (
  agentId, tenantId, agentName, hostName, ipAddress, agentTokenHash, statusFlag,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
) VALUES (
  'agent-local', 'default', '本机开发代理', 'localhost', '127.0.0.1',
  '4a1209434b00b83b89b56e53bf652d9aa3d63c7c0bd9492724cfd2ca38030360',
  'Y', 'system', 'system', 'INIT', 1, 'Y',
  '开发态固定令牌，生产环境登记后只显示一次'
);
