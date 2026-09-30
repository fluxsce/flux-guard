<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsPagination } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import HostDetail from './components/HostDetail.vue'
import HostGrid from './components/HostGrid.vue'
import HostRules from './components/HostRules.vue'
import { hostColumns, hostMenuItems, hostSearchFields, useHostCatalog } from './composables/useHostCatalog'
import { useHostDetail } from './composables/useHostDetail'
import type { HostMonitor, SeriesRange } from './api'

const moduleId = 'fg0005'
const {
  page,
  pageSize,
  error,
  loading,
  active,
  viewing,
  total,
  pageRows,
  sort,
  onSort,
  select,
  show,
  back: leaveList,
  onSearch,
  onReset,
  reload,
} = useHostCatalog()
const {
  detail,
  curves,
  error: detailError,
  loading: detailLoading,
  load: loadDetail,
  clear: clearDetail,
} = useHostDetail()
const rulesOpen = ref(false)
const seriesRange = ref<SeriesRange | null>(null)

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'refresh', label: '刷新', icon: 'rotate-ccw', code: 'fg0005:search', aiRisk: 'read', onClick: () => load() },
  { key: 'rules', label: '预警规则', icon: 'bell', code: 'fg0005:view', aiRisk: 'read', onClick: () => { rulesOpen.value = true } },
  {
    key: 'monitor',
    label: '监控',
    icon: 'activity',
    code: 'fg0005:view',
    aiRisk: 'read',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openMonitor(active.value)
    },
  },
])

function openMonitor(row: HostMonitor) {
  if (!show(row)) return
}

function onDetailRefresh(range: SeriesRange) {
  seriesRange.value = range
  void load()
}

function back() {
  leaveList()
  clearDetail()
}

async function load() {
  const current = await reload()
  if (viewing.value && current) await loadDetail(current, true, seriesRange.value)
  else if (!viewing.value) clearDetail()
}

function onMenu(payload: { key: string; row: HostMonitor | null }) {
  if (payload.row && payload.key === 'monitor') openMonitor(payload.row)
}

onMounted(() => {
  void load()
})

useModuleAi(() => ({
  moduleId,
  title: '主机监控',
  actions: [{ key: 'refresh', label: '刷新', risk: 'read', run: () => load() }],
  snapshot: () => ({
    error: error.value,
    total: total.value,
    page: page.value,
    pageSize: pageSize.value,
    selected: active.value?.agentId ?? '',
    viewing: viewing.value,
    hosts: pageRows.value.map((row) => ({
      agentId: row.agentId,
      hostName: row.reportName || row.hostName,
      cpuUsagePercent: row.cpuUsagePercent,
      memUsedPercent: row.memUsedPercent,
      diskUsedPercent: row.diskUsedPercent,
    })),
  }),
}))
</script>

<template>
  <div class="fg-page fg-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <HostRules v-if="rulesOpen" @back="rulesOpen = false" />
    <FgCatalog v-else-if="!viewing">
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="hostSearchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <HostGrid
        :data="pageRows"
        :columns="hostColumns"
        :loading="loading"
        :menu-items="hostMenuItems"
        :module-id="moduleId"
        :sort="sort"
        @update:sort="onSort"
        @row-click="select"
        @row-dblclick="openMonitor"
        @menu-click="onMenu"
      />
      <template #pager>
        <RsPagination
          v-model:page="page"
          v-model:page-size="pageSize"
          :total="total"
          size="sm"
          align="end"
          show-page-size
          :page-size-options="pageSizeChoices"
        />
      </template>
    </FgCatalog>
    <HostDetail
      v-else-if="active"
      :host="active"
      :detail="detail"
      :curves="curves"
      :loading="detailLoading"
      @back="back"
      @refresh="onDetailRefresh"
    />
  </div>
</template>

<style scoped>
.fg-host {
  position: relative;
  z-index: 0;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
