-- 预警模板。一种渠道种类对应一种正文格式，渠道只保存模板编码。
CREATE TABLE IF NOT EXISTS `FG_ALERT_TEMPLATE` (
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `templateId` VARCHAR(32) NOT NULL COMMENT '模板ID，联合主键',
  `templateName` VARCHAR(100) NOT NULL COMMENT '模板名称',
  `channelKind` VARCHAR(16) NOT NULL COMMENT '渠道种类：email、wecom、dingtalk、qq、feishu、webhook、sms',
  `contentType` VARCHAR(16) NOT NULL DEFAULT 'text' COMMENT '正文类型：text、markdown、html',
  `subjectText` VARCHAR(255) NOT NULL COMMENT '标题模板',
  `bodyText` TEXT NOT NULL COMMENT '正文模板',
  `builtInFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '是否内置(Y内置,N自定义)',
  `statusFlag` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '是否启用(Y启用,N停用)',
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
  PRIMARY KEY (`tenantId`, `templateId`),
  KEY `IDX_FG_ALERT_TEMPLATE_KIND` (`tenantId`, `channelKind`),
  KEY `IDX_FG_ALERT_TEMPLATE_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='预警模板';

-- 默认租户内置模板。渠道种类对应 tpl-{kind}，开发种子只执行一次。
INSERT INTO `FG_ALERT_TEMPLATE` (
  `tenantId`, `templateId`, `templateName`, `channelKind`, `contentType`, `subjectText`, `bodyText`,
  `builtInFlag`, `statusFlag`, `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES
(
  'default', 'tpl-email', '邮件', 'email', 'html',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '<p><b>{{level}} {{scene}} {{ruleName}}</b></p><p>主机 {{hostName}}（{{agentId}}）</p><p>指标 {{metricName}} 当前 {{value}}，阈值 {{operator}} {{threshold}}。</p><p>{{message}}</p>',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
),
(
  'default', 'tpl-wecom', '企业微信', 'wecom', 'markdown',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '**{{level}} {{scene}} {{ruleName}}**
> 主机：{{hostName}}
> 指标：{{metricName}} {{value}}（阈值 {{operator}} {{threshold}}）
> {{message}}',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
),
(
  'default', 'tpl-dingtalk', '钉钉', 'dingtalk', 'markdown',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '**{{level}} {{scene}} {{ruleName}}**
> 主机：{{hostName}}
> 指标：{{metricName}} {{value}}（阈值 {{operator}} {{threshold}}）
> {{message}}',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
),
(
  'default', 'tpl-qq', 'QQ', 'qq', 'text',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '主机 {{hostName}}（{{agentId}}）
指标 {{metricName}} 当前 {{value}}，阈值 {{operator}} {{threshold}}。
{{message}}',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
),
(
  'default', 'tpl-feishu', '飞书', 'feishu', 'text',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '主机 {{hostName}}（{{agentId}}）
指标 {{metricName}} 当前 {{value}}，阈值 {{operator}} {{threshold}}。
{{message}}',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
),
(
  'default', 'tpl-webhook', 'Webhook', 'webhook', 'text',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '主机 {{hostName}}（{{agentId}}）
指标 {{metricName}} 当前 {{value}}，阈值 {{operator}} {{threshold}}。
{{message}}',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
),
(
  'default', 'tpl-sms', '短信', 'sms', 'text',
  '[{{level}}] {{scene}} {{ruleName}} {{hostName}}',
  '{{level}} {{hostName}} {{metricName}} {{value}}，阈值 {{operator}} {{threshold}}',
  'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y', '内置模板'
);
