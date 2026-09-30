import { computed, defineComponent, h, markRaw, ref } from 'vue'
import type { Component } from 'vue'
import { defineStore } from 'pinia'

const STORAGE_KEY = 'fg_workspace_tabs'
const MAX_TABS = 20

export interface WorkspacePage {
  path: string
  title: string
  flush: boolean
}

/**
 * 页面组件和壳都不进响应式对象。
 * remember 只登记；壳在第一次打开时才建。KeepAlive 见到壳才会挂载。
 * 告警和插件占位共用同一个页面组件，所以每个路径单独包一层稳定名字。
 */
const views = new Map<string, Component>()
const shells = new Map<string, Component>()

function cacheName(path: string) {
  return `FgTab:${path}`
}

function readPaths() {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is string => typeof item === 'string' && item.startsWith('/'))
  } catch {
    return []
  }
}

function release(path: string) {
  views.delete(path)
  shells.delete(path)
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const pages = ref<WorkspacePage[]>([])
  const cacheNames = computed(() => pages.value.map((page) => cacheName(page.path)))

  function persist() {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(pages.value.map((page) => page.path)))
    } catch {
      // 写不进时只保留本次会话里的标签。
    }
  }

  function remember(page: WorkspacePage, view: Component) {
    if (!shells.has(page.path)) views.set(page.path, markRaw(view))
    const found = pages.value.find((item) => item.path === page.path)
    if (found) {
      found.title = page.title
      found.flush = page.flush
      return
    }
    pages.value.push({ path: page.path, title: page.title, flush: page.flush })
    while (pages.value.length > MAX_TABS) {
      const index = pages.value.findIndex((item) => item.path !== page.path)
      if (index < 0) break
      const [removed] = pages.value.splice(index, 1)
      if (removed) release(removed.path)
    }
    persist()
  }

  /** 当前页第一次打开时建壳。之前只登记、没挂载的页从这里进入 KeepAlive。 */
  function present(path: string) {
    if (shells.has(path)) return
    const view = views.get(path)
    if (!view) return
    shells.set(
      path,
      markRaw(
        defineComponent({
          name: cacheName(path),
          setup() {
            return () => h(view)
          },
        }),
      ),
    )
  }

  function retitle(path: string, title: string) {
    const found = pages.value.find((item) => item.path === path)
    if (!found || found.title === title) return
    found.title = title
  }

  function viewOf(path: string) {
    return shells.get(path)
  }

  function drop(paths: string[]) {
    if (!paths.length) return
    const closing = new Set(paths)
    for (const path of closing) release(path)
    pages.value = pages.value.filter((item) => !closing.has(item.path))
    persist()
  }

  function reorder(drag: string, target: string) {
    const from = pages.value.findIndex((item) => item.path === drag)
    const to = pages.value.findIndex((item) => item.path === target)
    if (from < 0 || to < 0 || from === to) return
    const next = pages.value.slice()
    const [moved] = next.splice(from, 1)
    if (!moved) return
    next.splice(to, 0, moved)
    pages.value = next
    persist()
  }

  function reset() {
    views.clear()
    shells.clear()
    pages.value = []
    try {
      sessionStorage.removeItem(STORAGE_KEY)
    } catch {
      // 清不掉时下次登录再按授权过滤。
    }
  }

  return { pages, cacheNames, storedPaths: readPaths, remember, present, retitle, viewOf, drop, reorder, reset }
})
