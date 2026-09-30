import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { createTemplate, deleteTemplate, previewTemplate, queryTemplates, updateTemplate, type TemplateInput } from './api'

const flag = (name: string, description: string) => ({
  name,
  type: 'string' as const,
  description,
  enum: ['Y', 'N'],
})

function templateBody(args: Record<string, unknown>): TemplateInput {
  return {
    templateId: argString(args, 'templateId'),
    templateName: argString(args, 'templateName'),
    channelKind: argString(args, 'channelKind'),
    subjectText: argString(args, 'subjectText'),
    bodyText: argString(args, 'bodyText'),
    contentType: argString(args, 'contentType') || 'text',
    statusFlag: argString(args, 'statusFlag') || 'Y',
    activeFlag: 'Y',
    noteText: argString(args, 'noteText'),
  }
}

const templateFields = [
  { name: 'templateId', type: 'string' as const, description: '模板编码，新增可留空' },
  { name: 'templateName', type: 'string' as const, description: '模板名称', required: true },
  { name: 'channelKind', type: 'string' as const, description: '渠道种类 email、wecom、dingtalk、qq、feishu、webhook、sms', required: true },
  { name: 'subjectText', type: 'string' as const, description: '标题，可用 {{hostName}} 等占位符', required: true },
  { name: 'bodyText', type: 'string' as const, description: '正文。邮件用 HTML，企业微信和钉钉用 Markdown', required: true },
  { name: 'contentType', type: 'string' as const, description: 'text、markdown 或 html' },
  flag('statusFlag', '状态，Y 启用，N 停用'),
  { name: 'noteText', type: 'string' as const, description: '备注' },
]

defineTools([
  {
    name: 'fg0041.query',
    moduleId: 'fg0041',
    title: '查询预警模板',
    description: '按页列出租户下的预警模板。返回 items 和 total。',
    risk: 'read',
    resourceCode: 'fg0041:view',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'templateName', type: 'string', description: '模板名，包含匹配' },
      { name: 'channelKind', type: 'string', description: '渠道种类' },
      flag('statusFlag', '状态'),
    ],
    execute: async (args) => unwrap(await queryTemplates(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      templateName: argString(args, 'templateName'),
      channelKind: argString(args, 'channelKind'),
      statusFlag: argString(args, 'statusFlag'),
    })), '加载失败'),
  },
  {
    name: 'fg0041.create',
    moduleId: 'fg0041',
    title: '新增预警模板',
    description: '创建一种渠道种类的标题和正文。同一种类的渠道可以共用。',
    risk: 'write',
    resourceCode: 'fg0041:edit',
    parameters: templateFields,
    execute: async (args) => unwrap(await createTemplate(templateBody(args)), '保存失败'),
  },
  {
    name: 'fg0041.update',
    moduleId: 'fg0041',
    title: '修改预警模板',
    description: '修改模板文案。内置模板不能改种类。',
    risk: 'write',
    resourceCode: 'fg0041:edit',
    parameters: templateFields.map((field) => (field.name === 'templateId' ? { ...field, required: true } : field)),
    execute: async (args) => unwrap(await updateTemplate(templateBody(args)), '保存失败'),
  },
  {
    name: 'fg0041.delete',
    moduleId: 'fg0041',
    title: '删除预警模板',
    description: '按模板编码删除自定义模板。内置模板不能删除。',
    risk: 'write',
    resourceCode: 'fg0041:edit',
    parameters: [{ name: 'templateId', type: 'string', description: '模板编码', required: true }],
    execute: async (args) => unwrap(await deleteTemplate(argString(args, 'templateId')), '删除失败'),
  },
  {
    name: 'fg0041.preview',
    moduleId: 'fg0041',
    title: '预览预警模板',
    description: '用样例数据渲染标题和正文，不发送。',
    risk: 'read',
    resourceCode: 'fg0041:view',
    parameters: [
      { name: 'templateId', type: 'string', description: '已保存的模板编码。填写后忽略下面的正文' },
      { name: 'channelKind', type: 'string', description: '渠道种类' },
      { name: 'subjectText', type: 'string', description: '标题' },
      { name: 'bodyText', type: 'string', description: '正文' },
      { name: 'contentType', type: 'string', description: 'text、markdown 或 html' },
    ],
    execute: async (args) => unwrap(await previewTemplate({
      templateId: argString(args, 'templateId'),
      channelKind: argString(args, 'channelKind'),
      subjectText: argString(args, 'subjectText'),
      bodyText: argString(args, 'bodyText'),
      contentType: argString(args, 'contentType'),
    }), '预览失败'),
  },
  {
    name: 'fg0041.applySearch',
    moduleId: 'fg0041',
    title: '筛选预警模板',
    description: '把筛选条件应用到已经打开的模板页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0041:view',
    scope: 'scene',
    parameters: [
      { name: 'templateName', type: 'string', description: '名称' },
      { name: 'channelKind', type: 'string', description: '渠道种类' },
      flag('statusFlag', '状态'),
    ],
  },
])
