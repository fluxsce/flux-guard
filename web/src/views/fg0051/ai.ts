import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { getLog, queryLogs } from './api'

defineTools([
  {
    name: 'fg0051.query',
    moduleId: 'fg0051',
    title: '查询系统操作日志',
    description: '按页列出租户下的登录、登出和管理端写操作。返回 items 和 total。',
    risk: 'read',
    resourceCode: 'fg0051:search',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'userName', type: 'string', description: '操作人账号或姓名，包含匹配' },
      { name: 'action', type: 'string', description: '动作', enum: ['CREATE', 'UPDATE', 'DELETE', 'GRANT', 'COMMAND', 'LOGIN', 'LOGIN_FAIL', 'LOGOUT'] },
      { name: 'moduleCode', type: 'string', description: '模块编码，包含匹配' },
      { name: 'resultFlag', type: 'string', description: '结果', enum: ['Y', 'N'] },
      { name: 'fromTime', type: 'string', description: '开始时间，含该时刻' },
      { name: 'toTime', type: 'string', description: '结束时间，含该时刻' },
    ],
    execute: async (args) => unwrap(await queryLogs(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      userName: argString(args, 'userName'),
      action: argString(args, 'action'),
      moduleCode: argString(args, 'moduleCode'),
      resultFlag: argString(args, 'resultFlag'),
      fromTime: argString(args, 'fromTime'),
      toTime: argString(args, 'toTime'),
    })), '加载失败'),
  },
  {
    name: 'fg0051.get',
    moduleId: 'fg0051',
    title: '查看系统操作日志',
    description: '按审计编码读取一行操作日志。',
    risk: 'read',
    resourceCode: 'fg0051:view',
    parameters: [{ name: 'auditId', type: 'string', description: '审计编码', required: true }],
    execute: async (args) => unwrap(await getLog(argString(args, 'auditId')), '加载失败'),
  },
  {
    name: 'fg0051.applySearch',
    moduleId: 'fg0051',
    title: '筛选系统操作日志',
    description: '把筛选条件应用到已经打开的操作日志页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0051:search',
    scope: 'scene',
    parameters: [
      { name: 'userName', type: 'string', description: '操作人' },
      { name: 'action', type: 'string', description: '动作 CREATE、UPDATE、DELETE、GRANT、COMMAND、LOGIN、LOGIN_FAIL、LOGOUT' },
      { name: 'moduleCode', type: 'string', description: '模块编码' },
      { name: 'resultFlag', type: 'string', description: '结果 Y 或 N' },
      { name: 'fromTime', type: 'string', description: '开始时间' },
      { name: 'toTime', type: 'string', description: '结束时间' },
    ],
  },
])
