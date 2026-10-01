import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import { placeSavedRow } from '@/components/grid/placeRow'
import type { FgGridColumn, FgGridMenuItem } from '@/components/grid/types'
import { useSessionStore } from '@/stores/session'
import { rsConfirm, useRsToast } from '@/ui'
import { getDefaultPageSize, listParams, type PageData } from '@/api/pagination'
import { connFormFields, layoutOf, packOption } from './forms'
import {
  createConn,
  deleteConn,
  probeConn,
  queryCatalog,
  queryConns,
  testConn,
  updateConn,
  type ConnTrial,
  type ComponentConn,
  type ComponentInput,
  type Kind,
} from '../api'

export type FlagOption = { label: string; value: string }

export const statusOptions: FlagOption[] = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]
export const upOptions: FlagOption[] = [
  { label: '连通', value: 'Y' },
  { label: '不通', value: 'N' },
]
export const secretOptions: FlagOption[] = [
  { label: '已设置', value: 'Y' },
  { label: '未设置', value: 'N' },
]

export function connColumnsFor(headlines: FgGridColumn[]): FgGridColumn[] {
  return [
    { key: 'componentName', title: '名称', minWidth: 140 },
    { key: 'layout', title: '部署', width: 120 },
    { key: 'address', title: '地址', minWidth: 160 },
    { key: 'upFlag', title: '连通', width: 80 },
    ...headlines,
    { key: 'intervalSec', title: '间隔(秒)', width: 96 },
    { key: 'statusFlag', title: '启用', width: 80 },
    { key: 'probedTime', title: '最近采集', minWidth: 160 },
  ]
}

export const connMenuItems: FgGridMenuItem[] = [
  { key: 'monitor', label: '监控', icon: 'activity', code: 'fg0007:view' },
  { key: 'probe', label: '立即采集', icon: 'refresh-cw', code: 'fg0007:edit' },
  { key: 'edit', label: '修改', icon: 'pencil', code: 'fg0007:edit' },
  { key: 'delete', label: '删除', icon: 'trash', code: 'fg0007:edit' },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

export function useComponentCatalog() {
  const toast = useRsToast()
  const session = useSessionStore()
  const rows = ref<ComponentConn[]>([])
  const kinds = ref<Kind[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const kindCode = ref('')
  const query = ref({ componentName: '', kindCode: '', statusFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const active = ref<ComponentConn | null>(null)
  const viewing = ref(false)
  const testing = ref(false)
  const trial = ref<{ level: 'ok' | 'alert'; text: string } | null>(null)

  const visibleKinds = computed(() =>
    kinds.value.filter((item) => session.allowAction('fg0007', item.kindCode)),
  )
  const kindOptions = computed<FlagOption[]>(() =>
    visibleKinds.value.map((item) => ({ label: item.kindName, value: item.kindCode })),
  )
  const searchFields = computed<FgSearchField[]>(() => [
    { field: 'componentName', label: '名称', placeholder: '请输入名称' },
    { field: 'statusFlag', label: '启用', type: 'select', placeholder: '全部', options: [{ label: '全部', value: '' }, ...statusOptions] },
  ])
  const formFields = computed<FgFormField[]>(() => connFormFields(kindOptions.value))
  const pageRows = computed(() => rows.value)
  const activeKind = computed(() => kinds.value.find((item) => item.kindCode === active.value?.kindCode) ?? null)

  function fill(row: ComponentConn) {
    const layout = layoutOf(row.kindCode, row.optionText, row.userName, row.secretSet)
    return { ...row, secret: '', deployMode: layout.deployMode, authMode: layout.authMode }
  }

  function openCreate() {
    formMode.value = 'create'
    current.value = {
      componentId: '',
      componentName: '',
      kindCode: kindCode.value || visibleKinds.value[0]?.kindCode || 'redis',
      deployMode: kindCode.value === 'kafka' ? 'cluster' : 'standalone',
      authMode: 'none',
      address: '',
      userName: '',
      secret: '',
      databaseName: '',
      optionText: '',
      intervalSec: 30,
      statusFlag: 'Y',
      noteText: '',
    }
    trial.value = null
    formOpen.value = true
  }

  function openEdit(row: ComponentConn) {
    active.value = row
    formMode.value = 'edit'
    current.value = fill(row)
    trial.value = null
    formOpen.value = true
  }

  function describeTrial(item: ConnTrial) {
    if (item.upFlag === 'Y') {
      const parts = ['已连通', item.versionText, item.roleName, item.summaryText].filter(Boolean)
      return { level: 'ok' as const, text: parts.join(' · ') }
    }
    return { level: 'alert' as const, text: item.errorText || '未连通' }
  }

  async function testDraft(values: Record<string, unknown>) {
    testing.value = true
    trial.value = null
    try {
      const result = await testConn(inputOf(values))
      if (!isApiSuccess(result)) {
        trial.value = { level: 'alert', text: getApiMessage(result, '测试失败') }
        return
      }
      const item = parseApiData<ConnTrial>(result)
      trial.value = item ? describeTrial(item) : { level: 'alert', text: '测试失败' }
    } catch (err) {
      trial.value = { level: 'alert', text: getHttpErrorMessage(err, '测试失败') }
    } finally {
      testing.value = false
    }
  }

  function inputOf(values: Record<string, unknown>): ComponentInput {
    const interval = Number(str(values.intervalSec) || '30')
    const kindCode = str(values.kindCode).trim()
    const authMode = str(values.authMode).trim()
    const usesUser = (kindCode === 'redis' && authMode === 'acl')
      || kindCode === 'mysql'
      || (kindCode === 'mongo' && authMode === 'password')
      || (kindCode === 'kafka' && (authMode === 'plain' || authMode === 'scram256' || authMode === 'scram512'))
      || (kindCode === 'nacos' && authMode === 'login')
    const usesDatabase = (kindCode === 'redis' && (str(values.deployMode) === 'standalone' || str(values.deployMode) === 'sentinel'))
      || kindCode === 'mysql'
      || (kindCode === 'mongo' && authMode === 'password')
    return {
      componentId: str(values.componentId).trim(),
      kindCode,
      componentName: str(values.componentName).trim(),
      address: str(values.address).trim(),
      userName: usesUser ? str(values.userName).trim() : '',
      secret: str(values.secret),
      databaseName: usesDatabase ? str(values.databaseName).trim() : '',
      optionText: packOption(kindCode, str(values.deployMode), authMode),
      intervalSec: Number.isFinite(interval) ? interval : 30,
      statusFlag: str(values.statusFlag) || 'Y',
      noteText: str(values.noteText).trim(),
    }
  }

  async function loadKinds() {
    const result = await queryCatalog()
    if (!isApiSuccess(result)) return
    kinds.value = parseApiData<Kind[]>(result) ?? []
  }

  async function reload() {
    const result = await queryConns(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<ComponentConn>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value) {
      active.value = rows.value.find((item) => item.componentId === active.value?.componentId) ?? active.value
    }
  }

  function place(saved: ComponentConn) {
    const placed = placeSavedRow(rows.value, saved, (item) => item.componentId === saved.componentId)
    rows.value = placed.rows
    if (placed.created) {
      total.value += 1
      page.value = 1
    }
    active.value = saved
  }

  async function submit(values: Record<string, unknown>) {
    error.value = ''
    try {
      const body = inputOf(values)
      const result = formMode.value === 'create' ? await createConn(body) : await updateConn(body)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const saved = parseApiData<ComponentConn>(result)
      if (!saved?.componentId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      place(saved)
      formOpen.value = false
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
  }

  async function remove(row: ComponentConn) {
    const ok = await rsConfirm.confirm({
      title: '删除连接',
      description: `确定删除 ${row.componentName}？历史曲线仍保留。`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0007',
    })
    if (!ok) return
    const result = await deleteConn(row.componentId)
    if (!isApiSuccess(result)) {
      toast.error(getApiMessage(result, '删除失败'))
      return
    }
    if (active.value?.componentId === row.componentId) {
      active.value = null
      viewing.value = false
    }
    await reload()
  }

  async function probe(row: ComponentConn) {
    const result = await probeConn(row.componentId)
    if (!isApiSuccess(result)) {
      toast.error(getApiMessage(result, '采集失败'))
      return
    }
    const saved = parseApiData<ComponentConn>(result)
    if (saved?.componentId) place(saved)
    if (saved?.upFlag === 'Y') toast.success(saved.summaryText || '已连通')
    else toast.error(saved?.errorText || '未连通')
  }

  function show(row: ComponentConn) {
    active.value = row
    viewing.value = true
  }

  function back() {
    viewing.value = false
  }

  function onRow(row: ComponentConn) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      componentName: str(values.componentName),
      kindCode: kindCode.value,
      statusFlag: str(values.statusFlag),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { componentName: '', kindCode: kindCode.value, statusFlag: '' }
  }

  watch(visibleKinds, (list) => {
    if (!list.some((item) => item.kindCode === kindCode.value)) {
      kindCode.value = list[0]?.kindCode ?? ''
    }
  })

  watch(kindCode, (value) => {
    viewing.value = false
    if (active.value && active.value.kindCode !== value) active.value = null
    page.value = 1
    query.value = { ...query.value, kindCode: value }
  })

  watch([page, pageSize, query], () => {
    if (!query.value.kindCode) return
    void reload()
  })

  onMounted(() => {
    void loadKinds()
  })

  return {
    page,
    pageSize,
    error,
    formOpen,
    formMode,
    current,
    active,
    viewing,
    total,
    pageRows,
    kinds,
    kindCode,
    visibleKinds,
    kindOptions,
    searchFields,
    formFields,
    activeKind,
    openCreate,
    openEdit,
    submit,
    remove,
    probe,
    testing,
    trial,
    testDraft,
    show,
    back,
    onRow,
    onSearch,
    onReset,
    reload,
  }
}
