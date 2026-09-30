<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsDrawer, RsEmpty, RsTag } from '@/ui'
import {
  durationText,
  jobColumns,
  jobLogColumns,
  jobMenuItems,
  jobSearchFields,
  kindText,
  modeText,
  resultText,
  scheduleText,
  sourceText,
  triggerText,
  useJobCatalog,
} from './composables/useJobCatalog'
import type { JobLog, JobRow } from './api'
import CronSchedule from './components/CronSchedule.vue'

const moduleId = 'fg0025'
const pageReady = ref(false)
onMounted(() => {
  pageReady.value = true
})

const {
  editable,
  rows,
  active,
  error,
  loading,
  running,
  formOpen,
  formMode,
  current,
  formFields,
  kindChoices,
  logsOpen,
  logsTitle,
  logs,
  logsLoading,
  onSearch,
  onReset,
  onRow,
  openCreate,
  openDetail,
  openLogs,
  submit,
  remove,
  run,
} = useJobCatalog()

const formTitle = computed(() => {
  if (formMode.value === 'create') return '新增任务'
  if (formMode.value === 'edit') return '修改任务'
  return '查看任务'
})

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'add', label: '新增', icon: 'plus', code: 'fg0025:edit', disabled: !editable.value, onClick: openCreate },
  {
    key: 'edit',
    label: '修改',
    icon: 'pencil',
    code: 'fg0025:edit',
    disabled: !editable.value || !active.value,
    onClick: () => {
      if (active.value) void openDetail(active.value, 'edit')
    },
  },
  {
    key: 'delete',
    label: '删除',
    icon: 'trash',
    code: 'fg0025:edit',
    disabled: !editable.value || !active.value?.deletable,
    onClick: () => {
      if (active.value) void remove(active.value)
    },
  },
  {
    key: 'view',
    label: '查看',
    icon: 'eye',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void openDetail(active.value, 'view')
    },
  },
  {
    key: 'logs',
    label: '执行日志',
    icon: 'list',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void openLogs(active.value)
    },
  },
  {
    key: 'run',
    label: '执行',
    icon: 'play',
    code: 'fg0025:edit',
    loading: running.value,
    disabled: !editable.value || !active.value?.enabled || running.value,
    onClick: () => {
      if (active.value) void run(active.value)
    },
  },
])

const menuItems = computed(() => jobMenuItems(editable.value))

useModuleAi(
  () => ({
    moduleId,
    title: '任务调度',
    snapshot: () => ({
      total: rows.value.length,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      selected: active.value?.code ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0025.open',
      execute: async (args) => {
        const code = typeof args.code === 'string' ? args.code : ''
        const row = rows.value.find((item) => item.code === code)
        if (row) await openDetail(row, 'view')
        return { selected: active.value?.code ?? '' }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: JobRow | null }) {
  if (!payload.row) return
  if (payload.key === 'view') void openDetail(payload.row, 'view')
  if (payload.key === 'logs') void openLogs(payload.row)
  if (payload.key === 'edit') void openDetail(payload.row, 'edit')
  if (payload.key === 'delete') void remove(payload.row)
  if (payload.key === 'run') void run(payload.row)
}
</script>

<template>
  <div class="fg-page fg-job-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog>
      <template #search>
        <FgSearchForm
          plain
          :module-id="moduleId"
          :fields="jobSearchFields"
          :toolbar-buttons="toolbarButtons"
          @search="onSearch"
          @reset="onReset"
        />
      </template>
      <p v-if="error" class="fg-catalog-error">{{ error }}</p>
      <FgGrid
        :data="rows"
        :columns="jobColumns"
        row-key="code"
        :loading="loading"
        :module-id="moduleId"
        :menu-items="menuItems"
        @row-click="onRow"
        @row-dblclick="openDetail($event, 'view')"
        @menu-click="onMenu"
      >
        <template #kindTitle="{ row }">
          {{ kindText(row as JobRow, kindChoices) }}
        </template>
        <template #source="{ row }">
          {{ sourceText((row as JobRow).source) }}
        </template>
        <template #enabled="{ row }">
          <RsTag size="sm" :variant="(row as JobRow).enabled ? 'success' : 'default'">
            {{ (row as JobRow).enabled ? '启用' : '停用' }}
          </RsTag>
        </template>
        <template #mode="{ row }">
          {{ modeText((row as JobRow).mode) }}
        </template>
        <template #schedule="{ row }">
          {{ scheduleText(row as JobRow) }}
        </template>
        <template #timeout="{ row }">
          {{ (row as JobRow).timeoutSeconds ? `${(row as JobRow).timeoutSeconds} 秒` : '—' }}
        </template>
        <template #retry="{ row }">
          {{ (row as JobRow).retryCount ?? '—' }}
        </template>
        <template #nextStart="{ row }">
          {{ (row as JobRow).nextStart || '不自动执行' }}
        </template>
        <template #lastRunAt="{ row }">
          {{ (row as JobRow).ranAtLeast ? (row as JobRow).lastRunAt || '—' : '尚未执行' }}
        </template>
        <template #lastTrigger="{ row }">
          {{ (row as JobRow).ranAtLeast ? triggerText((row as JobRow).lastTrigger) : '—' }}
        </template>
        <template #lastOk="{ row }">
          <RsTag v-if="(row as JobRow).ranAtLeast" size="sm" :variant="(row as JobRow).lastOk === false ? 'danger' : 'success'">
            {{ (row as JobRow).lastOk === false ? '失败' : '成功' }}
          </RsTag>
          <span v-else>—</span>
        </template>
        <template #lastDetail="{ row }">
          {{ (row as JobRow).lastOk === false ? (row as JobRow).lastError || '未知错误' : resultText((row as JobRow).lastResult) || '—' }}
        </template>
        <template #duration="{ row }">
          {{ (row as JobRow).ranAtLeast ? durationText((row as JobRow).lastDurationMs) : '—' }}
        </template>
      </FgGrid>
    </FgCatalog>
    <FgDataFormModal
      v-if="pageReady"
      v-model:open="formOpen"
      :mode="formMode"
      :title="formTitle"
      :fields="formFields"
      :initial-data="current"
      :auto-close="false"
      :module-id="moduleId"
      save-action="edit"
      width="720px"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submit"
    >
      <template #cronBuilder="{ formModel, disabled }">
        <CronSchedule :model="formModel" :disabled="disabled" />
      </template>
    </FgDataFormModal>
    <RsDrawer v-if="pageReady" v-model:open="logsOpen" :title="logsTitle" side="right" size="lg" :teleport-to="false">
      <RsEmpty v-if="!logsLoading && logs.length === 0" description="还没有执行记录" />
      <FgGrid
        v-else
        :data="logs"
        :columns="jobLogColumns"
        row-key="logId"
        :loading="logsLoading"
        :module-id="moduleId"
      >
        <template #trigger="{ row }">
          {{ triggerText((row as JobLog).trigger) }}
        </template>
        <template #ok="{ row }">
          <RsTag size="sm" :variant="(row as JobLog).ok ? 'success' : 'danger'">
            {{ (row as JobLog).ok ? '成功' : '失败' }}
          </RsTag>
        </template>
        <template #duration="{ row }">
          {{ durationText((row as JobLog).durationMs) }}
        </template>
        <template #detail="{ row }">
          {{ (row as JobLog).ok ? resultText((row as JobLog).result) || '—' : (row as JobLog).errorText || '未知错误' }}
        </template>
      </FgGrid>
    </RsDrawer>
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-job-host {
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
