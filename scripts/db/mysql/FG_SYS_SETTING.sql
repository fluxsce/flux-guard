-- 环境设置。每个租户每个分组一行，content 为该分组的 JSON。
CREATE TABLE IF NOT EXISTS `FG_SYS_SETTING` (
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `groupCode` VARCHAR(64) NOT NULL COMMENT '分组编码，联合主键',
  `content` TEXT NOT NULL COMMENT '分组内容，JSON',
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
  PRIMARY KEY (`tenantId`, `groupCode`),
  KEY `IDX_FG_SYS_SETTING_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='环境设置';

-- 开发态上报令牌明文是 agt_dev_local，与 configs 里的 agent_token 相同。摘要只存这一份，各代理复用。
INSERT INTO `FG_SYS_SETTING` (
  `tenantId`, `groupCode`, `content`, `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES (
  'default', 'ingestToken',
  '{"tokenHash":"4a1209434b00b83b89b56e53bf652d9aa3d63c7c0bd9492724cfd2ca38030360","statusFlag":"Y","hint":"ocal"}',
  'system', 'system', 'INIT', 1, 'Y',
  '开发态上报令牌，生产环境在环境设置中轮换'
);
