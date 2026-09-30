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
import type { Channel } from './api'
import {
  channelColumns,
  channelMenuItems,
  channelSearchFields,
  flagLabel,
  kindOptions,
  secretOptions,
  statusOptions,
  useChannelCatalog,
} from './composables/useChannelCatalog'

const moduleId = 'fg0042'
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
  submitChannel,
  removeChannel,
  sendTest,
  formFields,
  onRow,
  onSearch,
  onReset,
} = useChannelCatalog()

const formTitle = computed(() => {
  if (formMode.value === 'create') return '新增通知渠道'
  if (formMode.value === 'edit') return '修改通知渠道'
  return '通知渠道'
})

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'create', label: '新增', icon: 'plus', onClick: openCreate },
  {
    key: 'edit',
    label: '修改',
    icon: 'pencil',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openEdit(active.value)
    },
  },
  {
    key: 'delete',
    label: '删除',
    icon: 'trash',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void removeChannel(active.value)
    },
  },
  {
    key: 'view',
    label: '查看',
    icon: 'eye',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openView(active.value)
    },
  },
  {
    key: 'test',
    label: '测试',
    icon: 'send',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void sendTest(active.value)
    },
  },
])

useModuleAi(
  () => ({
    moduleId,
    title: '通知渠道',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      selectedChannelId: active.value?.channelId ?? '',
      selectedChannelName: active.value?.channelName ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0042.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: Channel | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
  if (payload.key === 'edit') openEdit(payload.row)
  if (payload.key === 'delete') void removeChannel(payload.row)
  if (payload.key === 'test') void sendTest(payload.row)
}
</script>

<template>
  <div class="fg-page fg-channel-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog>
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="channelSearchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="channelColumns"
        row-key="channelId"
        :module-id="moduleId"
        :menu-items="channelMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #channelKind="{ row }">
          {{ flagLabel(kindOptions, String((row as Channel).channelKind ?? '')) }}
        </template>
        <template #secretSet="{ row }">
          {{ flagLabel(secretOptions, String((row as Channel).secretSet ?? '')) }}
        </template>
        <template #statusFlag="{ row }">
          <RsTag size="sm" :variant="(row as Channel).statusFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(statusOptions, String((row as Channel).statusFlag ?? '')) }}
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
      width="720px"
      :module-id="moduleId"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submitChannel"
    />
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-channel-host {
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
