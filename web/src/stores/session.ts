import { computed, ref } from 'vue'
import { isApiSuccess, parseApiData, updateTimeout } from '@/api/request'
import { applyPageSizePolicy } from '@/api/pagination'
import { SESSION_TOKEN_KEY } from '@/api/requestHeader'
import { login as loginApi, logout as logoutApi, me as meApi, menus as menusApi, type LoginCaptcha } from '@/views/login/api'
import { querySettings, type Settings } from '@/views/fg0024/api'
import type { Menu, SessionUser } from '@/views/login/api'
import { clearUserCookie, readUserCookie, writeUserCookie } from '@/views/login/sessionCookie'
import { defineStore } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'

/** 控制台页面路径。站点前缀由 Vite base（/fluxguard/）补上，这里只放模块号。 */
export function modulePath(code: string) {
  return code ? `/${code}` : ''
}

export const useSessionStore = defineStore('session', () => {
  const user = ref<SessionUser | null>(readUserCookie())
  const menus = ref<Menu[]>([])
  const loaded = ref(false)

  /** 已授权模块号。菜单变化时重建，进入页面时按它查找。 */
  const codes = computed(() => new Set(menus.value.map((item) => item.resourceCode)))
  /** 已授权按钮编码，如 fg0022:add。 */
  const buttonCodes = computed(() => {
    const granted = new Set<string>()
    for (const menu of menus.value) {
      for (const code of menu.buttons ?? []) granted.add(code)
    }
    return granted
  })
  /** 模块登记过的按钮动作，如 fg0022:add。没登记的键不参与校验。 */
  const actionCodes = computed(() => {
    const known = new Set<string>()
    for (const menu of menus.value) {
      for (const action of menu.actions ?? []) known.add(`${menu.resourceCode}:${action}`)
    }
    return known
  })

  /** 租户管理员。菜单仍按角色授权返回，按钮和页面入口由前端放行。 */
  function tenantAdmin() {
    return user.value?.tenantAdminFlag === 'Y'
  }

  /** 判断完整按钮编码是否已授权。工具条写了 code 时走这里。租户管理员直接通过。 */
  function allowButton(code: string) {
    return tenantAdmin() || buttonCodes.value.has(code)
  }

  /**
   * 按模块号和按钮键判断，例如 fg0022 + add。
   * 租户管理员直接通过。模块登记了该动作才校验授权；没登记则保持显示。
   */
  function allowAction(moduleId: string, key: string) {
    if (tenantAdmin()) return true
    const code = `${moduleId}:${key}`
    if (!actionCodes.value.has(code)) return true
    return buttonCodes.value.has(code)
  }

  /** 判断当前用户能否进入这个模块。租户管理员直接通过。 */
  function allowModule(code: string) {
    return tenantAdmin() || codes.value.has(code)
  }

  /** 判断路由路径是否落在已授权模块上。路径形如 /fg0022。租户管理员直接通过。 */
  function allowPath(path: string) {
    return tenantAdmin() || (path.startsWith('/') && codes.value.has(path.slice(1)))
  }

  /** 登录后的落地页。取第一条已授权模块，没有则去登录页。 */
  function homePath() {
    const first = menus.value.find((item) => item.resourceCode)
    return first ? modulePath(first.resourceCode) : '/login'
  }

  /** 清掉内存里的用户、菜单和本地登录 Cookie。 */
  function reset() {
    user.value = null
    menus.value = []
    loaded.value = false
    clearUserCookie()
  }

  /** 拉取当前用户和菜单。任一失败则清会话。菜单里的按钮授权在这里进入索引。 */
  async function load() {
    const [userResult, menuResult] = await Promise.all([meApi(), menusApi()])
    if (!isApiSuccess(userResult) || !isApiSuccess(menuResult)) {
      reset()
      return false
    }
    const me = parseApiData<SessionUser & {
      timeout?: number
      defaultPageSize?: number
      maxPageSize?: number
    }>(userResult)
    user.value = me ?? null
    if (user.value) writeUserCookie(user.value)
    else clearUserCookie()
    if (me?.timeout) updateTimeout(me.timeout)
    if (me?.defaultPageSize && me.maxPageSize) {
      applyPageSizePolicy(me.defaultPageSize, me.maxPageSize)
    }
    menus.value = parseApiData<Menu[]>(menuResult) ?? []
    loaded.value = true
    await applyAccessPolicy()
    return true
  }

  /** 用环境设置覆盖接口超时和默认分页。读取失败时保留已有值。 */
  async function applyAccessPolicy() {
    try {
      const result = await querySettings()
      if (!isApiSuccess(result)) return
      const data = parseApiData<Settings>(result)
      const timeout = data?.webTimeout
      if (!timeout) return
      updateTimeout(timeout.requestTimeoutSeconds * 1000)
      applyPageSizePolicy(timeout.defaultPageSize, timeout.maxPageSize)
    } catch {
      // 策略读取失败时继续使用默认超时和分页。
    }
  }

  /** 登录成功后写入票据和用户，再加载菜单。 */
  async function signIn(userId: string, password: string, captcha?: LoginCaptcha) {
    const result = await loginApi(userId, password, captcha)
    if (!isApiSuccess(result)) return result
    const data = parseApiData<SessionUser & {
      token?: string
      timeout?: number
      defaultPageSize?: number
      maxPageSize?: number
    }>(result)
    if (data?.token) sessionStorage.setItem(SESSION_TOKEN_KEY, data.token)
    if (data?.userId) {
      user.value = {
        userId: data.userId,
        userName: data.userName,
        realName: data.realName,
        tenantId: data.tenantId,
        tenantAdminFlag: data.tenantAdminFlag,
        expireAt: data.expireAt,
      }
      writeUserCookie(user.value)
    }
    if (data?.timeout) updateTimeout(data.timeout)
    if (data?.defaultPageSize && data.maxPageSize) {
      applyPageSizePolicy(data.defaultPageSize, data.maxPageSize)
    }
    await load()
    return result
  }

  /** 本人资料保存后，把新姓名写回当前会话。 */
  function applyRealName(realName: string) {
    if (!user.value || !realName) return
    user.value = { ...user.value, realName }
    writeUserCookie(user.value)
  }

  /** 通知服务端作废会话，并清掉本地票据、Cookie 和工作区。 */
  async function signOut() {
    try {
      await logoutApi()
    } catch {
      // 会话已经失效时直接清本地状态。
    }
    sessionStorage.removeItem(SESSION_TOKEN_KEY)
    clearUserCookie()
    useWorkspaceStore().reset()
    reset()
  }

  return {
    user,
    menus,
    loaded,
    allowModule,
    allowButton,
    allowAction,
    allowPath,
    homePath,
    reset,
    load,
    applyRealName,
    signIn,
    signOut,
  }
})
