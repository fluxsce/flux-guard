import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { queryOverview } from './api'

defineTools([
  {
    name: 'fg0001.overview',
    moduleId: 'fg0001',
    title: '总览',
    description: '读取代理数量、在线数、插件数和事件数。',
    risk: 'read',
    resourceCode: 'fg0001:view',
    parameters: [],
    execute: async () => unwrap(await queryOverview(), '加载失败'),
  },
])
