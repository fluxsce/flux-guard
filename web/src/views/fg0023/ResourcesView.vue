<script setup lang="ts">
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import type { Resource } from './api'
import {
  resourceColumns,
  resourceFormFields,
  resourceMenuItems,
  resourceSearchFields,
  resourceTreeConfig,
  resourceTypeLabel,
  useResourceCatalog,
} from './composables/useResourceCatalog'

const moduleId = 'fg0023'

const {
  page,
  pageSize,
  error,
  expandedKeys,
  formOpen,
  current,
  active,
  filtered,
  pageRows,
  toolbarButtons,
  openView,
  onRow,
  onMenu,
  onSearch,
  onReset,
} = useResourceCatalog()

useModuleAi(
  () => ({
    moduleId,
    title: '资源管理',
    snapshot: () => ({
      total: filtered.value.length,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      selectedResourceId: active.value?.resourceId ?? '',
      selectedResourceCode: active.value?.resourceCode ?? '',
      selectedResourceName: active.value?.resourceName ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0023.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: filtered.value.length }
      },
    },
  ],
)
</script>

<template>
  <div class="fg-page" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
  <FgCatalog>
    <template #search>
      <FgSearchForm
        plain
        :module-id="moduleId"
        :fields="resourceSearchFields"
        :toolbar-buttons="toolbarButtons"
        @search="onSearch"
        @reset="onReset"
      />
    </template>
    <FgGrid
      :data="pageRows"
      :columns="resourceColumns"
      row-key="resourceId"
      :module-id="moduleId"
      :tree-config="resourceTreeConfig"
      :menu-items="resourceMenuItems"
      v-model:expanded-row-keys="expandedKeys"
      @row-click="onRow"
      @row-dblclick="openView"
      @menu-click="onMenu"
    >
      <template #resourceType="{ row }">
        <RsTag size="sm">{{ resourceTypeLabel(String((row as Resource).resourceType ?? '')) }}</RsTag>
      </template>
    </FgGrid>
    <template #pager>
      <RsPagination
        v-model:page="page"
        v-model:page-size="pageSize"
        :total="filtered.length"
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
    title="资源"
    :fields="resourceFormFields"
    :module-id="moduleId"
    :initial-data="current"
    :teleport-to="`#${moduleId}`"
  />
  </div>
</template>
