<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAiStore } from '@/ai'
import type { Component } from 'vue'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { moduleIcon } from '@/router'
import { modulePath, useSessionStore } from '@/stores/session'
import { useWorkspaceStore } from '@/stores/workspace'
import { RsTabs, type RsTabItem } from '@/ui'

const route = useRoute()
const router = useRouter()
const ai = useAiStore()
const session = useSessionStore()
const workspace = useWorkspaceStore()
const { t } = useModuleI18n('shell')
const pendingDrop = new Set<string>()

const tabItems = computed<RsTabItem[]>(() => {
  const home = session.homePath()
  const closable = workspace.pages.length > 1
  return workspace.pages.map((page) => {
    const menu = session.menus.find((item) => modulePath(item.resourceCode) === page.path)
    const pinned = page.path === home
    return {
      value: page.path,
      label: menu?.resourceName || page.title,
      icon: menu ? moduleIcon(menu.resourceCode) : undefined,
      fixed: pinned,
      closable: !pinned && closable,
    }
  })
})

const current = computed(() => workspace.viewOf(route.path))
const flush = computed(() => workspace.pages.some((page) => page.path === route.path && page.flush))

function describe(path: string) {
  if (!path.startsWith('/') || !session.allowPath(path)) return null
  const resolved = router.resolve(path)
  const record = resolved.matched.at(-1)
  const view = record?.components?.default
  if (!view || resolved.path === '/login') return null
  const menu = session.menus.find((item) => modulePath(item.resourceCode) === path)
  return {
    path,
    title: menu?.resourceName || path,
    flush: record?.meta?.flush === true,
    view: view as Component,
  }
}

function ensure(path: string) {
  const page = describe(path)
  if (!page) return
  workspace.remember(page, page.view)
  if (path === route.path) workspace.present(path)
}

function flushDrops() {
  if (!pendingDrop.size) return
  const paths = [...pendingDrop].filter((path) => path !== route.path)
  pendingDrop.clear()
  workspace.drop(paths)
}

function onSelect(path: string) {
  if (!path || path === route.path) return
  void router.push(path)
}

function onClose(path: string) {
  pendingDrop.add(path)
  if (path !== route.path) flushDrops()
}

function onCloseBatch(values: string[], action: string) {
  if (action === 'all') {
    const home = session.homePath()
    for (const page of workspace.pages) {
      if (page.path !== home) pendingDrop.add(page.path)
    }
    if (route.path !== home) void router.push(home)
    else flushDrops()
    return
  }
  for (const path of values) pendingDrop.add(path)
  if (!values.includes(route.path)) flushDrops()
}

function onReorder(drag: string, target: string) {
  workspace.reorder(drag, target)
}

const home = session.homePath()
if (home.startsWith('/') && home !== '/login') ensure(home)
for (const path of workspace.storedPaths()) {
  if (session.allowPath(path)) ensure(path)
}
ensure(route.path)

watch(
  () => route.path,
  (path) => {
    flushDrops()
    ensure(path)
  },
)

watch(
  () => {
    const named = route.meta.module
    if (typeof named === 'string' && named) return named
    const param = route.params.module
    return typeof param === 'string' ? param : ''
  },
  (moduleId) => ai.setActive(moduleId),
  { immediate: true },
)
</script>

<template>
  <div class="fg-workspace-main">
    <RsTabs
      v-if="tabItems.length"
      class="fg-tabs"
      :model-value="route.path"
      :items="tabItems"
      variant="line"
      size="md"
      overflow="scroll"
      panelless
      closable
      draggable
      context-menu
      :aria-label="t('tabs')"
      @update:model-value="onSelect"
      @close="onClose"
      @close-batch="onCloseBatch"
      @reorder="onReorder"
    />
    <div class="fg-pages">
      <div class="fg-content" :class="{ 'fg-content--flush': flush }">
        <KeepAlive :include="workspace.cacheNames">
          <component v-if="current" :is="current" :key="route.path" />
        </KeepAlive>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fg-workspace-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.fg-tabs {
  flex: none;
  --rs-control-height-md: 2.25rem;
  --rs-space-sm: 0.75rem;
  --rs-tabs-surface: transparent;
  --rs-tabs-border: transparent;
  --rs-tabs-border-subtle: transparent;
  --rs-tabs-text: var(--rs-text-secondary);
  --rs-tabs-text-hover: var(--rs-text-primary);
  --rs-tabs-text-active: var(--rs-primary);
  --rs-tabs-ink: var(--rs-primary);
  --rs-tabs-ink-size: 2px;
  --rs-tabs-close-size: 1rem;
  --rs-tabs-trigger-gap: 0.375rem;
}

.fg-pages {
  position: relative;
  flex: 1;
  min-height: 0;
}

.fg-pages > .fg-content {
  position: absolute;
  inset: 0;
}
</style>
