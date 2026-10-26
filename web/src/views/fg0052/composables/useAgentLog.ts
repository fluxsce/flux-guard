import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { basicTab, auditTab, stampFields, tabOf } from '@/components/form/audit'
import type { FgFormField, FgSearchField } from '@/components/form/types'
import type { FgGridMenuItem } from '@/components/grid/types'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { useRsToast } from '@/ui'
import { getLog, queryLogs, type AgentLog } from '../api'

/** 列表行带合成主键，供表格选中。 */
export type AgentLogRow = AgentLog & { rowKey: string }

/** 下拉选项。value 与接口枚举一致。 */
export type FlagOption = { label: string; value: string }

export const sourceOptions: FlagOption[] = [
  { label: '命令回执', value: 'COMMAND' },
  { label: '终端账本', value: 'SHELL' },
]

export const eventOptions: FlagOption[] = [
  { label: '立即采集', value: 'host.collect.now' },
  { label: '修改间隔', value: 'host.interval.set' },
  { label: '替换应用文件', value: 'app.file.replace' },
  { label: '升级运行时', value: 'app.runtime.upgrade' },
  { label: '启动运行时', value: 'app.runtime.start' },
  { label: '停止运行时', value: 'app.runtime.stop' },
  { label: '重启运行时', value: 'app.runtime.restart' },
  { label: '打印线程堆栈', value: 'app.runtime.threaddump' },
  { label: '生成堆转储', value: 'app.runtime.heapdump' },
  { label: '回滚应用文件', value: 'app.file.rollback' },
  { label: '推送文件到中央', value: 'app.file.push' },
  { label: '列出目录清单', value: 'app.file.list' },
  { label: '代理自更新', value: 'agent.self.update' },
  { label: '打开终端', value: 'OPEN' },
  { label: '关闭终端', value: 'CLOSE' },
  { label: '终端输入', value: 'INPUT' },
]

export const statusOptions: FlagOption[] = [
  { label: '拨号中', value: 'D' },
  { label: '待取', value: 'W' },
  { label: '执行中', value: 'R' },
  { label: '完成', value: 'Y' },
  { label: '失败', value: 'N' },
]

export const deliverOptions: FlagOption[] = [
  { label: '拨入', value: 'push' },
  { label: '拉取', value: 'pull' },
]

export const logColumns = [
  { key: 'addTime', title: '时间', width: 170 },
  { key: 'agentName', title: '代理', minWidth: 140 },
  { key: 'userName', title: '操作人', minWidth: 110 },
  { key: 'source', title: '类型', width: 100 },
  { key: 'eventType', title: '动作', minWidth: 120 },
  { key: 'statusFlag', title: '状态', width: 88 },
  { key: 'detailText', title: '回执 / 命令', minWidth: 260 },
]

export const logMenuItems: FgGridMenuItem[] = [{ key: 'view', label: '查看', icon: 'eye' }]

export const logSearchFields: FgSearchField[] = [
  { field: 'agentName', label: '代理', placeholder: '名称或编码' },
  { field: 'userName', label: '操作人', placeholder: '账号或姓名' },
  { field: 'source', label: '类型', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...sourceOptions] },
  { field: 'eventType', label: '动作', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...eventOptions] },
]

export const logMoreFields: FgSearchField[] = [
  { field: 'statusFlag', label: '命令状态', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...statusOptions] },
  { field: 'fromTime', label: '开始时间', type: 'datetime' },
  { field: 'toTime', label: '结束时间', type: 'datetime' },
]

/**
 * 详情字段。命令看回执，终端输入看命令行；技术编号放其它信息。
 * @param row - 当前打开的行，未打开时按命令回执布局
 */
export function logFormFieldsOf(row?: Pick<AgentLog, 'source' | 'eventType'> | null): FgFormField[] {
  const shell = row?.source === 'SHELL'
  const input = shell && row?.eventType === 'INPUT'
  const contentLabel = !shell ? '回执结果' : input ? '执行的命令' : '说明'
  const contentPlaceholder = !shell ? '（尚未返回回执）' : input ? '（无命令行）' : '—'
  const basic: FgFormField[] = [
    { field: 'agentName', label: '代理', primary: true },
    { field: 'userName', label: '操作人' },
    { field: 'sourceLabel', label: '类型' },
    { field: 'eventLabel', label: '动作' },
  ]
  if (!shell) {
    basic.push(
      { field: 'statusLabel', label: '执行状态' },
      { field: 'deliverLabel', label: '投递方式' },
      { field: 'intervalLabel', label: '采集间隔' },
    )
  }
  basic.push({
    field: 'contentText',
    label: contentLabel,
    type: 'textarea',
    span: 24,
    placeholder: contentPlaceholder,
  })
  return [
    ...tabOf(basicTab, basic),
    ...tabOf(auditTab, [
      { field: 'agentId', label: '代理编号' },
      { field: 'userId', label: '操作人账号' },
      { field: 'logId', label: '记录编号', span: 24 },
    ]),
    ...stampFields,
  ]
}

/**
 * 把枚举值显示成中文。找不到时原样返回。
 * @param options - 枚举列表
 * @param value - 接口返回的编码
 * @returns 显示文案
 */
export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

/**
 * 命令动作与终端事件的中文名。
 * @param value - actionName 或 eventType
 * @returns 显示文案
 */
export function eventLabel(value: string) {
  return flagLabel(eventOptions, value)
}

/**
 * 列表里「回执 / 命令」一列的文案。
 * 命令优先回执；改间隔无回执时带上目标秒数；终端 INPUT 显示命令行。
 * @param row - 列表行
 * @returns 显示文案
 */
export function contentPreview(row: Pick<AgentLog, 'source' | 'eventType' | 'detailText' | 'intervalSec' | 'statusFlag'>) {
  const detail = (row.detailText || '').trim()
  if (row.source === 'COMMAND') {
    if (detail) return detail
    if (row.eventType === 'host.interval.set' && row.intervalSec > 0) {
      return `目标间隔 ${row.intervalSec} 秒；等待回执…`
    }
    if (row.statusFlag === 'Y' || row.statusFlag === 'N') return '（无回执正文）'
    return '等待回执…'
  }
  if (row.eventType === 'INPUT') return detail || '（空命令行）'
  if (row.eventType === 'OPEN') return detail || '打开终端'
  if (row.eventType === 'CLOSE') return detail || '关闭终端'
  return detail || '—'
}

/**
 * 详情里「回执 / 命令」正文。
 * @param row - 详情数据
 * @returns 显示文案
 */
export function contentBody(row: Pick<AgentLog, 'source' | 'eventType' | 'detailText' | 'intervalSec' | 'statusFlag'>) {
  const detail = (row.detailText || '').trim()
  if (row.source === 'COMMAND') {
    if (detail) return detail
    if (row.eventType === 'host.interval.set' && row.intervalSec > 0) {
      return `已下发：采集间隔改为 ${row.intervalSec} 秒。\n（代理尚未返回回执）`
    }
    return '（尚未返回回执）'
  }
  if (row.eventType === 'INPUT') return detail || '（空命令行）'
  if (row.eventType === 'OPEN') return detail || '打开了远程终端会话'
  if (row.eventType === 'CLOSE') return detail || '关闭了远程终端会话'
  return detail || '—'
}

/**
 * 行主键：来源 + 业务号，避免两张表编号撞车。
 * @param row - 列表行
 * @returns 稳定键
 */
export function rowKeyOf(row: Pick<AgentLog, 'source' | 'logId'>) {
  return `${row.source}:${row.logId}`
}

function withRowKey(row: AgentLog): AgentLogRow {
  return { ...row, rowKey: rowKeyOf(row) }
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

const emptyQuery = {
  agentName: '',
  userName: '',
  source: '',
  eventType: '',
  statusFlag: '',
  fromTime: '',
  toTime: '',
}

/**
 * 代理操作日志列表：服务端分页、筛选和只读详情。
 * @returns 页面状态和打开详情的方法
 */
export function useAgentLog() {
  const toast = useRsToast()
  const rows = ref<AgentLogRow[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ ...emptyQuery })
  const error = ref('')
  const formOpen = ref(false)
  const current = ref<Record<string, unknown>>({})
  const active = ref<AgentLogRow | null>(null)
  const formFields = ref(logFormFieldsOf(null))

  const pageRows = computed(() => rows.value)

  const toolbarButtons = computed<FgToolbarButton[]>(() => [
    {
      key: 'view',
      label: '查看',
      icon: 'eye',
      code: 'fg0052:view',
      disabled: !active.value,
      onClick: () => {
        if (active.value) void openView(active.value)
      },
    },
  ])

  async function reload() {
    try {
      const result = await queryLogs(listParams(page.value, pageSize.value, query.value))
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<PageData<AgentLog>>(result)
      rows.value = (data?.items ?? []).map(withRowKey)
      total.value = data?.total ?? 0
      if (active.value && !rows.value.some((item) => item.rowKey === active.value?.rowKey)) {
        active.value = null
      }
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  async function openView(row: AgentLogRow) {
    active.value = row
    try {
      const result = await getLog(row.source, row.logId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<AgentLog>(result)
      if (!data) return
      formFields.value = logFormFieldsOf(data)
      current.value = {
        agentName: data.agentName || data.agentId || '—',
        agentId: data.agentId || '—',
        userName: data.userName || data.userId || '—',
        userId: data.userId || '—',
        sourceLabel: flagLabel(sourceOptions, data.source),
        eventLabel: eventLabel(data.eventType),
        statusLabel: data.statusFlag ? flagLabel(statusOptions, data.statusFlag) : '—',
        deliverLabel: data.deliverMode ? flagLabel(deliverOptions, data.deliverMode) : '—',
        intervalLabel:
          data.eventType === 'host.interval.set' && data.intervalSec > 0
            ? `${data.intervalSec} 秒`
            : '—',
        contentText: contentBody(data),
        logId: data.logId,
        addTime: data.addTime,
        addWho: data.addWho,
        editTime: data.editTime,
        editWho: data.editWho,
      }
      formOpen.value = true
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  function onRow(row: AgentLogRow) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      agentName: str(values.agentName),
      userName: str(values.userName),
      source: str(values.source),
      eventType: str(values.eventType),
      statusFlag: str(values.statusFlag),
      fromTime: str(values.fromTime),
      toTime: str(values.toTime),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { ...emptyQuery }
  }

  watch([page, pageSize, query], () => {
    void reload()
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  onMounted(() => {
    void reload()
  })

  return {
    page,
    pageSize,
    total,
    error,
    formOpen,
    formFields,
    current,
    active,
    pageRows,
    toolbarButtons,
    openView,
    onRow,
    onSearch,
    onReset,
  }
}
