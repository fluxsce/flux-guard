<script setup lang="ts">
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import type { OperationLog } from './api'
import {
  failReasonOf,
  flagLabel,
  logColumns,
  moduleLabel,
  logMenuItems,
  logMoreFields,
  logSearchFields,
  resultOptions,
  summaryOf,
  targetLabelOf,
  useOperationLog,
} from './composables/useOperationLog'

const moduleId = 'fg0051'

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
} = useOperationLog()

useModuleAi(
  () => ({
    moduleId,
    title: '系统操作日志',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      selectedAuditId: active.value?.auditId ?? '',
      selectedUserName: active.value?.userName ?? '',
      selectedAction: active.value?.action ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0051.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: OperationLog | null }) {
  if (payload.key === 'view' && payload.row) void openView(payload.row)
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
        row-key="auditId"
        :module-id="moduleId"
        :menu-items="logMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #userName="{ row }">
          {{ (row as OperationLog).userName || (row as OperationLog).userId }}
        </template>
        <template #targetName="{ row }">
          {{ targetLabelOf(row as OperationLog) }}
        </template>
        <template #moduleCode="{ row }">
          {{ moduleLabel(String((row as OperationLog).moduleCode ?? '')) }}
        </template>
        <template #detailText="{ row }">
          {{ summaryOf(row as OperationLog) }}
        </template>
        <template #resultFlag="{ row }">
          <RsTag size="sm" :variant="(row as OperationLog).resultFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(resultOptions, String((row as OperationLog).resultFlag ?? '')) }}
          </RsTag>
        </template>
        <template #failReason="{ row }">
          <span v-if="failReasonOf(row as OperationLog)" class="fail-reason" :title="failReasonOf(row as OperationLog)">
            {{ failReasonOf(row as OperationLog) }}
          </span>
          <span v-else>—</span>
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
      title="系统操作日志"
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
.fail-reason {
  color: var(--rs-color-text-secondary, #6b7280);
  font-size: 12px;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 220px;
  display: inline-block;
  vertical-align: bottom;
}
</style>
