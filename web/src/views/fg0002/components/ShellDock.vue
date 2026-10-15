<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import { useSessionStore } from '@/stores/session'
import { RsTerminal, useRsToast, type RsTerminalExpose } from '@/ui'
import {
  closeShell,
  openShell,
  queryShells,
  shellInput,
  shellOutput,
  shellResize,
  type Agent,
  type ShellChunk,
  type ShellSession,
} from '../api'

const props = defineProps<{
  agent: Agent | null
}>()

const emit = defineEmits<{
  close: []
}>()

interface Live {
  term: RsTerminalExpose | null
  offset: number
  ended: boolean
  seen: boolean
  warned: boolean
  send: Promise<void>
  pulling: boolean
  cols: number
  rows: number
}

const session = useSessionStore()
const toast = useRsToast()
const shells = ref<ShellSession[]>([])
const activeId = ref('')
const busy = ref(false)
const activeSeen = ref(false)
const activeEnded = ref(false)
const lives = new Map<string, Live>()
let listTimer = 0
let outputTimer = 0

const mine = computed(() => session.user?.userId ?? '')
const admin = computed(() => session.user?.tenantAdminFlag === 'Y')
const current = computed(() => shells.value.find((item) => item.shellId === activeId.value) ?? null)
const own = computed(() => !!current.value && current.value.userId === mine.value)
const mineShells = computed(() => shells.value.filter((item) => item.userId === mine.value))
const screenNote = computed(() => {
  if (activeEnded.value) return '这一路已经关闭。'
  if (!activeSeen.value) {
    return '等待代理接入会话隧道。本机代理需打开 shell_enabled 后重新启动，并在这台正在上报的主机上开终端。'
  }
  return ''
})

function canClose(item: ShellSession) {
  return item.userId === mine.value || admin.value
}

function tabLabel(item: ShellSession) {
  if (item.userId === mine.value) return item.shellTitle
  return `${item.shellTitle} · ${item.userName || item.userId}`
}

function ensureLive(shellId: string): Live {
  let live = lives.get(shellId)
  if (!live) {
    live = {
      term: null,
      offset: 0,
      ended: false,
      seen: false,
      warned: false,
      send: Promise.resolve(),
      pulling: false,
      cols: 0,
      rows: 0,
    }
    lives.set(shellId, live)
  }
  return live
}

function syncFlags() {
  const live = lives.get(activeId.value)
  activeSeen.value = !!live?.seen
  activeEnded.value = !!live?.ended
}

function bindTerm(shellId: string, el: unknown) {
  if (!el || typeof el !== 'object' || !('write' in el)) {
    const live = lives.get(shellId)
    if (live) live.term = null
    return
  }
  ensureLive(shellId).term = el as RsTerminalExpose
}

function onData(shellId: string, data: string) {
  const live = lives.get(shellId)
  if (!live || !data || live.ended) return
  live.send = live.send
    .then(async () => {
      const result = await shellInput(shellId, data)
      if (!isApiSuccess(result)) {
        if (!live.warned) {
          live.warned = true
          toast.error(getApiMessage(result, '发送失败'))
        }
        return
      }
      live.warned = false
    })
    .catch((err: unknown) => {
      if (!live.warned) {
        live.warned = true
        toast.error(getHttpErrorMessage(err, '发送失败'))
      }
    })
}

function onResize(shellId: string, payload: { cols: number; rows: number }) {
  const live = lives.get(shellId)
  if (!live) return
  if (payload.cols < 1 || payload.rows < 1 || (payload.cols === live.cols && payload.rows === live.rows)) return
  live.cols = payload.cols
  live.rows = payload.rows
  void shellResize(shellId, payload.cols, payload.rows)
}

function dropGone() {
  const ids = new Set(mineShells.value.map((item) => item.shellId))
  for (const id of lives.keys()) {
    if (!ids.has(id)) lives.delete(id)
  }
}

async function reload() {
  if (!props.agent) return
  const result = await queryShells(props.agent.agentId)
  if (!isApiSuccess(result)) return
  const list = parseApiData<ShellSession[]>(result) ?? []
  shells.value = list
  if (!list.some((item) => item.shellId === activeId.value)) {
    const ownTab = list.find((item) => item.userId === mine.value)
    activeId.value = (ownTab ?? list[0])?.shellId ?? ''
  }
}

async function add() {
  if (!props.agent || busy.value) return
  busy.value = true
  try {
    const result = await openShell(props.agent.agentId)
    if (!isApiSuccess(result)) {
      toast.error(getApiMessage(result, '打开终端失败'))
      return
    }
    const view = parseApiData<ShellSession>(result)
    if (view) activeId.value = view.shellId
    await reload()
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '打开终端失败'))
  } finally {
    busy.value = false
  }
}

async function closeTab(item: ShellSession) {
  if (!canClose(item) || busy.value) return
  busy.value = true
  try {
    const result = await closeShell(item.shellId)
    if (!isApiSuccess(result)) {
      toast.error(getApiMessage(result, '关闭终端失败'))
      return
    }
    if (activeId.value === item.shellId) activeId.value = ''
    await reload()
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '关闭终端失败'))
  } finally {
    busy.value = false
  }
}

async function pull(id: string) {
  const live = lives.get(id)
  if (!live || live.ended || live.pulling) return
  live.pulling = true
  try {
    const result = await shellOutput(id, live.offset)
    if (!isApiSuccess(result) || !lives.has(id)) return
    const data = parseApiData<ShellChunk>(result)
    if (!data) return
    if (data.text) {
      live.term?.write(data.text)
      live.seen = true
    }
    live.offset = data.offset
    if (data.closed) live.ended = true
    if (id === activeId.value) syncFlags()
  } finally {
    const currentLive = lives.get(id)
    if (currentLive) currentLive.pulling = false
  }
}

watch([shells, activeId], () => {
  dropGone()
  if (activeId.value) ensureLive(activeId.value)
  syncFlags()
  if (own.value && activeId.value) void pull(activeId.value)
})

watch(
  () => props.agent?.agentId,
  () => {
    void reload()
  },
  { immediate: true },
)

listTimer = window.setInterval(() => {
  void reload()
}, 4000)
outputTimer = window.setInterval(() => {
  if (own.value && activeId.value) void pull(activeId.value)
}, 100)

function releaseOnLeave() {
  const url = `${moduleApiPrefix('fg0002')}/closeShell`
  for (const item of shells.value) {
    if (item.userId !== mine.value) continue
    const body = new Blob([JSON.stringify({ shellId: item.shellId })], { type: 'application/json' })
    navigator.sendBeacon(url, body)
  }
}

window.addEventListener('pagehide', releaseOnLeave)

onScopeDispose(() => {
  window.removeEventListener('pagehide', releaseOnLeave)
  window.clearInterval(listTimer)
  window.clearInterval(outputTimer)
  lives.clear()
})

defineExpose({ add })
</script>

<template>
  <section class="shell-dock" aria-label="终端">
    <header class="shell-bar">
      <div class="shell-tabs">
        <button
          v-for="item in shells"
          :key="item.shellId"
          type="button"
          class="shell-tab"
          :class="{ active: item.shellId === activeId, mine: item.userId === mine }"
          @click="activeId = item.shellId"
        >
          <span>{{ tabLabel(item) }}</span>
          <span
            v-if="canClose(item)"
            class="shell-x"
            role="button"
            tabindex="0"
            @click.stop="closeTab(item)"
            @keydown.enter.stop="closeTab(item)"
          >×</span>
        </button>
      </div>
      <button type="button" class="shell-add" :disabled="busy || !agent" @click="add">新开</button>
      <button type="button" class="shell-fold" @click="emit('close')">收起</button>
    </header>
    <div class="shell-screen">
      <p v-if="!current" class="shell-empty">这台代理上还没有打开的终端。点「新开」再开一路，其他人打开的也会出现在上面。</p>
      <template v-else-if="!own">
        <pre class="shell-body">这是别人的终端，只能看到标签，不能查看或输入。</pre>
      </template>
      <div v-show="own && current" class="shell-stage">
        <RsTerminal
          v-for="item in mineShells"
          :key="item.shellId"
          v-show="item.shellId === activeId"
          :ref="(el) => bindTerm(item.shellId, el)"
          class="shell-term"
          :input-enabled="item.shellId !== activeId || !activeEnded"
          :overlay="item.shellId === activeId ? screenNote : ''"
          :convert-eol="false"
          :right-click-selects-word="false"
          wheel-scroll-modifier="shift"
          :scrollback="2000"
          @data="onData(item.shellId, $event)"
          @resize="onResize(item.shellId, $event)"
        />
      </div>
    </div>
  </section>
</template>

<style scoped>
.shell-dock {
  flex: 1 1 50%;
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-top: 1px solid var(--rs-border);
  background: var(--rs-surface);
}
.shell-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 36px;
  padding: 0 8px;
  border-bottom: 1px solid var(--rs-border-subtle);
}
.shell-tabs {
  display: flex;
  flex: 1;
  gap: 4px;
  min-width: 0;
  overflow-x: auto;
}
.shell-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 220px;
  padding: 4px 8px;
  border: 0;
  border-radius: 6px 6px 0 0;
  background: transparent;
  color: var(--rs-text-secondary);
  cursor: pointer;
  white-space: nowrap;
}
.shell-tab.active {
  background: var(--rs-surface-elevated);
  color: var(--rs-text-primary);
}
.shell-tab.mine.active {
  box-shadow: inset 0 -2px 0 var(--rs-primary);
}
.shell-x {
  color: var(--rs-text-tertiary);
}
.shell-add,
.shell-fold {
  border: 1px solid var(--rs-border);
  background: var(--rs-surface-elevated);
  color: var(--rs-text-primary);
  border-radius: 6px;
  padding: 2px 8px;
  cursor: pointer;
}
.shell-screen {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 0;
  overflow: hidden;
  color: var(--rs-text-primary);
}
.shell-empty,
.shell-body {
  margin: 0;
  padding: 10px 12px;
}
.shell-body,
.shell-empty {
  white-space: pre-wrap;
  font-family: var(--rs-font-mono);
  font-size: 13px;
  line-height: 1.5;
}
.shell-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.shell-stage {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
}
.shell-term {
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
}
</style>
