-- 一个应用落在某套运行时上的一个实例。身份是代理、实例名加应用名。
-- 一台机器一个应用代理，可以看守多套 Tomcat。应用进程停了这一行还在。
CREATE TABLE IF NOT EXISTS `FG_APP_INST` (
  `instId` VARCHAR(32) NOT NULL COMMENT '实例ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `agentId` VARCHAR(32) NOT NULL COMMENT '应用侧代理ID',
  `appCode` VARCHAR(100) NOT NULL COMMENT '应用编码',
  `machineId` VARCHAR(64) DEFAULT NULL COMMENT '机器编号',
  `hostName` VARCHAR(100) DEFAULT NULL COMMENT '主机名',
  `ipAddress` VARCHAR(128) DEFAULT NULL COMMENT 'IP 地址',
  `instanceName` VARCHAR(100) NOT NULL COMMENT '实例名',
  `runtimeKind` VARCHAR(16) NOT NULL COMMENT '运行方式(systemd,windows,docker)',
  `artifactKind` VARCHAR(16) NOT NULL COMMENT '制品种类(war,jar,static)',
  `artifactPath` VARCHAR(255) DEFAULT NULL COMMENT '制品路径',
  `versionText` VARCHAR(64) DEFAULT NULL COMMENT '当前版本',
  `tomcatVersion` VARCHAR(64) DEFAULT NULL COMMENT 'Tomcat 版本',
  `runningFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '应用进程是否在跑(Y在跑,N已停)',
  `commandAddress` VARCHAR(128) DEFAULT NULL COMMENT '命令端口',
  `reportedTime` DATETIME NOT NULL COMMENT '最近一次上报时间',
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
  PRIMARY KEY (`tenantId`, `instId`),
  UNIQUE KEY `IDX_FG_APP_INST_SLOT` (`tenantId`, `agentId`, `instanceName`, `appCode`),
  KEY `IDX_FG_APP_INST_APP` (`tenantId`, `appCode`),
  KEY `IDX_FG_APP_INST_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='应用实例';

ALTER TABLE `FG_APP_INST` DROP INDEX `IDX_FG_APP_INST_AGENT`;
ALTER TABLE `FG_APP_INST` ADD UNIQUE KEY `IDX_FG_APP_INST_SLOT` (`tenantId`, `agentId`, `instanceName`, `appCode`);
