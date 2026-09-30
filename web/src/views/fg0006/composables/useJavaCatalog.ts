import { computed, ref, watch } from 'vue'
import { useRsToast } from '@/ui'
import { useSessionStore } from '@/stores/session'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import type { FgSearchField } from '@/components/form/types'
import type { FgGridColumn, FgGridMenuItem, FgGridSort } from '@/components/grid/types'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { queryApps, type JavaApp } from '../api'

export const javaSearchFields: FgSearchField[] = [
  { field: 'appName', label: '应用', placeholder: '请输入应用名' },
  { field: 'hostName', label: '主机', placeholder: '请输入主机名' },
]

export const javaColumns: FgGridColumn[] = [
  { key: 'finding', title: '判断', minWidth: 220, ellipsis: true, sortable: false },
  { key: 'appName', title: '应用', minWidth: 140, ellipsis: true },
  { key: 'javaVersion', title: '版本', minWidth: 120, ellipsis: true },
  { key: 'hostName', title: '主机', minWidth: 120, ellipsis: true },
  { key: 'ipAddress', title: '地址', minWidth: 130, ellipsis: true },
  { key: 'heapUsedPercent', title: '堆', width: 88, align: 'right' },
  { key: 'threadCount', title: '线程', width: 80, align: 'right' },
  { key: 'collectionCount', title: 'GC 次数', width: 100, align: 'right' },
  { key: 'reportedTime', title: '上报时间', width: 180 },
].map((column) => ({ ...column, sortable: column.sortable !== false }) as FgGridColumn)

export const javaMenuItems: FgGridMenuItem[] = [{ key: 'monitor', label: '监控', icon: 'activity', code: 'fg0006:view' }]

function textOf(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number') return value.toString()
  return ''
}

export function useJavaCatalog() {
  const toast = useRsToast()
  const session = useSessionStore()
  const rows = ref<JavaApp[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ appName: '', hostName: '' })
  const error = ref('')
  const loading = ref(false)
  const active = ref<JavaApp | null>(null)
  const viewing = ref(false)
  const sort = ref<FgGridSort | null>(null)

  const pageRows = computed(() => rows.value)

  function select(row: JavaApp) {
    active.value = row
  }

  function show(row: JavaApp) {
    if (!session.allowButton('fg0006:view')) return false
    active.value = row
    viewing.value = true
    return true
  }

  function back() {
    viewing.value = false
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = { appName: textOf(values.appName), hostName: textOf(values.hostName) }
  }

  function onReset() {
    page.value = 1
    query.value = { appName: '', hostName: '' }
  }

  function onSort(next: FgGridSort | null) {
    sort.value = next
    page.value = 1
  }

  async function reload() {
    if (!session.allowButton('fg0006:search')) return active.value
    error.value = ''
    loading.value = true
    try {
      const result = await queryApps(listParams(page.value, pageSize.value, query.value, sort.value))
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return active.value
      }
      const data = parseApiData<PageData<JavaApp>>(result)
      rows.value = data?.items ?? []
      total.value = data?.total ?? 0
      const current = rows.value.find((row) => row.agentId === active.value?.agentId) ?? null
      active.value = current
      if (!current) viewing.value = false
      return current
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
      return active.value
    } finally {
      loading.value = false
    }
  }

  watch([page, pageSize, query, sort], () => {
    void reload()
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  return {
    page,
    pageSize,
    total,
    error,
    loading,
    active,
    viewing,
    pageRows,
    sort,
    onSort,
    select,
    show,
    back,
    onSearch,
    onReset,
    reload,
  }
}
