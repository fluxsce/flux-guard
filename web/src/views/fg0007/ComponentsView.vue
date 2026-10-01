<script setup lang="ts">
import { computed } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsButton, RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import ComponentDetail from './components/ComponentDetail.vue'
import type { ComponentConn } from './api'
import {
  connColumns,
  connMenuItems,
  flagLabel,
  statusOptions,
  upOptions,
  useComponentCatalog,
} from './composables/useComponentCatalog'
import { layoutOf } from './composables/forms'

const moduleId = 'fg0007'
const {
  page,
  pageSize,
  error,
  formOpen,
  formMode,
  current,
  active,
  viewing,
  total,
  pageRows,
  searchFields,
  formFields,
  openCreate,
  openEdit,
  submit,
  remove,
  probe,
  testing,
  trial,
  testDraft,
  show,
  back,
  onRow,
  onSearch,
  onReset,
  reload,
} = useComponentCatalog()

const formTitle = computed(() => (formMode.value === 'create' ? '新增组件连接' : '修改组件连接'))

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'create', label: '新增', icon: 'plus', code: 'fg0007:edit', onClick: openCreate },
  {
    key: 'edit',
    label: '修改',
    icon: 'pencil',
    code: 'fg0007:edit',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openEdit(active.value)
    },
  },
  {
    key: 'delete',
    label: '删除',
    icon: 'trash',
    code: 'fg0007:edit',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void remove(active.value)
    },
  },
  {
    key: 'probe',
    label: '立即采集',
    icon: 'refresh-cw',
    code: 'fg0007:edit',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void probe(active.value)
    },
  },
  {
    key: 'monitor',
    label: '监控',
    icon: 'activity',
    code: 'fg0007:view',
    disabled: !active.value,
    onClick: () => {
      if (active.value) show(active.value)
    },
  },
  { key: 'refresh', label: '刷新', icon: 'rotate-ccw', code: 'fg0007:search', onClick: () => void reload() },
])

useModuleAi(
  () => ({
    moduleId,
    title: '组件监控',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      error: error.value,
      selected: active.value?.componentId ?? '',
      viewing: viewing.value,
      connections: pageRows.value.map((row) => ({
        componentId: row.componentId,
        componentName: row.componentName,
        kindCode: row.kindCode,
        upFlag: row.upFlag,
        summaryText: row.summaryText,
      })),
    }),
  }),
  () => [],
)

function onMenu(payload: { key: string; row: ComponentConn | null }) {
  if (!payload.row) return
  if (payload.key === 'monitor') show(payload.row)
  if (payload.key === 'probe') void probe(payload.row)
  if (payload.key === 'edit') openEdit(payload.row)
  if (payload.key === 'delete') void remove(payload.row)
}

async function refreshDetail() {
  await reload()
  if (active.value) await probe(active.value)
}
</script>

<template>
  <div class="fg-page fg-component-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog v-if="!viewing">
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="searchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="connColumns"
        row-key="componentId"
        :module-id="moduleId"
        :menu-items="connMenuItems"
        @row-click="onRow"
        @row-dblclick="show"
        @menu-click="onMenu"
      >
        <template #kindName="{ row }">
          {{ (row as ComponentConn).kindName || (row as ComponentConn).kindCode }}
        </template>
        <template #layout="{ row }">
          {{ layoutOf((row as ComponentConn).kindCode, (row as ComponentConn).optionText, (row as ComponentConn).userName, (row as ComponentConn).secretSet).label }}
        </template>
        <template #upFlag="{ row }">
          <RsTag size="sm" :variant="(row as ComponentConn).upFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(upOptions, String((row as ComponentConn).upFlag ?? '')) }}
          </RsTag>
        </template>
        <template #statusFlag="{ row }">
          {{ flagLabel(statusOptions, String((row as ComponentConn).statusFlag ?? '')) }}
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
    <ComponentDetail
      v-else-if="active"
      :conn="active"
      @back="back"
      @refresh="refreshDetail"
    />
    <FgDataFormModal
      v-model:open="formOpen"
      :mode="formMode"
      :title="formTitle"
      :fields="formFields"
      :initial-data="current"
      :auto-close="false"
      save-action="edit"
      :testing="testing"
      width="720px"
      :module-id="moduleId"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submit"
    >
      <template #footer="{ values }">
        <RsButton size="sm" :loading="testing" :disabled="testing" @click="testDraft(values())">测试</RsButton>
      </template>
      <template #extra>
        <p v-if="trial" class="fg-trial" :data-level="trial.level">{{ trial.text }}</p>
      </template>
    </FgDataFormModal>
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-component-host {
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
.fg-trial {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.5;
}
.fg-trial[data-level='ok'] {
  color: #1a9a62;
}
.fg-trial[data-level='alert'] {
  color: #e23b55;
}
</style>
