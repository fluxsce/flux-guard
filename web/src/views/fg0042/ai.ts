import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { createChannel, deleteChannel, queryChannels, testChannel, updateChannel, type ChannelInput } from './api'

const flag = (name: string, description: string) => ({
  name,
  type: 'string' as const,
  description,
  enum: ['Y', 'N'],
})

function channelBody(args: Record<string, unknown>): ChannelInput {
  return {
    channelId: argString(args, 'channelId'),
    channelName: argString(args, 'channelName'),
    channelKind: argString(args, 'channelKind'),
    endpoint: argString(args, 'endpoint'),
    port: argNumber(args, 'port', 0),
    username: argString(args, 'username'),
    secret: argString(args, 'secret'),
    sender: argString(args, 'sender'),
    target: argString(args, 'target'),
    optionText: argString(args, 'optionText'),
    extraText: argString(args, 'extraText'),
    templateId: argString(args, 'templateId'),
    statusFlag: argString(args, 'statusFlag') || 'Y',
    activeFlag: 'Y',
    noteText: argString(args, 'noteText'),
  }
}

const channelFields = [
  { name: 'channelId', type: 'string' as const, description: '渠道编码，新增可留空' },
  { name: 'channelName', type: 'string' as const, description: '渠道名称', required: true },
  { name: 'channelKind', type: 'string' as const, description: '种类 email、wecom、dingtalk、qq、feishu、webhook、sms', required: true },
  { name: 'endpoint', type: 'string' as const, description: '地址。不要在回复里复述 Webhook 地址' },
  { name: 'port', type: 'number' as const, description: '端口，邮件默认 465' },
  { name: 'username', type: 'string' as const, description: '账号' },
  { name: 'sender', type: 'string' as const, description: '发件人' },
  { name: 'target', type: 'string' as const, description: '收件人、群号或手机号' },
  { name: 'optionText', type: 'string' as const, description: '邮件 Y 为 STARTTLS；QQ 为 group 或 private' },
  { name: 'extraText', type: 'string' as const, description: '扩展 JSON' },
  { name: 'templateId', type: 'string' as const, description: '预警模板编码，留空使用该种类的内置模板' },
  flag('statusFlag', '状态，Y 启用，N 停用'),
  { name: 'noteText', type: 'string' as const, description: '备注' },
]

defineTools([
  {
    name: 'fg0042.query',
    moduleId: 'fg0042',
    title: '查询通知渠道',
    description: '按页列出租户下的通知渠道。返回 items 和 total。密钥和完整地址不会出现在页面快照里。',
    risk: 'read',
    resourceCode: 'fg0042:view',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'channelName', type: 'string', description: '渠道名，包含匹配' },
      { name: 'channelKind', type: 'string', description: '渠道类型' },
      flag('statusFlag', '状态'),
    ],
    execute: async (args) => unwrap(await queryChannels(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      channelName: argString(args, 'channelName'),
      channelKind: argString(args, 'channelKind'),
      statusFlag: argString(args, 'statusFlag'),
    })), '加载失败'),
  },
  {
    name: 'fg0042.create',
    moduleId: 'fg0042',
    title: '新增通知渠道',
    description: '创建渠道。密钥只在本次调用中使用。',
    risk: 'write',
    resourceCode: 'fg0042:edit',
    parameters: [{ name: 'secret', type: 'string' as const, description: '密钥', secret: true }, ...channelFields],
    execute: async (args) => unwrap(await createChannel(channelBody(args)), '保存失败'),
  },
  {
    name: 'fg0042.update',
    moduleId: 'fg0042',
    title: '修改通知渠道',
    description: '修改渠道。密钥留空表示不改。',
    risk: 'write',
    resourceCode: 'fg0042:edit',
    parameters: [
      { name: 'secret', type: 'string' as const, description: '新密钥，留空则不修改', secret: true },
      ...channelFields.map((field) => (field.name === 'channelId' ? { ...field, required: true } : field)),
    ],
    execute: async (args) => unwrap(await updateChannel(channelBody(args)), '保存失败'),
  },
  {
    name: 'fg0042.delete',
    moduleId: 'fg0042',
    title: '删除通知渠道',
    description: '按渠道编码删除。',
    risk: 'write',
    resourceCode: 'fg0042:edit',
    parameters: [{ name: 'channelId', type: 'string', description: '渠道编码', required: true }],
    execute: async (args) => unwrap(await deleteChannel(argString(args, 'channelId')), '删除失败'),
  },
  {
    name: 'fg0042.test',
    moduleId: 'fg0042',
    title: '测试通知渠道',
    description: '向该渠道发送一条样例预警。',
    risk: 'write',
    resourceCode: 'fg0042:edit',
    parameters: [{ name: 'channelId', type: 'string', description: '渠道编码', required: true }],
    execute: async (args) => unwrap(await testChannel(argString(args, 'channelId')), '发送失败'),
  },
  {
    name: 'fg0042.applySearch',
    moduleId: 'fg0042',
    title: '筛选通知渠道',
    description: '把筛选条件应用到已经打开的渠道页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0042:view',
    scope: 'scene',
    parameters: [
      { name: 'channelName', type: 'string', description: '名称' },
      { name: 'channelKind', type: 'string', description: '种类' },
      flag('statusFlag', '状态'),
    ],
  },
])
