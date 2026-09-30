<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { useProductVersion } from '@/productVersion'
import { consoleMenu } from '@/router'
import { useSessionStore } from '@/stores/session'
import { RsIcon, RsMenu, RsSidebar, type RsMenuItem } from '@/ui'

const OPEN_KEY = 'fg_menu_open'

const collapsed = defineModel<boolean>('collapsed', { required: true })

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { t } = useModuleI18n('shell')
const { version } = useProductVersion()

function readOpenKeys() {
  try {
    const raw = localStorage.getItem(OPEN_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    if (Array.isArray(parsed)) return parsed.filter((item) => typeof item === 'string')
  } catch {
    // 读不到时按当前路由自动展开所在分组。
  }
  return []
}

const openKeys = ref<string[]>(readOpenKeys())

const menuItems = computed(() => consoleMenu(session.menus))

function onMenuClick(item: RsMenuItem, event: MouseEvent) {
  if (!item.to || item.children?.length) return
  event.preventDefault()
  void router.push(item.to)
}

watch(openKeys, (keys) => {
  try {
    localStorage.setItem(OPEN_KEY, JSON.stringify(keys))
  } catch {
    // 写不进时只保留本次会话里的展开状态。
  }
})
</script>

<template>
  <RsSidebar
    v-model:collapsed="collapsed"
    class="fg-sider"
    width="md"
    :aria-label="t('nav')"
  >
    <template #header>
      <button type="button" class="fg-brand" @click="router.push(session.homePath())">
        <span class="fg-mark" aria-hidden="true">
          <RsIcon name="activity" :size="16" />
        </span>
        <span v-if="!collapsed" class="fg-brand-copy">
          <strong>Flux Guard</strong>
          <span><i class="fg-live" />{{ t('product') }}<template v-if="version"> · v{{ version }}</template></span>
        </span>
      </button>
    </template>

    <RsMenu
      class="fg-menu"
      :model-value="route.path"
      v-model:open-keys="openKeys"
      :items="menuItems"
      mode="vertical"
      :collapsed="collapsed"
      :aria-label="t('nav')"
      @click="onMenuClick"
    />

    <template #footer>
      <button
        type="button"
        class="fg-collapse"
        :aria-label="collapsed ? t('expandNav') : t('collapseNav')"
        @click="collapsed = !collapsed"
      >
        <RsIcon :name="collapsed ? 'panel-left-open' : 'panel-left-close'" :size="16" />
        <span v-if="!collapsed">{{ t('collapse') }}</span>
      </button>
    </template>
  </RsSidebar>
</template>

<style scoped>
.fg-sider {
  block-size: 100%;
  border: 0;
  box-shadow: inset calc(var(--fg-line) * -1) 0 0 var(--rs-border);
}

.fg-sider :deep(.rs-sidebar__header),
.fg-sider :deep(.rs-sidebar__footer) {
  box-sizing: border-box;
  border: 0;
  padding: 0;
}

.fg-sider :deep(.rs-sidebar__header) {
  height: var(--fg-header-h);
  min-height: var(--fg-header-h);
  max-height: var(--fg-header-h);
  box-shadow: inset 0 calc(var(--fg-line) * -1) 0 var(--rs-border);
}

.fg-sider :deep(.rs-sidebar__footer) {
  height: var(--fg-footer-h);
  min-height: var(--fg-footer-h);
  max-height: var(--fg-footer-h);
  box-shadow: inset 0 var(--fg-line) 0 var(--rs-border);
}

.fg-menu {
  width: 100%;
}

.fg-brand,
.fg-collapse {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0 16px;
  border: 0;
  background: transparent;
  color: var(--rs-text-primary);
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.fg-collapse {
  gap: 8px;
  color: var(--rs-text-secondary);
  font-size: 12px;
}

.fg-sider.rs-sidebar--collapsed .fg-brand,
.fg-sider.rs-sidebar--collapsed .fg-collapse {
  justify-content: center;
  padding-inline: 0;
}

.fg-mark {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  flex: none;
  border-radius: 6px;
  color: #fff;
  background: var(--rs-primary);
}

.fg-brand-copy {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  line-height: 1.15;
}

.fg-brand-copy strong {
  font-size: 14px;
  font-weight: 650;
  letter-spacing: -0.03em;
}

.fg-brand-copy span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--rs-text-secondary);
}

.fg-live {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--rs-success);
}

.fg-collapse:hover,
.fg-brand:hover .fg-brand-copy strong {
  color: var(--rs-text-primary);
}

.fg-brand:focus-visible,
.fg-collapse:focus-visible {
  outline: 2px solid var(--rs-primary);
  outline-offset: 1px;
}
</style>
