import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { queryPlugins, type PluginInfo } from './api'

defineTools([
  {
    name: 'fg0026.query',
    moduleId: 'fg0026',
    title: '查询插件',
    description: '列出已登记插件的版本、接入组件和进程状态。',
    risk: 'read',
    resourceCode: 'fg0026:view',
    parameters: [],
    execute: async () => {
      const rows = (await unwrap<PluginInfo[]>(await queryPlugins(), '加载失败')) ?? []
      return rows.map((row: PluginInfo) => ({
        pluginCode: row.pluginCode,
        pluginName: row.pluginName,
        version: row.version,
        component: row.component,
        lang: row.lang,
        startup: row.startup,
        status: row.status,
        pid: row.pid ?? 0,
        methods: (row.methods ?? []).map((method) => ({
          name: method.name,
          ops: method.ops ?? [],
        })),
      }))
    },
  },
])
