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
import { inferArtifactFromFile, kindLabel, moduleId } from '../composables/useAppWorkbench'

export type UploadMode = 'update' | 'create'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  mode: UploadMode
  /** 新增时相对站点根的子目录（可空）。 */
  cwd?: string
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
  create: [payload: { appCode: string; kind: string }]
}>()

const toast = useRsToast()
const files = ref<File[]>([])
const createAppCode = ref('')
const createKind = ref('')

const isCreate = computed(() => props.mode === 'create')

const formModel = computed(() => ({
  version: props.version,
  canaryCount: props.canaryCount,
  file: files.value,
  appCode: createAppCode.value,
  kind: createKind.value,
}))

const ready = computed(() => {
  if (!props.canSubmit || files.value.length === 0 || !props.version.trim()) return false
  if (isCreate.value) {
    return !!createAppCode.value.trim() && !!createKind.value
  }
  return true
})

const title = computed(() => (isCreate.value ? '上传新增' : '上传更新'))
const submitLabel = computed(() => (isCreate.value ? '上传新增' : '上传更新'))

watch(open, (value) => {
  if (!value) {
    files.value = []
    createAppCode.value = ''
    createKind.value = ''
    emit('pick-file', null)
  }
})

watch(files, (list) => {
  const file = list[0] ?? null
  emit('pick-file', file)
  if (!isCreate.value || !file) return
  const inferred = inferArtifactFromFile(file.name, props.cwd || '')
  createAppCode.value = inferred.appCode
  createKind.value = inferred.kind
  if (inferred.version && !props.version.trim()) {
    emit('update:version', inferred.version)
  }
})

function onReject(errors: RsUploadValidationError[]) {
  const first = errors[0]
  if (!first) return
  if (first.reason === 'maxCount') {
    toast.error('一次只能选一个包')
    return
  }
  if (first.reason === 'maxSize') {
    toast.error(`文件过大：${first.file.name}`)
    return
  }
  toast.error(`不支持的文件：${first.file.name}`)
}

function onSubmit() {
  if (!ready.value) return
  if (isCreate.value) {
    emit('create', { appCode: createAppCode.value.trim(), kind: createKind.value })
    return
  }
  emit('submit')
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    :title="title"
    layout="form"
    width="480px"
    :modal="false"
    :teleport-to="`#${moduleId}`"
  >
    <p v-if="isCreate" class="hint">
      在当前运行时目录新增制品，不必勾选已有文件。
      <template v-if="cwd">当前目录 {{ cwd }}。</template>
      应用编号与种类可按文件名自动填入，也可改。
    </p>
    <p v-else class="hint">
      已选 {{ pickedCount }} 个文件
      <template v-if="releaseAppCode"> · 应用 {{ releaseAppCode }}</template>
      <template v-if="releaseKind"> · {{ kindLabel(releaseKind) }}</template>
      。一次须同一应用；files / 日志只换文件或下载，不重启进程。
    </p>
    <RsForm :model="formModel" label-position="top" size="sm">
      <RsFormItem v-if="isCreate" name="appCode" label="应用编号" required>
        <RsInput v-model="createAppCode" placeholder="例如 order 或 sub/order" clearable />
      </RsFormItem>
      <RsFormItem v-if="isCreate" name="kind" label="制品种类" required>
        <RsInput v-model="createKind" placeholder="war / jar / static" clearable />
      </RsFormItem>
      <RsFormItem name="version" label="目标版本" required>
        <RsInput
          :model-value="version"
          placeholder="例如 1.2.0"
          clearable
          @update:model-value="emit('update:version', String($event ?? ''))"
        />
      </RsFormItem>
      <RsFormItem
        v-if="!isCreate"
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
          :label="isCreate ? '选择或拖入新制品' : '选择或拖入更新包'"
          hint="一次一个文件"
          @reject="onReject"
        />
      </RsFormItem>
    </RsForm>
    <template #footer>
      <RsButton size="sm" variant="ghost" @click="open = false">取消</RsButton>
      <RsButton size="sm" variant="primary" :disabled="!ready" @click="onSubmit">{{ submitLabel }}</RsButton>
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
