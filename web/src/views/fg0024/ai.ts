import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { FgAiError } from '@/ai/error'
import { argNumber, argString } from '@/ai/validate'
import { querySettings, saveIngestToken, saveSettings, type IngestToken, type Settings, type WebTimeout } from './api'

function publicSettings(data: Settings) {
  return {
    editable: data.editable,
    webTimeout: {
      requestTimeoutSeconds: data.webTimeout.requestTimeoutSeconds,
      sessionExpireHours: data.webTimeout.sessionExpireHours,
      defaultPageSize: data.webTimeout.defaultPageSize,
      maxPageSize: data.webTimeout.maxPageSize,
    },
    envVars: (data.envVars?.items ?? []).map((item) => ({
      name: item.name,
      kind: item.kind,
      hasValue: item.hasValue,
      note: item.note,
    })),
    ingest: {
      configured: data.ingestToken?.configured ?? false,
      statusFlag: data.ingestToken?.statusFlag ?? '',
      hint: data.ingestToken?.hint ?? '',
    },
  }
}

defineTools([
  {
    name: 'fg0024.query',
    moduleId: 'fg0024',
    title: '查询环境设置',
    description: '读取访问超时、环境变量名和上报令牌状态。不返回变量值和令牌明文。',
    risk: 'read',
    resourceCode: 'fg0024:view',
    parameters: [],
    execute: async () => {
      const data = await unwrap<Settings>(await querySettings(), '加载失败')
      if (!data) throw new FgAiError('加载失败')
      return publicSettings(data)
    },
  },
  {
    name: 'fg0024.saveWebTimeout',
    moduleId: 'fg0024',
    title: '保存访问设置',
    description: '修改接口超时、会话时长或分页大小。未传的项保持原值。',
    risk: 'write',
    resourceCode: 'fg0024:edit',
    parameters: [
      { name: 'requestTimeoutSeconds', type: 'number', description: '接口超时秒数' },
      { name: 'sessionExpireHours', type: 'number', description: '会话时长小时数' },
      { name: 'defaultPageSize', type: 'number', description: '默认分页条数' },
      { name: 'maxPageSize', type: 'number', description: '最大分页条数' },
    ],
    execute: async (args) => {
      const data = await unwrap<Settings>(await querySettings(), '加载失败')
      if (!data) throw new FgAiError('加载失败')
      const current = data.webTimeout
      const saved = await unwrap<WebTimeout>(
        await saveSettings({
          requestTimeoutSeconds: argNumber(args, 'requestTimeoutSeconds', current.requestTimeoutSeconds),
          sessionExpireHours: argNumber(args, 'sessionExpireHours', current.sessionExpireHours),
          defaultPageSize: argNumber(args, 'defaultPageSize', current.defaultPageSize),
          maxPageSize: argNumber(args, 'maxPageSize', current.maxPageSize),
          currentVersion: current.currentVersion,
        }),
        '保存失败',
      )
      return saved
    },
  },
  {
    name: 'fg0024.changeIngest',
    moduleId: 'fg0024',
    title: '变更上报令牌',
    description: '轮换、启用或停用上报令牌。返回值只有状态和后四位，没有明文。',
    risk: 'write',
    resourceCode: 'fg0024:edit',
    parameters: [
      { name: 'action', type: 'string', description: 'rotate 轮换，enable 启用，disable 停用', required: true, enum: ['rotate', 'enable', 'disable'] },
    ],
    execute: async (args) => {
      const data = await unwrap<Settings>(await querySettings(), '加载失败')
      if (!data?.ingestToken) throw new FgAiError('加载失败')
      const action = argString(args, 'action') as 'rotate' | 'enable' | 'disable'
      const saved = await unwrap<IngestToken>(await saveIngestToken(data.ingestToken.currentVersion, action), '保存失败')
      if (!saved) return {}
      return { configured: saved.configured, statusFlag: saved.statusFlag, hint: saved.hint }
    },
  },
  {
    name: 'fg0024.open',
    moduleId: 'fg0024',
    title: '打开设置分组',
    description: '切换已经打开的环境设置页分组。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0024:view',
    scope: 'scene',
    parameters: [
      {
        name: 'tab',
        type: 'string',
        description: '分组',
        required: true,
        enum: ['webTimeout', 'envVars', 'ingestToken'],
      },
    ],
  },
])
