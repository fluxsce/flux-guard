<script setup lang="ts">
import { computed, useSlots } from 'vue'
import {
  RsDatePicker,
  RsFieldset,
  RsFormItem,
  RsInput,
  RsInputNumber,
  RsSelect,
  RsTextarea,
} from '@/ui'
import type { FgFormField, FgFormMode } from './types'

const props = defineProps<{
  fields: FgFormField[]
  formModel: Record<string, unknown>
  mode: FgFormMode
}>()

function visible(field: FgFormField) {
  if (field.modes && !field.modes.includes(props.mode)) return false
  if (field.show == null) return true
  if (typeof field.show === 'function') return !!field.show(props.formModel)
  return field.show !== false
}

function locked(field: FgFormField) {
  if (props.mode === 'view') return true
  if (field.disabled) return true
  return props.mode === 'edit' && field.primary === true
}

function textValue(name: string) {
  const value = props.formModel[name]
  return value == null ? '' : String(value)
}

function numberValue(name: string) {
  const value = props.formModel[name]
  if (value === '' || value == null) return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

function setValue(name: string, value: unknown) {
  props.formModel[name] = value
}

const slots = useSlots()
const slotNames = computed(() => Object.keys(slots))

defineSlots<{
  [name: string]: (props: { formModel: Record<string, unknown>; disabled: boolean }) => unknown
}>()
</script>

<template>
  <div class="fg-form-grid">
    <template v-for="field in fields" :key="field.field">
      <div
        v-if="field.type === 'fieldset' && visible(field)"
        class="fg-form-cell fg-form-cell--fieldset"
        style="grid-column: span 24"
      >
        <RsFieldset
          class="fg-form-fieldset"
          :legend="field.label"
          :tooltip="field.tips || undefined"
          size="sm"
          border-style="dashed"
          border-tone="faded"
          title-tone="muted"
          title-weight="regular"
          title-size="xs"
        >
          <FgFormFields :fields="field.children ?? []" :form-model="formModel" :mode="mode">
            <template v-for="name in slotNames" :key="name" #[name]="scope">
              <slot :name="name" v-bind="scope || {}" />
            </template>
          </FgFormFields>
        </RsFieldset>
      </div>
      <div v-else-if="field.type === 'slot' && visible(field)" class="fg-form-cell" style="grid-column: span 24">
        <slot :name="field.field" :form-model="formModel" :disabled="locked(field)" />
      </div>
      <RsFormItem
        v-else-if="visible(field)"
        class="fg-form-cell"
        :style="{ gridColumn: `span ${field.span ?? 12}` }"
        :name="field.field"
        :label="field.label"
        :tooltip="field.tips || undefined"
        :required="mode !== 'view' && !!field.required"
      >
        <RsTextarea
          v-if="field.type === 'textarea'"
          :model-value="textValue(field.field)"
          :placeholder="field.placeholder"
          :disabled="locked(field)"
          @update:model-value="setValue(field.field, $event)"
        />
        <RsInputNumber
          v-else-if="field.type === 'number'"
          :model-value="numberValue(field.field)"
          :disabled="locked(field)"
          @update:model-value="setValue(field.field, $event)"
        />
        <RsSelect
          v-else-if="field.type === 'select'"
          :model-value="textValue(field.field)"
          :options="field.options ?? []"
          :placeholder="field.placeholder"
          :disabled="locked(field)"
          @update:model-value="setValue(field.field, $event)"
        />
        <RsDatePicker
          v-else-if="field.type === 'date' || field.type === 'datetime'"
          :model-value="textValue(field.field)"
          :placeholder="field.placeholder"
          :disabled="locked(field)"
          :with-time="field.type === 'datetime'"
          :with-seconds="field.type === 'datetime'"
          value-format="string"
          size="sm"
          label-position="top"
          @update:model-value="setValue(field.field, $event)"
        />
        <RsInput
          v-else
          :model-value="textValue(field.field)"
          :type="field.type === 'password' ? 'password' : 'text'"
          :placeholder="field.placeholder"
          :disabled="locked(field)"
          clearable
          @update:model-value="setValue(field.field, $event)"
        />
      </RsFormItem>
    </template>
  </div>
</template>

<style scoped>
.fg-form-grid {
  display: grid;
  grid-template-columns: repeat(24, minmax(0, 1fr));
  gap: 4px 16px;
}
.fg-form-cell--fieldset {
  min-width: 0;
}
.fg-form-fieldset {
  margin: 0;
}
</style>
