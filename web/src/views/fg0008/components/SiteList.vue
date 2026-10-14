<script setup lang="ts">
import { computed } from 'vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsButton, RsEmpty, RsInput, RsTag } from '@/ui'
import type { AppSite } from '../api'
import { modeLabel, moduleId } from '../composables/useAppWorkbench'

const props = defineProps<{
  sites: AppSite[]
  current: AppSite | null
  loading?: boolean
  keyword: string
  siteKey: (row: AppSite) => string
}>()

const emit = defineEmits<{
  'update:keyword': [value: string]
  search: []
  select: [row: AppSite]
  refresh: []
}>()

const toolbarButtons: FgToolbarButton[] = [
  {
    key: 'refresh',
    label: '刷新',
    icon: 'rotate-ccw',
    code: 'fg0008:search',
    aiRisk: 'read',
    onClick: () => emit('refresh'),
  },
]

const empty = computed(() => !props.loading && !props.sites.length)

function isActive(row: AppSite) {
  return !!props.current && props.siteKey(props.current) === props.siteKey(row)
}
</script>

<template>
  <aside class="fg-site-list">
    <FgToolbar plain wrap title="运行时" :buttons="toolbarButtons" :module-id="moduleId" />
    <form class="fg-site-search" @submit.prevent="emit('search')">
      <RsInput
        class="fg-site-keyword"
        size="sm"
        clearable
        :model-value="keyword"
        placeholder="实例 / 主机 / 路径"
        @update:model-value="emit('update:keyword', String($event ?? ''))"
      />
      <RsButton size="sm" type="submit" @click.prevent="emit('search')">搜索</RsButton>
    </form>
    <RsEmpty
      v-if="empty"
      class="fg-site-empty"
      title="暂无上报的运行时"
      description="请确认 appside 已配置并在线。"
    />
    <ul v-else class="fg-site-ul" :aria-busy="loading || undefined">
      <li v-for="row in sites" :key="siteKey(row)">
        <button
          type="button"
          class="fg-site-item"
          :class="{ active: isActive(row) }"
          @click="emit('select', row)"
        >
          <div class="fg-site-top">
            <strong>{{ row.instanceName }}</strong>
            <RsTag size="sm" :variant="row.onlineFlag === 'Y' ? 'success' : 'default'">
              {{ row.onlineFlag === 'Y' ? '在线' : '离线' }}
            </RsTag>
          </div>
          <span class="meta">
            {{ modeLabel(row.modeCode) }} · {{ row.fileCount }} 项
          </span>
          <span class="meta">
            {{ row.hostName || '—' }}
            <template v-if="row.ipAddress"> · {{ row.ipAddress }}</template>
          </span>
          <span v-if="row.basePath" class="meta path">{{ row.basePath }}</span>
        </button>
      </li>
    </ul>
  </aside>
</template>

<style scoped>
.fg-site-list {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--fg-panel-bg, var(--rs-surface));
}

.fg-site-search {
  display: flex;
  gap: 6px;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-site-keyword {
  flex: 1;
  min-width: 0;
}

.fg-site-empty {
  flex: 1;
  min-height: 0;
}

.fg-site-ul {
  flex: 1;
  min-height: 0;
  margin: 0;
  padding: 6px;
  list-style: none;
  overflow: auto;
}

.fg-site-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  margin-bottom: 4px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  color: inherit;
}

.fg-site-item:hover {
  background: color-mix(in srgb, var(--rs-primary, #2563eb) 6%, transparent);
}

.fg-site-item.active {
  border-color: color-mix(in srgb, var(--rs-primary, #2563eb) 35%, transparent);
  background: color-mix(in srgb, var(--rs-primary, #2563eb) 10%, transparent);
}

.fg-site-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.fg-site-top strong {
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta {
  font-size: 12px;
  color: var(--rs-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta.path {
  opacity: 0.85;
}
</style>
