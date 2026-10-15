<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, isApiSuccess, parseApiData } from '@/api/request'
import { loadProductBuild } from '@/productVersion'
import {
  RsButton,
  RsCheckbox,
  RsDialog,
  RsForm,
  RsFormItem,
  RsInput,
  RsInputNumber,
  RsSelect,
  useRsToast,
  type RsSelectOption,
} from '@/ui'
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
const picked = ref<string[]>([])
const busy = ref(false)
const releasing = ref<AgentRelease | null>(null)

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

watch(agentCode, () => {
  picked.value = []
})

watch(open, (value) => {
  if (!value) return
  picked.value = []
  releasing.value = null
  canaryCount.value = 1
  void loadProductBuild().then((build) => {
    if (build?.version) version.value = build.version
    if (build?.goos) goos.value = build.goos
    if (build?.goarch) goarch.value = build.goarch
  })
})

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
    layout="form"
    width="720px"
    :modal="false"
    :teleport-to="`#${moduleId}-layer`"
  >
    <p class="hint">
      安装包使用中央发行目录 <code>{{ packLabel }}</code>（平台 {{ platformLabel }}），无需本机上传。
      默认金丝雀先升 1 台，确认版本对齐后再继续下一批。代理自己拉取、校验并重启，失败会回滚上一版二进制。
    </p>
    <RsForm :model="formModel" label-position="top" size="sm" class="upgrade-form">
      <div class="row">
        <RsFormItem name="agentCode" label="种类" required>
          <RsSelect v-model="agentCode" :options="codeOptions" block match-trigger-width />
        </RsFormItem>
        <RsFormItem name="platform" label="平台">
          <RsInput :model-value="platformLabel" disabled />
        </RsFormItem>
      </div>
      <div class="row">
        <RsFormItem name="version" label="目标版本" required class="grow">
          <RsInput v-model="version" placeholder="默认与中央产品版本一致" clearable />
        </RsFormItem>
        <RsFormItem
          name="canaryCount"
          label="金丝雀台数"
          tooltip="先只升级少数代理（默认 1 台），确认无问题后再点「继续下一批」。"
        >
          <RsInputNumber v-model="canaryCount" :min="1" :max="32" />
        </RsFormItem>
      </div>
    </RsForm>
    <div class="pick-head">
      <RsCheckbox v-model="allPicked" :indeterminate="halfPicked" size="sm" :disabled="!candidates.length">
        匹配 {{ platformLabel }} 在线代理（{{ candidates.length }}）
        <template v-if="skippedPlatform"> · 平台不符或未上报 {{ skippedPlatform }}</template>
      </RsCheckbox>
    </div>
    <ul class="pick-list">
      <li v-for="row in candidates" :key="row.agentId">
        <RsCheckbox
          :model-value="isPicked(row.agentId)"
          size="sm"
          @update:model-value="setPicked(row.agentId, $event)"
        >
          <span class="name">{{ row.agentName }}</span>
          <span class="meta">{{ row.agentVersion || '—' }} · {{ row.goos }}/{{ row.goarch }}</span>
        </RsCheckbox>
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
    <template #footer>
      <RsButton size="sm" variant="ghost" @click="open = false">关闭</RsButton>
      <RsButton size="sm" variant="primary" :loading="busy" :disabled="busy || !!releasing" @click="submit">
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
.name {
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
}
.empty {
  color: var(--rs-text-muted, #667);
}
.progress {
  margin-top: 12px;
}
</style>
