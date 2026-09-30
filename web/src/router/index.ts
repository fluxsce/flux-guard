import type { RouteRecordRaw, Router } from 'vue-router'
import { SESSION_TOKEN_KEY } from '@/api/requestHeader'
import { readUserCookie } from '@/views/login/sessionCookie'
import { loadRouteMessages } from '@/locales'
import { modulePath, useSessionStore } from '@/stores/session'
import type { Menu } from '@/views/login/api'
import type { RsMenuItem } from '@/ui'

declare module 'vue-router' {
  interface RouteMeta {
    /** 模块编码，与 FG_AUTH_RESOURCE.resourceCode 一致。守卫按它判断能否进入。 */
    module?: string
    /** 侧栏图标，Lucide 名称。菜单文案仍来自授权接口。 */
    icon?: string
    /** 侧栏分组键，与资源父节点一致。界面上的分组名来自授权接口。 */
    group?: string
    /** 插件菜单占位。只有授权菜单里有同一路径才放行。 */
    slot?: boolean
    /** 内容区不留内边距，页面自己贴边布局。 */
    flush?: boolean
  }
}

/** 侧栏分组图标。名称不写在这里，避免和授权目录里的分组名分叉。 */
export const menuGroups = [
  { key: 'group:observe', icon: 'radar' },
  { key: 'group:alert', icon: 'bell' },
  { key: 'group:system', icon: 'settings' },
  { key: 'group:log', icon: 'scroll-text' },
] as const

/**
 * 控制台模块路由。地址是 /fluxguard/{模块号}，路径段不写页面含义。
 * 0001–0020 运行观测，0021–0040 系统管理，0041–0050 预警管理，0051–0070 日志管理。目前用到 fg0041 模板、fg0042 渠道、fg0043 事件、fg0051 系统操作日志、fg0052 代理操作日志。预警规则在 fg0005。新增页面只加这一张表，并在权限种子里登记同名模块。
 * 未单独登记的模块号落到最后一条，给插件菜单用。
 */
export const moduleRoutes: RouteRecordRaw[] = [
  { path: 'fg0001', component: () => import('@/views/fg0001/OverviewView.vue'), meta: { module: 'fg0001', icon: 'layout-dashboard', group: 'group:observe' } },
  { path: 'fg0002', component: () => import('@/views/fg0002/AgentsView.vue'), meta: { module: 'fg0002', icon: 'server', group: 'group:observe', flush: true } },
  { path: 'fg0003', component: () => import('@/views/fg0003/PluginsView.vue'), meta: { module: 'fg0003', icon: 'puzzle', group: 'group:system' } },
  { path: 'fg0005', component: () => import('@/views/fg0005/HostsView.vue'), meta: { module: 'fg0005', icon: 'cpu', group: 'group:observe', flush: true } },
  { path: 'fg0006', component: () => import('@/views/fg0006/JavaView.vue'), meta: { module: 'fg0006', icon: 'coffee', group: 'group:observe', flush: true } },
  { path: 'fg0100', component: () => import('@/views/common/PluginSlotView.vue'), meta: { module: 'fg0100', icon: 'bell', group: 'group:observe' } },
  { path: 'fg0021', component: () => import('@/views/fg0021/UsersView.vue'), meta: { module: 'fg0021', icon: 'users', group: 'group:system', flush: true } },
  { path: 'fg0022', component: () => import('@/views/fg0022/RolesView.vue'), meta: { module: 'fg0022', icon: 'shield', group: 'group:system', flush: true } },
  { path: 'fg0023', component: () => import('@/views/fg0023/ResourcesView.vue'), meta: { module: 'fg0023', icon: 'layers', group: 'group:system', flush: true } },
  { path: 'fg0024', component: () => import('@/views/fg0024/SettingsView.vue'), meta: { module: 'fg0024', icon: 'sliders-horizontal', group: 'group:system', flush: true } },
  { path: 'fg0025', component: () => import('@/views/fg0025/JobsView.vue'), meta: { module: 'fg0025', icon: 'timer', group: 'group:system', flush: true } },
  { path: 'fg0041', component: () => import('@/views/fg0041/TemplatesView.vue'), meta: { module: 'fg0041', icon: 'bell-ring', group: 'group:alert', flush: true } },
  { path: 'fg0042', component: () => import('@/views/fg0042/ChannelsView.vue'), meta: { module: 'fg0042', icon: 'send', group: 'group:alert', flush: true } },
  { path: 'fg0043', component: () => import('@/views/fg0043/EventsView.vue'), meta: { module: 'fg0043', icon: 'bell', group: 'group:alert', flush: true } },
  { path: 'fg0051', component: () => import('@/views/fg0051/LogsView.vue'), meta: { module: 'fg0051', icon: 'scroll-text', group: 'group:log', flush: true } },
  { path: 'fg0052', component: () => import('@/views/fg0052/LogsView.vue'), meta: { module: 'fg0052', icon: 'square-terminal', group: 'group:log', flush: true } },
  { path: ':module(fg\\d+)', component: () => import('@/views/common/PluginSlotView.vue'), meta: { slot: true } },
]

function moduleRoute(code: string) {
  return moduleRoutes.find((item) => item.meta?.module === code)
}

export function moduleIcon(code: string) {
  const icon = moduleRoute(code)?.meta?.icon
  return typeof icon === 'string' ? icon : 'circle'
}

function groupIcon(key: string) {
  return menuGroups.find((item) => item.key === key)?.icon ?? 'layers'
}

/** 按路由上的分组键把已授权菜单收成侧栏树。分组名和页面名都用接口返回的文案。 */
export function consoleMenu(menus: Menu[]): RsMenuItem[] {
  const order: string[] = []
  const buckets = new Map<string, { label: string; icon: string; items: RsMenuItem[] }>()
  for (const menu of menus) {
    const route = moduleRoute(menu.resourceCode)
    const fromRoute = route?.meta?.group
    const groupKey = (typeof fromRoute === 'string' && fromRoute) || menu.parentResourceId || 'other'
    let bucket = buckets.get(groupKey)
    if (!bucket) {
      bucket = { label: menu.groupName || '其它', icon: groupIcon(groupKey), items: [] }
      buckets.set(groupKey, bucket)
      order.push(groupKey)
    }
    const to = modulePath(menu.resourceCode)
    bucket.items.push({
      key: to,
      label: menu.resourceName,
      icon: moduleIcon(menu.resourceCode),
      to,
    })
  }
  return order.map((key) => {
    const bucket = buckets.get(key)
    return {
      key,
      label: bucket?.label ?? '',
      icon: bucket?.icon ?? 'layers',
      children: bucket?.items ?? [],
    }
  })
}

export const routes: RouteRecordRaw[] = [
  { path: '/login', component: () => import('@/views/login/LoginView.vue') },
  {
    path: '/',
    component: () => import('@/layouts/ConsoleLayout.vue'),
    redirect: '/fg0001',
    children: moduleRoutes,
  },
  { path: '/:pathMatch(.*)*', redirect: '/fg0001' },
]

/** 未登录进登录页。已登录但没有该模块授权时，回到该用户的第一个菜单。 */
export function installGuard(router: Router) {
  router.beforeEach(async (to) => {
    const token = sessionStorage.getItem(SESSION_TOKEN_KEY) || readUserCookie()?.userId
    if (to.path === '/login') {
      if (token) return '/'
      await loadRouteMessages(to)
      return true
    }
    if (!token) return '/login'

    const session = useSessionStore()
    if (!session.loaded) {
      const ok = await session.load()
      if (!ok) {
        session.reset()
        sessionStorage.removeItem(SESSION_TOKEN_KEY)
        return '/login'
      }
    }

    const named = typeof to.meta.module === 'string' ? to.meta.module : ''
    const slotted = to.meta.slot && typeof to.params.module === 'string' ? to.params.module : ''
    const module = named || slotted
    if (module && !session.allowModule(module)) {
      const home = session.homePath()
      if (home !== to.path) return home
    }
    await loadRouteMessages(to)
    return true
  })
}
