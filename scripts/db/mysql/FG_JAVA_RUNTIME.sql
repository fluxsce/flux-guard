-- JVM 进程 CPU 和类加载。每次上报只插入。
CREATE TABLE IF NOT EXISTS `FG_JAVA_RUNTIME` (
  `agentId` VARCHAR(32) NOT NULL COMMENT '代理ID',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `pointId` VARCHAR(32) NOT NULL COMMENT '记录ID，联合主键',
  `batchId` VARCHAR(64) NOT NULL COMMENT '批次号',
  `processCpuPercent` DECIMAL(8, 3) NOT NULL DEFAULT 0 COMMENT '进程 CPU 使用率，百分比；-1 表示本批没有读到',
  `loadedClassCount` BIGINT NOT NULL DEFAULT 0 COMMENT '当前已加载类数',
  `unloadedClassCount` BIGINT NOT NULL DEFAULT 0 COMMENT '累计已卸载类数',
  `observedTime` DATETIME NOT NULL COMMENT '观测时间',
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
  PRIMARY KEY (`tenantId`, `pointId`),
  KEY `IDX_FG_JAVA_RUNTIME_TIME` (`tenantId`, `agentId`, `observedTime`),
  KEY `IDX_FG_JAVA_RUNTIME_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='Java运行时记录表';
ALTER TABLE `FG_JAVA_RUNTIME` ADD COLUMN `pauseOverheadPercent` DECIMAL(8, 3) NOT NULL DEFAULT -1 COMMENT '本拍停顿开销百分比，未知为 -1';
