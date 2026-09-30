CREATE TABLE IF NOT EXISTS `FG_TENANT` (
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，主键',
  `tenantName` VARCHAR(100) NOT NULL COMMENT '租户名称',
  `tenantStatus` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '租户状态(Y启用,N停用)',
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
  PRIMARY KEY (`tenantId`),
  KEY `IDX_FG_TENANT_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='租户表';

INSERT INTO `FG_TENANT` (
  `tenantId`, `tenantName`, `tenantStatus`, `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
) VALUES (
  'default', '默认租户', 'Y', 'system', 'system', 'INIT', 1, 'Y'
);
