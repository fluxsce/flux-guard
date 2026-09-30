<script setup lang="ts">
import { computed, reactive, ref, useSlots, watch } from 'vue'
import { RsForm, RsTabs } from '@/ui'
import type { RsTabItem } from '@/ui'
import FgFormFields from './FgFormFields.vue'
import type { FgFormField, FgFormMode } from './types'
import { walkLeafFields } from './types'

const props = withDefaults(
  defineProps<{
    fields: FgFormField[]
    mode?: FgFormMode
    initialData?: Record<string, unknown>
    /** 模块号。随表单传给页面，保存按钮的权限在外层对话框判断。 */
    moduleId?: string
  }>(),
  {
    mode: 'view',
    initialData: () => ({}),
  },
)

const formModel = reactive<Record<string, unknown>>({})
const slots = useSlots()
const slotNames = computed(() => Object.keys(slots))

function fieldVisible(field: FgFormField) {
  if (field.modes && !field.modes.includes(props.mode)) return false
  if (field.show == null) return true
  if (typeof field.show === 'function') return !!field.show(formModel)
  return field.show !== false
}

/** 页签收集：fieldset 用自身 tab；叶子也看 tab。隐藏的 fieldset 不占页签。 */
function collectTabFields(fields: FgFormField[], out: FgFormField[] = []): FgFormField[] {
  for (const field of fields) {
    if (!fieldVisible(field)) continue
    if (field.type === 'fieldset') {
      out.push(field)
      continue
    }
    out.push(field)
  }
  return out
}

const formTab = ref('')
const tabItems = computed<RsTabItem[]>(() => {
  const names: string[] = []
  for (const field of collectTabFields(props.fields)) {
    if (field.tab && !names.includes(field.tab)) names.push(field.tab)
  }
  return names.map((name) => ({ value: name, label: name }))
})

const shownFields = computed(() => {
  const visible = props.fields.filter((field) => fieldVisible(field))
  if (tabItems.value.length < 2) return visible
  const name = formTab.value || tabItems.value[0]?.value
  return visible.filter((field) => (field.tab || name) === name)
})

watch(
  () => props.initialData,
  (data) => {
    for (const key of Object.keys(formModel)) delete formModel[key]
    walkLeafFields(props.fields, (field) => {
      const value = data?.[field.field]
      formModel[field.field] = value ?? ''
    })
    formTab.value = tabItems.value[0]?.value ?? ''
  },
  { immediate: true },
)

watch(tabItems, (items) => {
  if (!items.some((item) => item.value === formTab.value)) {
    formTab.value = items[0]?.value ?? ''
  }
})

defineExpose({
  getValues: () => ({ ...formModel }),
  model: formModel,
})
</script>

<template>
  <RsForm :model="formModel" :disabled="mode === 'view'" label-position="top" size="sm" :data-fg-module="moduleId || undefined">
    <RsTabs v-if="tabItems.length > 1" v-model="formTab" class="fg-form-tabs" :items="tabItems" size="sm" panelless />
    <FgFormFields :fields="shownFields" :form-model="formModel" :mode="mode">
      <template v-for="name in slotNames" :key="name" #[name]="scope">
        <slot :name="name" v-bind="scope || {}" />
      </template>
    </FgFormFields>
  </RsForm>
</template>

<style scoped>
.fg-form-tabs {
  margin-bottom: 12px;
}
</style>
