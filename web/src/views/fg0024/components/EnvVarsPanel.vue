<script setup lang="ts">
import { computed, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import FgGrid from '@/components/grid/FgGrid.vue'
import {
  RsButton,
  RsDialog,
  RsForm,
  RsFormItem,
  RsInput,
  RsSelect,
  RsSwitch,
  rsConfirm,
  useRsToast,
} from '@/ui'
import { saveEnvVars, type EnvVarInput, type EnvVarItem, type EnvVarKind, type EnvVars } from '../api'

const props = defineProps<{
  vars: EnvVars
  editable: boolean
  moduleId: string
}>()

const emit = defineEmits<{
  saved: [vars: EnvVars]
  error: [message: string]
}>()

const toast = useRsToast()

const kindOptions = [
  { label: '文本', value: 'text' },
  { label: '数字', value: 'number' },
  { label: '布尔', value: 'bool' },
  { label: '密文', value: 'secret' },
]

const kindLabel: Record<EnvVarKind, string> = {
  text: '文本',
  number: '数字',
  bool: '布尔',
  secret: '密文',
}

const columns = [
  { key: 'name', title: '变量名', dataIndex: 'name', width: 180 },
  { key: 'kind', title: '类型', dataIndex: 'kind', width: 88 },
  { key: 'value', title: '值', dataIndex: 'value', ellipsis: true },
  { key: 'note', title: '备注', dataIndex: 'note', ellipsis: true },
  { key: 'actions', title: '操作', width: 140 },
]

const open = ref(false)
const editing = ref('')
const draft = ref({ name: '', kind: 'text' as EnvVarKind, value: '', note: '', boolValue: false })

const rows = computed(() => props.vars.items ?? [])

function openCreate() {
  editing.value = ''
  draft.value = { name: '', kind: 'text', value: '', note: '', boolValue: false }
  open.value = true
}

function openEdit(row: EnvVarItem) {
  editing.value = row.name
  draft.value = {
    name: row.name,
    kind: row.kind,
    value: row.kind === 'secret' ? '' : row.value,
    note: row.note,
    boolValue: row.value === 'true',
  }
  open.value = true
}

function displayValue(row: EnvVarItem) {
  if (row.kind === 'bool') return row.value === 'true' ? '是' : '否'
  if (!row.value) return '—'
  return row.value
}

function draftValue() {
  if (draft.value.kind === 'bool') return draft.value.boolValue ? 'true' : 'false'
  return draft.value.value
}

function buildItems(): EnvVarInput[] | string {
  const value = draftValue()
  const name = draft.value.name.trim()
  if (!name) return '请输入变量名'
  if (rows.value.some((item) => item.name === name && item.name !== editing.value)) return '变量名已存在'
  if (draft.value.kind === 'secret' && !editing.value && !value) return '请输入密文'
  if (draft.value.kind !== 'secret' && draft.value.kind !== 'bool' && !value.trim()) return '请输入变量值'
  const next: EnvVarInput[] = rows.value
    .filter((item) => item.name !== editing.value)
    .map((item) => ({
      name: item.name,
      kind: item.kind,
      value: item.value,
      note: item.note,
    }))
  next.push({
    name,
    originalName: editing.value || undefined,
    kind: draft.value.kind,
    value,
    note: draft.value.note.trim(),
  })
  return next
}

async function persist(items: EnvVarInput[]) {
  try {
    const result = await saveEnvVars(props.vars.currentVersion, items)
    if (!isApiSuccess(result)) {
      const message = getApiMessage(result, '保存失败')
      toast.error(message)
      emit('error', message)
      return false
    }
    const data = parseApiData<EnvVars>(result)
    if (data) emit('saved', data)
    return true
  } catch (err) {
    const message = getHttpErrorMessage(err, '保存失败')
    toast.error(message)
    emit('error', message)
    return false
  }
}

async function confirm() {
  const items = buildItems()
  if (typeof items === 'string') {
    toast.error(items)
    emit('error', items)
    return
  }
  if (await persist(items)) open.value = false
}

async function remove(row: EnvVarItem) {
  const ok = await rsConfirm.confirm({
    title: '删除环境变量',
    description: `确定删除变量 ${row.name}？引用它的配置将得到空值。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
    teleportTo: `#${props.moduleId}`,
  })
  if (!ok) return
  const items = rows.value
    .filter((item) => item.name !== row.name)
    .map((item) => ({ name: item.name, kind: item.kind, value: item.value, note: item.note }))
  await persist(items)
}
</script>

<template>
  <div class="fg-env-vars">
    <p class="fg-env-vars-hint">进程内全局可读，文本里用 ${NAME} 引用，缺省值写成 ${NAME:default}。</p>
    <div class="fg-env-vars-bar">
      <RsButton variant="primary" size="md" icon="plus" :disabled="!editable" @click="openCreate">新增变量</RsButton>
    </div>
    <FgGrid :data="rows" :columns="columns" row-key="name">
      <template #empty>暂无环境变量</template>
      <template #kind="{ row }">
        {{ kindLabel[(row as EnvVarItem).kind] || (row as EnvVarItem).kind }}
      </template>
      <template #value="{ row }">
        {{ displayValue(row as EnvVarItem) }}
      </template>
      <template #actions="{ row }">
        <RsButton v-if="editable" variant="text" size="sm" @click="openEdit(row as EnvVarItem)">编辑</RsButton>
        <RsButton v-if="editable" variant="text" size="sm" @click="remove(row as EnvVarItem)">删除</RsButton>
      </template>
    </FgGrid>

    <RsDialog
      v-model:open="open"
      :title="editing ? '编辑环境变量' : '新增环境变量'"
      width="480px"
      :modal="false"
      :teleport-to="`#${moduleId}`"
    >
      <RsForm :model="draft" label-position="left" label-align="end" label-width="5rem" size="md" class="fg-env-var-form">
        <RsFormItem name="name" label="变量名">
          <RsInput v-model="draft.name" placeholder="字母或下划线开头" size="md" label-position="top" />
        </RsFormItem>
        <RsFormItem name="kind" label="类型">
          <RsSelect v-model="draft.kind" :options="kindOptions" block match-trigger-width size="md" />
        </RsFormItem>
        <RsFormItem name="value" label="变量值">
          <RsSwitch v-if="draft.kind === 'bool'" v-model="draft.boolValue" size="md" />
          <RsInput
            v-else
            v-model="draft.value"
            :type="draft.kind === 'secret' ? 'password' : 'text'"
            :placeholder="draft.kind === 'secret' && editing ? '不修改请留空' : '请输入变量值'"
            size="md"
            label-position="top"
          />
        </RsFormItem>
        <RsFormItem name="note" label="备注">
          <RsInput v-model="draft.note" placeholder="可选说明" size="md" label-position="top" />
        </RsFormItem>
      </RsForm>
      <template #footer>
        <RsButton size="md" @click="open = false">取消</RsButton>
        <RsButton variant="primary" size="md" @click="confirm">保存</RsButton>
      </template>
    </RsDialog>
  </div>
</template>

<style scoped>
.fg-env-vars {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.fg-env-vars-hint {
  flex: none;
  margin: 0;
  color: var(--rs-muted, var(--rs-text-secondary));
  font-size: 13px;
}
.fg-env-vars-bar {
  flex: none;
  display: flex;
}
.fg-env-var-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
