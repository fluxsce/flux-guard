import { computed, ref, watch } from 'vue'
import { useRsToast } from '@/ui'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { basicTab, stampFields, tabOf } from '@/components/form/audit'
import type { FgFormField, FgSearchField } from '@/components/form/types'
import type { FgGridMenuItem } from '@/components/grid/types'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { getDefaultPageSize, pageSizePolicy } from '@/api/pagination'
import { getResource, queryResources, type Resource } from '../api'

export const resourceTreeConfig = {
  childrenField: 'children',
  expandColumnKey: 'resourceName',
  indent: 16,
}

export const resourceColumns = [
  { key: 'resourceName', title: '名称', minWidth: 240 },
  { key: 'resourceCode', title: '编码', minWidth: 160 },
  { key: 'resourceType', title: '类型', width: 96 },
  { key: 'resourcePath', title: '路径', minWidth: 220 },
  { key: 'addTime', title: '创建时间', width: 180 },
  { key: 'addWho', title: '创建人', minWidth: 120 },
  { key: 'editTime', title: '修改时间', width: 180 },
  { key: 'editWho', title: '修改人', minWidth: 120 },
]

export const resourceMenuItems: FgGridMenuItem[] = [{ key: 'view', label: '查看', icon: 'eye' }]

const typeOptions = [
  { label: '分组', value: 'GROUP' },
  { label: '模块', value: 'MODULE' },
  { label: '按钮', value: 'BUTTON' },
  { label: '接口', value: 'API' },
]

export const resourceSearchFields: FgSearchField[] = [
  { field: 'resourceName', label: '名称', placeholder: '请输入名称' },
  { field: 'resourceCode', label: '编码', placeholder: '请输入编码' },
  { field: 'resourceType', label: '类型', type: 'select', placeholder: '全部类型', options: [{ label: '全部', value: '' }, ...typeOptions] },
]

export const resourceFormFields: FgFormField[] = [
  ...tabOf(basicTab, [
    { field: 'resourceName', label: '名称', primary: true },
    { field: 'resourceCode', label: '编码', primary: true },
    { field: 'resourceType', label: '类型', type: 'select', options: typeOptions },
    { field: 'resourcePath', label: '路径', span: 24 },
  ]),
  ...stampFields,
]

export function resourceTypeLabel(type: string) {
  return typeOptions.find((item) => item.value === type)?.label || type || '资源'
}

function parentIds(items: Resource[], keys: string[] = []) {
  for (const item of items) {
    if (item.children?.length) {
      keys.push(item.resourceId)
      parentIds(item.children, keys)
    }
  }
  return keys
}

function text(row: Resource, key: keyof Resource) {
  const value = row[key]
  return value == null ? '' : String(value)
}

export function useResourceCatalog() {
  const toast = useRsToast()
  const rows = ref<Resource[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const query = ref({ resourceName: '', resourceCode: '', resourceType: '' })
  const error = ref('')
  const expandedKeys = ref<string[]>([])
  const formOpen = ref(false)
  const current = ref<Record<string, unknown>>({})
  const active = ref<Resource | null>(null)

  const filtered = computed(() => rows.value)

  const pageRows = computed(() => {
    const start = (page.value - 1) * pageSize.value
    return filtered.value.slice(start, start + pageSize.value)
  })

  function fill(row: Resource) {
    current.value = {
      resourceName: text(row, 'resourceName'),
      resourceCode: text(row, 'resourceCode'),
      resourceType: text(row, 'resourceType'),
      resourcePath: text(row, 'resourcePath'),
      addTime: text(row, 'addTime'),
      addWho: text(row, 'addWho'),
      editTime: text(row, 'editTime'),
      editWho: text(row, 'editWho'),
    }
  }

  async function openView(row: Resource) {
    active.value = row
    try {
      const result = await getResource(row.resourceId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      const data = parseApiData<Resource>(result)
      if (!data) return
      fill(data)
      formOpen.value = true
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  const toolbarButtons = computed<FgToolbarButton[]>(() => [
    {
      key: 'view',
      label: '查看',
      icon: 'eye',
      disabled: !active.value,
      onClick: () => {
        if (active.value) openView(active.value)
      },
    },
  ])

  function onRow(row: Resource) {
    active.value = row
  }

  function onMenu(payload: { key: string; row: Resource | null }) {
    if (payload.key === 'view' && payload.row) openView(payload.row)
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      resourceName: String(values.resourceName ?? ''),
      resourceCode: String(values.resourceCode ?? ''),
      resourceType: String(values.resourceType ?? ''),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { resourceName: '', resourceCode: '', resourceType: '' }
  }

  watch(filtered, (items) => {
    expandedKeys.value = parentIds(items)
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  watch(query, async (filters) => {
    try {
      const result = await queryResources(filters)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return
      }
      rows.value = parseApiData<Resource[]>(result) ?? []
      expandedKeys.value = parentIds(rows.value)
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }, { immediate: true })

  return {
    page,
    pageSize,
    error,
    expandedKeys,
    formOpen,
    current,
    active,
    filtered,
    pageRows,
    toolbarButtons,
    openView,
    onRow,
    onMenu,
    onSearch,
    onReset,
  }
}
