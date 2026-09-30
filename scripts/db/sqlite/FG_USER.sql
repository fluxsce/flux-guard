-- 口令使用 bcrypt，不能还原明文。下面的哈希对应开发口令 fluxguard，上线前必须更换。
-- 用户不挂部门。tenantAdminFlag 为 Y 的是租户管理员。其余能看什么、能改什么，由角色资源决定。
CREATE TABLE IF NOT EXISTS FG_USER (
  userId TEXT NOT NULL,
  tenantId TEXT NOT NULL,
  userName TEXT NOT NULL,
  password TEXT NOT NULL,
  realName TEXT NOT NULL,
  email TEXT,
  mobile TEXT,
  avatar TEXT,
  gender INTEGER NOT NULL DEFAULT 0,
  statusFlag TEXT NOT NULL DEFAULT 'Y',
  tenantAdminFlag TEXT NOT NULL DEFAULT 'N',
  userExpireDate TEXT NOT NULL,
  lastLoginTime TEXT,
  lastLoginIp TEXT,
  pwdUpdateTime TEXT,
  mustChangePwd TEXT NOT NULL DEFAULT 'N',
  pwdErrorCount INTEGER NOT NULL DEFAULT 0,
  lockTime TEXT,
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
  PRIMARY KEY (tenantId, userId)
);
CREATE INDEX IF NOT EXISTS IDX_FG_USER_NAME ON FG_USER(tenantId, userName);
CREATE INDEX IF NOT EXISTS IDX_FG_USER_STATUS ON FG_USER(statusFlag);
CREATE INDEX IF NOT EXISTS IDX_FG_USER_ADD ON FG_USER(tenantId, addTime);

INSERT INTO FG_USER (
  userId, tenantId, userName, password, realName, email,
  statusFlag, tenantAdminFlag, userExpireDate, mustChangePwd,
  addWho, editWho, oprSeqFlag, currentVersion, activeFlag, noteText
) VALUES
(
  'admin', 'default', 'admin',
  '$2a$10$uA9NBgCnzMPOkTZW3E1Xxurn1xjj9raTjnUa1AbHzgPxHPGJMfTtK',
  '系统管理员', 'admin@example.com',
  'Y', 'Y', datetime('now', '+5 years'), 'N',
  'system', 'system', 'INIT', 1, 'Y', '骨架开发账号，上线前更换口令'
),
(
  'viewer', 'default', 'viewer',
  '$2a$10$uA9NBgCnzMPOkTZW3E1Xxurn1xjj9raTjnUa1AbHzgPxHPGJMfTtK',
  '只读用户', 'viewer@example.com',
  'Y', 'N', datetime('now', '+5 years'), 'N',
  'system', 'system', 'INIT', 1, 'Y', '只读账号，口令与开发管理员相同'
);
