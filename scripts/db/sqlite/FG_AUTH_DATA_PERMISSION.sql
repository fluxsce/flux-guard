-- 数据范围挂在角色上。scopeValue 由应用解释。不要把 filterCondition 拼进 SQL。
CREATE TABLE IF NOT EXISTS FG_AUTH_DATA_PERMISSION (
  dataPermissionId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  userId TEXT,
  roleId TEXT,
  resourceType TEXT NOT NULL,
  resourceCode TEXT NOT NULL,
  scopeValue TEXT,
  filterCondition TEXT,
  columnPermissions TEXT,
  operationPermissions TEXT DEFAULT 'read',
  effectiveTime TEXT,
  expireTime TEXT,
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
  PRIMARY KEY (tenantId, dataPermissionId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_DATA_PERM_ROLE ON FG_AUTH_DATA_PERMISSION(tenantId, roleId);
CREATE INDEX IF NOT EXISTS IDX_FG_DATA_PERM_RES ON FG_AUTH_DATA_PERMISSION(resourceType, resourceCode);
CREATE INDEX IF NOT EXISTS IDX_FG_AUTH_DATA_PERMISSION_ADD ON FG_AUTH_DATA_PERMISSION(tenantId, addTime);
