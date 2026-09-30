<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import {
  RsDatePicker,
  RsForm,
  RsFormItem,
  RsInput,
  RsInputNumber,
  RsSelect,
  RsSwitch,
} from '@/ui'
import type { FgSearchField, FgSearchFieldType } from './types'

const props = withDefaults(
  defineProps<{
    fields: FgSearchField[]
    moreFields?: FgSearchField[]
    labelWidth?: string
    cols?: number
    searchText?: string
    resetText?: string
    moreText?: string
    /** 页面自带工具条时关掉内置查询条 */
    showToolbar?: boolean
    /** 嵌进贴边页面时去掉卡片边框 */
    plain?: boolean
    /** 业务按钮，排在查询、重置前面 */
    toolbarButtons?: FgToolbarButton[]
    /** 模块号。查询按钮按「模块:search」判断，并传给内嵌工具条。 */
    moduleId?: string
  }>(),
  {
    moreFields: () => [],
    labelWidth: '7rem',
    cols: 24,
    searchText: '查询',
    resetText: '重置',
    moreText: '更多条件',
    showToolbar: true,
    plain: false,
    toolbarButtons: () => [],
    moduleId: '',
  },
)

const emit = defineEmits<{
  search: [values: Record<string, unknown>]
  reset: []
}>()

const formData = reactive<Record<string, unknown>>({})
const showMore = ref(false)

const dateTypes = new Set<FgSearchFieldType>(['date', 'daterange', 'datetime', 'datetimerange'])

function isDate(type?: FgSearchFieldType) {
  return type != null && dateTypes.has(type)
}

function isDateRange(type?: FgSearchFieldType) {
  return type === 'daterange' || type === 'datetimerange'
}

function isDateTime(type?: FgSearchFieldType) {
  return type === 'datetime' || type === 'datetimerange'
}

function blank(field: FgSearchField): unknown {
  if (field.type === 'number') return null
  if (field.type === 'switch') return false
  if (isDateRange(field.type)) return { start: '', end: '' }
  return ''
}

function resetValues() {
  for (const key of Object.keys(formData)) delete formData[key]
  for (const field of [...props.fields, ...props.moreFields]) {
    formData[field.field] = blank(field)
  }
}

resetValues()

const visibleFields = computed(() => {
  if (!showMore.value || props.moreFields.length === 0) return props.fields
  return [...props.fields, ...props.moreFields]
})

const hasMore = computed(() => props.moreFields.length > 0)
const defaultSpan = computed(() => Math.floor(props.cols / 4))

const gridStyle = computed(() => ({
  gridTemplateColumns: `repeat(${props.cols}, minmax(0, 1fr))`,
  '--rs-field-label-width': props.labelWidth,
}))

function textValue(field: string) {
  const value = formData[field]
  return value == null ? '' : String(value)
}

function numberValue(field: string) {
  const value = formData[field]
  if (value === '' || value == null) return null
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

function dateValue(field: string): string | { start: string; end: string } {
  const value = formData[field]
  if (value && typeof value === 'object' && 'start' in value) {
    const range = value as { start?: unknown; end?: unknown }
    return { start: String(range.start ?? ''), end: String(range.end ?? '') }
  }
  return value == null ? '' : String(value)
}

function switchValue(field: string) {
  return formData[field] === true
}

function setField(field: string, value: unknown) {
  formData[field] = value
}

function snapshot() {
  return { ...formData }
}

function onSearch() {
  emit('search', snapshot())
}

function onReset() {
  resetValues()
  showMore.value = false
  emit('reset')
}

function toggleMore() {
  showMore.value = !showMore.value
}

const actionButtons = computed<FgToolbarButton[]>(() => {
  const buttons: FgToolbarButton[] = [...props.toolbarButtons]
  const hasSearch = buttons.some((button) => button.key === 'search')
  const hasReset = buttons.some((button) => button.key === 'reset')
  if (!hasSearch) {
    buttons.push({
      key: 'search',
      label: props.searchText,
      icon: 'search',
      type: 'primary',
      onClick: onSearch,
    })
  }
  if (!hasReset) {
    buttons.push({
      key: 'reset',
      label: props.resetText,
      icon: 'rotate-ccw',
      onClick: onReset,
    })
  }
  if (hasMore.value && !buttons.some((button) => button.key === 'more')) {
    buttons.push({
      key: 'more',
      label: showMore.value ? '收起条件' : props.moreText,
      icon: 'sliders-horizontal',
      onClick: toggleMore,
    })
  }
  return buttons
})

defineExpose({
  getFormData: snapshot,
  resetForm: onReset,
  toggleMore,
})
</script>

<template>
  <section class="fg-search" :class="{ 'fg-search--plain': plain }">
    <FgToolbar v-if="showToolbar" class="fg-search-bar" border="bottom" plain size="sm" :module-id="moduleId" :buttons="actionButtons">
      <template v-if="$slots.extra" #left>
        <slot name="extra" />
      </template>
    </FgToolbar>
    <RsForm
      :model="formData"
      label-position="left"
      label-align="end"
      :label-width="labelWidth"
      size="md"
      gap="sm"
      max-width="full"
      class="fg-search-form"
      @submit="onSearch"
    >
      <div class="fg-search-grid" :style="gridStyle">
        <div
          v-for="field in visibleFields"
          :key="field.field"
          class="fg-search-cell"
          :style="{ gridColumn: `span ${field.span || defaultSpan}` }"
        >
          <RsFormItem :name="field.field" :label="field.label" label-position="left" class="fg-search-item">
            <RsInputNumber
              v-if="field.type === 'number'"
              :model-value="numberValue(field.field)"
              :placeholder="field.placeholder || `请输入${field.label}`"
              :disabled="field.disabled"
              size="md"
              label-position="top"
              @update:model-value="setField(field.field, $event)"
            />
            <RsDatePicker
              v-else-if="isDate(field.type)"
              :model-value="dateValue(field.field)"
              :placeholder="field.placeholder || `请选择${field.label}`"
              :disabled="field.disabled"
              :range="isDateRange(field.type)"
              :with-time="isDateTime(field.type)"
              :with-seconds="isDateTime(field.type)"
              value-format="string"
              size="md"
              label-position="top"
              @update:model-value="setField(field.field, $event)"
            />
            <RsSelect
              v-else-if="field.type === 'select'"
              :model-value="textValue(field.field)"
              :options="field.options ?? []"
              :placeholder="field.placeholder || `请选择${field.label}`"
              :disabled="field.disabled"
              :clearable="field.clearable !== false"
              size="md"
              match-trigger-width
              block
              @update:model-value="setField(field.field, $event)"
            />
            <RsSwitch
              v-else-if="field.type === 'switch'"
              :model-value="switchValue(field.field)"
              :disabled="field.disabled"
              size="md"
              @update:model-value="setField(field.field, $event)"
            />
            <RsInput
              v-else
              :model-value="textValue(field.field)"
              :placeholder="field.placeholder || `请输入${field.label}`"
              :disabled="field.disabled"
              :clearable="field.clearable !== false"
              size="md"
              label-position="top"
              @update:model-value="setField(field.field, $event)"
            />
          </RsFormItem>
        </div>
      </div>
    </RsForm>
  </section>
</template>

<style scoped>
.fg-search {
  width: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  background: var(--fg-panel-bg, var(--rs-surface));
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  border-radius: var(--rs-radius, 8px);
  box-shadow: var(--fg-panel-shadow, none);
}
.fg-search--plain {
  background: transparent;
  border: none;
  border-radius: 0;
  box-shadow: none;
}
.fg-search :deep(.fg-search-bar) {
  --rs-toolbar-height: 36px;
  min-height: 36px;
  height: 36px;
  padding-block: 0;
}
.fg-search-form {
  width: 100%;
}
.fg-search-grid {
  width: 100%;
  display: grid;
  align-items: start;
  box-sizing: border-box;
  column-gap: 12px;
  row-gap: 10px;
  padding: 8px 12px;
}
.fg-search-cell {
  min-width: 0;
}
.fg-search-item {
  width: 100%;
  min-width: 0;
}
</style>
