<script setup lang="ts">
import { ref, watch } from 'vue'
import { FG_AI_DOCK_ID } from '@/ai'
import ConsoleContent from './ConsoleContent.vue'
import ConsoleHeader from './ConsoleHeader.vue'
import ConsoleSider from './ConsoleSider.vue'

const SIDEBAR_KEY = 'fg_sidebar_collapsed'

function readCollapsed() {
  try {
    return localStorage.getItem(SIDEBAR_KEY) === '1'
  } catch {
    return false
  }
}

const collapsed = ref(readCollapsed())

watch(collapsed, (value) => {
  try {
    localStorage.setItem(SIDEBAR_KEY, value ? '1' : '0')
  } catch {
    // 写不进时只保留本次会话里的折叠状态。
  }
})
</script>

<template>
  <div class="fg-shell" :class="{ 'fg-shell--collapsed': collapsed }">
    <ConsoleSider v-model:collapsed="collapsed" />
    <div class="fg-workspace">
      <ConsoleHeader />
      <ConsoleContent />
    </div>
    <aside :id="FG_AI_DOCK_ID" class="fg-ai-dock" data-fg-ai="dock" />
  </div>
</template>

<style scoped>
.fg-shell {
  grid-template-columns: var(--fg-sider-w) minmax(0, 1fr);
  transition: grid-template-columns 0.2s ease;
}

.fg-shell--collapsed {
  grid-template-columns: var(--fg-sider-collapsed) minmax(0, 1fr);
}

.fg-shell:has(.fg-ai-dock:not(:empty)) {
  grid-template-columns: var(--fg-sider-w) minmax(0, 1fr) minmax(280px, 22rem);
}

.fg-shell--collapsed:has(.fg-ai-dock:not(:empty)) {
  grid-template-columns: var(--fg-sider-collapsed) minmax(0, 1fr) minmax(280px, 22rem);
}

.fg-ai-dock:empty {
  display: none;
}

.fg-ai-dock:not(:empty) {
  min-width: 0;
  min-height: 0;
  overflow: auto;
  border-left: 1px solid var(--rs-border);
  background: var(--rs-surface);
}

.fg-workspace {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background: var(--rs-bg);
}
</style>
