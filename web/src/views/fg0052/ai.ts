import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { getLog, queryLogs } from './api'

defineTools([
  {
    name: 'fg0052.query',
    moduleId: 'fg0052',
    title: '查询代理操作日志',
    description: '按页列出命令回执与终端账本的统一时间线。返回 items 和 total。',
    risk: 'read',
    resourceCode: 'fg0052:search',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'agentName', type: 'string', description: '代理名称或编码，包含匹配' },
      { name: 'userName', type: 'string', description: '操作人账号或姓名，包含匹配' },
      { name: 'source', type: 'string', description: '来源', enum: ['COMMAND', 'SHELL'] },
      { name: 'eventType', type: 'string', description: '事件：命令动作或 OPEN、CLOSE、INPUT' },
      { name: 'statusFlag', type: 'string', description: '命令状态', enum: ['D', 'W', 'R', 'Y', 'N'] },
      { name: 'fromTime', type: 'string', description: '开始时间，含该时刻' },
      { name: 'toTime', type: 'string', description: '结束时间，含该时刻' },
    ],
    execute: async (args) => unwrap(await queryLogs(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      agentName: argString(args, 'agentName'),
      userName: argString(args, 'userName'),
      source: argString(args, 'source'),
      eventType: argString(args, 'eventType'),
      statusFlag: argString(args, 'statusFlag'),
      fromTime: argString(args, 'fromTime'),
      toTime: argString(args, 'toTime'),
    })), '加载失败'),
  },
  {
    name: 'fg0052.get',
    moduleId: 'fg0052',
    title: '查看代理操作日志',
    description: '按来源和记录号读取一行代理操作日志。',
    risk: 'read',
    resourceCode: 'fg0052:view',
    parameters: [
      { name: 'source', type: 'string', description: 'COMMAND 或 SHELL', required: true, enum: ['COMMAND', 'SHELL'] },
      { name: 'logId', type: 'string', description: '命令号或账本号', required: true },
    ],
    execute: async (args) => unwrap(await getLog(argString(args, 'source'), argString(args, 'logId')), '加载失败'),
  },
  {
    name: 'fg0052.applySearch',
    moduleId: 'fg0052',
    title: '筛选代理操作日志',
    description: '把筛选条件应用到已经打开的代理操作日志页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0052:search',
    scope: 'scene',
    parameters: [
      { name: 'agentName', type: 'string', description: '代理' },
      { name: 'userName', type: 'string', description: '操作人' },
      { name: 'source', type: 'string', description: '来源 COMMAND 或 SHELL' },
      { name: 'eventType', type: 'string', description: '事件' },
      { name: 'statusFlag', type: 'string', description: '命令状态 D、W、R、Y、N' },
      { name: 'fromTime', type: 'string', description: '开始时间' },
      { name: 'toTime', type: 'string', description: '结束时间' },
    ],
  },
])
