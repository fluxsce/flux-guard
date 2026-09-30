import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { auditFields, basicTab, fieldset, tabOf } from '@/components/form/audit'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import { placeSavedRow } from '@/components/grid/placeRow'
import type { FgGridMenuItem } from '@/components/grid/types'
import { rsConfirm, useRsToast } from '@/ui'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { createRule, deleteRule, queryRules, updateRule, type Rule, type RuleInput } from '../api'

export type FlagOption = { label: string; value: string }

export const statusOptions: FlagOption[] = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]
export const levelOptions: FlagOption[] = [
  { label: '提示', value: 'info' },
  { label: '警告', value: 'warn' },
  { label: '严重', value: 'critical' },
]
export const metricOptions: FlagOption[] = [
  { label: '主机 CPU 使用率', value: 'cpu.usage' },
  { label: '内存使用率', value: 'mem.usage' },
  { label: '磁盘使用率', value: 'disk.usage' },
  { label: 'inode 使用率', value: 'disk.inode' },
  { label: '1 分钟负载', value: 'load.1' },
  { label: '文件句柄使用率', value: 'fd.usage' },
  { label: '代理离线分钟', value: 'agent.offline' },
]
export const opOptions: FlagOption[] = [
  { label: '大于', value: 'gt' },
  { label: '大于等于', value: 'gte' },
  { label: '小于', value: 'lt' },
  { label: '小于等于', value: 'lte' },
]

export const ruleColumns = [
  { key: 'ruleName', title: '名称', minWidth: 140 },
  { key: 'metricCode', title: '指标', minWidth: 140 },
  { key: 'compareOp', title: '比较', width: 100 },
  { key: 'thresholdValue', title: '阈值', width: 90 },
  { key: 'durationSec', title: '持续秒', width: 90 },
  { key: 'levelCode', title: '级别', width: 88 },
  { key: 'agentId', title: '代理', minWidth: 120 },
  { key: 'channelId', title: '渠道', minWidth: 120 },
  { key: 'waitMinutes', title: '升级分钟', width: 100 },
  { key: 'statusFlag', title: '状态', width: 88 },
  { key: 'noteText', title: '备注', minWidth: 140 },
]

export const ruleMenuItems: FgGridMenuItem[] = [
  { key: 'view', label: '查看', icon: 'eye' },
  { key: 'edit', label: '修改', icon: 'pencil' },
  { key: 'delete', label: '删除', icon: 'trash' },
]

export const ruleFormFields: FgFormField[] = [
  ...tabOf(basicTab, [
    fieldset('identity', '规则', [
      { field: 'ruleId', label: '编码', primary: true, placeholder: '留空则自动生成', tips: '留空自动生成，保存后不能修改。' },
      { field: 'ruleName', label: '名称', required: true },
      { field: 'levelCode', label: '级别', type: 'select', options: levelOptions, tips: '写入预警日志的级别。未选时按警告。' },
      { field: 'statusFlag', label: '状态', type: 'select', options: statusOptions, tips: '停用后不再检查这条规则。' },
      { field: 'noteText', label: '备注', type: 'textarea', span: 24 },
    ]),
    fieldset('condition', '判定', [
      { field: 'metricCode', label: '指标', type: 'select', required: true, options: metricOptions, tips: '使用率按百分比。1 分钟负载是负载值，代理离线按分钟。' },
      { field: 'compareOp', label: '比较', type: 'select', required: true, options: opOptions },
      { field: 'thresholdValue', label: '阈值', type: 'number', required: true, tips: '和指标同一单位。使用率一般填 0 到 100。' },
      { field: 'durationSec', label: '持续秒数', type: 'number', tips: '0 表示一越阈就触发。大于 0 时要连续达到这么多秒才触发。' },
      { field: 'agentId', label: '代理编码', placeholder: '留空表示全部主机', tips: '留空检查全部主机。填写后只检查这一台。' },
    ], { tips: '一条规则只判断一个指标和一个阈值。' }),
    fieldset('notify', '通知', [
      { field: 'channelId', label: '通知渠道', placeholder: '留空只记录，不发送', tips: '填写渠道编码。留空只记录，不按渠道发送。' },
      { field: 'silenceStart', label: '静默开始', type: 'datetime', tips: '和静默结束要一起填。这段时间内不发送。' },
      { field: 'silenceEnd', label: '静默结束', type: 'datetime' },
      { field: 'waitMinutes', label: '升级等待分钟', type: 'number', tips: '触发后仍未恢复，超过这么多分钟再发一次。0 表示不升级。' },
      { field: 'escalateChannelId', label: '升级渠道', placeholder: '留空则仍用通知渠道', tips: '留空时升级仍发到通知渠道。' },
    ]),
  ]),
  ...auditFields,
]

export const ruleSearchFields: FgSearchField[] = [
  { field: 'ruleName', label: '名称', placeholder: '请输入名称' },
  { field: 'metricCode', label: '指标', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...metricOptions] },
  { field: 'levelCode', label: '级别', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...levelOptions] },
  { field: 'statusFlag', label: '状态', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...statusOptions] },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function text(row: Rule, key: keyof Rule) {
  const value = row[key]
  if (typeof value === 'string') return value
  if (typeof value === 'number') return value.toString()
  return ''
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function wallClock(value: string) {
  const text = value.trim()
  if (!text) return ''
  const utc = /[zZ]$|[+-]\d\d:\d\d$/.test(text) ? text : `${text.includes('T') ? text : text.replace(' ', 'T')}Z`
  const when = new Date(utc)
  if (Number.isNaN(when.getTime())) return text
  return `${when.getFullYear()}-${pad(when.getMonth() + 1)}-${pad(when.getDate())} ${pad(when.getHours())}:${pad(when.getMinutes())}:${pad(when.getSeconds())}`
}

function toUtcText(value: string) {
  const text = value.trim()
  if (!text) return ''
  if (/[zZ]$|[+-]\d\d:\d\d$/.test(text)) return text
  const when = new Date(text.includes('T') ? text : text.replace(' ', 'T'))
  if (Number.isNaN(when.getTime())) return text
  return when.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

export function useHostRules() {
  const toast = useRsToast()
  const rows = ref<Rule[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ ruleName: '', metricCode: '', levelCode: '', statusFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const active = ref<Rule | null>(null)

  const pageRows = computed(() => rows.value)

  function fill(row: Rule) {
    return {
      ruleId: text(row, 'ruleId'),
      ruleName: text(row, 'ruleName'),
      metricCode: text(row, 'metricCode'),
      compareOp: text(row, 'compareOp'),
      thresholdValue: row.thresholdValue,
      durationSec: row.durationSec,
      levelCode: text(row, 'levelCode'),
      agentId: text(row, 'agentId'),
      channelId: text(row, 'channelId'),
      silenceStart: wallClock(text(row, 'silenceStart')),
      silenceEnd: wallClock(text(row, 'silenceEnd')),
      waitMinutes: row.waitMinutes,
      escalateChannelId: text(row, 'escalateChannelId'),
      statusFlag: text(row, 'statusFlag'),
      noteText: text(row, 'noteText'),
      currentVersion: row.currentVersion,
      oprSeqFlag: text(row, 'oprSeqFlag'),
      addTime: text(row, 'addTime'),
      addWho: text(row, 'addWho'),
      editTime: text(row, 'editTime'),
      editWho: text(row, 'editWho'),
    }
  }

  function openView(row: Rule) {
    active.value = row
    formMode.value = 'view'
    current.value = fill(row)
    formOpen.value = true
  }

  function openCreate() {
    formMode.value = 'create'
    current.value = {
      ruleId: '',
      ruleName: '',
      metricCode: 'cpu.usage',
      compareOp: 'gt',
      thresholdValue: 80,
      durationSec: 0,
      levelCode: 'warn',
      agentId: '',
      channelId: '',
      silenceStart: '',
      silenceEnd: '',
      waitMinutes: 0,
      escalateChannelId: '',
      statusFlag: 'Y',
      activeFlag: 'Y',
      noteText: '',
    }
    formOpen.value = true
  }

  function openEdit(row: Rule) {
    active.value = row
    formMode.value = 'edit'
    current.value = fill(row)
    formOpen.value = true
  }

  function inputOf(values: Record<string, unknown>): RuleInput {
    const threshold = Number(str(values.thresholdValue) || '0')
    const duration = Number(str(values.durationSec) || '0')
    const wait = Number(str(values.waitMinutes) || '0')
    return {
      ruleId: str(values.ruleId).trim(),
      ruleName: str(values.ruleName).trim(),
      metricCode: str(values.metricCode),
      compareOp: str(values.compareOp),
      thresholdValue: Number.isFinite(threshold) ? threshold : 0,
      durationSec: Number.isFinite(duration) ? duration : 0,
      levelCode: str(values.levelCode) || 'warn',
      agentId: str(values.agentId).trim(),
      channelId: str(values.channelId).trim(),
      silenceStart: toUtcText(str(values.silenceStart)),
      silenceEnd: toUtcText(str(values.silenceEnd)),
      waitMinutes: Number.isFinite(wait) ? wait : 0,
      escalateChannelId: str(values.escalateChannelId).trim(),
      statusFlag: str(values.statusFlag) || 'Y',
      activeFlag: 'Y',
      noteText: str(values.noteText).trim(),
    }
  }

  async function reload() {
    const result = await queryRules(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<Rule>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value && !rows.value.some((item) => item.ruleId === active.value?.ruleId)) {
      active.value = null
    }
  }

  function placeRule(saved: Rule) {
    const placed = placeSavedRow(rows.value, saved, (item) => item.ruleId === saved.ruleId)
    rows.value = placed.rows
    if (placed.created) {
      total.value += 1
      page.value = 1
    }
    active.value = saved
  }

  async function submitRule(values: Record<string, unknown>) {
    error.value = ''
    try {
      const body = inputOf(values)
      const result = formMode.value === 'create' ? await createRule(body) : await updateRule(body)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const saved = result.data
      if (!saved?.ruleId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      placeRule(saved)
      formOpen.value = false
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
  }

  async function removeRule(row: Rule) {
    const ok = await rsConfirm.confirm({
      title: '删除规则',
      description: `确定删除规则 ${row.ruleName}？`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0005',
    })
    if (!ok) return
    error.value = ''
    try {
      const result = await deleteRule(row.ruleId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '删除失败')
        toast.error(error.value)
        return
      }
      if (active.value?.ruleId === row.ruleId) active.value = null
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '删除失败')
      toast.error(error.value)
    }
  }

  function onRow(row: Rule) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      ruleName: str(values.ruleName),
      metricCode: str(values.metricCode),
      levelCode: str(values.levelCode),
      statusFlag: str(values.statusFlag),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { ruleName: '', metricCode: '', levelCode: '', statusFlag: '' }
  }

  watch([page, pageSize, query], () => {
    void reload()
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  onMounted(async () => {
    try {
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  })

  return {
    page,
    pageSize,
    total,
    error,
    formOpen,
    formMode,
    current,
    active,
    pageRows,
    openView,
    openCreate,
    openEdit,
    submitRule,
    removeRule,
    onRow,
    onSearch,
    onReset,
  }
}
