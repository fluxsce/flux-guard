-- 口令使用 bcrypt，不能还原明文。下面的哈希对应开发口令 fluxguard，上线前必须更换。
CREATE TABLE IF NOT EXISTS `FG_USER` (
  `userId` VARCHAR(32) NOT NULL COMMENT '用户ID，联合主键',
  `tenantId` VARCHAR(32) NOT NULL COMMENT '租户ID，联合主键',
  `userName` VARCHAR(50) NOT NULL COMMENT '用户自定义名称',
  `password` VARCHAR(128) NOT NULL COMMENT '口令，bcrypt 哈希',
  `realName` VARCHAR(50) NOT NULL COMMENT '真实姓名',
  `email` VARCHAR(255) DEFAULT NULL COMMENT '电子邮箱',
  `mobile` VARCHAR(20) DEFAULT NULL COMMENT '手机号码',
  `avatar` VARCHAR(255) DEFAULT NULL COMMENT '头像 URL，大图不要放进此列',
  `gender` INT NOT NULL DEFAULT 0 COMMENT '性别：1男，2女，0未知',
  `statusFlag` VARCHAR(1) NOT NULL DEFAULT 'Y' COMMENT '状态：Y启用，N停用',
  `tenantAdminFlag` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '是否租户管理员：Y是，N否',
  `userExpireDate` DATETIME NOT NULL COMMENT '用户过期时间',
  `lastLoginTime` DATETIME DEFAULT NULL COMMENT '最后登录时间',
  `lastLoginIp` VARCHAR(128) DEFAULT NULL COMMENT '最后登录IP',
  `pwdUpdateTime` DATETIME DEFAULT NULL COMMENT '口令最后更新时间',
  `mustChangePwd` VARCHAR(1) NOT NULL DEFAULT 'N' COMMENT '是否必须修改口令：Y是，N否',
  `pwdErrorCount` INT NOT NULL DEFAULT 0 COMMENT '口令错误次数',
  `lockTime` DATETIME DEFAULT NULL COMMENT '账号锁定时间',
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
  PRIMARY KEY (`tenantId`, `userId`),
  KEY `IDX_FG_USER_NAME` (`tenantId`, `userName`),
  KEY `IDX_FG_USER_STATUS` (`statusFlag`),
  KEY `IDX_FG_USER_ADD` (`tenantId`, `addTime`)
) ENGINE=InnoDB COMMENT='用户信息表';

INSERT INTO `FG_USER` (
  `userId`, `tenantId`, `userName`, `password`, `realName`, `email`,
  `statusFlag`, `tenantAdminFlag`, `userExpireDate`, `mustChangePwd`,
  `addWho`, `editWho`, `oprSeqFlag`, `currentVersion`, `activeFlag`, `noteText`
) VALUES
(
  'admin', 'default', 'admin',
  '$2a$10$uA9NBgCnzMPOkTZW3E1Xxurn1xjj9raTjnUa1AbHzgPxHPGJMfTtK',
  '系统管理员', 'admin@example.com',
  'Y', 'Y', DATE_ADD(NOW(), INTERVAL 5 YEAR), 'N',
  'system', 'system', 'INIT', 1, 'Y', '骨架开发账号，上线前更换口令'
),
(
  'viewer', 'default', 'viewer',
  '$2a$10$uA9NBgCnzMPOkTZW3E1Xxurn1xjj9raTjnUa1AbHzgPxHPGJMfTtK',
  '只读用户', 'viewer@example.com',
  'Y', 'N', DATE_ADD(NOW(), INTERVAL 5 YEAR), 'N',
  'system', 'system', 'INIT', 1, 'Y', '只读账号，口令与开发管理员相同'
);
