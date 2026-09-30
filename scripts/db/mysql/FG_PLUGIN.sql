CREATE TABLE IF NOT EXISTS `FG_PLUGIN` (
  `pluginId` VARCHAR(32) NOT NULL COMMENT '插件ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `pluginCode` VARCHAR(64) NOT NULL COMMENT '插件编码，同租户内由应用保证唯一',
  `pluginName` VARCHAR(100) NOT NULL COMMENT '插件名称',
  `pluginKind` VARCHAR(20) NOT NULL COMMENT '服务器组件种类(SINK转发,MODULE菜单)',
  `pluginScope` VARCHAR(20) NOT NULL COMMENT '运行位置，固定 SERVER',
  `builtinFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '内置插件标记(Y内置,N外部)',
  `pluginStatus` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '插件状态(Y启用,N停用)',
  `entryPoint` VARCHAR(255) DEFAULT NULL COMMENT 'Go 包路径或外部命令',
  `configSchema` TEXT DEFAULT NULL COMMENT '配置说明，JSON',
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
  PRIMARY KEY (`tenantId`, `pluginId`),
  KEY `IDX_FG_PLUGIN_CODE` (`tenantId`, `pluginCode`),
  KEY `IDX_FG_PLUGIN_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='插件目录表';

INSERT INTO `FG_PLUGIN` (
  `pluginId`, `tenantId`, `pluginCode`, `pluginName`, `pluginKind`, `pluginScope`,
  `builtinFlag`, `pluginStatus`, `entryPoint`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`
) VALUES
('plugin-examplealert', 'default', 'examplealert', '示例告警', 'MODULE', 'SERVER', 'Y', 'Y', 'fluxguard/server/plugin/examplealert', 'system', 'system', 'INIT', 1, 'Y'),
('plugin-recent', 'default', 'recent', '近期样本库', 'SINK', 'SERVER', 'Y', 'Y', 'fluxguard/server/ingest', 'system', 'system', 'INIT', 1, 'Y');
