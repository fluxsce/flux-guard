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
const mongoAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: '用户名口令', value: 'password' },
]
const kafkaAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: 'SASL PLAIN', value: 'plain' },
]
const nacosAuthOptions: FlagOption[] = [
  { label: '无', value: 'none' },
  { label: '控制台登录', value: 'login' },
]

const deployNames: Record<string, string> = {
  standalone: '单机',
  sentinel: '哨兵',
  cluster: '集群',
  server: '服务',
}
const authNames: Record<string, string> = {
  none: '无认证',
  password: '口令',
  acl: 'ACL',
  plain: 'PLAIN',
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
    deploy = 'standalone'
    auth = auth === 'password' ? 'password' : 'none'
  } else if (kindCode === 'kafka') {
    deploy = 'cluster'
    auth = auth === 'plain' ? 'plain' : 'none'
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
  if (kindCode === 'mongo') return `standalone:${authMode === 'password' ? 'password' : 'none'}`
  if (kindCode === 'kafka') return `cluster:${authMode === 'plain' ? 'plain' : 'none'}`
  if (kindCode === 'zk') return 'standalone:none'
  if (kindCode === 'nacos') return `server:${authMode === 'login' ? 'login' : 'none'}`
  return ''
}

export function connFormFields(kindOptions: FlagOption[]): FgFormField[] {
  return [
    ...tabOf(basicTab, [
      fieldset('basicGroup', '基本信息', [
        { field: 'componentName', label: '名称', required: true },
        { field: 'kindCode', label: '种类', type: 'select', required: true, options: kindOptions },
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
        { tips: '页面会标出复制延迟和缓冲池命中。复制延迟没有数据时表示当前看不出主从。', show: kindIs('mysql') },
      ),
      fieldset(
        'mongoGroup',
        'MongoDB',
        [
          { field: 'address', label: '地址', span: 16, required: true, placeholder: 'host:27017' },
          { field: 'authMode', label: '认证', type: 'select', required: true, options: mongoAuthOptions },
          { field: 'userName', label: '用户名', required: true, show: authIs('mongo', 'password') },
          { field: 'secret', label: '口令', type: 'password', modes: ['create', 'edit'], placeholder: '留空表示不修改', show: authIs('mongo', 'password') },
          { field: 'secretSet', label: '口令', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('mongo', 'password') },
          { field: 'databaseName', label: '认证库', placeholder: '默认 admin', show: authIs('mongo', 'password') },
        ],
        { tips: '无认证可读 serverStatus。用户名口令当前不走 SCRAM，采集会记为连不上。', show: kindIs('mongo') },
      ),
      fieldset(
        'kafkaGroup',
        'Kafka',
        [
          { field: 'address', label: 'Broker', span: 16, required: true, placeholder: 'host:9092，多个用逗号分隔' },
          { field: 'authMode', label: '认证', type: 'select', required: true, options: kafkaAuthOptions },
          { field: 'userName', label: '用户名', required: true, show: authIs('kafka', 'plain') },
          { field: 'secret', label: '口令', type: 'password', modes: ['create', 'edit'], placeholder: '留空表示不修改', show: authIs('kafka', 'plain') },
          { field: 'secretSet', label: '口令', type: 'select', options: secretOptions, disabled: true, modes: ['view'], show: authIs('kafka', 'plain') },
        ],
        { tips: '监控分开看 Topic 和消费组。消费延迟是已提交位移相对日志末端的差值，只取前 8 个消费组。', show: kindIs('kafka') },
      ),
      fieldset(
        'zkGroup',
        'ZooKeeper',
        [{ field: 'address', label: '地址', span: 16, required: true, placeholder: 'host:2181' }],
        { tips: '用四字命令 mntr。服务端没打开时，页面会显示连不上。', show: kindIs('zk') },
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
