CREATE TABLE IF NOT EXISTS `FG_AUTH_USER_ROLE` (
  `userRoleId` VARCHAR(64) NOT NULL COMMENT '用户角色关联ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `userId` VARCHAR(32) NOT NULL COMMENT '用户ID',
  `roleId` VARCHAR(32) NOT NULL COMMENT '角色ID',
  `grantedBy` VARCHAR(32) NOT NULL COMMENT '授权人ID',
  `grantedTime` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '授权时间',
  `expireTime` DATETIME DEFAULT NULL COMMENT '过期时间，空表示不过期',
  `primaryRoleFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '主要角色标记(Y主要,N次要)',
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
  PRIMARY KEY (`tenantId`, `userRoleId`),
  KEY `IDX_FG_USER_ROLE_UQ` (`tenantId`, `userId`, `roleId`),
  KEY `IDX_FG_USER_ROLE_USER` (`tenantId`, `userId`),
  KEY `IDX_FG_AUTH_USER_ROLE_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='用户角色关联表';

INSERT INTO `FG_AUTH_USER_ROLE` (
  `userRoleId`, `tenantId`, `userId`, `roleId`, `grantedBy`, `primaryRoleFlag`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
) VALUES
('UR_ADMIN_SA', 'default', 'admin', 'ROLE_SUPER_ADMIN', 'system', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('UR_VIEWER_V', 'default', 'viewer', 'ROLE_VIEWER', 'system', 'Y', 'system', 'system', 'INIT', 1, 'Y');
