<script setup lang="ts">
import FgGrid from '@/components/grid/FgGrid.vue'
import { RsTag } from '@/ui'
import type { FgGridColumn, FgGridMenuItem, FgGridSort } from '@/components/grid/types'
import { clock, count, fixed, percent, rate } from '../composables/format'
import type { HostMonitor } from '../api'

defineProps<{
  data: HostMonitor[]
  columns: FgGridColumn[]
  loading: boolean
  menuItems: FgGridMenuItem[]
  sort: FgGridSort | null
  moduleId?: string
}>()

const emit = defineEmits<{
  rowClick: [row: HostMonitor]
  rowDblclick: [row: HostMonitor]
  menuClick: [payload: { key: string; row: HostMonitor | null }]
  'update:sort': [value: FgGridSort | null]
}>()

function hostOf(row: unknown) {
  return row as HostMonitor
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
    <template #onlineFlag="{ row }">
      <RsTag size="sm" :variant="hostOf(row).onlineFlag === 'Y' ? 'success' : 'default'">
        {{ hostOf(row).onlineFlag === 'Y' ? '在线' : '离线' }}
      </RsTag>
    </template>
    <template #hostName="{ row }">{{ hostOf(row).reportName || hostOf(row).hostName || '—' }}</template>
    <template #cpuUsagePercent="{ row }">{{ percent(hostOf(row).cpuUsagePercent) }}</template>
    <template #loadAvg1="{ row }">{{ fixed(hostOf(row).loadAvg1, 2) }}</template>
    <template #memUsedPercent="{ row }">{{ percent(hostOf(row).memUsedPercent) }}</template>
    <template #diskUsedPercent="{ row }">{{ percent(hostOf(row).diskUsedPercent) }}</template>
    <template #diskReadBytesPerSec="{ row }">{{ rate(hostOf(row).diskReadBytesPerSec) }}</template>
    <template #diskWriteBytesPerSec="{ row }">{{ rate(hostOf(row).diskWriteBytesPerSec) }}</template>
    <template #netReceiveBytesPerSec="{ row }">{{ rate(hostOf(row).netReceiveBytesPerSec) }}</template>
    <template #netTransmitBytesPerSec="{ row }">{{ rate(hostOf(row).netTransmitBytesPerSec) }}</template>
    <template #tcpEstablished="{ row }">{{ count(hostOf(row).tcpEstablished) }}</template>
    <template #tcpTimeWait="{ row }">{{ count(hostOf(row).tcpTimeWait) }}</template>
    <template #tcpCloseWait="{ row }">{{ count(hostOf(row).tcpCloseWait) }}</template>
    <template #fdAllocated="{ row }">{{ count(hostOf(row).fdAllocated) }}</template>
    <template #procCount="{ row }">{{ count(hostOf(row).procCount) }}</template>
    <template #zombieCount="{ row }">{{ count(hostOf(row).zombieCount) }}</template>
    <template #reportedTime="{ row }">{{ clock(hostOf(row).reportedTime) }}</template>
  </FgGrid>
</template>
