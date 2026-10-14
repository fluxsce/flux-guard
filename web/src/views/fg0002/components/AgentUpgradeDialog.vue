<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsDialog, useRsToast } from '@/ui'
import {
  continueAgentRelease,
  queryAgentRelease,
  upgradeAgents,
  type Agent,
  type AgentRelease,
} from '../api'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  agents: Agent[]
  moduleId: string
}>()

const toast = useRsToast()
const agentCode = ref('host')
const version = ref('')
const goos = ref('linux')
const goarch = ref('amd64')
const canaryCount = ref(1)
const file = ref<File | null>(null)
const picked = ref<string[]>([])
const busy = ref(false)
const releasing = ref<AgentRelease | null>(null)

const codeOptions = [
  { value: 'host', label: '主机代理' },
  { value: 'go', label: 'Go 运行时' },
  { value: 'appside', label: '应用代理' },
]

const osOptions = [
  { value: 'linux', label: 'linux' },
  { value: 'windows', label: 'windows' },
  { value: 'darwin', label: 'darwin' },
]

const archOptions = [
  { value: 'amd64', label: 'amd64' },
  { value: 'arm64', label: 'arm64' },
]

const candidates = computed(() =>
  props.agents.filter((row) => {
    if (row.agentCode !== agentCode.value || row.onlineFlag !== 'Y' || row.statusFlag !== 'Y') return false
    if (!row.goos || !row.goarch) return false
    return row.goos === goos.value && row.goarch === goarch.value
  }),
)

const skippedPlatform = computed(
  () =>
    props.agents.filter(
      (row) =>
        row.agentCode === agentCode.value &&
        row.onlineFlag === 'Y' &&
        row.statusFlag === 'Y' &&
        (!row.goos || !row.goarch || row.goos !== goos.value || row.goarch !== goarch.value),
    ).length,
)

watch([agentCode, goos, goarch], () => {
  picked.value = []
})

watch(open, (value) => {
  if (!value) return
  version.value = ''
  file.value = null
  picked.value = []
  releasing.value = null
  canaryCount.value = 1
})

function onFile(ev: Event) {
  const input = ev.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
}

function toggle(id: string) {
  if (picked.value.includes(id)) {
    picked.value = picked.value.filter((item) => item !== id)
    return
  }
  picked.value = [...picked.value, id]
}

function selectAll() {
  picked.value = candidates.value.map((row) => row.agentId)
}

async function submit() {
  if (!file.value) {
    toast.error('请选择发行包里的代理 tar.gz')
    return
  }
  if (!version.value.trim()) {
    toast.error('请填写目标版本')
    return
  }
  if (picked.value.length === 0) {
    toast.error('请选择要升级的代理')
    return
  }
  const form = new FormData()
  form.set('agentCode', agentCode.value)
  form.set('version', version.value.trim())
  form.set('goos', goos.value)
  form.set('goarch', goarch.value)
  form.set('canaryCount', String(canaryCount.value || 1))
  form.set('agentIds', picked.value.join(','))
  form.set('file', file.value)
  busy.value = true
  try {
    const res = await upgradeAgents(form)
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '升级下发失败'))
      return
    }
    const next = parseApiData<AgentRelease>(res)
    if (!next) return
    releasing.value = next
    const pending = next.pendingCount ?? 0
    toast.success(pending > 0 ? `已下发首批 ${next.items?.length ?? 0} 台，待继续 ${pending} 台` : `已下发 ${next.items?.length ?? 0} 台`)
    void poll()
  } finally {
    busy.value = false
  }
}

async function continueBatch() {
  const id = releasing.value?.releaseId
  if (!id) return
  busy.value = true
  try {
    const res = await continueAgentRelease(id)
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '继续下发失败'))
      return
    }
    const next = parseApiData<AgentRelease>(res)
    if (!next) return
    releasing.value = next
    toast.success(`已继续下发，剩余 ${next.pendingCount ?? 0} 台`)
    void poll()
  } finally {
    busy.value = false
  }
}

async function poll() {
  const id = releasing.value?.releaseId
  if (!id) return
  const res = await queryAgentRelease(id)
  if (!isApiSuccess(res)) return
  const next = parseApiData<AgentRelease>(res)
  if (!next) return
  releasing.value = next
  const pending = (next.items ?? []).some(
    (item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running',
  )
  if (pending) window.setTimeout(() => void poll(), 2000)
}

function statusText(status: string) {
  if (status === 'done') return '完成'
  if (status === 'failed') return '失败'
  if (status === 'waiting') return '待取'
  if (status === 'running') return '执行中'
  if (status === 'dialing') return '拨号'
  return status
}

function matchText(flag?: string) {
  if (flag === 'Y') return '版本已对齐'
  if (flag === 'N') return '版本未对齐'
  return '待上报'
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    title="升级代理"
    width="720px"
    :modal="false"
    :teleport-to="`#${moduleId}-layer`"
  >
    <div class="upgrade-body">
      <p class="hint">
        首次部署仍手工安装。后续上传对应平台的代理 tar.gz；默认金丝雀先升 1 台，确认版本对齐后再继续下一批。代理自己拉取、校验并重启，失败会回滚上一版二进制。
      </p>
      <div class="row">
        <label class="field">
          <span>种类</span>
          <select v-model="agentCode">
            <option v-for="item in codeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
        <label class="field">
          <span>系统</span>
          <select v-model="goos">
            <option v-for="item in osOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
        <label class="field">
          <span>架构</span>
          <select v-model="goarch">
            <option v-for="item in archOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </label>
      </div>
      <div class="row">
        <label class="field grow">
          <span>目标版本</span>
          <input v-model="version" placeholder="与 package.json 版本一致，例如 1.1.3" />
        </label>
        <label class="field">
          <span>金丝雀台数</span>
          <input v-model.number="canaryCount" type="number" min="1" max="32" />
        </label>
      </div>
      <label class="field">
        <span>安装包</span>
        <input type="file" accept=".tar.gz,.tgz,application/gzip" @change="onFile" />
      </label>
      <div class="pick-head">
        <span>
          匹配 {{ goos }}/{{ goarch }} 在线代理（{{ candidates.length }}）
          <template v-if="skippedPlatform"> · 平台不符或未上报 {{ skippedPlatform }}</template>
        </span>
        <RsButton size="sm" variant="ghost" :disabled="!candidates.length" @click="selectAll">全选</RsButton>
      </div>
      <ul class="pick-list">
        <li v-for="row in candidates" :key="row.agentId">
          <label>
            <input type="checkbox" :checked="picked.includes(row.agentId)" @change="toggle(row.agentId)" />
            <span>{{ row.agentName }}</span>
            <span class="meta">{{ row.agentVersion || '—' }} · {{ row.goos }}/{{ row.goarch }}</span>
          </label>
        </li>
        <li v-if="!candidates.length" class="empty">没有匹配该平台的在线代理（需先有一次心跳上报 goos/goarch）</li>
      </ul>
      <div v-if="releasing" class="progress">
        <div class="pick-head">
          <span>进度 · {{ releasing.versionText }} · {{ releasing.goos }}/{{ releasing.goarch }}</span>
          <RsButton
            v-if="(releasing.pendingCount ?? 0) > 0"
            size="sm"
            :loading="busy"
            :disabled="busy"
            @click="continueBatch"
          >
            继续下一批（剩 {{ releasing.pendingCount }}）
          </RsButton>
        </div>
        <ul>
          <li v-for="item in releasing.items" :key="item.commandId">
            <span>{{ item.agentName || item.agentId }} · {{ statusText(item.status) }}</span>
            <span class="meta">
              {{ item.agentVersion || '—' }} / {{ releasing.versionText }} · {{ matchText(item.versionMatched) }}
              <template v-if="item.message"> · {{ item.message }}</template>
            </span>
          </li>
        </ul>
      </div>
    </div>
    <template #footer>
      <RsButton variant="ghost" @click="open = false">关闭</RsButton>
      <RsButton :loading="busy" :disabled="busy || !!releasing" @click="submit">下发金丝雀</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.upgrade-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.hint {
  margin: 0;
  color: var(--rs-text-muted, #667);
  font-size: 13px;
  line-height: 1.5;
}
.row {
  display: flex;
  gap: 12px;
}
.field {
  display: grid;
  gap: 6px;
  font-size: 13px;
  min-width: 0;
}
.field.grow {
  flex: 1;
}
.field input,
.field select {
  min-height: 32px;
  padding: 0 10px;
}
.pick-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 13px;
}
.pick-list,
.progress ul {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 220px;
  overflow: auto;
  border: 1px solid var(--rs-border, #ddd);
  border-radius: 6px;
}
.pick-list li,
.progress li {
  padding: 8px 10px;
  border-bottom: 1px solid var(--rs-border, #eee);
  font-size: 13px;
  display: flex;
  gap: 8px;
  align-items: center;
}
.pick-list li:last-child,
.progress li:last-child {
  border-bottom: 0;
}
.pick-list label {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
}
.meta {
  margin-left: auto;
  color: var(--rs-text-muted, #667);
  text-align: right;
}
.empty {
  color: var(--rs-text-muted, #667);
}
</style>
