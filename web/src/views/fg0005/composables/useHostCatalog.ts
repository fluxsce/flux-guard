import { computed, ref, watch } from 'vue'
import { useRsToast } from '@/ui'
import { useSessionStore } from '@/stores/session'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import type { FgSearchField } from '@/components/form/types'
import type { FgGridColumn, FgGridMenuItem, FgGridSort } from '@/components/grid/types'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { queryHosts, type HostMonitor } from '../api'

export const hostSearchFields: FgSearchField[] = [
  { field: 'hostName', label: '主机', placeholder: '请输入主机名' },
  { field: 'ipAddress', label: '地址', placeholder: '请输入地址' },
]

export const hostColumns: FgGridColumn[] = [
  { key: 'onlineFlag', title: '在线', width: 88, sortable: false },
  { key: 'hostName', title: '主机', minWidth: 140, ellipsis: true },
  { key: 'ipAddress', title: '地址', minWidth: 140, ellipsis: true },
  { key: 'cpuUsagePercent', title: 'CPU', width: 88, align: 'right' },
  { key: 'loadAvg1', title: '负载', width: 80, align: 'right' },
  { key: 'memUsedPercent', title: '内存', width: 88, align: 'right' },
  { key: 'diskUsedPercent', title: '磁盘容量', width: 96, align: 'right' },
  { key: 'diskReadBytesPerSec', title: '磁盘读', width: 100, align: 'right' },
  { key: 'diskWriteBytesPerSec', title: '磁盘写', width: 100, align: 'right' },
  { key: 'netReceiveBytesPerSec', title: '网卡接收', width: 110, align: 'right' },
  { key: 'netTransmitBytesPerSec', title: '网卡发送', width: 110, align: 'right' },
  { key: 'tcpEstablished', title: 'ESTABLISHED', width: 130, align: 'right' },
  { key: 'tcpTimeWait', title: 'TIME_WAIT', width: 110, align: 'right' },
  { key: 'tcpCloseWait', title: 'CLOSE_WAIT', width: 120, align: 'right' },
  { key: 'fdAllocated', title: '句柄数', width: 88, align: 'right' },
  { key: 'procCount', title: '进程', width: 80, align: 'right' },
  { key: 'zombieCount', title: '僵尸', width: 80, align: 'right' },
  { key: 'reportedTime', title: '上报时间', width: 180 },
].map((column) => ({ ...column, sortable: column.sortable !== false }) as FgGridColumn)

export const hostMenuItems: FgGridMenuItem[] = [{ key: 'monitor', label: '监控', icon: 'activity', code: 'fg0005:view' }]

function textOf(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number') return value.toString()
  return ''
}

export function useHostCatalog() {
  const toast = useRsToast()
  const session = useSessionStore()
  const rows = ref<HostMonitor[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ hostName: '', ipAddress: '' })
  const error = ref('')
  const loading = ref(false)
  const active = ref<HostMonitor | null>(null)
  const viewing = ref(false)
  const sort = ref<FgGridSort | null>(null)

  const pageRows = computed(() => rows.value)

  function select(row: HostMonitor) {
    active.value = row
  }

  function show(row: HostMonitor) {
    if (!session.allowButton('fg0005:view')) return false
    active.value = row
    viewing.value = true
    return true
  }

  function back() {
    viewing.value = false
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = { hostName: textOf(values.hostName), ipAddress: textOf(values.ipAddress) }
  }

  function onReset() {
    page.value = 1
    query.value = { hostName: '', ipAddress: '' }
  }

  function onSort(next: FgGridSort | null) {
    sort.value = next
    page.value = 1
  }

  async function reload() {
    if (!session.allowButton('fg0005:search')) return active.value
    error.value = ''
    loading.value = true
    try {
      const result = await queryHosts(listParams(page.value, pageSize.value, query.value, sort.value))
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return active.value
      }
      const data = parseApiData<PageData<HostMonitor>>(result)
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
