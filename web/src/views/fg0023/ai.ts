import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { queryResources } from './api'

defineTools([
  {
    name: 'fg0023.query',
    moduleId: 'fg0023',
    title: '查询资源',
    description: '按条件读取权限资源列表。不传条件时返回整棵树。',
    risk: 'read',
    resourceCode: 'fg0023:search',
    parameters: [],
    execute: async () => unwrap(await queryResources(), '加载失败'),
  },
  {
    name: 'fg0023.applySearch',
    moduleId: 'fg0023',
    title: '筛选资源',
    description: '把筛选条件应用到已经打开的资源页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0023:search',
    scope: 'scene',
    parameters: [
      { name: 'resourceName', type: 'string', description: '名称' },
      { name: 'resourceCode', type: 'string', description: '编码' },
      { name: 'resourceType', type: 'string', description: '类型', enum: ['GROUP', 'MODULE', 'BUTTON', 'API'] },
    ],
  },
])
