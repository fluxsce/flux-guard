-- 登录鉴权按资源编码精确匹配，不会把 MODULE 自动展开成按钮。
-- 超级管理员获得全部有效资源。只读用户获得分组、模块，编码以 :search、:view 结尾的按钮，以及组件监控的种类按钮。
-- 打开终端（fg0002:shell、fg0007:shell）不给只读用户。
CREATE TABLE IF NOT EXISTS `FG_AUTH_ROLE_RESOURCE` (
  `roleResourceId` VARCHAR(100) NOT NULL COMMENT '角色资源关联ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `roleId` VARCHAR(32) NOT NULL COMMENT '角色ID',
  `resourceId` VARCHAR(100) NOT NULL COMMENT '资源ID',
  `permissionType` VARCHAR(20) NOT NULL DEFAULT 'ALLOW' COMMENT '权限类型(ALLOW允许,DENY拒绝)',
  `grantedBy` VARCHAR(32) NOT NULL COMMENT '授权人ID',
  `grantedTime` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '授权时间',
  `expireTime` DATETIME DEFAULT NULL COMMENT '过期时间，空表示不过期',
  `addTime` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `addWho` VARCHAR(32) NOT NULL COMMENT '创建人ID',
  `editTime` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后修改时间',
  `editWho` VARCHAR(32) NOT NULL COMMENT '最后修改人ID',
  `oprSeqFlag` VARCHAR(32) NOT NULL COMMENT '操作序列标识',
  `currentVersion` INT NOT NULL DEFAULT 1 COMMENT '当前版本号',
  `activeFlag` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '活动状态标记(N非活动,Y活动)',
  `noteText` VARCHAR(500) DEFAULT NULL COMMENT '备注信息',
  `extProperty` TEXT DEFAULT NULL COMMENT '扩展属性，JSON格式',
  `reserved1` VARCHAR(64) DEFAULT NULL COMMENT '预留字段1',
  `reserved2` VARCHAR(64) DEFAULT NULL COMMENT '预留字段2',
  `reserved3` VARCHAR(100) DEFAULT NULL COMMENT '预留字段3',
  `reserved4` VARCHAR(100) DEFAULT NULL COMMENT '预留字段4',
  `reserved5` VARCHAR(255) DEFAULT NULL COMMENT '预留字段5',
  PRIMARY KEY (`tenantId`, `roleResourceId`),
  KEY `IDX_FG_ROLE_RES_UQ` (`tenantId`, `roleId`, `resourceId`),
  KEY `IDX_FG_ROLE_RES_ROLE` (`tenantId`, `roleId`),
  KEY `IDX_FG_AUTH_ROLE_RESOURCE_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='角色资源关联表';

INSERT IGNORE INTO `FG_AUTH_ROLE_RESOURCE` (
  `roleResourceId`, `tenantId`, `roleId`, `resourceId`, `permissionType`, `grantedBy`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
)
SELECT
  CONCAT('RR_SA_', REPLACE(`resourceId`, ':', '_')),
  `tenantId`, 'ROLE_SUPER_ADMIN', `resourceId`, 'ALLOW', 'system',
  'system', 'system', 'INIT', 1, 'Y'
FROM `FG_AUTH_RESOURCE`
WHERE `tenantId` = 'default' AND `activeFlag` = 'Y' AND `resourceStatus` = 'Y';

INSERT IGNORE INTO `FG_AUTH_ROLE_RESOURCE` (
  `roleResourceId`, `tenantId`, `roleId`, `resourceId`, `permissionType`, `grantedBy`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
)
SELECT
  CONCAT('RR_VW_', REPLACE(`resourceId`, ':', '_')),
  `tenantId`, 'ROLE_VIEWER', `resourceId`, 'ALLOW', 'system',
  'system', 'system', 'INIT', 1, 'Y'
FROM `FG_AUTH_RESOURCE`
WHERE `tenantId` = 'default' AND `activeFlag` = 'Y' AND `resourceStatus` = 'Y'
  AND (
    `resourceType` IN ('GROUP', 'MODULE')
    OR `resourceCode` LIKE '%:search'
    OR `resourceCode` LIKE '%:view'
    OR `resourceCode` IN ('fg0007:redis', 'fg0007:mysql', 'fg0007:mongo', 'fg0007:kafka', 'fg0007:zk', 'fg0007:nacos')
  );

-- 跟 FG_AUTH_RESOURCE 里的模块改名对齐。先改按钮，避免编码替换碰到模块行。
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0026:view', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0003_view', 'fg0026_view')
WHERE `resourceId` = 'fg0003:view';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0026', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0003', 'fg0026')
WHERE `resourceId` = 'fg0003';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0008:release', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0071_release', 'fg0008_release')
WHERE `resourceId` = 'fg0071:release';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0008:edit', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0071_edit', 'fg0008_edit')
WHERE `resourceId` = 'fg0071:edit';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0008:view', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0071_view', 'fg0008_view')
WHERE `resourceId` = 'fg0071:view';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0008:search', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0071_search', 'fg0008_search')
WHERE `resourceId` = 'fg0071:search';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `resourceId` = 'fg0008', `roleResourceId` = REPLACE(`roleResourceId`, 'fg0071', 'fg0008')
WHERE `resourceId` = 'fg0071';
UPDATE `FG_AUTH_ROLE_RESOURCE`
SET `activeFlag` = 'N'
WHERE `resourceId` = 'group:app';

-- 后来新增的内置授权拆成 VALUES；语句哈希未变则跳过，改过或新增才再跑。
INSERT IGNORE INTO `FG_AUTH_ROLE_RESOURCE` (
  `roleResourceId`, `tenantId`, `roleId`, `resourceId`, `permissionType`, `grantedBy`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
) VALUES
('RR_SA_fg0001_upgrade', 'default', 'ROLE_SUPER_ADMIN', 'fg0001:upgrade', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0002_upgrade', 'default', 'ROLE_SUPER_ADMIN', 'fg0002:upgrade', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0002_delete', 'default', 'ROLE_SUPER_ADMIN', 'fg0002:delete', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0008', 'default', 'ROLE_SUPER_ADMIN', 'fg0008', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0008_search', 'default', 'ROLE_SUPER_ADMIN', 'fg0008:search', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0008_view', 'default', 'ROLE_SUPER_ADMIN', 'fg0008:view', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0008_edit', 'default', 'ROLE_SUPER_ADMIN', 'fg0008:edit', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_SA_fg0008_release', 'default', 'ROLE_SUPER_ADMIN', 'fg0008:release', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_VW_fg0008', 'default', 'ROLE_VIEWER', 'fg0008', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_VW_fg0008_search', 'default', 'ROLE_VIEWER', 'fg0008:search', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y'),
('RR_VW_fg0008_view', 'default', 'ROLE_VIEWER', 'fg0008:view', 'ALLOW', 'system', 'system', 'system', 'INIT', 1, 'Y');
