-- 一个应用落在某套运行时上的一个实例。身份是代理、实例名加应用名。
-- 一台机器一个应用代理，可以看守多套 Tomcat。应用进程停了这一行还在。
CREATE TABLE IF NOT EXISTS FG_APP_INST (
  instId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  agentId TEXT NOT NULL,
  appCode TEXT NOT NULL,
  machineId TEXT,
  hostName TEXT,
  ipAddress TEXT,
  instanceName TEXT NOT NULL,
  runtimeKind TEXT NOT NULL,
  artifactKind TEXT NOT NULL,
  artifactPath TEXT,
  versionText TEXT,
  tomcatVersion TEXT,
  runningFlag TEXT NOT NULL DEFAULT 'N',
  commandAddress TEXT,
  reportedTime TEXT NOT NULL,
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
  PRIMARY KEY (tenantId, instId)
);
DROP INDEX IF EXISTS IDX_FG_APP_INST_AGENT;
CREATE UNIQUE INDEX IF NOT EXISTS IDX_FG_APP_INST_SLOT ON FG_APP_INST(tenantId, agentId, instanceName, appCode);
CREATE INDEX IF NOT EXISTS IDX_FG_APP_INST_APP ON FG_APP_INST(tenantId, appCode);
CREATE INDEX IF NOT EXISTS IDX_FG_APP_INST_ADD ON FG_APP_INST(tenantId, addTime);
