import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import type { FgGridMenuItem } from '@/components/grid/types'
import { useRsToast } from '@/ui'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { queryLogs, type AlertLog } from '../api'

export type FlagOption = { label: string; value: string }

export const levelOptions: FlagOption[] = [
  { label: '提示', value: 'info' },
  { label: '警告', value: 'warn' },
  { label: '严重', value: 'critical' },
]
export const sceneOptions: FlagOption[] = [
  { label: '触发', value: 'fire' },
  { label: '恢复', value: 'resolve' },
  { label: '升级', value: 'escalate' },
  { label: '测试', value: 'test' },
]
export const sendStatusOptions: FlagOption[] = [
  { label: '待发送', value: 'pending' },
  { label: '成功', value: 'ok' },
  { label: '失败', value: 'fail' },
]

export const logColumns = [
  { key: 'alertTimestamp', title: '时间', width: 170 },
  { key: 'alertLevel', title: '级别', width: 88 },
  { key: 'alertType', title: '类型', width: 100 },
  { key: 'sceneCode', title: '场景', width: 88 },
  { key: 'alertTitle', title: '标题', minWidth: 160 },
  { key: 'sendStatus', title: '发送', width: 88 },
  { key: 'sendTime', title: '发送时间', width: 170 },
  { key: 'sendError', title: '错误', minWidth: 140 },
]

export const logMenuItems: FgGridMenuItem[] = [{ key: 'view', label: '查看', icon: 'eye' }]

export const logFormFields: FgFormField[] = [
  { field: 'alertLogId', label: '日志' },
  { field: 'alertLevel', label: '级别', type: 'select', options: levelOptions },
  { field: 'alertType', label: '类型' },
  { field: 'sceneCode', label: '场景', type: 'select', options: sceneOptions },
  { field: 'alertTitle', label: '标题', span: 24 },
  { field: 'channelId', label: '渠道' },
  { field: 'sendStatus', label: '发送', type: 'select', options: sendStatusOptions },
  { field: 'sendTime', label: '发送时间' },
  { field: 'failCount', label: '失败次数', type: 'number' },
  { field: 'alertTimestamp', label: '预警时间' },
  { field: 'alertContent', label: '内容', type: 'textarea', span: 24 },
  { field: 'sendError', label: '错误', type: 'textarea', span: 24 },
  { field: 'alertExtra', label: '扩展', type: 'textarea', span: 24 },
]

export const logSearchFields: FgSearchField[] = [
  { field: 'alertTitle', label: '标题', placeholder: '请输入标题' },
  { field: 'alertType', label: '类型', placeholder: '如 host' },
  { field: 'alertLevel', label: '级别', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...levelOptions] },
  { field: 'sendStatus', label: '发送', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...sendStatusOptions] },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

export function useEventCatalog() {
  const toast = useRsToast()
  const rows = ref<AlertLog[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ alertTitle: '', alertType: '', alertLevel: '', sendStatus: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const active = ref<AlertLog | null>(null)

  const pageRows = computed(() => rows.value)

  function openView(row: AlertLog) {
    active.value = row
    formMode.value = 'view'
    current.value = { ...row }
    formOpen.value = true
  }

  async function reload() {
    const result = await queryLogs(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<AlertLog>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value && !rows.value.some((item) => item.alertLogId === active.value?.alertLogId)) {
      active.value = null
    }
  }

  function onRow(row: AlertLog) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      alertTitle: str(values.alertTitle),
      alertType: str(values.alertType),
      alertLevel: str(values.alertLevel),
      sendStatus: str(values.sendStatus),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { alertTitle: '', alertType: '', alertLevel: '', sendStatus: '' }
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
    onRow,
    onSearch,
    onReset,
  }
}
