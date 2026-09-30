<script setup lang="ts">
import { computed } from 'vue'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import type { Rule } from '../api'
import {
  flagLabel,
  levelOptions,
  metricOptions,
  opOptions,
  ruleColumns,
  ruleFormFields,
  ruleMenuItems,
  ruleSearchFields,
  statusOptions,
  useHostRules,
} from '../composables/useHostRules'

const moduleId = 'fg0005'
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
  openCreate,
  openEdit,
  submitRule,
  removeRule,
  onRow,
  onSearch,
  onReset,
} = useHostRules()

const formFields = ruleFormFields
const formTitle = computed(() => {
  if (formMode.value === 'create') return '新增预警规则'
  if (formMode.value === 'edit') return '修改预警规则'
  return '预警规则'
})

const emit = defineEmits<{ back: [] }>()

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'back', label: '主机', icon: 'cpu', code: 'fg0005:search', aiRisk: 'read', onClick: () => emit('back') },
  { key: 'create', label: '新增', icon: 'plus', code: 'fg0005:edit', onClick: openCreate },
  {
    key: 'edit',
    label: '修改',
    icon: 'pencil',
    code: 'fg0005:edit',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openEdit(active.value)
    },
  },
  {
    key: 'delete',
    label: '删除',
    icon: 'trash',
    code: 'fg0005:edit',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void removeRule(active.value)
    },
  },
  {
    key: 'view',
    label: '查看',
    icon: 'eye',
    code: 'fg0005:view',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openView(active.value)
    },
  },
])

function onMenu(payload: { key: string; row: Rule | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
  if (payload.key === 'edit') openEdit(payload.row)
  if (payload.key === 'delete') void removeRule(payload.row)
}
</script>

<template>
  <div class="fg-page fg-rule-host">
    <FgCatalog>
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="ruleSearchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="ruleColumns"
        row-key="ruleId"
        :module-id="moduleId"
        :menu-items="ruleMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #metricCode="{ row }">
          {{ flagLabel(metricOptions, String((row as Rule).metricCode ?? '')) }}
        </template>
        <template #compareOp="{ row }">
          {{ flagLabel(opOptions, String((row as Rule).compareOp ?? '')) }}
        </template>
        <template #levelCode="{ row }">
          <RsTag size="sm" :variant="(row as Rule).levelCode === 'critical' ? 'info' : 'default'">
            {{ flagLabel(levelOptions, String((row as Rule).levelCode ?? '')) }}
          </RsTag>
        </template>
        <template #statusFlag="{ row }">
          <RsTag size="sm" :variant="(row as Rule).statusFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(statusOptions, String((row as Rule).statusFlag ?? '')) }}
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
      :fields="formFields"
      :initial-data="current"
      :auto-close="false"
      :module-id="moduleId"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submitRule"
    />
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-rule-host {
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
