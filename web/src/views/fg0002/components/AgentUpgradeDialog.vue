<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, isApiSuccess, parseApiData } from '@/api/request'
import { loadProductBuild } from '@/productVersion'
import {
  RsButton,
  RsCheckbox,
  RsDialog,
  RsEmpty,
  RsForm,
  RsFormItem,
  RsInput,
  RsInputNumber,
  RsSelect,
  RsTag,
  useRsToast,
  type RsSelectOption,
} from '@/ui'
import {
  continueAgentRelease,
  queryAgentRelease,
  queryAgentReleases,
  upgradeAgents,
  type Agent,
  type AgentRelease,
} from '../api'
import {
  agentCodeLabel,
  upgradeStatusText,
  upgradeStatusVariant,
} from '../composables/useAgentCatalog'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  agents: Agent[]
  moduleId: string
}>()

const emit = defineEmits<{
  changed: []
}>()

const toast = useRsToast()
const agentCode = ref('host')
const version = ref('')
const goos = ref('linux')
const goarch = ref('amd64')
const canaryCount = ref(1)
const picked = ref<string[]>([])
const busy = ref(false)
const loadingShared = ref(false)
const releasing = ref<AgentRelease | null>(null)
const releases = ref<AgentRelease[]>([])
let pollTimer = 0

const codeOptions: RsSelectOption[] = [
  { value: 'host', label: '主机代理' },
  { value: 'go', label: 'Go 运行时' },
  { value: 'appside', label: '应用代理' },
]

const formModel = computed(() => ({
  agentCode: agentCode.value,
  version: version.value,
  goos: goos.value,
  goarch: goarch.value,
  canaryCount: canaryCount.value,
}))

const platformLabel = computed(() => `${goos.value}/${goarch.value}`)

const packLabel = computed(() => {
  if (agentCode.value === 'go') return 'agents/flux-guard-agent-goruntime.tar.gz'
  if (agentCode.value === 'appside') return 'agents/flux-guard-agent-appside.tar.gz'
  return 'agents/flux-guard-agent-host.tar.gz'
})

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

const allPicked = computed({
  get: () => candidates.value.length > 0 && picked.value.length === candidates.value.length,
  set: (value: boolean) => {
    picked.value = value ? candidates.value.map((row) => row.agentId) : []
  },
})

const halfPicked = computed(
  () => picked.value.length > 0 && picked.value.length < candidates.value.length,
)

const releaseOpen = computed(() => {
  const row = releasing.value
  if (!row) return false
  if ((row.pendingCount ?? 0) > 0) return true
  return (row.items ?? []).some(
    (item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running',
  )
})

const canSubmit = computed(() => !busy.value && !releaseOpen.value)

const progressSummary = computed(() => {
  const items = releasing.value?.items ?? []
  const total = items.length
  const done = items.filter((item) => item.status === 'done').length
  const failed = items.filter((item) => item.status === 'failed').length
  const running = items.filter(
    (item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running',
  ).length
  const pending = releasing.value?.pendingCount ?? 0
  return { total, done, failed, running, pending }
})

watch(agentCode, () => {
  picked.value = []
})

watch(open, (value) => {
  if (!value) {
    stopPoll()
    return
  }
  picked.value = []
  canaryCount.value = 1
  void loadProductBuild().then((build) => {
    if (build?.version) version.value = build.version
    if (build?.goos) goos.value = build.goos
    if (build?.goarch) goarch.value = build.goarch
  })
  void loadShared()
})

function stopPoll() {
  if (pollTimer) {
    window.clearTimeout(pollTimer)
    pollTimer = 0
  }
}

function isPicked(id: string) {
  return picked.value.includes(id)
}

function setPicked(id: string, on: boolean) {
  if (on) {
    if (!picked.value.includes(id)) picked.value = [...picked.value, id]
    return
  }
  picked.value = picked.value.filter((item) => item !== id)
}

function isReleaseActive(row: AgentRelease) {
  return row.openFlag === 'Y' || (row.pendingCount ?? 0) > 0
}

function codeText(code: string) {
  return agentCodeLabel(code)
}

async function loadShared() {
  loadingShared.value = true
  try {
    const res = await queryAgentReleases(undefined, 20)
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '加载升级记录失败'))
      releases.value = []
      releasing.value = null
      return
    }
    const list = parseApiData<AgentRelease[]>(res) ?? []
    releases.value = list
    const active = list.find((row) => isReleaseActive(row))
    if (active) {
      await openRelease(active.releaseId, false)
      return
    }
    releasing.value = null
  } finally {
    loadingShared.value = false
  }
}

async function openRelease(releaseId: string, syncList = true) {
  const res = await queryAgentRelease(releaseId)
  if (!isApiSuccess(res)) {
    toast.error(getApiMessage(res, '加载升级进度失败'))
    return
  }
  const next = parseApiData<AgentRelease>(res)
  if (!next) return
  releasing.value = next
  agentCode.value = next.agentCode || agentCode.value
  if (next.goos) goos.value = next.goos
  if (next.goarch) goarch.value = next.goarch
  if (next.versionText) version.value = next.versionText
  if (syncList) {
    const idx = releases.value.findIndex((row) => row.releaseId === releaseId)
    if (idx >= 0) {
      releases.value[idx] = {
        ...releases.value[idx],
        pendingCount: next.pendingCount,
        openFlag: next.openFlag,
      }
    }
  }
  schedulePoll()
  emit('changed')
}

function schedulePoll() {
  stopPoll()
  if (!open.value || !releasing.value) return
  if (!releaseOpen.value) return
  pollTimer = window.setTimeout(() => void poll(), 2000)
}

async function poll() {
  const id = releasing.value?.releaseId
  if (!id || !open.value) return
  const res = await queryAgentRelease(id)
  if (!isApiSuccess(res)) return
  const next = parseApiData<AgentRelease>(res)
  if (!next) return
  releasing.value = next
  emit('changed')
  schedulePoll()
}

async function submit() {
  if (!version.value.trim()) {
    toast.error('请填写目标版本')
    return
  }
  if (picked.value.length === 0) {
    toast.error('请选择要升级的代理')
    return
  }
  busy.value = true
  try {
    const res = await upgradeAgents({
      agentCode: agentCode.value,
      version: version.value.trim(),
      goos: goos.value,
      goarch: goarch.value,
      agentIds: picked.value,
      canaryCount: canaryCount.value || 1,
    })
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '升级下发失败'))
      return
    }
    const next = parseApiData<AgentRelease>(res)
    if (!next) return
    releasing.value = next
    const pending = next.pendingCount ?? 0
    toast.success(pending > 0 ? `已下发首批 ${next.items?.length ?? 0} 台，待继续 ${pending} 台` : `已下发 ${next.items?.length ?? 0} 台`)
    const listRes = await queryAgentReleases(undefined, 20)
    if (isApiSuccess(listRes)) {
      releases.value = parseApiData<AgentRelease[]>(listRes) ?? []
    }
    schedulePoll()
    emit('changed')
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
    schedulePoll()
    emit('changed')
  } finally {
    busy.value = false
  }
}

function matchText(flag?: string) {
  if (flag === 'Y') return '版本已对齐'
  if (flag === 'N') return '版本未对齐'
  return '待上报'
}

/**
 * 升级勾选列表里区分同名代理：显示名旁附主机与地址。
 * @param row - 代理行
 */
function agentPickLabel(row: Agent) {
  const parts = [row.agentName || row.agentId]
  if (row.hostName) parts.push(row.hostName)
  if (row.ipAddress) parts.push(row.ipAddress)
  return parts.join(' · ')
}

/**
 * 进度明细标识；优先用当前列表里的主机/地址，否则回退到显示名。
 * @param item - 一次升级中的单台明细
 */
function releaseItemLabel(item: { agentId: string; agentName?: string }) {
  const row = props.agents.find((agent) => agent.agentId === item.agentId)
  if (row) return agentPickLabel(row)
  return item.agentName || item.agentId
}
</script>

<template>
  <RsDialog
    v-model:open="open"
    title="升级代理"
    layout="form"
    width="780px"
    :modal="false"
    :teleport-to="`#${moduleId}-layer`"
  >
    <p class="hint">
      安装包使用中央发行目录 <code>{{ packLabel }}</code>（平台 {{ platformLabel }}），无需本机上传。
      进行中的升级对本租户其他用户可见；默认金丝雀先升 1 台，确认后再继续下一批。
    </p>
    <RsForm :model="formModel" label-position="top" size="sm" class="upgrade-form">
      <div class="row">
        <RsFormItem name="agentCode" label="种类" required>
          <RsSelect
            v-model="agentCode"
            :options="codeOptions"
            block
            match-trigger-width
            :disabled="releaseOpen"
          />
        </RsFormItem>
        <RsFormItem name="platform" label="平台">
          <RsInput :model-value="platformLabel" disabled />
        </RsFormItem>
      </div>
      <div class="row">
        <RsFormItem name="version" label="目标版本" required class="grow">
          <RsInput v-model="version" placeholder="默认与中央产品版本一致" clearable :disabled="releaseOpen" />
        </RsFormItem>
        <RsFormItem
          name="canaryCount"
          label="金丝雀台数"
          tooltip="先只升级少数代理（默认 1 台），确认无问题后再点「继续下一批」。"
        >
          <RsInputNumber v-model="canaryCount" :min="1" :max="32" :disabled="releaseOpen" />
        </RsFormItem>
      </div>
    </RsForm>
    <div class="pick-head">
      <RsCheckbox
        v-model="allPicked"
        :indeterminate="halfPicked"
        size="sm"
        :disabled="!candidates.length || releaseOpen"
      >
        匹配 {{ platformLabel }} 在线代理（{{ candidates.length }}）
        <template v-if="skippedPlatform"> · 平台不符或未上报 {{ skippedPlatform }}</template>
      </RsCheckbox>
    </div>
    <ul class="pick-list">
      <li v-for="row in candidates" :key="row.agentId">
        <RsCheckbox
          :model-value="isPicked(row.agentId)"
          size="sm"
          :disabled="releaseOpen"
          @update:model-value="setPicked(row.agentId, $event)"
        >
          <span class="name">{{ agentPickLabel(row) }}</span>
          <span class="meta">{{ row.agentVersion || '—' }} · {{ row.goos }}/{{ row.goarch }}</span>
        </RsCheckbox>
      </li>
      <li v-if="!candidates.length" class="empty">没有匹配该平台的在线代理（需先有一次心跳上报 goos/goarch）</li>
    </ul>

    <div class="shared">
      <div class="progress" :class="{ muted: !releasing }">
        <div class="pick-head">
          <span v-if="releasing">
            进度 · {{ codeText(releasing.agentCode) }} · {{ releasing.versionText }}
            · {{ releasing.goos }}/{{ releasing.goarch }}
            <template v-if="releasing.addWho"> · {{ releasing.addWho }}</template>
          </span>
          <span v-else>进度</span>
          <RsButton
            v-if="releasing && (releasing.pendingCount ?? 0) > 0"
            size="sm"
            :loading="busy"
            :disabled="busy"
            @click="continueBatch"
          >
            继续下一批（剩 {{ releasing.pendingCount }}）
          </RsButton>
        </div>
        <p v-if="releasing" class="summary">
          已下发 {{ progressSummary.total }}
          · 完成 {{ progressSummary.done }}
          · 进行中 {{ progressSummary.running }}
          · 失败 {{ progressSummary.failed }}
          <template v-if="progressSummary.pending"> · 待继续 {{ progressSummary.pending }}</template>
        </p>
        <RsEmpty
          v-if="!releasing && !loadingShared"
          class="empty-box"
          title="暂无进行中的升级"
          description="下发金丝雀后，本租户其他用户打开此对话框也能看到进度。"
        />
        <ul v-else-if="releasing?.items?.length">
          <li v-for="item in releasing.items" :key="item.commandId" class="progress-row">
            <span class="name">{{ releaseItemLabel(item) }}</span>
            <RsTag size="sm" :variant="upgradeStatusVariant(item.status, item.versionMatched)">
              {{ upgradeStatusText(item.status, item.versionMatched) }}
            </RsTag>
            <span class="meta">
              {{ item.agentVersion || '—' }} / {{ releasing.versionText }} · {{ matchText(item.versionMatched) }}
              <template v-if="item.message"> · {{ item.message }}</template>
            </span>
          </li>
        </ul>
      </div>
      <div class="history">
        <div class="pick-head"><span>最近升级</span></div>
        <RsEmpty
          v-if="!releases.length && !loadingShared"
          class="empty-box"
          title="尚无记录"
          description="成功下发后会出现在这里。"
        />
        <ul v-else class="history-list">
          <li v-for="row in releases" :key="row.releaseId">
            <button type="button" :class="{ active: releasing?.releaseId === row.releaseId }" @click="openRelease(row.releaseId)">
              <span>{{ row.addTime || '—' }} · {{ codeText(row.agentCode) }} · {{ row.versionText }}</span>
              <RsTag v-if="isReleaseActive(row)" size="sm" variant="warning">进行中</RsTag>
              <RsTag v-else-if="(row.pendingCount ?? 0) === 0" size="sm" variant="default">已结束</RsTag>
            </button>
          </li>
        </ul>
      </div>
    </div>

    <template #footer>
      <RsButton size="sm" variant="ghost" @click="open = false">关闭</RsButton>
      <RsButton size="sm" variant="primary" :loading="busy" :disabled="!canSubmit" @click="submit">
        下发金丝雀
      </RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.hint {
  margin: 0 0 12px;
  color: var(--rs-text-secondary, #667);
  font-size: 12px;
  line-height: 1.5;
}
.hint code {
  font-size: 12px;
}
.upgrade-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}
.row {
  display: flex;
  gap: 12px;
}
.row :deep(.rs-form-item) {
  flex: 1;
  min-width: 0;
}
.row :deep(.rs-form-item.grow),
.grow {
  flex: 1.4;
}
.pick-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 13px;
  margin-bottom: 8px;
}
.pick-list,
.progress ul,
.history-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 180px;
  overflow: auto;
  border: 1px solid var(--rs-border, #ddd);
  border-radius: 6px;
}
.pick-list li,
.progress li,
.history-list li {
  padding: 8px 10px;
  border-bottom: 1px solid var(--rs-border, #eee);
  font-size: 13px;
}
.pick-list li:last-child,
.progress li:last-child,
.history-list li:last-child {
  border-bottom: 0;
}
.pick-list :deep(.rs-checkbox) {
  width: 100%;
  justify-content: flex-start;
}
.pick-list :deep(.rs-checkbox__label) {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
  min-width: 0;
}
.progress-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  margin-left: auto;
  color: var(--rs-text-muted, #667);
  text-align: right;
  flex-shrink: 0;
  font-size: 12px;
}
.empty {
  color: var(--rs-text-muted, #667);
}
.shared {
  margin-top: 12px;
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(200px, 0.9fr);
  gap: 12px;
}
.progress,
.history {
  min-width: 0;
}
.summary {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--rs-text-secondary, #667);
}
.empty-box {
  min-height: 120px;
}
.history-list button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border: 0;
  background: transparent;
  padding: 0;
  text-align: left;
  cursor: pointer;
  color: inherit;
  font: inherit;
}
.history-list button.active {
  color: var(--rs-primary, #2563eb);
}
@media (max-width: 720px) {
  .shared {
    grid-template-columns: 1fr;
  }
}
</style>
