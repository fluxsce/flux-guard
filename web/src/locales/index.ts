import { createI18n, type Composer } from 'vue-i18n'

/** 与组件库 RsLocale 一致，并写入 vue-i18n。 */
export const LOCALE_STORAGE_KEY = 'fg_locale'

export type AppLocale = 'zh-CN' | 'en-US'

const LOCALE_DIR: Record<AppLocale, string> = {
  'zh-CN': 'zh-CN',
  'en-US': 'en-US',
}

function readLocale(): AppLocale {
  try {
    const raw = localStorage.getItem(LOCALE_STORAGE_KEY)
    if (raw === 'zh-CN' || raw === 'en-US') return raw
  } catch {
    // 读不到本地偏好时用简体中文。
  }
  return 'zh-CN'
}

export const i18n = createI18n({
  legacy: false,
  locale: readLocale(),
  fallbackLocale: false,
  messages: {
    'zh-CN': {},
    'en-US': {},
  },
  missingWarn: false,
  fallbackWarn: false,
})

export const locale = (i18n.global as Composer).locale

const moduleLoadCache = new Map<string, Promise<Record<string, unknown>>>()
const loadedModules = new Set<string>()

function cacheKey(moduleName: string, lang: AppLocale) {
  return `${moduleName}:${lang}`
}

export function isModuleMessagesLoaded(moduleName: string, lang: AppLocale = getCurrentLocale()) {
  return loadedModules.has(cacheKey(moduleName, lang))
}

export function getCurrentLocale(): AppLocale {
  const value = locale.value
  return value === 'en-US' ? 'en-US' : 'zh-CN'
}

/** 按模块动态导入语言包。同一模块同一语言只拉一次。 */
export function loadModuleMessages(moduleName: string, lang: AppLocale = getCurrentLocale()) {
  const key = cacheKey(moduleName, lang)
  if (loadedModules.has(key)) {
    const current = i18n.global.getLocaleMessage(lang) as Record<string, unknown>
    const found = current[moduleName]
    return Promise.resolve(found && typeof found === 'object' ? (found as Record<string, unknown>) : {})
  }
  const pending = moduleLoadCache.get(key)
  if (pending) return pending

  const task = (async () => {
    try {
      const dir = LOCALE_DIR[lang]
      const pack = await import(`./${dir}/${moduleName}.ts`)
      const current = i18n.global.getLocaleMessage(lang) as Record<string, unknown>
      i18n.global.setLocaleMessage(lang, {
        ...current,
        [moduleName]: pack.default,
      })
      loadedModules.add(key)
      return pack.default as Record<string, unknown>
    } catch {
      return {}
    } finally {
      moduleLoadCache.delete(key)
    }
  })()

  moduleLoadCache.set(key, task)
  return task
}

/** 进入页面前加载壳层或登录包，以及该页模块包。 */
export async function loadRouteMessages(to: { path: string; meta: { module?: unknown; slot?: unknown }; params?: { module?: unknown } }) {
  const lang = getCurrentLocale()
  const jobs: Promise<unknown>[] = []
  if (to.path === '/login') {
    jobs.push(loadModuleMessages('login', lang))
  } else {
    jobs.push(loadModuleMessages('shell', lang))
    const named = typeof to.meta.module === 'string' ? to.meta.module : ''
    const slotted = to.meta.slot && typeof to.params?.module === 'string' ? to.params.module : ''
    const moduleName = named || slotted
    if (moduleName) jobs.push(loadModuleMessages(moduleName, lang))
  }
  await Promise.all(jobs)
}

export async function setLocale(next: AppLocale) {
  if (locale.value === next) return
  locale.value = next
  document.documentElement.lang = next
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, next)
  } catch {
    // 写不进时只保留本次会话里的选择。
  }
  const names = new Set<string>()
  for (const key of loadedModules) {
    const name = key.split(':')[0]
    if (name) names.add(name)
  }
  await Promise.all([...names].map((name) => loadModuleMessages(name, next)))
}
