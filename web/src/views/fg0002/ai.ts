import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString } from '@/ai/validate'
import { dispatchCommand, queryAgents, registerAgent } from './api'

defineTools([
  {
    name: 'fg0002.query',
    moduleId: 'fg0002',
    title: '查询代理',
    description: '列出已登记的采集代理和在线状态。',
    risk: 'read',
    resourceCode: 'fg0002:search',
    parameters: [],
    execute: async () => unwrap(await queryAgents(), '加载失败'),
  },
  {
    name: 'fg0002.register',
    moduleId: 'fg0002',
    title: '登记代理',
    description: '登记一台采集代理的名称和主机，便于在列表里识别。',
    risk: 'write',
    resourceCode: 'fg0002:edit',
    parameters: [
      { name: 'agentName', type: 'string', description: '代理名称', required: true },
      { name: 'hostName', type: 'string', description: '主机名' },
    ],
    execute: async (args) => unwrap(await registerAgent(argString(args, 'agentName'), argString(args, 'hostName')), '登记失败'),
  },
  {
    name: 'fg0002.command',
    moduleId: 'fg0002',
    title: '下发命令',
    description: '对一台主机代理下发立即采集或修改上报间隔。不接受任意命令行。',
    risk: 'write',
    resourceCode: 'fg0002:command',
    parameters: [
      { name: 'agentId', type: 'string', description: '代理编号', required: true },
      { name: 'action', type: 'string', description: '动作', required: true, enum: ['host.collect.now', 'host.interval.set'] },
      { name: 'intervalSec', type: 'number', description: '上报间隔秒数，仅修改间隔时需要' },
    ],
    execute: async (args) => unwrap(
      await dispatchCommand(argString(args, 'agentId'), argString(args, 'action'), argNumber(args, 'intervalSec', 0) || undefined),
      '下发失败',
    ),
  },
  {
    name: 'fg0002.applySearch',
    moduleId: 'fg0002',
    title: '筛选代理列表',
    description: '把筛选条件应用到已经打开的代理页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0002:search',
    scope: 'scene',
    parameters: [
      { name: 'agentName', type: 'string', description: '代理名称' },
      { name: 'hostName', type: 'string', description: '主机名' },
      { name: 'onlineFlag', type: 'string', description: '在线状态', enum: ['Y', 'N'] },
    ],
  },
])
