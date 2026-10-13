<script setup lang="ts">
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgGridColumn, FgGridMenuItem, FgGridSort } from '@/components/grid/types'
import { bytes, clock, count, millis, percent } from '../composables/format'
import type { JavaApp } from '../api'

defineProps<{
  data: JavaApp[]
  columns: FgGridColumn[]
  loading: boolean
  menuItems: FgGridMenuItem[]
  sort: FgGridSort | null
  moduleId?: string
}>()

const emit = defineEmits<{
  rowClick: [row: JavaApp]
  rowDblclick: [row: JavaApp]
  menuClick: [payload: { key: string; row: JavaApp | null }]
  'update:sort': [value: FgGridSort | null]
}>()

function appOf(row: unknown) {
  return row as JavaApp
}

function findingText(row: JavaApp) {
  const lines = row.findings ?? []
  return lines.length > 0 ? lines.join('；') : '—'
}
</script>

<template>
  <FgGrid
    :data="data"
    :columns="columns"
    :loading="loading"
    row-key="agentId"
    :menu-items="menuItems"
    :module-id="moduleId"
    :sort="sort"
    remote-sort
    @row-click="emit('rowClick', $event)"
    @row-dblclick="emit('rowDblclick', $event)"
    @menu-click="emit('menuClick', $event)"
    @update:sort="emit('update:sort', $event)"
  >
    <template #finding="{ row }">
      <span class="fg-finding" :data-level="appOf(row).findingLevel || 'ok'">{{ findingText(appOf(row)) }}</span>
    </template>
    <template #appName="{ row }">{{ appOf(row).appName || '—' }}</template>
    <template #javaVersion="{ row }">{{ appOf(row).javaVersion || '—' }}</template>
    <template #hostName="{ row }">{{ appOf(row).hostName || '—' }}</template>
    <template #heapUsedPercent="{ row }">
      {{ appOf(row).heapUsedPercent < 0 ? '—' : percent(appOf(row).heapUsedPercent) }}
      <span v-if="appOf(row).heapMaxBytes > 0" class="fg-sub">{{ bytes(appOf(row).heapUsedBytes) }}</span>
    </template>
    <template #threadCount="{ row }">{{ count(appOf(row).threadCount) }}</template>
    <template #collectionCount="{ row }">
      {{ count(appOf(row).collectionCount) }}
      <span class="fg-sub">{{ millis(appOf(row).collectionTimeMs) }}</span>
    </template>
    <template #reportedTime="{ row }">{{ clock(appOf(row).reportedTime) }}</template>
  </FgGrid>
</template>

<style scoped>
.fg-sub {
  margin-left: 4px;
  color: var(--rs-text-secondary);
  font-size: 12px;
}

.fg-finding[data-level='alert'] {
  color: #e23b55;
}

.fg-finding[data-level='watch'] {
  color: #c49212;
}

.fg-finding[data-level='ok'] {
  color: var(--rs-text-secondary);
}
</style>
