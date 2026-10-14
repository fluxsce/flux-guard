<script setup lang="ts">
import { computed } from 'vue'
import { useButtonAllowed } from '@/components/buttonAccess'
import { RsTable, type RsContextMenuItem, type RsTableColumn, type RsTableTreeConfig } from '@/ui'
import type { FgGridColumn, FgGridMenuItem, FgGridSort } from './types'

const props = withDefaults(
  defineProps<{
    data: any[]
    columns: FgGridColumn[]
    rowKey?: string
    loading?: boolean
    treeConfig?: RsTableTreeConfig<any>
    menuItems?: FgGridMenuItem[]
    expandedRowKeys?: string[]
    /** 是否显示勾选列，支持多选（表头自带全选）。 */
    selectable?: boolean
    /** 受控勾选行键。与 selectable 一起用。 */
    selectedRowKeys?: string[]
    /** 按行决定能否勾选；目录等不可选行返回 false。 */
    rowSelectable?: (row: any, index: number) => boolean
    /** 表头拖动调整列宽 */
    resizable?: boolean
    /** 当前排序。传入后由外部排序，表格只显示表头状态。 */
    sort?: FgGridSort | null
    remoteSort?: boolean
    /** 模块号。右键项的 key 拼成「模块:键」后按菜单授权决定是否出现。 */
    moduleId?: string
  }>(),
  {
    rowKey: 'id',
    loading: false,
    menuItems: () => [],
    selectable: false,
    selectedRowKeys: () => [],
    resizable: true,
    remoteSort: false,
  },
)

const emit = defineEmits<{
  rowClick: [row: any]
  rowDblclick: [row: any]
  menuClick: [payload: { key: string; row: any }]
  'update:expandedRowKeys': [keys: string[]]
  'update:selectedRowKeys': [keys: string[]]
  selectionChange: [keys: string[]]
  'update:sort': [value: FgGridSort | null]
}>()

const tableColumns = computed<RsTableColumn[]>(() =>
  props.columns.map((column) => ({
    key: column.key,
    title: column.title,
    dataIndex: column.dataIndex ?? column.key,
    width: column.width,
    minWidth: column.minWidth,
    ellipsis: column.ellipsis,
    align: column.align,
    sortable: column.sortable,
  })),
)

const buttonAllowed = useButtonAllowed()

function allowed(item: FgGridMenuItem) {
  return buttonAllowed(props.moduleId, item.key, item.code)
}

const contextMenu = computed(() => props.menuItems.some(allowed))

function menuForRow(row: any): RsContextMenuItem[] {
  if (!row) return []
  return props.menuItems.filter(allowed).map((item) => ({
    key: item.key,
    label: item.label,
    icon: item.icon,
    danger: item.danger,
    disabled: typeof item.disabled === 'function' ? item.disabled(row) : item.disabled,
  }))
}

function onMenu(key: string, row: any) {
  emit('menuClick', { key, row })
}
</script>

<template>
  <div class="fg-grid">
    <RsTable
      class="fg-grid-table"
      :columns="tableColumns"
      :data="data"
      :loading="loading"
      :row-key="rowKey"
      :tree-config="treeConfig"
      :expanded-row-keys="expandedRowKeys"
      :selectable="selectable"
      :selected-row-keys="selectable ? selectedRowKeys : undefined"
      :row-selectable="rowSelectable"
      :context-menu="contextMenu"
      :context-menu-items="menuForRow"
      bordered
      column-bordered
      striped
      fill
      size="sm"
      compact
      :resizable="resizable"
      :sort="sort"
      :remote-sort="remoteSort"
      highlight-row
      highlight-row-on-click
      null-label="—"
      cell-focus="none"
      @row-click="(row) => emit('rowClick', row)"
      @row-dblclick="(row) => emit('rowDblclick', row)"
      @context-menu-select="onMenu"
      @update:expanded-row-keys="emit('update:expandedRowKeys', $event)"
      @update:selected-row-keys="emit('update:selectedRowKeys', $event)"
      @selection-change="emit('selectionChange', $event)"
      @update:sort="emit('update:sort', $event)"
    >
      <template v-for="(_, name) in $slots" #[name]="slotProps">
        <slot :name="name" v-bind="slotProps || {}" />
      </template>
    </RsTable>
  </div>
</template>

<style scoped>
.fg-grid {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.fg-grid-table {
  flex: 1;
  min-height: 0;
}
</style>
