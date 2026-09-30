import { computed, ref, type Ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useRsToast, type RsTreeNode } from '@/ui'
import { useSessionStore } from '@/stores/session'
import { queryResources, saveRoleResources, type Resource, type Role } from '../api'

function resourceIcon(type: string) {
  if (type === 'MODULE') return 'layout-grid'
  if (type === 'BUTTON') return 'circle-check'
  if (type === 'API') return 'server'
  return 'layers'
}

function toTree(resources: Resource[]): RsTreeNode[] {
  return resources.map((item) => ({
    key: item.resourceId,
    label: item.resourceName,
    icon: resourceIcon(item.resourceType),
    children: item.children?.length ? toTree(item.children) : undefined,
  }))
}

function checkedIds(resources: Resource[]) {
  const keys: string[] = []
  function walk(items: Resource[]) {
    for (const item of items) {
      if (item.checked) keys.push(item.resourceId)
      if (item.children?.length) walk(item.children)
    }
  }
  walk(resources)
  return keys
}

function expandableIds(resources: Resource[]) {
  const keys: string[] = []
  function walk(items: Resource[]) {
    for (const item of items) {
      if (item.children?.length) {
        keys.push(item.resourceId)
        walk(item.children)
      }
    }
  }
  walk(resources)
  return keys
}

export function useRoleAuth(active: Ref<Role | null>, error: Ref<string>) {
  const toast = useRsToast()
  const session = useSessionStore()
  const authOpen = ref(false)
  const treeData = ref<RsTreeNode[]>([])
  const checkedKeys = ref<string[]>([])
  const expandedKeys = ref<string[]>([])
  const keyword = ref('')

  const authTitle = computed(() => (active.value ? `${active.value.roleName} 授权` : '授权'))

  async function loadResources() {
    if (!active.value) return
    const result = await queryResources(active.value.roleId)
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const list = parseApiData<Resource[]>(result) ?? []
    treeData.value = toTree(list)
    checkedKeys.value = checkedIds(list)
    expandedKeys.value = expandableIds(list)
  }

  async function openAuth(row: Role) {
    if (!session.allowButton('fg0022:grant')) return
    active.value = row
    error.value = ''
    keyword.value = ''
    authOpen.value = true
    try {
      await loadResources()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  async function save() {
    if (!active.value || !session.allowButton('fg0022:grant')) return
    error.value = ''
    try {
      const result = await saveRoleResources(active.value.roleId, checkedKeys.value)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      toast.success('已保存')
      await loadResources()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
  }

  function onMenu(payload: { key: string; row: Role | null }, openView: (row: Role) => void) {
    if (!payload.row) return
    if (payload.key === 'view') openView(payload.row)
    if (payload.key === 'auth') void openAuth(payload.row)
  }

  return {
    authOpen,
    treeData,
    checkedKeys,
    expandedKeys,
    keyword,
    authTitle,
    openAuth,
    save,
    onMenu,
  }
}
