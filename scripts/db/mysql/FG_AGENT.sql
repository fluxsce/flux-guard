-- agentTokenHash 是令牌的 SHA-256。明文只在登记接口返回一次。
-- 开发代理的明文令牌是 agt_dev_local，只用于本机骨架。
CREATE TABLE IF NOT EXISTS `FG_AGENT` (
  `agentId` VARCHAR(32) NOT NULL COMMENT '代理ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `agentName` VARCHAR(100) NOT NULL COMMENT '代理名称',
  `hostName` VARCHAR(100) DEFAULT NULL COMMENT '主机名',
  `ipAddress` VARCHAR(128) DEFAULT NULL COMMENT 'IP 地址',
  `osName` VARCHAR(64) DEFAULT NULL COMMENT '操作系统',
  `agentCode` VARCHAR(64) DEFAULT NULL COMMENT '代理种类，例如 host、app',
  `machineId` VARCHAR(64) DEFAULT NULL COMMENT '操作系统机器编号',
  `agentTokenHash` VARCHAR(64) NOT NULL COMMENT '代理令牌 SHA-256',
  `statusFlag` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '是否允许上报：Y允许，N停用',
  `lastSeenTime` DATETIME DEFAULT NULL COMMENT '最近心跳时间',
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
  PRIMARY KEY (`tenantId`, `agentId`),
  KEY `IDX_FG_AGENT_NAME` (`tenantId`, `agentName`),
  KEY `IDX_FG_AGENT_SEEN` (`tenantId`, `lastSeenTime`),
  KEY `IDX_FG_AGENT_MACHINE` (`tenantId`, `agentCode`, `machineId`),
  KEY `IDX_FG_AGENT_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='采集代理表';

INSERT INTO `FG_AGENT` (
  `agentId`, `tenantId`, `agentName`, `hostName`, `ipAddress`, `agentTokenHash`, `statusFlag`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES (
  'agent-local', 'default', '本机开发代理', 'localhost', '127.0.0.1',
  '4a1209434b00b83b89b56e53bf652d9aa3d63c7c0bd9492724cfd2ca38030360',
  'Y', 'system', 'system', 'INIT', 1, 'Y',
  '开发态固定令牌，生产环境登记后只显示一次'
);
