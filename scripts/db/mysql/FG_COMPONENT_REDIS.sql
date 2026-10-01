-- Redis 当前值。每个连接一行，采集时覆盖。列表只读这张表，不扫 FG_COMPONENT_REDIS_METRIC。
-- 字段不超过 32 列，只放列表要看的指标。
CREATE TABLE IF NOT EXISTS `FG_COMPONENT_REDIS` (
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `componentId` VARCHAR(32) NOT NULL COMMENT '连接ID，联合主键',
  `observedTime` DATETIME NOT NULL COMMENT '最近观测时间',
  `usedMemoryBytes` DECIMAL(20, 3) NOT NULL DEFAULT 0 COMMENT '已用内存，字节',
  `memUsagePercent` DECIMAL(20, 3) NOT NULL DEFAULT 0 COMMENT '已用占上限的百分比。未设上限时为 -1',
  `aofOk` DECIMAL(20, 3) NOT NULL DEFAULT 0 COMMENT '最近一次 AOF：1 成功，0 失败，-1 未开启',
  `slowlogLen` DECIMAL(20, 3) NOT NULL DEFAULT 0 COMMENT '慢操作条数',
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
  PRIMARY KEY (`tenantId`, `componentId`),
  KEY `IDX_FG_COMPONENT_REDIS_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='Redis 当前值表';
