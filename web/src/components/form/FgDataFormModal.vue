<script setup lang="ts">
import { computed, useSlots, useTemplateRef } from 'vue'
import { useButtonAllowed } from '@/components/buttonAccess'
import { RsButton, RsDialog } from '@/ui'
import FgDataForm from './FgDataForm.vue'
import type { FgFormField, FgFormMode } from './types'

const open = defineModel<boolean>('open', { default: false })

const props = withDefaults(
  defineProps<{
    title?: string
    mode?: FgFormMode
    fields: FgFormField[]
    initialData?: Record<string, unknown>
    /** 点保存后是否立刻关闭。异步保存失败时由调用方保持打开。 */
    autoClose?: boolean
    /** 挂到页面根节点，如 #fg0022。不传则沿用对话框默认目标。 */
    teleportTo?: string
    /** 模块号。保存与工具条同一套判断：新增认 add，修改认 edit。 */
    moduleId?: string
    /** 保存按钮的动作键。不写时新增认 add，修改认 edit。 */
    saveAction?: string
    /** 对话框宽度。 */
    width?: string
  }>(),
  {
    title: '',
    mode: 'view',
    initialData: () => ({}),
    autoClose: true,
    teleportTo: undefined,
    width: '640px',
  },
)

const emit = defineEmits<{
  submit: [values: Record<string, unknown>]
}>()

const allowed = useButtonAllowed()
const slots = useSlots()
const forwardNames = computed(() => Object.keys(slots).filter((name) => name !== 'footer'))
const formRef = useTemplateRef('formRef')
const dialogTitle = computed(() => props.title || (props.mode === 'view' ? '查看' : '编辑'))
const canSave = computed(() => {
  if (props.mode === 'view') return false
  const action = props.saveAction || (props.mode === 'create' ? 'add' : 'edit')
  return allowed(props.moduleId, action)
})

function close() {
  open.value = false
}

function confirm() {
  emit('submit', formRef.value?.getValues() ?? {})
  if (props.autoClose) open.value = false
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    :title="dialogTitle"
    :width="width"
    :modal="false"
    :teleport-to="teleportTo"
  >
    <FgDataForm
      ref="formRef"
      :fields="fields"
      :mode="mode"
      :module-id="moduleId"
      :initial-data="initialData"
    >
      <template v-for="name in forwardNames" :key="name" #[name]="scope">
        <slot :name="name" v-bind="scope || {}" />
      </template>
    </FgDataForm>
    <slot v-if="formRef?.model" name="extra" :model="formRef.model" />
    <template #footer>
      <RsButton size="sm" @click="close">{{ mode === 'view' ? '关闭' : '取消' }}</RsButton>
      <RsButton v-if="canSave" variant="primary" size="sm" @click="confirm">保存</RsButton>
    </template>
  </RsDialog>
</template>
