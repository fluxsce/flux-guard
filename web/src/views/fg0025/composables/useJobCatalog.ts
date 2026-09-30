import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import { fieldset } from '@/components/form/audit'
import type { FgGridMenuItem } from '@/components/grid/types'
import { rsConfirm, useRsToast } from '@/ui'
import { deleteJob, getJob, queryJobLogs, queryJobs, runJob, saveJob, type ArchiveJob, type JobKind, type JobList, type JobLog, type JobRow } from '../api'
import { previewCron } from '../cronPreview'

const modeOptions = [
  { label: '仅手动（点击执行）', value: 'manual' },
  { label: '按间隔重复', value: 'interval' },
  { label: '指定时间，只执行一次', value: 'at' },
  { label: '按日历重复', value: 'cron' },
]

const intervalPresets = [
  { label: '每 15 分钟', value: '15' },
  { label: '每 30 分钟', value: '30' },
  { label: '每小时', value: '60' },
  { label: '每 6 小时', value: '360' },
  { label: '每天', value: '1440' },
  { label: '自定义分钟数', value: 'custom' },
]

const customKind = '__custom__'

const enabledOptions = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]

export const jobColumns = [
  { key: 'title', title: '任务', minWidth: 140 },
  { key: 'code', title: '编码', minWidth: 120 },
  { key: 'kindTitle', title: '种类', minWidth: 120 },
  { key: 'source', title: '来源', width: 80 },
  { key: 'enabled', title: '启用', width: 80 },
  { key: 'mode', title: '调度方式', width: 100 },
  { key: 'schedule', title: '调度计划', minWidth: 160 },
  { key: 'nextStart', title: '下次开始', width: 170 },
  { key: 'timeout', title: '超时', width: 88 },
  { key: 'retry', title: '重试', width: 72 },
  { key: 'lastRunAt', title: '最近执行', width: 170 },
  { key: 'lastTrigger', title: '触发', width: 88 },
  { key: 'lastOk', title: '结果', width: 80 },
  { key: 'duration', title: '耗时', width: 96 },
  { key: 'lastDetail', title: '说明', minWidth: 180 },
]

export const jobLogColumns = [
  { key: 'endTime', title: '结束时间', width: 170 },
  { key: 'startTime', title: '开始时间', width: 170 },
  { key: 'trigger', title: '触发', width: 88 },
  { key: 'ok', title: '结果', width: 80 },
  { key: 'duration', title: '耗时', width: 96 },
  { key: 'detail', title: '说明', minWidth: 220 },
]

export const jobSearchFields: FgSearchField[] = [
  { field: 'keyword', label: '任务', placeholder: '名称或编码' },
  {
    field: 'enabled',
    label: '启用',
    type: 'select',
    placeholder: '全部',
    options: [{ label: '全部', value: '' }, ...enabledOptions],
  },
]

export function jobMenuItems(editable: boolean): FgGridMenuItem[] {
  return [
    { key: 'view', label: '查看', icon: 'eye' },
    { key: 'logs', label: '执行日志', icon: 'list' },
    { key: 'edit', label: '修改', icon: 'pencil', code: 'fg0025:edit', disabled: !editable },
    { key: 'run', label: '执行', icon: 'play', code: 'fg0025:edit', disabled: (row) => !editable || !row?.enabled },
    {
      key: 'delete',
      label: '删除',
      icon: 'trash',
      danger: true,
      code: 'fg0025:edit',
      disabled: (row) => !editable || !row?.deletable,
    },
  ]
}

const modeLabel: Record<string, string> = {
  manual: '手动',
  interval: '按间隔',
  at: '指定时间',
  cron: '按日历',
}

export function modeText(mode?: string) {
  return modeLabel[mode || ''] || '按间隔'
}

export function triggerText(trigger?: string) {
  return modeLabel[trigger || ''] || (trigger ? trigger : '—')
}

export function sourceText(source?: string) {
  return source === 'external' ? '新建' : '内置'
}

export function kindText(row: { kind?: string; title?: string }, choices: { value: string; label: string }[]) {
  return choices.find((item) => item.value === row.kind)?.label || row.kind || '—'
}

export function scheduleText(row: { mode?: string; intervalMinutes?: number; runAt?: string; cron?: string }) {
  if (row.mode === 'manual') return '仅手动'
  if (row.mode === 'at') return row.runAt ? row.runAt : '指定时间'
  if (row.mode === 'cron') return previewCron(row.cron || '').summary || '按日历'
  return `每 ${row.intervalMinutes || 60} 分钟`
}

export function resultText(raw?: string) {
  if (!raw || raw === '{}') return ''
  try {
    const data = JSON.parse(raw) as { deleted?: unknown; detail?: unknown }
    if (data && typeof data === 'object') {
      const detail = typeof data.detail === 'string' ? data.detail : ''
      if (detail) return detail
      if (typeof data.deleted === 'number') return `删除 ${data.deleted} 行`
    }
  } catch {
    return raw
  }
  return raw
}

export function durationText(ms?: number) {
  if (ms == null || ms < 0) return '—'
  if (ms < 1000) return `${ms} 毫秒`
  const sec = ms / 1000
  if (sec < 60) return `${sec.toFixed(1)} 秒`
  const min = Math.floor(sec / 60)
  const rest = Math.round(sec % 60)
  return `${min} 分 ${rest} 秒`
}

export function useJobCatalog() {
  const toast = useRsToast()
  const editable = ref(false)
  const items = ref<JobRow[]>([])
  const kinds = ref<JobKind[]>([])
  const active = ref<JobRow | null>(null)
  const keyword = ref('')
  const enabledFilter = ref('')
  const error = ref('')
  const loading = ref(false)
  const running = ref(false)
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const logsOpen = ref(false)
  const logsTitle = ref('')
  const logsCode = ref('')
  const logs = ref<JobLog[]>([])
  const logsLoading = ref(false)

  const rows = computed(() => {
    const word = keyword.value.trim().toLowerCase()
    return items.value.filter((item) => {
      if (enabledFilter.value === 'Y' && !item.enabled) return false
      if (enabledFilter.value === 'N' && item.enabled) return false
      if (!word) return true
      return item.title.toLowerCase().includes(word) || item.code.toLowerCase().includes(word)
    })
  })

  const kindChoices = computed(() => {
    const map = new Map<string, { label: string; value: string; description: string }>()
    for (const kind of kinds.value) {
      if (!kind.kind) continue
      map.set(kind.kind, { label: kind.title || kind.kind, value: kind.kind, description: kind.description || '' })
    }
    for (const item of items.value) {
      if (!item.kind || item.source === 'external' || map.has(item.kind)) continue
      map.set(item.kind, { label: item.title || item.kind, value: item.kind, description: item.description || '' })
    }
    const list = [...map.values()]
    list.push({ label: '自定义', value: customKind, description: '填写已登记的种类编码，参数用 JSON 自己写。' })
    return list
  })

  const formFields = computed<FgFormField[]>(() => jobFormFields(kindChoices.value, formMode.value, String(current.value.source || '')))

  async function reload() {
    error.value = ''
    loading.value = true
    try {
      const result = await queryJobs()
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<JobList>(result)
      editable.value = Boolean(data?.editable)
      items.value = data?.items ?? []
      kinds.value = data?.kinds ?? []
      if (active.value && !items.value.some((item) => item.code === active.value?.code)) {
        active.value = null
      }
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    } finally {
      loading.value = false
    }
  }

  function onSearch(values: Record<string, unknown>) {
    keyword.value = String(values.keyword || '')
    enabledFilter.value = String(values.enabled || '')
  }

  function onReset() {
    keyword.value = ''
    enabledFilter.value = ''
  }

  function onRow(row: JobRow) {
    active.value = row
  }

  function openCreate() {
    const kind = kindChoices.value[0]
    formMode.value = 'create'
    current.value = {
      code: '',
      title: '',
      description: kind?.description || '',
      kind: kind?.value || '',
      kindTitle: kind?.label || '',
      source: 'external',
      enabled: 'Y',
      mode: 'interval',
      intervalMinutes: 60,
      runAt: '',
      customKind: '',
      intervalPreset: '60',
      cronExpr: '0 2 * * *',
      manualNote: '不会按时间自动执行',
      paramsText: '{\n}',
      timeoutSeconds: 600,
      retryCount: 1,
      auditLogDays: 90,
      opsLogDays: 30,
      alertDays: 90,
      sampleDays: 14,
      currentVersion: 0,
    }
    formOpen.value = true
  }

  async function openDetail(row: JobRow, mode: FgFormMode) {
    active.value = row
    try {
      const result = await getJob(row.code)
      if (!isApiSuccess(result)) {
        toast.error(getApiMessage(result, '加载失败'))
        return
      }
      const data = parseApiData<ArchiveJob>(result)
      if (!data) return
      formMode.value = mode
      current.value = formOf(data, kindChoices.value)
      formOpen.value = true
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '加载失败'))
    }
  }

  async function submit(values: Record<string, unknown>) {
    error.value = ''
    if (!resolveKind(values)) {
      toast.error('请填写任务种类')
      return
    }
    try {
      const result = await saveJob(bodyOf(values))
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      formOpen.value = false
      toast.success('已保存')
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
  }

  async function remove(row: JobRow) {
    if (!row.deletable) {
      toast.error('内置任务不能删除')
      return
    }
    const ok = await rsConfirm.confirm({
      title: '删除任务',
      description: `确定删除任务 ${row.title}？已有执行记录仍保留。`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0025',
    })
    if (!ok) return
    try {
      const result = await deleteJob(row.code)
      if (!isApiSuccess(result)) {
        toast.error(getApiMessage(result, '删除失败'))
        return
      }
      if (active.value?.code === row.code) active.value = null
      toast.success('已删除')
      await reload()
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '删除失败'))
    }
  }

  async function run(row: JobRow) {
    if (!row.enabled) {
      toast.error('任务未启用')
      return
    }
    running.value = true
    try {
      const result = await runJob(row.code)
      if (!isApiSuccess(result)) {
        toast.error(getApiMessage(result, '执行失败'))
        await reload()
        return
      }
      toast.success('已执行')
      await reload()
      if (logsOpen.value && logsCode.value === row.code) await openLogs(row)
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '执行失败'))
    } finally {
      running.value = false
    }
  }

  async function openLogs(row: JobRow) {
    active.value = row
    logsCode.value = row.code
    logsTitle.value = `${row.title} 的执行日志`
    logsOpen.value = true
    logsLoading.value = true
    try {
      const result = await queryJobLogs(row.code)
      if (!isApiSuccess(result)) {
        toast.error(getApiMessage(result, '加载执行日志失败'))
        logs.value = []
        return
      }
      logs.value = parseApiData<{ items?: JobLog[] }>(result)?.items ?? []
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '加载执行日志失败'))
      logs.value = []
    } finally {
      logsLoading.value = false
    }
  }

  watch(kindChoices, (choices) => {
    if (!formOpen.value || formMode.value !== 'create' || current.value.kind) return
    const kind = choices[0]
    if (!kind) return
    current.value = {
      ...current.value,
      kind: kind.value,
      kindTitle: kind.label,
      description: current.value.description || kind.description,
    }
  })

  onMounted(reload)

  return {
    editable,
    rows,
    active,
    kindChoices,
    error,
    loading,
    running,
    formOpen,
    formMode,
    current,
    formFields,
    logsOpen,
    logsTitle,
    logs,
    logsLoading,
    reload,
    onSearch,
    onReset,
    onRow,
    openCreate,
    openDetail,
    openLogs,
    submit,
    remove,
    run,
  }
}

function jobFormFields(choices: { label: string; value: string }[], mode: FgFormMode, source: string): FgFormField[] {
  const lockTitle = mode === 'edit' && source !== 'external'
  return [
    { field: 'code', label: '任务编码', primary: true, required: true, placeholder: '字母开头，如 archive-night', span: 12 },
    { field: 'title', label: '任务名称', required: true, disabled: lockTitle, span: 12 },
    { field: 'kind', label: '任务种类', type: 'select', required: true, options: choices, placeholder: '请选择任务种类', modes: ['create'], span: 24 },
    {
      field: 'customKind',
      label: '种类编码',
      required: true,
      placeholder: '已登记的种类，如 archive',
      modes: ['create'],
      span: 12,
      show: (form) => form.kind === customKind,
    },
    { field: 'kindTitle', label: '任务种类', disabled: true, modes: ['edit', 'view'], span: 12 },
    { field: 'enabled', label: '启用', type: 'select', options: enabledOptions, span: 12 },
    { field: 'description', label: '说明', type: 'textarea', span: 24 },
    { field: 'mode', label: '调度方式', type: 'select', options: modeOptions, span: 24 },
    fieldset('intervalBox', '按间隔重复', [
      { field: 'intervalPreset', label: '重复频率', type: 'select', options: intervalPresets, span: 12 },
      {
        field: 'intervalMinutes',
        label: '自定义间隔（分钟）',
        type: 'number',
        placeholder: '1–1440',
        span: 12,
        show: (form) => form.intervalPreset === 'custom',
      },
    ], {
      tips: '到点若上一次还在执行，本次跳过。间隔 1 到 1440 分钟。',
      show: (form) => form.mode === 'interval',
    }),
    fieldset('atBox', '指定时间', [
      { field: 'runAt', label: '执行时间（UTC）', type: 'datetime', span: 12 },
    ], {
      tips: '只自动执行一次。时间按 UTC。过了这个时间之后只能手动执行。',
      show: (form) => form.mode === 'at',
    }),
    fieldset('cronBox', '按日历重复', [
      { field: 'cronExpr', label: '周期', show: () => false },
      { field: 'cronBuilder', label: '周期', type: 'slot', span: 24 },
    ], {
      tips: '分钟、小时、日期、月份、星期都可以选全部、指定几个、一段区间，或每隔几次。时间按 UTC。下面会写成一句话，并列出接下来 5 次。日期和星期都指定具体值时，命中任意一边就会执行。',
      show: (form) => form.mode === 'cron',
    }),
    fieldset('manualBox', '仅手动', [
      { field: 'manualNote', label: '自动执行', disabled: true, span: 24 },
    ], {
      tips: '不会按时间自动调用。需要运行时，在列表里点执行。',
      show: (form) => form.mode === 'manual',
    }),
    fieldset('guard', '执行保护', [
      { field: 'timeoutSeconds', label: '单次超时（秒）', type: 'number', span: 12 },
      { field: 'retryCount', label: '失败后再试（次）', type: 'number', span: 12 },
    ], { tips: '单次调用超过超时时间会中断。0 表示失败不重试，最多 5 次。' }),
    fieldset('archiveParams', '归档范围', [
      { field: 'auditLogDays', label: '审计 / 登录（天）', type: 'number', span: 12 },
      { field: 'opsLogDays', label: '终端 / 命令（天）', type: 'number', span: 12 },
      { field: 'alertDays', label: '预警记录（天）', type: 'number', span: 12 },
      { field: 'sampleDays', label: '主机样本（天）', type: 'number', span: 12 },
    ], {
      tips: '这四项写入策略 JSON 的 params，不是单独的表列。填 0 表示这一类本轮不删。未恢复预警和未完成命令不会删除。',
      show: (form) => resolveKind(form) === 'archive',
    }),
    fieldset('paramsBox', '参数', [
      {
        field: 'paramsText',
        label: '参数 JSON',
        type: 'textarea',
        span: 24,
        placeholder: '{\n  "path": "/tmp"\n}',
      },
    ], {
      tips: '写成 JSON 对象。调度时原样传给该种类的执行器。空对象写 {}。',
      show: (form) => resolveKind(form) !== 'archive',
    }),
    { field: 'nextStart', label: '下次开始', disabled: true, modes: ['view'], span: 12 },
    { field: 'lastRunAt', label: '结束时间', disabled: true, modes: ['view'], span: 12 },
    { field: 'lastStatus', label: '结果', disabled: true, modes: ['view'], span: 12 },
    { field: 'lastDuration', label: '耗时', disabled: true, modes: ['view'], span: 12 },
    { field: 'lastDetail', label: '说明', type: 'textarea', disabled: true, modes: ['view'], span: 24 },
  ]
}

function formOf(detail: ArchiveJob, choices: { label: string; value: string }[]) {
  const kind = choices.find((item) => item.value === detail.kind)
  const minutes = String(detail.intervalMinutes || '')
  return {
    code: detail.code,
    title: detail.title,
    description: detail.description,
    kind: detail.kind,
    customKind: '',
    kindTitle: kind?.label || detail.kind,
    source: detail.source || '',
    enabled: detail.enabled ? 'Y' : 'N',
    mode: detail.mode || 'interval',
    intervalPreset: intervalPresets.some((item) => item.value === minutes) ? minutes : 'custom',
    intervalMinutes: detail.intervalMinutes,
    runAt: detail.runAt || '',
    cronExpr: detail.cron || '0 2 * * *',
    manualNote: '不会按时间自动执行',
    paramsText: detail.params || '{\n}',
    timeoutSeconds: detail.timeoutSeconds ?? 600,
    retryCount: detail.retryCount ?? 1,
    auditLogDays: detail.auditLogDays,
    opsLogDays: detail.opsLogDays,
    alertDays: detail.alertDays,
    sampleDays: detail.sampleDays,
    currentVersion: detail.currentVersion,
    nextStart: detail.nextStart || '不自动执行',
    lastRunAt: detail.lastRunAt || '—',
    lastStatus: !detail.ranAtLeast ? '尚未执行' : detail.lastOk === false ? '失败' : '成功',
    lastDuration: durationText(detail.lastDurationMs),
    lastDetail: detail.lastOk === false ? detail.lastError || '未知错误' : resultText(detail.lastResult) || '—',
  }
}

function resolveKind(values: Record<string, unknown>) {
  const kind = String(values.kind || '')
  if (kind === customKind) return String(values.customKind || '').trim()
  return kind.trim()
}

function bodyOf(values: Record<string, unknown>) {
  const preset = String(values.intervalPreset || '')
  const kind = resolveKind(values)
  return {
    code: String(values.code || '').trim(),
    title: String(values.title || '').trim(),
    description: String(values.description || '').trim(),
    kind,
    enabled: values.enabled !== 'N',
    mode: String(values.mode || 'interval'),
    intervalMinutes: preset && preset !== 'custom' ? Number(preset) : Number(values.intervalMinutes || 0),
    runAt: String(values.runAt || ''),
    cron: String(values.cronExpr || '').trim(),
    timeoutSeconds: Number(values.timeoutSeconds || 0),
    retryCount: Number(values.retryCount || 0),
    auditLogDays: Number(values.auditLogDays || 0),
    opsLogDays: Number(values.opsLogDays || 0),
    alertDays: Number(values.alertDays || 0),
    sampleDays: Number(values.sampleDays || 0),
    params: kind === 'archive'
      ? JSON.stringify({
          auditLogDays: Number(values.auditLogDays || 0),
          opsLogDays: Number(values.opsLogDays || 0),
          alertDays: Number(values.alertDays || 0),
          sampleDays: Number(values.sampleDays || 0),
        })
      : String(values.paramsText || '{}'),
    currentVersion: Number(values.currentVersion || 0),
  }
}
