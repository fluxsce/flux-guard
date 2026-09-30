import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { queryPlugins, type PluginInfo } from './api'

defineTools([
  {
    name: 'fg0003.query',
    moduleId: 'fg0003',
    title: '查询插件',
    description: '列出已加载的插件。',
    risk: 'read',
    resourceCode: 'fg0003:view',
    parameters: [],
    execute: async () => {
      const rows = (await unwrap<PluginInfo[]>(await queryPlugins(), '加载失败')) ?? []
      return rows.map((row: PluginInfo) => ({
        pluginCode: row.pluginCode,
        pluginName: row.pluginName,
        pluginKind: row.pluginKind,
        pluginScope: row.pluginScope,
        builtinFlag: row.builtinFlag,
      }))
    },
  },
])
