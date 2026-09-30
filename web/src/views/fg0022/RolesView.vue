<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import RoleAuthDrawer from './components/RoleAuthDrawer.vue'
import {
  builtinOptions,
  flagLabel,
  roleColumns,
  roleFormFields,
  roleMenuItems,
  roleSearchFields,
  statusOptions,
  useRoleCatalog,
} from './composables/useRoleCatalog'
import { useRoleAuth } from './composables/useRoleAuth'
import type { Role } from './api'

const moduleId = 'fg0022'
const pageReady = ref(false)
onMounted(() => {
  pageReady.value = true
})

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
  submitRole,
  removeRole,
  onRow,
  onSearch,
  onReset,
} = useRoleCatalog()

const {
  authOpen,
  treeData,
  checkedKeys,
  expandedKeys,
  keyword,
  authTitle,
  openAuth,
  save,
} = useRoleAuth(active, error)

const formFields = roleFormFields
const formTitle = computed(() => {
  if (formMode.value === 'create') return '新增角色'
  if (formMode.value === 'edit') return '修改角色'
  return '角色'
})

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'add', label: '新增', icon: 'plus', onClick: openCreate },
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
    disabled: !active.value || active.value.builtInFlag === 'Y',
    onClick: () => {
      if (active.value) void removeRole(active.value)
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
    key: 'grant',
    label: '授权',
    icon: 'shield',
    disabled: !active.value,
    onClick: () => {
      if (active.value) void openAuth(active.value)
    },
  },
])

useModuleAi(
  () => ({
    moduleId,
    title: '角色管理',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      authOpen: authOpen.value,
      selectedRoleId: active.value?.roleId ?? '',
      selectedRoleName: active.value?.roleName ?? '',
      builtInFlag: active.value?.builtInFlag ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0022.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: Role | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
  if (payload.key === 'edit') openEdit(payload.row)
  if (payload.key === 'delete') void removeRole(payload.row)
  if (payload.key === 'grant') void openAuth(payload.row)
}
</script>

<template>
  <div class="fg-page fg-role-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
  <FgCatalog>
    <template #search>
      <FgSearchForm
        plain
        :module-id="moduleId"
        :fields="roleSearchFields"
        :toolbar-buttons="toolbarButtons"
        @search="onSearch"
        @reset="onReset"
      />
    </template>
    <FgGrid
      :data="pageRows"
      :columns="roleColumns"
      row-key="roleId"
      :module-id="moduleId"
      :menu-items="roleMenuItems"
      @row-click="onRow"
      @row-dblclick="openView"
      @menu-click="onMenu"
    >
      <template #roleStatus="{ row }">
        <RsTag size="sm" :variant="(row as Role).roleStatus === 'Y' ? 'success' : 'default'">
          {{ flagLabel(statusOptions, String((row as Role).roleStatus ?? '')) }}
        </RsTag>
      </template>
      <template #builtInFlag="{ row }">
        <RsTag size="sm" :variant="(row as Role).builtInFlag === 'Y' ? 'info' : 'default'">
          {{ flagLabel(builtinOptions, String((row as Role).builtInFlag ?? '')) }}
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
    @submit="submitRole"
  />
  <RoleAuthDrawer
    v-if="pageReady"
    :module-id="moduleId"
    v-model:open="authOpen"
    v-model:keyword="keyword"
    v-model:checked-keys="checkedKeys"
    v-model:expanded-keys="expandedKeys"
    :title="authTitle"
    :tree-data="treeData"
    @save="save"
  />
  <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-role-host {
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
