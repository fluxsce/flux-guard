import { fieldset, tabOf, basicTab, auditFields } from '@/components/form/audit'
import type { FgFormField } from '@/components/form/types'

type FlagOption = { label: string; value: string }

const statusOptions: FlagOption[] = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]
const secretOptions: FlagOption[] = [
  { label: '已设置', value: 'Y' },
  { label: '未设置', value: 'N' },
]

const kindIs = (kind: string) => (form: Record<string, unknown>) => form.kindCode === kind
const redisDeploy = (...modes: string[]) => (form: Record<string, unknown>) =>
  form.kindCode === 'redis' && modes.includes(String(form.deployMode ?? ''))
const authIs = (kind: string, ...modes: string[]) => (form: Record<string, unknown>) =>
  form.kindCode === kind && modes.includes(String(form.authMode ?? ''))

const redisDeployOptions: FlagOption[] = [
  { label: '单机', value: 'standalone' },
  { label: '哨兵', value: 'sentinel' },
  { label: '集群', value: 'cluster' },
]
const redisAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: '口令', value: 'password' },
  { label: 'ACL', value: 'acl' },
]
const mongoDeployOptions: FlagOption[] = [
  { label: '明文', value: 'standalone' },
  { label: 'TLS', value: 'tls' },
]
const mongoAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: 'SCRAM-SHA-256', value: 'scram256' },
  { label: 'SCRAM-SHA-1', value: 'scram1' },
  { label: 'LDAP PLAIN', value: 'plain' },
  { label: 'X.509', value: 'x509' },
  { label: 'AWS IAM', value: 'aws' },
  { label: 'OIDC', value: 'oidc' },
]
const mongoAuthCodes = ['scram256', 'scram1', 'plain', 'x509', 'aws', 'oidc']
const kafkaDeployOptions: FlagOption[] = [
  { label: '明文', value: 'cluster' },
  { label: 'TLS', value: 'tls' },
]
const kafkaAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: 'SASL PLAIN', value: 'plain' },
  { label: 'SCRAM-SHA-256', value: 'scram256' },
  { label: 'SCRAM-SHA-512', value: 'scram512' },
]
const nacosAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: '控制台登录', value: 'login' },
]

const deployNames: Record<string, string> = {
  standalone: '单机',
  sentinel: '哨兵',
  cluster: '集群',
  tls: 'TLS',
  server: '服务',
}
const authNames: Record<string, string> = {
  none: '无认证',
  password: '口令',
  acl: 'ACL',
  plain: 'PLAIN',
  scram1: 'SCRAM-SHA-1',
  scram256: 'SCRAM-SHA-256',
  scram512: 'SCRAM-SHA-512',
  x509: 'X.509',
  aws: 'AWS IAM',
  oidc: 'OIDC',
  login: '登录',
}

export function layoutOf(kindCode: string, optionText: string, userName: string, secretSet: string) {
  let [deploy, auth] = (optionText || '').split(':')
  if (kindCode === 'redis') {
    deploy = deploy || 'standalone'
    if (!auth) auth = userName ? 'acl' : secretSet === 'Y' ? 'password' : 'none'
  } else if (kindCode === 'mysql') {
    deploy = 'server'
    auth = 'password'
  } else if (kindCode === 'mongo') {
    if (auth === 'password') auth = 'scram256'
    if (!mongoAuthCodes.includes(auth)) auth = 'none'
    deploy = auth === 'x509' || deploy === 'tls' ? 'tls' : 'standalone'
  } else if (kindCode === 'kafka') {
    deploy = deploy === 'tls' ? 'tls' : 'cluster'
    auth = auth === 'plain' || auth === 'scram256' || auth === 'scram512' ? auth : 'none'
  } else if (kindCode === 'zk') {
    deploy = 'standalone'
    auth = 'none'
  } else if (kindCode === 'nacos') {
    deploy = 'server'
    auth = auth === 'login' ? 'login' : 'none'
  }
  return {
    deployMode: deploy,
    authMode: auth,
    label: [deployNames[deploy] || deploy, authNames[auth] || auth].filter(Boolean).join(' · '),
  }
}

export function packOption(kindCode: string, deployMode: string, authMode: string) {
  if (kindCode === 'redis') return `${deployMode || 'standalone'}:${authMode || 'none'}`
  if (kindCode === 'mysql') return 'server:password'
  if (kindCode === 'mongo') {
    const auth = authMode === 'password' ? 'scram256' : mongoAuthCodes.includes(authMode) ? authMode : 'none'
    const deploy = auth === 'x509' || deployMode === 'tls' ? 'tls' : 'standalone'
    return `${deploy}:${auth}`
  }
  if (kindCode === 'kafka') {
    const deploy = deployMode === 'tls' ? 'tls' : 'cluster'
    const auth = authMode === 'plain' || authMode === 'scram256' || authMode === 'scram512' ? authMode : 'none'
    return `${deploy}:${auth}`
  }
  if (kindCode === 'zk') return 'standalone:none'
  if (kindCode === 'nacos') return `server:${authMode === 'login' ? 'login' : 'none'}`
  return ''
}

export function connFormFields(kindOptions: FlagOption[]): FgFormField[] {
  return [
    ...tabOf(basicTab, [
      fieldset('basicGroup', '基本信息', [
        { field: 'componentName', label: '名称', required: true },
        { field: 'kindCode', label: '种类', type: 'select', required: true, disabled: true, options: kindOptions },
        { field: 'intervalSec', label: '间隔(秒)', type: 'number', placeholder: '30' },
        { field: 'statusFlag', label: '启用', type: 'select', options: statusOptions },
        { field: 'noteText', label: '备注', type: 'textarea', span: 24 },
      ]),
      fieldset(
        'redisGroup',
        'Redis',
        [
          { field: 'deployMode', label: '部署', type: 'select', required: true, options: redisDeployOptions },
          { field: 'authMode', label: '认证', type: 'select', required: true, options: redisAuthOptions },
          {
            field: 'address',
            label: '地址',
            span: 16,
            required: true,
            placeholder: 'host:port',
            show: redisDeploy('standalone'),
          },
          {
            field: 'address',
            label: '哨兵地址',
            span: 16,
            required: true,
            placeholder: 'host:26379，多个用逗号分隔',
            show: redisDeploy('sentinel'),
          },
          {
            field: 'databaseName',
            label: '主节点名',
            required: true,
            placeholder: 'mymaster',
            show: redisDeploy('sentinel'),
          },
          {
            field: 'address',
            label: '节点地址',
            span: 16,
            required: true,
            placeholder: 'host:6379，多个用逗号分隔',
            show: redisDeploy('cluster'),
          },
          {
            field: 'databaseName',
            label: '库号',
            placeholder: '0 到 15，可空',
            show: redisDeploy('standalone'),
          },
          {
            field: 'userName',
            label: '用户名',
            required: true,
            show: authIs('redis', 'acl'),
          },
          {
            field: 'secret',
            label: '口令',
            type: 'password',
            modes: ['create', 'edit'],
            placeholder: '留空表示不修改',
            show: authIs('redis', 'password', 'acl'),
          },
          {
            field: 'secretSet',
            label: '口令',
            type: 'select',
            options: secretOptions,
            disabled: true,
            modes: ['view'],
            show: authIs('redis', 'password', 'acl'),
          },
        ],
        { tips: '哨兵口令同时用于哨兵和主节点。监控分开看内存、碎片率、各库键数量和慢操作。', show: kindIs('redis') },
      ),
      fieldset(
        'mysqlGroup',
        'MySQL',
        [
          { field: 'address', label: '地址', span: 16, required: true, placeholder: 'host:3306' },
          { field: 'userName', label: '账号', required: true },
          { field: 'secret', label: '口令', type: 'password', modes: ['create', 'edit'], placeholder: '留空表示不修改' },
          { field: 'secretSet', label: '口令', type: 'select', options: secretOptions, disabled: true, modes: ['view'] },
          { field: 'databaseName', label: '库名', placeholder: '可空，只读账号即可' },
        ],
        { tips: '页面标出连接占用、两次采集之间的语句速率、缓冲池命中、慢查询和复制线程。看不出主从时，复制延迟没有数。', show: kindIs('mysql') },
      ),
      fieldset(
        'mongoGroup',
        'MongoDB',
        [
          { field: 'deployMode', label: '传输', type: 'select', required: true, options: mongoDeployOptions },
          { field: 'address', label: '地址', span: 16, required: true, placeholder: 'host:27017' },
          { field: 'authMode', label: '认证', type: 'select', required: true, options: mongoAuthOptions },
          { field: 'userName', label: '用户名', required: true, show: authIs('mongo', 'scram256', 'scram1', 'plain') },
          { field: 'userName', label: '访问密钥编号', required: true, show: authIs('mongo', 'aws') },
          { field: 'userName', label: '证书主体', placeholder: '可空，留空用证书上的主体', show: authIs('mongo', 'x509') },
          { field: 'secret', label: '口令', type: 'password', modes: ['create', 'edit'], placeholder: '留空表示不修改', show: authIs('mongo', 'scram256', 'scram1', 'plain') },
          { field: 'secret', label: '访问密钥', type: 'password', modes: ['create', 'edit'], placeholder: '临时凭证换行后再写会话令牌', show: authIs('mongo', 'aws') },
          { field: 'secret', label: '客户端证书', type: 'textarea', span: 24, modes: ['create', 'edit'], placeholder: 'PEM 证书和私钥', show: authIs('mongo', 'x509') },
          { field: 'secret', label: '访问令牌', type: 'textarea', span: 24, modes: ['create', 'edit'], placeholder: 'OIDC 访问令牌', show: authIs('mongo', 'oidc') },
          { field: 'secretSet', label: '口令', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('mongo', 'scram256', 'scram1', 'plain') },
          { field: 'secretSet', label: '访问密钥', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('mongo', 'aws') },
          { field: 'secretSet', label: '证书', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('mongo', 'x509') },
          { field: 'secretSet', label: '访问令牌', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('mongo', 'oidc') },
          { field: 'databaseName', label: '认证库', placeholder: '默认 admin', show: authIs('mongo', 'scram256', 'scram1') },
          { field: 'databaseName', label: '认证库', placeholder: '默认 $external', show: authIs('mongo', 'plain') },
        ],
        { tips: 'TLS 使用系统信任的证书。X.509 固定走 TLS。SCRAM 的认证库默认 admin，LDAP 默认 $external。AWS、X.509 和 OIDC 使用 $external。', show: kindIs('mongo') },
      ),
      fieldset(
        'kafkaGroup',
        'Kafka',
        [
          { field: 'deployMode', label: '传输', type: 'select', required: true, options: kafkaDeployOptions },
          { field: 'address', label: 'Broker', span: 16, required: true, placeholder: 'host:9092，多个用逗号分隔' },
          { field: 'authMode', label: '认证', type: 'select', required: true, options: kafkaAuthOptions },
          { field: 'userName', label: '用户名', required: true, show: authIs('kafka', 'plain', 'scram256', 'scram512') },
          { field: 'secret', label: '口令', type: 'password', modes: ['create', 'edit'], placeholder: '留空表示不修改', show: authIs('kafka', 'plain', 'scram256', 'scram512') },
          { field: 'secretSet', label: '口令', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('kafka', 'plain', 'scram256', 'scram512') },
        ],
        { tips: '会看每台 Broker、分区副本、主题最小 ISR 和消费组。延迟按分区计算。TLS 使用系统信任的证书。', show: kindIs('kafka') },
      ),
      fieldset(
        'zkGroup',
        'ZooKeeper',
        [{ field: 'address', label: '地址', span: 16, required: true, placeholder: 'host:2181' }],
        { tips: '先读 mntr。服务端默认只开放 srvr 时，改用 srvr 看连通、角色和节点数。', show: kindIs('zk') },
      ),
      fieldset(
        'nacosGroup',
        'Nacos',
        [
          { field: 'address', label: '地址', span: 16, required: true, placeholder: 'host:8848 或 http 地址' },
          { field: 'authMode', label: '认证', type: 'select', required: true, options: nacosAuthOptions },
          { field: 'userName', label: '用户名', required: true, show: authIs('nacos', 'login') },
          { field: 'secret', label: '口令', type: 'password', modes: ['create', 'edit'], placeholder: '留空表示不修改', show: authIs('nacos', 'login') },
          { field: 'secretSet', label: '口令', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('nacos', 'login') },
        ],
        { tips: '先看控制台健康。配置数和服务数来自 Prometheus 端点，没有时显示 0。', show: kindIs('nacos') },
      ),
    ]),
    ...auditFields,
  ]
}
