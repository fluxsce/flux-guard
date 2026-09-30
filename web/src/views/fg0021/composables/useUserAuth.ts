import { computed, nextTick, ref, type Ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useRsToast, type RsTreeNode } from '@/ui'
import { useSessionStore } from '@/stores/session'
import { queryUserRoles, saveUserRoles, type User, type UserRole } from '../api'

function toTree(roles: UserRole[]): RsTreeNode[] {
  return roles.map((item) => ({
    key: item.roleId,
    label: item.roleDescription ? `${item.roleName}（${item.roleDescription}）` : item.roleName,
    icon: 'shield',
  }))
}

export function useUserAuth(active: Ref<User | null>, error: Ref<string>, placeUser: (row: User) => void) {
  const toast = useRsToast()
  const session = useSessionStore()
  const authOpen = ref(false)
  const treeData = ref<RsTreeNode[]>([])
  const checkedKeys = ref<string[]>([])
  const expandedKeys = ref<string[]>([])
  const keyword = ref('')

  const authTitle = computed(() => (active.value ? `${active.value.realName || active.value.userName} 授权` : '授权'))

  async function loadRoles() {
    if (!active.value) return
    const result = await queryUserRoles(active.value.userId)
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    const list = parseApiData<UserRole[]>(result) ?? []
    treeData.value = toTree(list)
    checkedKeys.value = list.filter((item) => item.checked).map((item) => item.roleId)
  }

  async function openAuth(row: User) {
    if (!session.allowButton('fg0021:grant')) return
    active.value = row
    error.value = ''
    keyword.value = ''
    authOpen.value = false
    await nextTick()
    authOpen.value = true
    try {
      await loadRoles()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  }

  async function save() {
    if (!active.value || !session.allowButton('fg0021:grant')) return
    error.value = ''
    try {
      const result = await saveUserRoles(active.value.userId, checkedKeys.value)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '保存失败')
        toast.error(error.value)
        return
      }
      const savedUser = result.data
      if (!savedUser?.userId) {
        error.value = '保存失败'
        toast.error(error.value)
        return
      }
      placeUser(savedUser)
      toast.success('已保存')
      await loadRoles()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '保存失败')
      toast.error(error.value)
    }
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
  }
}
