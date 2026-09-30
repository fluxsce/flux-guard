import { computed, onMounted, ref, watch } from 'vue'
import { messageOfCode } from '@/api/errorText'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { auditTab, basicTab, stampFields, tabOf } from '@/components/form/audit'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import { placeSavedRow } from '@/components/grid/placeRow'
import type { FgGridMenuItem } from '@/components/grid/types'
import { useSessionStore } from '@/stores/session'
import { rsConfirm, useRsToast } from '@/ui'
import { getDefaultPageSize, listParams, pageSizePolicy, type PageData } from '@/api/pagination'
import { createRole, deleteRole, getRole, queryRoles, updateRole, type Role, type RoleInput } from '../api'

export type FlagOption = { label: string; value: string }

export const statusOptions: FlagOption[] = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]
export const builtinOptions: FlagOption[] = [
  { label: '内置', value: 'Y' },
  { label: '自定义', value: 'N' },
]
export const roleColumns = [
  { key: 'roleId', title: '编码', minWidth: 160 },
  { key: 'roleName', title: '名称', minWidth: 140 },
  { key: 'roleDescription', title: '说明', minWidth: 200 },
  { key: 'roleStatus', title: '状态', width: 88 },
  { key: 'builtInFlag', title: '类型', width: 88 },
  { key: 'addTime', title: '创建时间', width: 180 },
  { key: 'addWho', title: '创建人', minWidth: 120 },
  { key: 'editTime', title: '修改时间', width: 180 },
  { key: 'editWho', title: '修改人', minWidth: 120 },
  { key: 'noteText', title: '备注', minWidth: 160 },
]

export const roleMenuItems: FgGridMenuItem[] = [
  { key: 'view', label: '查看', icon: 'eye' },
  { key: 'edit', label: '修改', icon: 'pencil' },
  { key: 'delete', label: '删除', icon: 'trash', disabled: (row) => row?.builtInFlag === 'Y' },
  { key: 'grant', label: '授权', icon: 'shield' },
]

export const roleFormFields: FgFormField[] = [
  ...tabOf(basicTab, [
    { field: 'roleId', label: '编码', primary: true, required: true, placeholder: '字母开头，如 ROLE_OPS' },
    { field: 'roleName', label: '名称', required: true },
    { field: 'roleDescription', label: '说明', type: 'textarea', span: 24 },
    { field: 'roleStatus', label: '状态', type: 'select', options: statusOptions },
    { field: 'dataScope', label: '数据范围', span: 24 },
    { field: 'noteText', label: '备注', type: 'textarea', span: 24 },
  ]),
  { field: 'builtInFlag', label: '类型', type: 'select', options: builtinOptions, disabled: true, tab: auditTab },
  ...stampFields,
]

export const roleSearchFields: FgSearchField[] = [
  { field: 'roleName', label: '名称', placeholder: '请输入名称' },
  { field: 'roleStatus', label: '状态', type: 'select', placeholder: '全部状态', options: [{ label: '全部', value: '' }, ...statusOptions] },
  { field: 'builtInFlag', label: '类型', type: 'select', placeholder: '全部类型', options: [{ label: '全部', value: '' }, ...builtinOptions] },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function text(row: Role, key: keyof Role) {
  const value = row[key]
  return value == null ? '' : String(value)
}

export function useRoleCatalog() {
  const toast = useRsToast()
  const session = useSessionStore()
  const allow = (code: string) => session.allowButton(code)
  const rows = ref<Role[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ roleName: '', roleStatus: '', builtInFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const saving = ref(false)
  const current = ref<Record<string, unknown>>({})
  const active = ref<Role | null>(null)

  const pageRows = computed(() => rows.value)

  function fill(row: Role) {
    return {
      roleId: text(row, 'roleId'),
      roleName: text(row, 'roleName'),
      roleDescription: text(row, 'roleDescription'),
      roleStatus: text(row, 'roleStatus'),
      builtInFlag: text(row, 'builtInFlag'),
      dataScope: text(row, 'dataScope'),
      addTime: text(row, 'addTime'),
      addWho: text(row, 'addWho'),
      editTime: text(row, 'editTime'),
      editWho: text(row, 'editWho'),
      noteText: text(row, 'noteText'),
    }
  }

  function remember(row: Role) {
    active.value = row
    const index = rows.value.findIndex((item) => item.roleId === row.roleId)
    if (index < 0) return
    const next = rows.value.slice()
    next[index] = row
    rows.value = next
  }

  async function loadRole(row: Role): Promise<Role | null> {
    try {
      const result = await getRole(row.roleId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '加载失败')
        toast.error(error.value)
        return null
      }
      const data = parseApiData<Role>(result)
      if (!data?.roleId) {
        error.value = '加载失败'
        toast.error(error.value)
        return null
      }
      remember(data)
      return data
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
      return null
    }
  }

  async function openView(row: Role) {
    if (!allow('fg0022:view')) return
    const latest = await loadRole(row)
    if (!latest) return
    formMode.value = 'view'
    current.value = fill(latest)
    formOpen.value = true
  }

  function openCreate() {
    if (!allow('fg0022:add')) return
    formMode.value = 'create'
    current.value = {
      roleId: '',
      roleName: '',
      roleDescription: '',
      roleStatus: 'Y',
      activeFlag: 'Y',
      dataScope: '',
      noteText: '',
    }
    formOpen.value = true
  }

  async function openEdit(row: Role) {
    if (!allow('fg0022:edit')) return
    const latest = await loadRole(row)
    if (!latest) return
    formMode.value = 'edit'
    current.value = fill(latest)
    formOpen.value = true
  }

  function inputOf(values: Record<string, unknown>): RoleInput {
    return {
      roleId: String(values.roleId ?? '').trim(),
      roleName: String(values.roleName ?? '').trim(),
      roleDescription: String(values.roleDescription ?? '').trim(),
      roleStatus: String(values.roleStatus ?? 'Y'),
      dataScope: String(values.dataScope ?? '').trim(),
      activeFlag: 'Y',
      noteText: String(values.noteText ?? '').trim(),
    }
  }

  async function reload() {
    if (!allow('fg0022:search')) return
    const result = await queryRoles(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<Role>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value && !rows.value.some((item) => item.roleId === active.value?.roleId)) {
      active.value = null
    }
  }

  async function submitRole(values: Record<string, unknown>) {
    if (formMode.value === 'create' && !allow('fg0022:add')) return
    if (formMode.value === 'edit' && !allow('fg0022:edit')) return
    error.value = ''
    saving.value = true
    try {
      const body = inputOf(values)
      const result = formMode.value === 'create' ? await createRole(body) : await updateRole(body)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const saved = result.data
      if (!saved?.roleId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      const placed = placeSavedRow(rows.value, saved, (item) => item.roleId === saved.roleId)
      rows.value = placed.rows
      if (placed.created) {
        total.value += 1
        page.value = 1
      }
      active.value = saved
      formOpen.value = false
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    } finally {
      saving.value = false
    }
  }

  async function removeRole(row: Role) {
    if (!allow('fg0022:delete')) return
    if (row.builtInFlag === 'Y') {
      error.value = messageOfCode('role.builtin', '内置角色不能删除')
      toast.error(error.value)
      return
    }
    const ok = await rsConfirm.confirm({
      title: '删除角色',
      description: `确定删除角色 ${row.roleName}？已分配该角色的用户会失去这些权限。`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0022',
    })
    if (!ok) return
    error.value = ''
    try {
      const result = await deleteRole(row.roleId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '删除失败')
        toast.error(error.value)
        return
      }
      if (active.value?.roleId === row.roleId) active.value = null
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '删除失败')
      toast.error(error.value)
    }
  }

  function onRow(row: Role) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      roleName: String(values.roleName ?? ''),
      roleStatus: String(values.roleStatus ?? ''),
      builtInFlag: String(values.builtInFlag ?? ''),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { roleName: '', roleStatus: '', builtInFlag: '' }
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
    saving,
    current,
    active,
    pageRows,
    openView,
    openCreate,
    openEdit,
    submitRole,
    removeRole,
    onRow,
    onSearch,
    onReset,
  }
}
