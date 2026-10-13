-- 一个 Java 进程的一代。启动时间变化时收口上一行，再插入新行。
-- 次数和耗时是这一代结束前最后一次上报的累计值。当前代的 closedTime 为空。
CREATE TABLE IF NOT EXISTS `FG_JAVA_RUN` (
  `runId` VARCHAR(32) NOT NULL COMMENT '代次ID，联合主键',
  `agentId` VARCHAR(32) NOT NULL COMMENT '代理ID',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `processId` VARCHAR(32) DEFAULT NULL COMMENT '进程号',
  `startTime` DATETIME NOT NULL COMMENT 'JVM 启动时间',
  `openedTime` DATETIME NOT NULL COMMENT '这一代第一次上报的观测时间',
  `closedTime` DATETIME DEFAULT NULL COMMENT '下一代第一次上报的观测时间，当前代为空',
  `collectionCount` BIGINT NOT NULL DEFAULT 0 COMMENT '这一代累计回收次数',
  `collectionTimeMs` BIGINT NOT NULL DEFAULT 0 COMMENT '这一代累计回收耗时，毫秒',
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
  PRIMARY KEY (`tenantId`, `runId`),
  KEY `IDX_FG_JAVA_RUN_AGENT` (`tenantId`, `agentId`, `openedTime`)
) ENGINE=InnoDB COMMENT='Java进程代次表';
