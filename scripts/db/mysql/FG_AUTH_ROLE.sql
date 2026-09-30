CREATE TABLE IF NOT EXISTS `FG_AUTH_ROLE` (
  `roleId` VARCHAR(32) NOT NULL COMMENT '角色ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `roleName` VARCHAR(100) NOT NULL COMMENT '角色名称',
  `roleDescription` VARCHAR(255) DEFAULT NULL COMMENT '角色描述',
  `roleStatus` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '角色状态(Y启用,N停用)',
  `builtInFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '内置角色标记(Y内置,N自定义)',
  `dataScope` TEXT DEFAULT NULL COMMENT '数据权限范围，JSON。应用解释，不直接当 SQL',
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
  PRIMARY KEY (`tenantId`, `roleId`),
  KEY `IDX_FG_ROLE_NAME` (`tenantId`, `roleName`),
  KEY `IDX_FG_ROLE_STATUS` (`roleStatus`),
  KEY `IDX_FG_AUTH_ROLE_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='角色表';

INSERT INTO `FG_AUTH_ROLE` (
  `roleId`, `tenantId`, `roleName`, `roleDescription`, `roleStatus`, `builtInFlag`, `dataScope`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
) VALUES
(
  'ROLE_SUPER_ADMIN', 'default', '超级管理员', '拥有全部权限',
  'Y', 'Y', '{"type":"ALL"}', 'system', 'system', 'INIT', 1, 'Y'
),
(
  'ROLE_VIEWER', 'default', '只读用户', '可查看导航和查询，不能登记或修改',
  'Y', 'Y', '{"type":"ALL"}', 'system', 'system', 'INIT', 1, 'Y'
);
