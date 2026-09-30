CREATE TABLE IF NOT EXISTS FG_AUTH_ROLE (
  roleId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  roleName TEXT NOT NULL,
  roleDescription TEXT,
  roleStatus TEXT NOT NULL DEFAULT 'Y',
  builtInFlag TEXT NOT NULL DEFAULT 'N',
  dataScope TEXT,
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
  PRIMARY KEY (tenantId, roleId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_ROLE_NAME ON FG_AUTH_ROLE(tenantId, roleName);
CREATE INDEX IF NOT EXISTS IDX_FG_ROLE_STATUS ON FG_AUTH_ROLE(roleStatus);
CREATE INDEX IF NOT EXISTS IDX_FG_AUTH_ROLE_ADD ON FG_AUTH_ROLE(tenantId, addTime);

INSERT INTO FG_AUTH_ROLE (
  roleId, tenantId, roleName, roleDescription, roleStatus, builtInFlag, dataScope,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
) VALUES
(
  'ROLE_SUPER_ADMIN', 'default', '超级管理员', '拥有全部权限',
  'Y', 'Y', '{"type":"ALL"}', 'system', 'system', 'INIT', 1, 'Y'
),
(
  'ROLE_VIEWER', 'default', '只读用户', '可查看导航和查询，不能登记或修改',
  'Y', 'Y', '{"type":"ALL"}', 'system', 'system', 'INIT', 1, 'Y'
);
