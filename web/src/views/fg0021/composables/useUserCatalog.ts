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
import { createUser, deleteUser, queryUsers, updateUser, type User, type UserInput } from '../api'

export type FlagOption = { label: string; value: string }

export const statusOptions: FlagOption[] = [
  { label: '启用', value: 'Y' },
  { label: '停用', value: 'N' },
]
export const yesNoOptions: FlagOption[] = [
  { label: '是', value: 'Y' },
  { label: '否', value: 'N' },
]
export const genderOptions: FlagOption[] = [
  { label: '未知', value: '0' },
  { label: '男', value: '1' },
  { label: '女', value: '2' },
]

export const userColumns = [
  { key: 'userId', title: '用户ID', minWidth: 120 },
  { key: 'userName', title: '用户名', minWidth: 140 },
  { key: 'realName', title: '姓名', minWidth: 120 },
  { key: 'roleNames', title: '角色', minWidth: 180 },
  { key: 'email', title: '邮箱', minWidth: 180 },
  { key: 'mobile', title: '手机', minWidth: 140 },
  { key: 'gender', title: '性别', width: 72 },
  { key: 'statusFlag', title: '状态', width: 88 },
  { key: 'tenantAdminFlag', title: '租户管理员', width: 110 },
  { key: 'userExpireDate', title: '过期时间', width: 180 },
  { key: 'lastLoginTime', title: '最后登录时间', width: 180 },
  { key: 'addTime', title: '创建时间', width: 180 },
  { key: 'addWho', title: '创建人', minWidth: 120 },
  { key: 'editTime', title: '修改时间', width: 180 },
  { key: 'editWho', title: '修改人', minWidth: 120 },
  { key: 'noteText', title: '备注', minWidth: 160 },
]

export const userMenuItems: FgGridMenuItem[] = [
  { key: 'view', label: '查看', icon: 'eye' },
  { key: 'edit', label: '修改', icon: 'pencil' },
  { key: 'delete', label: '删除', icon: 'trash', disabled: (row) => row?.tenantAdminFlag === 'Y' },
  { key: 'grant', label: '授权', icon: 'shield' },
]

export const userFormFields: FgFormField[] = [
  ...tabOf(basicTab, [
    { field: 'userId', label: '用户ID', primary: true, required: true, placeholder: '登录时使用，字母开头，如 ops' },
    { field: 'userName', label: '用户名', required: true, placeholder: '自定义名称' },
    { field: 'password', label: '口令', type: 'password', required: true, modes: ['create'], placeholder: '至少 6 位' },
    { field: 'password', label: '新口令', type: 'password', modes: ['edit'], placeholder: '留空则不修改' },
    { field: 'realName', label: '姓名', required: true },
    { field: 'roleNames', label: '角色', span: 24, disabled: true, modes: ['view'] },
    { field: 'email', label: '邮箱' },
    { field: 'mobile', label: '手机' },
    { field: 'gender', label: '性别', type: 'select', options: genderOptions },
    { field: 'statusFlag', label: '状态', type: 'select', options: statusOptions },
    { field: 'tenantAdminFlag', label: '租户管理员', type: 'select', options: yesNoOptions },
    { field: 'userExpireDate', label: '过期时间', type: 'datetime', placeholder: '留空则 5 年后过期' },
    { field: 'noteText', label: '备注', type: 'textarea', span: 24 },
  ]),
  { field: 'lastLoginTime', label: '最后登录时间', type: 'datetime', disabled: true, tab: auditTab },
  { field: 'lastLoginIp', label: '最后登录IP', disabled: true, tab: auditTab },
  ...stampFields,
]

export const userSearchFields: FgSearchField[] = [
  { field: 'userName', label: '用户名', placeholder: '请输入用户名' },
  { field: 'realName', label: '姓名', placeholder: '请输入姓名' },
  { field: 'statusFlag', label: '状态', type: 'select', placeholder: '全部状态', options: [{ label: '全部', value: '' }, ...statusOptions] },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

function text(row: User, key: keyof User) {
  const value = row[key]
  if (typeof value === 'string') return value
  if (typeof value === 'number') return value.toString()
  if (Array.isArray(value)) return value.join('、')
  return ''
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

export function useUserCatalog() {
  const toast = useRsToast()
  const session = useSessionStore()
  const allow = (code: string) => session.allowButton(code)
  const rows = ref<User[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const total = ref(0)
  const query = ref({ userName: '', realName: '', statusFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const saving = ref(false)
  const current = ref<Record<string, unknown>>({})
  const active = ref<User | null>(null)

  const pageRows = computed(() => rows.value)

  function fill(row: User) {
    return {
      userId: text(row, 'userId'),
      userName: text(row, 'userName'),
      password: '',
      realName: text(row, 'realName'),
      email: text(row, 'email'),
      mobile: text(row, 'mobile'),
      gender: text(row, 'gender') || '0',
      roleNames: text(row, 'roleNames'),
      statusFlag: text(row, 'statusFlag'),
      tenantAdminFlag: text(row, 'tenantAdminFlag'),
      userExpireDate: text(row, 'userExpireDate'),
      lastLoginTime: text(row, 'lastLoginTime'),
      lastLoginIp: text(row, 'lastLoginIp'),
      addTime: text(row, 'addTime'),
      addWho: text(row, 'addWho'),
      editTime: text(row, 'editTime'),
      editWho: text(row, 'editWho'),
      noteText: text(row, 'noteText'),
    }
  }

  function openView(row: User) {
    if (!allow('fg0021:view')) return
    active.value = row
    formMode.value = 'view'
    current.value = fill(row)
    formOpen.value = true
  }

  function openCreate() {
    if (!allow('fg0021:add')) return
    formMode.value = 'create'
    current.value = {
      userId: '',
      userName: '',
      password: '',
      realName: '',
      email: '',
      mobile: '',
      gender: '0',
      statusFlag: 'Y',
      tenantAdminFlag: 'N',
      activeFlag: 'Y',
      userExpireDate: '',
      noteText: '',
    }
    formOpen.value = true
  }

  function openEdit(row: User) {
    if (!allow('fg0021:edit')) return
    active.value = row
    formMode.value = 'edit'
    current.value = fill(row)
    formOpen.value = true
  }

  function inputOf(values: Record<string, unknown>): UserInput {
    const gender = Number(str(values.gender) || '0')
    return {
      userId: str(values.userId).trim(),
      userName: str(values.userName).trim(),
      password: str(values.password),
      realName: str(values.realName).trim(),
      email: str(values.email).trim(),
      mobile: str(values.mobile).trim(),
      gender: Number.isFinite(gender) ? gender : 0,
      statusFlag: str(values.statusFlag) || 'Y',
      tenantAdminFlag: str(values.tenantAdminFlag) || 'N',
      userExpireDate: str(values.userExpireDate).trim(),
      activeFlag: 'Y',
      noteText: str(values.noteText).trim(),
    }
  }

  async function reload() {
    if (!allow('fg0021:search')) return
    const result = await queryUsers(listParams(page.value, pageSize.value, query.value))
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const data = parseApiData<PageData<User>>(result)
    rows.value = data?.items ?? []
    total.value = data?.total ?? 0
    if (active.value && !rows.value.some((item) => item.userId === active.value?.userId)) {
      active.value = null
    }
  }

  function placeUser(saved: User) {
    const placed = placeSavedRow(rows.value, saved, (item) => item.userId === saved.userId)
    rows.value = placed.rows
    if (placed.created) {
      total.value += 1
      page.value = 1
    }
    active.value = saved
  }

  async function submitUser(values: Record<string, unknown>) {
    if (formMode.value === 'create' && !allow('fg0021:add')) return
    if (formMode.value === 'edit' && !allow('fg0021:edit')) return
    error.value = ''
    saving.value = true
    try {
      const body = inputOf(values)
      const result = formMode.value === 'create' ? await createUser(body) : await updateUser(body)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const saved = result.data
      if (!saved?.userId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      placeUser(saved)
      formOpen.value = false
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    } finally {
      saving.value = false
    }
  }

  async function removeUser(row: User, selfUserId: string) {
    if (!allow('fg0021:delete')) return
    if (row.userId === selfUserId) {
      error.value = messageOfCode('user.self', '不能删除当前登录用户')
      toast.error(error.value)
      return
    }
    if (row.tenantAdminFlag === 'Y') {
      error.value = messageOfCode('user.admin', '不能删除管理员用户')
      toast.error(error.value)
      return
    }
    const ok = await rsConfirm.confirm({
      title: '删除用户',
      description: `确定删除用户 ${row.realName || row.userName}？该用户的角色分配会一并去掉。`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0021',
    })
    if (!ok) return
    error.value = ''
    try {
      const result = await deleteUser(row.userId)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '删除失败')
        toast.error(error.value)
        return
      }
      if (active.value?.userId === row.userId) active.value = null
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '删除失败')
      toast.error(error.value)
    }
  }

  function onRow(row: User) {
    active.value = row
  }

  function onSearch(values: Record<string, unknown>) {
    page.value = 1
    query.value = {
      userName: str(values.userName),
      realName: str(values.realName),
      statusFlag: str(values.statusFlag),
    }
  }

  function onReset() {
    page.value = 1
    query.value = { userName: '', realName: '', statusFlag: '' }
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
    reload,
    placeUser,
    openView,
    openCreate,
    openEdit,
    submitUser,
    removeUser,
    onRow,
    onSearch,
    onReset,
  }
}
