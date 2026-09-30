<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsPagination } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import JavaDetail from './components/JavaDetail.vue'
import JavaGrid from './components/JavaGrid.vue'
import { javaColumns, javaMenuItems, javaSearchFields, useJavaCatalog } from './composables/useJavaCatalog'
import { useJavaDetail } from './composables/useJavaDetail'
import type { JavaApp, SeriesRange } from './api'

const moduleId = 'fg0006'
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
} = useJavaCatalog()
const {
  detail,
  curves,
  loading: detailLoading,
  load: loadDetail,
  clear: clearDetail,
} = useJavaDetail()
const seriesRange = ref<SeriesRange | null>(null)

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'refresh', label: '刷新', icon: 'rotate-ccw', code: 'fg0006:search', aiRisk: 'read', onClick: () => load() },
  {
    key: 'monitor',
    label: '监控',
    icon: 'activity',
    code: 'fg0006:view',
    aiRisk: 'read',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openMonitor(active.value)
    },
  },
])

function openMonitor(row: JavaApp) {
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

function onMenu(payload: { key: string; row: JavaApp | null }) {
  if (payload.row && payload.key === 'monitor') openMonitor(payload.row)
}

onMounted(() => {
  void load()
})

useModuleAi(() => ({
  moduleId,
  title: 'Java 监控',
  actions: [{ key: 'refresh', label: '刷新', risk: 'read', run: () => load() }],
  snapshot: () => ({
    error: error.value,
    total: total.value,
    page: page.value,
    pageSize: pageSize.value,
    selected: active.value?.agentId ?? '',
    viewing: viewing.value,
    apps: pageRows.value.map((row) => ({
      agentId: row.agentId,
      appName: row.appName,
      heapUsedPercent: row.heapUsedPercent,
      threadCount: row.threadCount,
      finding: (row.findings ?? []).join('；'),
    })),
  }),
}))
</script>

<template>
  <div class="fg-page fg-java" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog v-if="!viewing">
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="javaSearchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <JavaGrid
        :data="pageRows"
        :columns="javaColumns"
        :loading="loading"
        :menu-items="javaMenuItems"
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
    <JavaDetail
      v-else-if="active"
      :app="active"
      :detail="detail"
      :curves="curves"
      :loading="detailLoading"
      @back="back"
      @refresh="onDetailRefresh"
    />
  </div>
</template>

<style scoped>
.fg-java {
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
