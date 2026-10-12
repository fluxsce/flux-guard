<script setup lang="ts">
import { computed } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsButton, RsEmpty, RsMenu, RsPagination, RsTag, type RsMenuItem } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import ComponentDetail from './components/ComponentDetail.vue'
import type { ComponentConn } from './api'
import { listText } from './composables/boards'
import {
  connColumnsFor,
  connMenuItems,
  flagLabel,
  statusOptions,
  upOptions,
  useComponentCatalog,
} from './composables/useComponentCatalog'
import { layoutOf } from './composables/forms'
import { kindIcon } from './composables/panels'

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
  kinds,
  kindCode,
  visibleKinds,
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
const kindItems = computed<RsMenuItem[]>(() =>
  visibleKinds.value.map((item) => ({ key: item.kindCode, label: item.kindName, icon: kindIcon(item.kindCode) })),
)
const currentKind = computed(() => kinds.value.find((item) => item.kindCode === kindCode.value) ?? null)
const headlineCols = computed(() => {
  const metrics = new Map((currentKind.value?.metrics ?? []).map((item) => [item.code, item]))
  const cols: { key: string; title: string; code: string; unit: string; width: number; align: 'right' | 'center' }[] = []
  for (const panel of currentKind.value?.panels ?? []) {
    if (panel.listFlag !== 'Y') continue
    for (const code of panel.metrics ?? []) {
      const metric = metrics.get(code)
      const text = code === 'aofOk'
      cols.push({
        key: `metric_${code}`,
        title: metric?.name || code,
        code,
        unit: metric?.unit || '',
        width: text ? 88 : 112,
        align: text ? 'center' : 'right',
      })
    }
  }
  return cols
})
const columns = computed(() =>
  connColumnsFor(headlineCols.value.map((col) => ({ key: col.key, title: col.title, width: col.width, align: col.align }))),
)
const gridRows = computed(() =>
  pageRows.value.map((row) => {
    const next: Record<string, unknown> = { ...row }
    for (const col of headlineCols.value) {
      const raw = row.headlines?.[col.code]
      next[col.key] = listText(col.code, col.unit, typeof raw === 'number' ? raw : null)
    }
    return next
  }),
)

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
      kind: kindCode.value,
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
    <aside v-show="!viewing" class="fg-kind-nav">
      <div class="fg-kind-title">组件</div>
      <RsMenu v-if="kindItems.length" v-model="kindCode" :items="kindItems" class="fg-kind-menu" aria-label="组件种类" />
      <p v-else class="fg-kind-empty">没有可查看的种类</p>
    </aside>
    <div class="fg-kind-main">
    <FgCatalog v-if="!viewing && kindCode">
      <template #search>
        <FgSearchForm plain :module-id="moduleId" :fields="searchFields" :toolbar-buttons="toolbarButtons" @search="onSearch" @reset="onReset" />
      </template>
      <FgGrid
        :data="gridRows"
        :columns="columns"
        row-key="componentId"
        :module-id="moduleId"
        :menu-items="connMenuItems"
        @row-click="onRow"
        @row-dblclick="show"
        @menu-click="onMenu"
      >
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
      v-else-if="viewing && active"
      :conn="active"
      :panels="currentKind?.panels ?? []"
      @back="back"
      @refresh="refreshDetail"
    />
    <RsEmpty v-else class="fg-kind-blank" title="选择左侧的组件种类" description="每种组件单独列出实例，这一类的当前值直接出现在表上。" />
    </div>
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
  flex-direction: row;
}
.fg-kind-nav {
  flex: 0 0 11rem;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px 8px;
  background: var(--rs-surface);
  border-right: 1px solid var(--rs-border);
}
.fg-kind-title {
  flex: none;
  padding: 0 8px 12px;
  color: var(--rs-text-secondary);
  font-size: 12px;
  font-weight: 600;
}
.fg-kind-menu {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.fg-kind-empty,
.fg-kind-blank {
  margin: 0;
  padding: 8px;
  color: var(--rs-text-secondary);
  font-size: 13px;
}
.fg-kind-main {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.fg-kind-blank {
  margin: auto;
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
