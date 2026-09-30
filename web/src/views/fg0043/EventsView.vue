<script setup lang="ts">
import { computed } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import type { AlertLog } from './api'
import {
  flagLabel,
  levelOptions,
  logColumns,
  logFormFields,
  logMenuItems,
  logSearchFields,
  sceneOptions,
  sendStatusOptions,
  useEventCatalog,
} from './composables/useEventCatalog'

const moduleId = 'fg0043'

const {
  page,
  pageSize,
  error,
  formOpen,
  formMode,
  current,
  active,
  total,
  pageRows,
  openView,
  onRow,
  onSearch,
  onReset,
} = useEventCatalog()

const formTitle = computed(() => '预警日志')
const toolbarButtons = computed<FgToolbarButton[]>(() => [
  {
    key: 'view',
    label: '查看',
    icon: 'eye',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openView(active.value)
    },
  },
])

useModuleAi(
  () => ({
    moduleId,
    title: '预警日志',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      selectedLogId: active.value?.alertLogId ?? '',
      selectedTitle: active.value?.alertTitle ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0043.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: AlertLog | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
}
</script>

<template>
  <div class="fg-page fg-event-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog>
      <template #alert>
        <p class="fg-event-hint">功能模块决定发送后写入一行。发送状态、时间和错误写在同一行。</p>
      </template>
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="logSearchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="logColumns"
        row-key="alertLogId"
        :module-id="moduleId"
        :menu-items="logMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #alertLevel="{ row }">
          <RsTag size="sm" :variant="(row as AlertLog).alertLevel === 'critical' ? 'info' : 'default'">
            {{ flagLabel(levelOptions, String((row as AlertLog).alertLevel ?? '')) }}
          </RsTag>
        </template>
        <template #sceneCode="{ row }">
          {{ flagLabel(sceneOptions, String((row as AlertLog).sceneCode ?? '')) }}
        </template>
        <template #sendStatus="{ row }">
          <RsTag size="sm" :variant="(row as AlertLog).sendStatus === 'ok' ? 'success' : 'default'">
            {{ flagLabel(sendStatusOptions, String((row as AlertLog).sendStatus ?? '')) }}
          </RsTag>
        </template>
      </FgGrid>
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
    <FgDataFormModal
      v-model:open="formOpen"
      :mode="formMode"
      :title="formTitle"
      :fields="logFormFields"
      :initial-data="current"
      :auto-close="false"
      :module-id="moduleId"
      :teleport-to="`#${moduleId}-layer`"
    />
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-event-host {
  position: relative;
  z-index: 0;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.fg-event-hint {
  margin: 0 0 8px;
  color: var(--fg-text-secondary, #667085);
  line-height: 1.5;
}
.fg-layer {
  position: absolute;
  inset: 0;
  z-index: 20;
  pointer-events: none;
}
.fg-layer:not(:empty) {
  pointer-events: auto;
}
</style>
