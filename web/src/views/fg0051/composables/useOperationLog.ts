import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { auditTab, basicTab, stampFields, tabOf } from '@/components/form/audit'
import type { FgFormField, FgSearchField } from '@/components/form/types'
import type { FgGridMenuItem } from '@/components/grid/types'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { useRsToast } from '@/ui'
import { getLog, queryLogs, type OperationLog } from '../api'

/** 下拉选项。value 与接口枚举一致。 */
export type FlagOption = { label: string; value: string }

export const actionOptions: FlagOption[] = [
  { label: '新增', value: 'CREATE' },
  { label: '修改', value: 'UPDATE' },
  { label: '删除', value: 'DELETE' },
  { label: '授权', value: 'GRANT' },
  { label: '命令', value: 'COMMAND' },
  { label: '登录', value: 'LOGIN' },
  { label: '登录失败', value: 'LOGIN_FAIL' },
  { label: '登出', value: 'LOGOUT' },
]

export const resultOptions: FlagOption[] = [
  { label: '成功', value: 'Y' },
  { label: '失败', value: 'N' },
]

export const moduleNames: Record<string, string> = {
  login: '登录',
  fg0001: '总览',
  fg0002: '采集代理',
  fg0003: '插件',
  fg0005: '主机监控',
  fg0006: 'Java 监控',
  fg0007: '组件监控',
  fg0021: '用户',
  fg0022: '角色',
  fg0023: '资源',
  fg0024: '环境设置',
  fg0025: '任务调度',
  fg0041: '预警模板',
  fg0042: '通知渠道',
  fg0043: '预警事件',
  fg0051: '系统操作日志',
  fg0052: '代理操作日志',
  fg0100: '示例告警',
}

export const targetTypeNames: Record<string, string> = {
  USER: '用户',
  ROLE: '角色',
  AGENT: '采集代理',
  SETTING: '环境设置',
  JOB: '系统任务',
  RULE: '预警规则',
  CHANNEL: '通知渠道',
  COMPONENT: '组件连接',
  TEMPLATE: '预警模板',
  EVENT: '预警事件',
}

export const logColumns = [
  { key: 'addTime', title: '时间', width: 170 },
  { key: 'userName', title: '操作人', minWidth: 110 },
  { key: 'targetName', title: '业务数据', minWidth: 150 },
  { key: 'detailText', title: '操作说明', minWidth: 220 },
  { key: 'resultFlag', title: '结果', width: 80 },
  { key: 'failReason', title: '失败原因', minWidth: 160 },
  { key: 'moduleCode', title: '功能模块', width: 110 },
  { key: 'clientIP', title: '来源 IP', minWidth: 120 },
]

export const logMenuItems: FgGridMenuItem[] = [{ key: 'view', label: '查看', icon: 'eye' }]

export const logSearchFields: FgSearchField[] = [
  { field: 'userName', label: '操作人', placeholder: '账号或姓名' },
  { field: 'action', label: '动作', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...actionOptions] },
  { field: 'moduleCode', label: '功能模块', placeholder: '如 用户、预警模板' },
  { field: 'resultFlag', label: '结果', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...resultOptions] },
]

export const logMoreFields: FgSearchField[] = [
  { field: 'fromTime', label: '开始时间', type: 'datetime' },
  { field: 'toTime', label: '结束时间', type: 'datetime' },
]

export function logFormFieldsOf(row?: Pick<OperationLog, 'resultFlag'> | null): FgFormField[] {
  const basic: FgFormField[] = [
    { field: 'userName', label: '操作人', primary: true },
    { field: 'userId', label: '账号' },
    { field: 'resultFlag', label: '结果', type: 'select', options: resultOptions },
    { field: 'moduleLabel', label: '功能模块' },
    { field: 'actionLabel', label: '动作' },
    { field: 'targetLabel', label: '业务数据', span: 24 },
    { field: 'summaryText', label: '操作说明', type: 'textarea', span: 24 },
  ]
  if (row?.resultFlag === 'N') {
    basic.push({ field: 'failReason', label: '失败原因', type: 'textarea', span: 24 })
  }
  basic.push({ field: 'clientIP', label: '来源 IP' })
  return [
    ...tabOf(basicTab, basic),
    ...tabOf(auditTab, [
      { field: 'resourceCode', label: '权限按钮码' },
      { field: 'requestMethod', label: 'HTTP 方法' },
      { field: 'requestPath', label: '接口路径', span: 24 },
      { field: 'auditId', label: '日志编号', span: 24 },
    ]),
    ...stampFields,
  ]
}

/** @deprecated 使用 logFormFieldsOf */
export const logFormFields = logFormFieldsOf(null)

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
 * 把模块编码显示成菜单名。带编码时写成「预警模板（fg0041）」。
 * @param code - 审计里的模块编码
 * @param withCode - 是否附上编码
 * @returns 显示文案
 */
export function moduleLabel(code: string, withCode = false) {
  const name = moduleNames[code]
  if (!name) return code || '—'
  return withCode ? `${name}（${code}）` : name
}

/**
 * 把对象类型编码显示成中文。
 * @param code - USER、TEMPLATE 等
 * @returns 显示文案
 */
export function targetTypeLabel(code: string) {
  return targetTypeNames[code] || code || '—'
}

/**
 * 本次操作关联的业务数据。写成「用户「admin」」，不暴露内部编码字段名。
 * @param row - 日志行
 * @returns 显示文案
 */
export function targetLabelOf(row: Pick<OperationLog, 'targetType' | 'targetName' | 'targetId'>) {
  const kind = targetTypeNames[row.targetType] || ''
  const name = (row.targetName || '').trim()
  const id = (row.targetId || '').trim()
  let who = ''
  if (name && id && name !== id) who = `${name}（${id}）`
  else who = name || id
  if (who.length > 36 && who === id) who = `${who.slice(0, 8)}…`
  if (kind && who) return `${kind}「${who}」`
  if (who) return who
  if (kind) return kind
  return '（未记录）'
}

const legacyFailSep = '；失败原因：'

/**
 * 列表「操作说明」：用业务写入的 detailText；旧数据若曾把失败原因拼在后面则去掉。
 * @param row - 日志行
 */
export function summaryOf(
  row: Pick<OperationLog, 'action' | 'detailText' | 'targetType' | 'targetName' | 'targetId'>,
) {
  let text = (row.detailText || '').trim()
  const at = text.indexOf(legacyFailSep)
  if (at >= 0) text = text.slice(0, at).trim()
  if (text) return text
  const action = flagLabel(actionOptions, row.action)
  const target = targetLabelOf(row)
  if (target !== '（未记录）') return `${action} · ${target}`
  return action || '—'
}

/**
 * 失败原因：优先用独立字段；兼容旧数据写在 detailText 里的拼接。
 * @param row - 日志行
 */
export function failReasonOf(
  row: Pick<OperationLog, 'resultFlag' | 'failReason' | 'detailText'>,
) {
  if (row.resultFlag !== 'N') return ''
  const direct = (row.failReason || '').trim()
  if (direct) return direct
  const text = (row.detailText || '').trim()
  const at = text.indexOf(legacyFailSep)
  if (at >= 0) return text.slice(at + legacyFailSep.length).trim()
  return ''
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

const emptyQuery = { userName: '', action: '', moduleCode: '', resultFlag: '', fromTime: '', toTime: '' }

/**
 * 操作日志列表：服务端分页、筛选和只读详情。
 * @returns 页面状态和打开详情的方法
 */
export function useOperationLog() {
  const toast = useRsToast()
  const rows = ref<OperationLog[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ ...emptyQuery })
  const error = ref('')
  const formOpen = ref(false)
  const formFields = ref(logFormFieldsOf(null))
  const current = ref<Record<string, unknown>>({})
  const active = ref<OperationLog | null>(null)

  const pageRows = computed(() => rows.value)

  const toolbarButtons = computed<FgToolbarButton[]>(() => [
    {
      key: 'view',
      label: '查看',
      icon: 'eye',
      code: 'fg0051:view',
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
      const data = parseApiData<PageData<OperationLog>>(result)
      rows.value = data?.items ?? []
      total.value = data?.total ?? 0
      if (active.value && !rows.value.some((item) => item.auditId === active.value?.auditId)) {
        active.value = null
      }
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  async function openView(row: OperationLog) {
    active.value = row
    try {
      const result = await getLog(row.auditId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<OperationLog>(result)
      if (!data) return
      const failReason = failReasonOf(data)
      formFields.value = logFormFieldsOf(data)
      current.value = {
        ...data,
        moduleLabel: moduleLabel(data.moduleCode),
        actionLabel: flagLabel(actionOptions, data.action),
        targetLabel: targetLabelOf(data),
        summaryText: summaryOf(data),
        failReason: failReason || '—',
        resourceCode: data.resourceCode || '—',
        requestMethod: data.requestMethod || '—',
        requestPath: data.requestPath || '—',
        clientIP: data.clientIP || '—',
      }
      formOpen.value = true
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  function onRow(row: OperationLog) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      userName: str(values.userName),
      action: str(values.action),
      moduleCode: str(values.moduleCode),
      resultFlag: str(values.resultFlag),
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
