-- 预警模板。一种渠道种类对应一种正文格式，渠道只保存模板编码。
CREATE TABLE IF NOT EXISTS FG_ALERT_TEMPLATE (
  tenantId TEXT NOT NULL,
  templateId TEXT NOT NULL,
  templateName TEXT NOT NULL,
  channelKind TEXT NOT NULL,
  contentType TEXT NOT NULL DEFAULT 'text',
  subjectText TEXT NOT NULL,
  bodyText TEXT NOT NULL,
  builtInFlag TEXT NOT NULL DEFAULT 'N',
  statusFlag TEXT NOT NULL DEFAULT 'Y',
  addTime TEXT NOT NULL DEFAULT (datetime('now')),
  addWho TEXT NOT NULL,
  editTime TEXT NOT NULL DEFAULT (datetime('now')),
  editWho TEXT NOT NULL,
  oprSeqFlag TEXT NOT NULL,
  currentVersion INTEGER NOT NULL DEFAULT 1,
  activeFlag TEXT NOT NULL DEFAULT 'Y',
  noteText TEXT,
  extProperty TEXT,
  reserved1 TEXT,
  reserved2 TEXT,
  reserved3 TEXT,
  reserved4 TEXT,
  reserved5 TEXT,
  PRIMARY KEY (tenantId, templateId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_TEMPLATE_KIND ON FG_ALERT_TEMPLATE(tenantId, channelKind);
CREATE INDEX IF NOT EXISTS IDX_FG_ALERT_TEMPLATE_ADD ON FG_ALERT_TEMPLATE(tenantId, addTime);

-- 默认租户内置模板。渠道种类对应 tpl-{kind}，开发种子只执行一次。
INSERT INTO FG_ALERT_TEMPLATE (
  tenantId, templateId, templateName, channelKind, contentType, subjectText, bodyText,
  builtInFlag, statusFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
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
