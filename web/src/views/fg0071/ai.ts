import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argString } from '@/ai/validate'
import { queryApps, queryInstances, type AppInstance, type AppRow } from './api'

defineTools([
  {
    name: 'fg0071.query',
    moduleId: 'fg0071',
    title: '查询应用',
    description: '列出应用侧代理上报的应用。',
    risk: 'read',
    resourceCode: 'fg0071:search',
    parameters: [{ name: 'appName', type: 'string', description: '显示名，可空', required: false }],
    execute: async (args) => {
      const rows = (await unwrap<AppRow[]>(await queryApps(argString(args, 'appName')), '加载失败')) ?? []
      return rows.map((row) => ({
        appCode: row.appCode,
        appName: row.appName,
        artifactKind: row.artifactKind,
        instanceCount: row.instanceCount,
        versionText: row.versionText,
        onlineCount: row.onlineCount,
      }))
    },
  },
  {
    name: 'fg0071.instances',
    moduleId: 'fg0071',
    title: '查询应用实例',
    description: '列出一个应用落在各台机器上的实例。',
    risk: 'read',
    resourceCode: 'fg0071:view',
    parameters: [{ name: 'appCode', type: 'string', description: '应用编码', required: true }],
    execute: async (args) => {
      const rows = (await unwrap<AppInstance[]>(await queryInstances(argString(args, 'appCode')), '加载失败')) ?? []
      return rows.map((row) => ({
        instId: row.instId,
        hostName: row.hostName,
        instanceName: row.instanceName,
        runtimeKind: row.runtimeKind,
        artifactKind: row.artifactKind,
        versionText: row.versionText,
        onlineFlag: row.onlineFlag,
        runningFlag: row.runningFlag,
      }))
    },
  },
])
