-- 登录鉴权按资源编码精确匹配，不会把 MODULE 自动展开成按钮。
-- 超级管理员获得全部有效资源。只读用户获得分组、模块，编码以 :search、:view 结尾的按钮，以及组件监控的种类按钮。
-- 打开终端（fg0002:shell、fg0007:shell）不给只读用户。
CREATE TABLE IF NOT EXISTS FG_AUTH_ROLE_RESOURCE (
  roleResourceId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  roleId TEXT NOT NULL,
  resourceId TEXT NOT NULL,
  permissionType TEXT NOT NULL DEFAULT 'ALLOW',
  grantedBy TEXT NOT NULL,
  grantedTime TEXT NOT NULL DEFAULT (datetime('now')),
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
  PRIMARY KEY (tenantId, roleResourceId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_ROLE_RES_UQ ON FG_AUTH_ROLE_RESOURCE(tenantId, roleId, resourceId);
CREATE INDEX IF NOT EXISTS IDX_FG_ROLE_RES_ROLE ON FG_AUTH_ROLE_RESOURCE(tenantId, roleId);
CREATE INDEX IF NOT EXISTS IDX_FG_AUTH_ROLE_RESOURCE_ADD ON FG_AUTH_ROLE_RESOURCE(tenantId, addTime);

INSERT INTO FG_AUTH_ROLE_RESOURCE (
  roleResourceId, tenantId, roleId, resourceId, permissionType, grantedBy,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
)
SELECT
  'RR_SA_' || REPLACE(resourceId, ':', '_'),
  tenantId, 'ROLE_SUPER_ADMIN', resourceId, 'ALLOW', 'system',
  'system', 'system', 'INIT', 1, 'Y'
FROM FG_AUTH_RESOURCE
WHERE tenantId = 'default' AND activeFlag = 'Y' AND resourceStatus = 'Y';

INSERT INTO FG_AUTH_ROLE_RESOURCE (
  roleResourceId, tenantId, roleId, resourceId, permissionType, grantedBy,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag
)
SELECT
  'RR_VW_' || REPLACE(resourceId, ':', '_'),
  tenantId, 'ROLE_VIEWER', resourceId, 'ALLOW', 'system',
  'system', 'system', 'INIT', 1, 'Y'
FROM FG_AUTH_RESOURCE
WHERE tenantId = 'default' AND activeFlag = 'Y' AND resourceStatus = 'Y'
  AND (
    resourceType IN ('GROUP', 'MODULE')
    OR resourceCode LIKE '%:search'
    OR resourceCode LIKE '%:view'
    OR resourceCode IN ('fg0007:redis', 'fg0007:mysql', 'fg0007:mongo', 'fg0007:kafka', 'fg0007:zk', 'fg0007:nacos')
  );

-- 跟 FG_AUTH_RESOURCE 里的模块改名对齐。先改按钮，避免编码替换碰到模块行。
UPDATE FG_AUTH_ROLE_RESOURCE
SET resourceId = 'fg0026:view', roleResourceId = REPLACE(roleResourceId, 'fg0003_view', 'fg0026_view')
WHERE resourceId = 'fg0003:view';
UPDATE FG_AUTH_ROLE_RESOURCE
SET resourceId = 'fg0026', roleResourceId = REPLACE(roleResourceId, 'fg0003', 'fg0026')
WHERE resourceId = 'fg0003';
