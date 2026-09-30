CREATE TABLE IF NOT EXISTS FG_AUTH_USER_ROLE (
  userRoleId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  userId TEXT NOT NULL,
  roleId TEXT NOT NULL,
  grantedBy TEXT NOT NULL,
  grantedTime TEXT NOT NULL DEFAULT (datetime('now')),
  expireTime TEXT,
  primaryRoleFlag TEXT NOT NULL DEFAULT 'N',
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
  PRIMARY KEY (tenantId, userRoleId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_USER_ROLE_UQ ON FG_AUTH_USER_ROLE(tenantId, userId, roleId);
CREATE INDEX IF NOT EXISTS IDX_FG_USER_ROLE_USER ON FG_AUTH_USER_ROLE(tenantId, userId);
CREATE INDEX IF NOT EXISTS IDX_FG_AUTH_USER_ROLE_ADD ON FG_AUTH_USER_ROLE(tenantId, addTime);

INSERT INTO FG_AUTH_USER_ROLE (
  userRoleId, tenantId, userId, roleId, grantedBy, primaryRoleFlag,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
) VALUES
('UR_ADMIN_SA', 'default', 'admin', 'ROLE_SUPER_ADMIN', 'system', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('UR_VIEWER_V', 'default', 'viewer', 'ROLE_VIEWER', 'system', 'Y', 'system', 'system', 'INIT', 1, 'Y');
