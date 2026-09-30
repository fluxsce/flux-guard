import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { queryLogs } from './api'

defineTools([
  {
    name: 'fg0043.query',
    moduleId: 'fg0043',
    title: '查询预警日志',
    description: '按页列出租户下的预警日志。返回 items 和 total。',
    risk: 'read',
    resourceCode: 'fg0043:view',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'alertTitle', type: 'string', description: '标题，包含匹配' },
      { name: 'alertType', type: 'string', description: '类型，如 host' },
      { name: 'alertLevel', type: 'string', description: '级别 info、warn、critical' },
      { name: 'sendStatus', type: 'string', description: '发送状态 pending、ok、fail' },
    ],
    execute: async (args) => unwrap(await queryLogs(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      alertTitle: argString(args, 'alertTitle'),
      alertType: argString(args, 'alertType'),
      alertLevel: argString(args, 'alertLevel'),
      sendStatus: argString(args, 'sendStatus'),
    })), '加载失败'),
  },
  {
    name: 'fg0043.applySearch',
    moduleId: 'fg0043',
    title: '筛选预警日志',
    description: '把筛选条件应用到已经打开的预警日志页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0043:view',
    scope: 'scene',
    parameters: [
      { name: 'alertTitle', type: 'string', description: '标题' },
      { name: 'alertType', type: 'string', description: '类型' },
      { name: 'alertLevel', type: 'string', description: '级别' },
      { name: 'sendStatus', type: 'string', description: '发送状态 pending、ok、fail' },
    ],
  },
])
