import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { FgAiError } from '@/ai/error'
import { argNumber, argString } from '@/ai/validate'
import { getJob, queryJobs, runJob, saveArchiveJob, type ArchiveJob, type JobList } from './api'

defineTools([
  {
    name: 'fg0025.query',
    moduleId: 'fg0025',
    title: '查询系统任务',
    description: '列出任务调度目录与最近执行摘要。',
    risk: 'read',
    resourceCode: 'fg0025:view',
    parameters: [],
    execute: async () => {
      const data = await unwrap<JobList>(await queryJobs(), '加载失败')
      if (!data) throw new FgAiError('加载失败')
      return {
        editable: data.editable,
        items: (data.items ?? []).map((item) => ({
          code: item.code,
          title: item.title,
          kind: item.kind,
          enabled: item.enabled,
          mode: item.mode,
          intervalMinutes: item.intervalMinutes,
          lastRunAt: item.lastRunAt,
          lastOk: item.lastOk,
          lastDurationMs: item.lastDurationMs,
          lastResult: item.lastResult,
        })),
      }
    },
  },
  {
    name: 'fg0025.getArchive',
    moduleId: 'fg0025',
    title: '查询数据归档任务',
    description: '读取数据归档的启停、调度方式与参数。',
    risk: 'read',
    resourceCode: 'fg0025:view',
    parameters: [],
    execute: async () => {
      const data = await unwrap<ArchiveJob>(await getJob('archive'), '加载失败')
      if (!data) throw new FgAiError('加载失败')
      return data
    },
  },
  {
    name: 'fg0025.saveArchive',
    moduleId: 'fg0025',
    title: '保存数据归档任务',
    description: '修改数据归档启停、调度方式与保留天数。未传的项保持原值。',
    risk: 'write',
    resourceCode: 'fg0025:edit',
    parameters: [
      { name: 'enabled', type: 'boolean', description: '是否启用' },
      { name: 'mode', type: 'string', description: '调度方式：manual 手动、interval 按间隔、at 指定时间、cron 五段表达式', enum: ['manual', 'interval', 'at', 'cron'] },
      { name: 'intervalMinutes', type: 'number', description: '执行间隔分钟，mode 为 interval 时有效' },
      { name: 'runAt', type: 'string', description: '指定执行时间，RFC3339，mode 为 at 时必填' },
      { name: 'cron', type: 'string', description: '五段 cron，UTC，分 时 日 月 周，mode 为 cron 时必填' },
      { name: 'timeoutSeconds', type: 'number', description: '单次调用超时秒数' },
      { name: 'retryCount', type: 'number', description: '失败后再试几次，0 表示不重试' },
      { name: 'auditLogDays', type: 'number', description: '审计日志天数' },
      { name: 'opsLogDays', type: 'number', description: '运维日志天数' },
      { name: 'alertDays', type: 'number', description: '预警记录天数' },
      { name: 'sampleDays', type: 'number', description: '主机样本天数' },
    ],
    execute: async (args) => {
      const current = await unwrap<ArchiveJob>(await getJob('archive'), '加载失败')
      if (!current) throw new FgAiError('加载失败')
      const enabled = args.enabled
      return unwrap(
        await saveArchiveJob({
          code: 'archive',
          enabled: typeof enabled === 'boolean' ? enabled : current.enabled,
          mode: argString(args, 'mode') || current.mode || 'interval',
          intervalMinutes: argNumber(args, 'intervalMinutes', current.intervalMinutes),
          runAt: argString(args, 'runAt') || current.runAt || '',
          cron: argString(args, 'cron') || current.cron || '',
          timeoutSeconds: argNumber(args, 'timeoutSeconds', current.timeoutSeconds ?? 600),
          retryCount: argNumber(args, 'retryCount', current.retryCount ?? 1),
          auditLogDays: argNumber(args, 'auditLogDays', current.auditLogDays),
          opsLogDays: argNumber(args, 'opsLogDays', current.opsLogDays),
          alertDays: argNumber(args, 'alertDays', current.alertDays),
          sampleDays: argNumber(args, 'sampleDays', current.sampleDays),
          currentVersion: current.currentVersion,
        }),
        '保存失败',
      )
    },
  },
  {
    name: 'fg0025.run',
    moduleId: 'fg0025',
    title: '立即执行任务',
    description: '按已保存的参数对象立即执行一次，结果写入执行日志。',
    risk: 'write',
    resourceCode: 'fg0025:edit',
    parameters: [
      { name: 'code', type: 'string', description: '任务编码', required: true, enum: ['archive'] },
    ],
    execute: async (args) => {
      const code = argString(args, 'code') || 'archive'
      return unwrap(await runJob(code), '执行失败')
    },
  },
  {
    name: 'fg0025.open',
    moduleId: 'fg0025',
    title: '打开系统任务',
    description: '切换已经打开的任务调度页到指定任务。',
    risk: 'read',
    resourceCode: 'fg0025:view',
    scope: 'scene',
    parameters: [
      { name: 'code', type: 'string', description: '任务编码', required: true, enum: ['archive'] },
    ],
  },
])
