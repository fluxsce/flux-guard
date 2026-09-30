<script setup lang="ts">
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import {
  contentPreview,
  eventLabel,
  flagLabel,
  logColumns,
  logMenuItems,
  logMoreFields,
  logSearchFields,
  sourceOptions,
  statusOptions,
  useAgentLog,
  type AgentLogRow,
} from './composables/useAgentLog'

const moduleId = 'fg0052'

const {
  page,
  pageSize,
  error,
  formOpen,
  formFields,
  current,
  active,
  total,
  pageRows,
  toolbarButtons,
  openView,
  onRow,
  onSearch,
  onReset,
} = useAgentLog()

useModuleAi(
  () => ({
    moduleId,
    title: '代理操作日志',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      selectedSource: active.value?.source ?? '',
      selectedLogId: active.value?.logId ?? '',
      selectedAgentId: active.value?.agentId ?? '',
      selectedEventType: active.value?.eventType ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0052.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: AgentLogRow | null }) {
  if (payload.key === 'view' && payload.row) void openView(payload.row)
}

function statusVariant(flag: string) {
  if (flag === 'Y') return 'success'
  if (flag === 'N') return 'info'
  return 'default'
}
</script>

<template>
  <div class="fg-page fg-log-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog>
      <template #search>
        <FgSearchForm
          plain
          :module-id="moduleId"
          :fields="logSearchFields"
          :more-fields="logMoreFields"
          :toolbar-buttons="toolbarButtons"
          @search="onSearch"
          @reset="onReset"
        />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="logColumns"
        row-key="rowKey"
        :module-id="moduleId"
        :menu-items="logMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #agentName="{ row }">
          {{ (row as AgentLogRow).agentName || (row as AgentLogRow).agentId || '—' }}
        </template>
        <template #userName="{ row }">
          {{ (row as AgentLogRow).userName || (row as AgentLogRow).userId }}
        </template>
        <template #source="{ row }">
          <RsTag size="sm" :variant="(row as AgentLogRow).source === 'SHELL' ? 'info' : 'default'">
            {{ flagLabel(sourceOptions, String((row as AgentLogRow).source ?? '')) }}
          </RsTag>
        </template>
        <template #eventType="{ row }">
          {{ eventLabel(String((row as AgentLogRow).eventType ?? '')) }}
        </template>
        <template #statusFlag="{ row }">
          <template v-if="(row as AgentLogRow).source === 'COMMAND' && (row as AgentLogRow).statusFlag">
            <RsTag size="sm" :variant="statusVariant(String((row as AgentLogRow).statusFlag))">
              {{ flagLabel(statusOptions, String((row as AgentLogRow).statusFlag ?? '')) }}
            </RsTag>
          </template>
          <template v-else>—</template>
        </template>
        <template #detailText="{ row }">
          {{ contentPreview(row as AgentLogRow) }}
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
      mode="view"
      title="代理操作日志"
      :fields="formFields"
      :initial-data="current"
      :module-id="moduleId"
      :teleport-to="`#${moduleId}-layer`"
    />
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-log-host {
  position: relative;
  z-index: 0;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.fg-layer {
  position: absolute;
  inset: 0;
  z-index: 30;
  pointer-events: none;
}
.fg-layer:deep(> *) {
  pointer-events: auto;
}
</style>
