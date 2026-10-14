<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  RsButton,
  RsDialog,
  RsForm,
  RsFormItem,
  RsInput,
  RsInputNumber,
  RsUpload,
  useRsToast,
  type RsUploadValidationError,
} from '@/ui'
import { kindLabel, moduleId } from '../composables/useAppWorkbench'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  pickedCount: number
  releaseAppCode: string
  releaseKind: string
  version: string
  canaryCount: number
  canSubmit: boolean
}>()

const emit = defineEmits<{
  'update:version': [value: string]
  'update:canaryCount': [value: number]
  'pick-file': [file: File | null]
  submit: []
}>()

const toast = useRsToast()
const files = ref<File[]>([])

const formModel = computed(() => ({
  version: props.version,
  canaryCount: props.canaryCount,
  file: files.value,
}))

const ready = computed(() => props.canSubmit && files.value.length > 0)

watch(open, (value) => {
  if (!value) {
    files.value = []
    emit('pick-file', null)
  }
})

watch(files, (list) => {
  emit('pick-file', list[0] ?? null)
})

function onReject(errors: RsUploadValidationError[]) {
  const first = errors[0]
  if (!first) return
  if (first.reason === 'maxCount') {
    toast.error('一次只能选一个更新包')
    return
  }
  if (first.reason === 'maxSize') {
    toast.error(`文件过大：${first.file.name}`)
    return
  }
  toast.error(`不支持的文件：${first.file.name}`)
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    title="上传更新"
    layout="form"
    width="480px"
    :modal="false"
    :teleport-to="`#${moduleId}`"
  >
    <p class="hint">
      已选 {{ pickedCount }} 个文件
      <template v-if="releaseAppCode"> · 应用 {{ releaseAppCode }}</template>
      <template v-if="releaseKind"> · {{ kindLabel(releaseKind) }}</template>
      。一次须同一应用；files / 日志只换文件或下载，不重启进程。
    </p>
    <RsForm :model="formModel" label-position="top" size="sm">
      <RsFormItem name="version" label="目标版本" required>
        <RsInput
          :model-value="version"
          placeholder="例如 1.2.0"
          clearable
          @update:model-value="emit('update:version', String($event ?? ''))"
        />
      </RsFormItem>
      <RsFormItem
        name="canaryCount"
        label="金丝雀台数"
        tooltip="先只更新少数实例（默认 1 台），确认无问题后再在操作日志里点「继续下一批」。不是自动全量发布。"
      >
        <RsInputNumber
          :model-value="canaryCount"
          :min="1"
          :max="32"
          @update:model-value="emit('update:canaryCount', Number($event) || 1)"
        />
      </RsFormItem>
      <RsFormItem name="file" label="本地文件" required>
        <RsUpload
          v-model="files"
          name="file"
          variant="dropzone"
          size="sm"
          :max-count="1"
          replace
          label="选择或拖入更新包"
          hint="一次一个文件"
          @reject="onReject"
        />
      </RsFormItem>
    </RsForm>
    <template #footer>
      <RsButton size="sm" variant="ghost" @click="open = false">取消</RsButton>
      <RsButton size="sm" variant="primary" :disabled="!ready" @click="emit('submit')">上传更新</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.hint {
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--rs-text-secondary);
  line-height: 1.5;
}
</style>
