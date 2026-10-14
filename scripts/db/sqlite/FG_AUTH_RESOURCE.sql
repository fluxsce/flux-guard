-- 资源目录在本脚本。初始授权在 FG_AUTH_ROLE_RESOURCE.sql，之后由角色页维护。
CREATE TABLE IF NOT EXISTS FG_AUTH_RESOURCE (
  resourceId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  resourceName TEXT NOT NULL,
  resourceCode TEXT NOT NULL,
  resourceType TEXT NOT NULL,
  resourcePath TEXT,
  resourceMethod TEXT,
  parentResourceId TEXT,
  resourceLevel INTEGER NOT NULL DEFAULT 1,
  sortOrder INTEGER NOT NULL DEFAULT 0,
  displayName TEXT,
  iconClass TEXT,
  description TEXT,
  language TEXT DEFAULT 'zh-CN',
  resourceStatus TEXT NOT NULL DEFAULT 'Y',
  builtInFlag TEXT NOT NULL DEFAULT 'N',
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
  PRIMARY KEY (tenantId, resourceId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_RES_CODE ON FG_AUTH_RESOURCE(tenantId, resourceCode);
CREATE INDEX IF NOT EXISTS IDX_FG_RES_PARENT ON FG_AUTH_RESOURCE(parentResourceId);
CREATE INDEX IF NOT EXISTS IDX_FG_RES_TYPE ON FG_AUTH_RESOURCE(resourceType);
CREATE INDEX IF NOT EXISTS IDX_FG_AUTH_RESOURCE_ADD ON FG_AUTH_RESOURCE(tenantId, addTime);

-- 内置菜单用 OR IGNORE：语句哈希变了会再跑时，已有行不报错，只补新行。
INSERT OR IGNORE INTO FG_AUTH_RESOURCE (
  resourceId, tenantId, resourceName, resourceCode, resourceType,
  resourcePath, parentResourceId, resourceLevel, sortOrder, iconClass, language,
  resourceStatus, builtInFlag, addWho, editWho, oprSeqFlag, currentVersion, activeFlag
) VALUES
('group:observe', 'default', '运行观测', 'group:observe', 'GROUP', NULL, NULL, 1, 10, 'activity', 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('group:system', 'default', '系统管理', 'group:system', 'GROUP', NULL, NULL, 1, 20, 'settings', 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0001', 'default', '总览', 'fg0001', 'MODULE', '/fg0001', 'group:observe', 2, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0001:view', 'default', '查看', 'fg0001:view', 'BUTTON', NULL, 'fg0001', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002', 'default', '采集代理', 'fg0002', 'MODULE', '/fg0002', 'group:observe', 2, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002:search', 'default', '查询', 'fg0002:search', 'BUTTON', NULL, 'fg0002', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002:view', 'default', '查看', 'fg0002:view', 'BUTTON', NULL, 'fg0002', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002:edit', 'default', '登记代理', 'fg0002:edit', 'BUTTON', NULL, 'fg0002', 3, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002:command', 'default', '下发命令', 'fg0002:command', 'BUTTON', NULL, 'fg0002', 3, 40, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002:upgrade', 'default', '升级代理', 'fg0002:upgrade', 'BUTTON', NULL, 'fg0002', 3, 45, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0002:shell', 'default', '打开终端', 'fg0002:shell', 'BUTTON', NULL, 'fg0002', 3, 50, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0005', 'default', '主机监控', 'fg0005', 'MODULE', '/fg0005', 'group:observe', 2, 45, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0005:search', 'default', '查询', 'fg0005:search', 'BUTTON', NULL, 'fg0005', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0005:view', 'default', '监控', 'fg0005:view', 'BUTTON', NULL, 'fg0005', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0005:edit', 'default', '维护规则', 'fg0005:edit', 'BUTTON', NULL, 'fg0005', 3, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0006', 'default', 'Java 监控', 'fg0006', 'MODULE', '/fg0006', 'group:observe', 2, 46, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0006:search', 'default', '查询', 'fg0006:search', 'BUTTON', NULL, 'fg0006', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0006:view', 'default', '监控', 'fg0006:view', 'BUTTON', NULL, 'fg0006', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007', 'default', '组件监控', 'fg0007', 'MODULE', '/fg0007', 'group:observe', 2, 47, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:search', 'default', '查询', 'fg0007:search', 'BUTTON', NULL, 'fg0007', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:view', 'default', '监控', 'fg0007:view', 'BUTTON', NULL, 'fg0007', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:edit', 'default', '维护连接', 'fg0007:edit', 'BUTTON', NULL, 'fg0007', 3, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:redis', 'default', 'Redis', 'fg0007:redis', 'BUTTON', NULL, 'fg0007', 3, 40, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:mysql', 'default', 'MySQL', 'fg0007:mysql', 'BUTTON', NULL, 'fg0007', 3, 50, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:mongo', 'default', 'MongoDB', 'fg0007:mongo', 'BUTTON', NULL, 'fg0007', 3, 60, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:kafka', 'default', 'Kafka', 'fg0007:kafka', 'BUTTON', NULL, 'fg0007', 3, 70, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:zk', 'default', 'ZooKeeper', 'fg0007:zk', 'BUTTON', NULL, 'fg0007', 3, 80, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:nacos', 'default', 'Nacos', 'fg0007:nacos', 'BUTTON', NULL, 'fg0007', 3, 90, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0007:shell', 'default', '打开终端', 'fg0007:shell', 'BUTTON', NULL, 'fg0007', 3, 100, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0008', 'default', '应用', 'fg0008', 'MODULE', '/fg0008', 'group:observe', 2, 48, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0008:search', 'default', '查询', 'fg0008:search', 'BUTTON', NULL, 'fg0008', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0008:view', 'default', '查看', 'fg0008:view', 'BUTTON', NULL, 'fg0008', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0008:edit', 'default', '改名', 'fg0008:edit', 'BUTTON', NULL, 'fg0008', 3, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0008:release', 'default', '更新', 'fg0008:release', 'BUTTON', NULL, 'fg0008', 3, 40, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021', 'default', '用户管理', 'fg0021', 'MODULE', '/fg0021', 'group:system', 2, 60, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021:search', 'default', '查询', 'fg0021:search', 'BUTTON', NULL, 'fg0021', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021:view', 'default', '查看', 'fg0021:view', 'BUTTON', NULL, 'fg0021', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021:add', 'default', '新增', 'fg0021:add', 'BUTTON', NULL, 'fg0021', 3, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021:edit', 'default', '修改', 'fg0021:edit', 'BUTTON', NULL, 'fg0021', 3, 40, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021:delete', 'default', '删除', 'fg0021:delete', 'BUTTON', NULL, 'fg0021', 3, 50, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0021:grant', 'default', '授权', 'fg0021:grant', 'BUTTON', NULL, 'fg0021', 3, 60, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022', 'default', '角色管理', 'fg0022', 'MODULE', '/fg0022', 'group:system', 2, 70, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022:search', 'default', '查询', 'fg0022:search', 'BUTTON', NULL, 'fg0022', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022:view', 'default', '查看', 'fg0022:view', 'BUTTON', NULL, 'fg0022', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022:add', 'default', '新增', 'fg0022:add', 'BUTTON', NULL, 'fg0022', 3, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022:edit', 'default', '修改', 'fg0022:edit', 'BUTTON', NULL, 'fg0022', 3, 40, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022:delete', 'default', '删除', 'fg0022:delete', 'BUTTON', NULL, 'fg0022', 3, 50, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0022:grant', 'default', '授权', 'fg0022:grant', 'BUTTON', NULL, 'fg0022', 3, 60, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0023', 'default', '资源管理', 'fg0023', 'MODULE', '/fg0023', 'group:system', 2, 80, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0023:search', 'default', '查询', 'fg0023:search', 'BUTTON', NULL, 'fg0023', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0023:view', 'default', '查看', 'fg0023:view', 'BUTTON', NULL, 'fg0023', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0024', 'default', '环境设置', 'fg0024', 'MODULE', '/fg0024', 'group:system', 2, 90, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0024:view', 'default', '查看', 'fg0024:view', 'BUTTON', NULL, 'fg0024', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0024:edit', 'default', '维护', 'fg0024:edit', 'BUTTON', NULL, 'fg0024', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0025', 'default', '任务调度', 'fg0025', 'MODULE', '/fg0025', 'group:system', 2, 95, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0025:view', 'default', '查看', 'fg0025:view', 'BUTTON', NULL, 'fg0025', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0025:edit', 'default', '维护', 'fg0025:edit', 'BUTTON', NULL, 'fg0025', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0026', 'default', '插件管理', 'fg0026', 'MODULE', '/fg0026', 'group:system', 2, 96, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0026:view', 'default', '查看', 'fg0026:view', 'BUTTON', NULL, 'fg0026', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('group:alert', 'default', '预警管理', 'group:alert', 'GROUP', NULL, NULL, 1, 15, 'bell', 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0041', 'default', '预警模板', 'fg0041', 'MODULE', '/fg0041', 'group:alert', 2, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0041:view', 'default', '查看', 'fg0041:view', 'BUTTON', NULL, 'fg0041', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0041:edit', 'default', '维护', 'fg0041:edit', 'BUTTON', NULL, 'fg0041', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0042', 'default', '通知渠道', 'fg0042', 'MODULE', '/fg0042', 'group:alert', 2, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0042:view', 'default', '查看', 'fg0042:view', 'BUTTON', NULL, 'fg0042', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0042:edit', 'default', '维护', 'fg0042:edit', 'BUTTON', NULL, 'fg0042', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0043', 'default', '预警事件', 'fg0043', 'MODULE', '/fg0043', 'group:alert', 2, 30, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0043:view', 'default', '查看', 'fg0043:view', 'BUTTON', NULL, 'fg0043', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0043:edit', 'default', '处理', 'fg0043:edit', 'BUTTON', NULL, 'fg0043', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('group:log', 'default', '日志管理', 'group:log', 'GROUP', NULL, NULL, 1, 25, 'scroll-text', 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0051', 'default', '系统操作日志', 'fg0051', 'MODULE', '/fg0051', 'group:log', 2, 110, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0051:search', 'default', '查询', 'fg0051:search', 'BUTTON', NULL, 'fg0051', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0051:view', 'default', '查看', 'fg0051:view', 'BUTTON', NULL, 'fg0051', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0052', 'default', '代理操作日志', 'fg0052', 'MODULE', '/fg0052', 'group:log', 2, 120, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0052:search', 'default', '查询', 'fg0052:search', 'BUTTON', NULL, 'fg0052', 3, 10, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y'),
('fg0052:view', 'default', '查看', 'fg0052:view', 'BUTTON', NULL, 'fg0052', 3, 20, NULL, 'zh-CN', 'Y', 'Y', 'system', 'system', 'INIT', 1, 'Y');

-- 插件管理从观测号段挪到系统管理。已有库每次启动都会走到这里，没有旧行时不影响。
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0026', resourceCode = 'fg0026', resourceName = '插件管理', resourcePath = '/fg0026', sortOrder = 96
WHERE tenantId = 'default' AND resourceId = 'fg0003';
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0026:view', resourceCode = 'fg0026:view', parentResourceId = 'fg0026'
WHERE tenantId = 'default' AND resourceId = 'fg0003:view';

-- 应用管理从独立分组 fg0071 挪到运行观测 fg0008。
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0008:release', resourceCode = 'fg0008:release', parentResourceId = 'fg0008'
WHERE tenantId = 'default' AND resourceId = 'fg0071:release';
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0008:edit', resourceCode = 'fg0008:edit', parentResourceId = 'fg0008'
WHERE tenantId = 'default' AND resourceId = 'fg0071:edit';
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0008:view', resourceCode = 'fg0008:view', parentResourceId = 'fg0008'
WHERE tenantId = 'default' AND resourceId = 'fg0071:view';
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0008:search', resourceCode = 'fg0008:search', parentResourceId = 'fg0008'
WHERE tenantId = 'default' AND resourceId = 'fg0071:search';
UPDATE FG_AUTH_RESOURCE
SET resourceId = 'fg0008', resourceCode = 'fg0008', resourcePath = '/fg0008', parentResourceId = 'group:observe', sortOrder = 48
WHERE tenantId = 'default' AND resourceId = 'fg0071';
UPDATE FG_AUTH_RESOURCE
SET activeFlag = 'N', resourceStatus = 'N'
WHERE tenantId = 'default' AND resourceId = 'group:app';

-- 已有库若已有 fg0008 行，校正归属到运行观测。
UPDATE FG_AUTH_RESOURCE
SET resourceName = '应用', resourceCode = 'fg0008', resourcePath = '/fg0008', parentResourceId = 'group:observe',
    sortOrder = 48, resourceStatus = 'Y', activeFlag = 'Y'
WHERE tenantId = 'default' AND resourceId = 'fg0008';

-- 示例告警没有业务页。已有库每次启动都会停用，侧栏不再挂出。
UPDATE FG_AUTH_RESOURCE
SET activeFlag = 'N', resourceStatus = 'N'
WHERE tenantId = 'default' AND resourceId IN ('fg0100', 'fg0100:view');
