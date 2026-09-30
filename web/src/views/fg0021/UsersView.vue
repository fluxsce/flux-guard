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
import { useSessionStore } from '@/stores/session'
import UserRoleDrawer from './components/UserRoleDrawer.vue'
import {
  flagLabel,
  genderOptions,
  statusOptions,
  useUserCatalog,
  userColumns,
  userFormFields,
  userMenuItems,
  userSearchFields,
  yesNoOptions,
} from './composables/useUserCatalog'
import { useUserAuth } from './composables/useUserAuth'
import type { User } from './api'

const session = useSessionStore()
const selfUserId = computed(() => session.user?.userId ?? '')
const moduleId = 'fg0021'
const pageReady = ref(false)
onMounted(() => {
  pageReady.value = true
})

const {
  page,
  pageSize,
  total,
  error,
  formOpen,
  formMode,
  current,
  active,
  pageRows,
  placeUser,
  openView,
  openCreate,
  openEdit,
  submitUser,
  removeUser,
  onRow,
  onSearch,
  onReset,
} = useUserCatalog()

const {
  authOpen,
  treeData,
  checkedKeys,
  expandedKeys,
  keyword,
  authTitle,
  openAuth,
  save,
} = useUserAuth(active, error, placeUser)

const formFields = userFormFields
const formTitle = computed(() => {
  if (formMode.value === 'create') return '新增用户'
  if (formMode.value === 'edit') return '修改用户'
  return '用户'
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
    disabled: !active.value || active.value.userId === selfUserId.value || active.value.tenantAdminFlag === 'Y',
    onClick: () => {
      if (active.value) void removeUser(active.value, selfUserId.value)
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
    title: '用户管理',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      selectedUserId: active.value?.userId ?? '',
      selectedUserName: active.value?.userName ?? '',
    }),
  }),
  () => [
    {
      name: 'fg0021.applySearch',
      execute: async (args) => {
        onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

function onMenu(payload: { key: string; row: User | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
  if (payload.key === 'edit') openEdit(payload.row)
  if (payload.key === 'delete') void removeUser(payload.row, selfUserId.value)
  if (payload.key === 'grant') void openAuth(payload.row)
}
</script>

<template>
  <div class="fg-page fg-user-host" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog>
      <template #search>
        <FgSearchForm
          plain
          :module-id="moduleId"
          :fields="userSearchFields"
          :toolbar-buttons="toolbarButtons"
          @search="onSearch"
          @reset="onReset"
        />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="userColumns"
        row-key="userId"
        :module-id="moduleId"
        :menu-items="userMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #gender="{ row }">
          {{ flagLabel(genderOptions, String((row as User).gender ?? 0)) }}
        </template>
        <template #statusFlag="{ row }">
          <RsTag size="sm" :variant="(row as User).statusFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(statusOptions, String((row as User).statusFlag ?? '')) }}
          </RsTag>
        </template>
        <template #tenantAdminFlag="{ row }">
          {{ flagLabel(yesNoOptions, String((row as User).tenantAdminFlag ?? '')) }}
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
      @submit="submitUser"
    />
    <UserRoleDrawer
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
.fg-user-host {
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
