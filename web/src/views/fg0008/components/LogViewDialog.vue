<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { getApiMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsDialog, useRsToast } from '@/ui'
import {
  closeLogView,
  logOutput,
  openLogView,
  touchLogView,
  type LogViewChunk,
  type LogViewSession,
} from '../api'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  instId: string
  fileName?: string
  moduleId: string
}>()

const toast = useRsToast()
const session = ref<LogViewSession | null>(null)
const text = ref('')
const offset = ref(0)
const busy = ref(false)
const follow = ref(true)
const closed = ref(false)
const preRef = ref<HTMLPreElement | null>(null)
let pollTimer = 0
let touchTimer = 0
let alive = false

watch(open, (value) => {
  if (value) {
    void start()
    return
  }
  void stop()
})

onUnmounted(() => {
  void stop()
})

async function start() {
  alive = true
  text.value = ''
  offset.value = 0
  closed.value = false
  follow.value = true
  session.value = null
  busy.value = true
  try {
    const res = await openLogView(props.instId)
    if (!alive) return
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '打开跟读失败'))
      open.value = false
      return
    }
    const next = parseApiData<LogViewSession>(res)
    if (!next?.logViewId) {
      toast.error('未返回跟读会话')
      open.value = false
      return
    }
    session.value = next
    schedulePoll()
    scheduleTouch()
  } finally {
    busy.value = false
  }
}

async function stop() {
  alive = false
  stopPoll()
  stopTouch()
  const id = session.value?.logViewId
  session.value = null
  if (id) {
    try {
      await closeLogView(id)
    } catch {
      // 关闭失败不打扰；会话会按空闲回收
    }
  }
}

function stopPoll() {
  if (pollTimer) {
    window.clearTimeout(pollTimer)
    pollTimer = 0
  }
}

function stopTouch() {
  if (touchTimer) {
    window.clearInterval(touchTimer)
    touchTimer = 0
  }
}

function schedulePoll() {
  stopPoll()
  if (!alive || !session.value || closed.value) return
  pollTimer = window.setTimeout(() => void poll(), 200)
}

function scheduleTouch() {
  stopTouch()
  touchTimer = window.setInterval(() => {
    const id = session.value?.logViewId
    if (!id || !alive) return
    void touchLogView(id)
  }, 20000)
}

async function poll() {
  const id = session.value?.logViewId
  if (!id || !alive) return
  const res = await logOutput(id, offset.value)
  if (!alive) return
  if (!isApiSuccess(res)) {
    schedulePoll()
    return
  }
  const chunk = parseApiData<LogViewChunk>(res)
  if (chunk) {
    if (chunk.text) {
      text.value += chunk.text
      // 控制台侧 ring 约 512KB；前端也截断，防超大 DOM。
      if (text.value.length > 600_000) {
        text.value = text.value.slice(text.value.length - 500_000)
      }
      offset.value = chunk.offset
      await nextTick()
      stickBottom()
    } else {
      offset.value = chunk.offset
    }
    if (chunk.closed) {
      closed.value = true
      return
    }
  }
  schedulePoll()
}

function stickBottom() {
  if (!follow.value || !preRef.value) return
  preRef.value.scrollTop = preRef.value.scrollHeight
}

function onScroll() {
  const el = preRef.value
  if (!el) return
  const dist = el.scrollHeight - el.scrollTop - el.clientHeight
  follow.value = dist < 48
}

const title = computed(() => session.value?.fileName || props.fileName || '日志')
</script>

<template>
  <RsDialog
    v-model:open="open"
    :title="title"
    layout="form"
    width="860px"
    :modal="false"
    :teleport-to="`#${moduleId}-layer`"
  >
    <p class="hint">
      经中央会话隧道跟读；默认显示末尾并持续追加。上滚暂停跟底，回到底部继续。
      <template v-if="closed"> · 已结束</template>
      <template v-else-if="busy"> · 连接中…</template>
      <template v-else-if="follow"> · 跟底</template>
      <template v-else> · 已暂停跟底</template>
    </p>
    <pre ref="preRef" class="log-body" @scroll="onScroll">{{ text || (busy ? '正在接入…' : '暂无输出') }}</pre>
    <template #footer>
      <RsButton size="sm" variant="ghost" @click="open = false">关闭</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.hint {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--rs-text-secondary, #667);
  line-height: 1.45;
}
.log-body {
  margin: 0;
  min-height: 360px;
  max-height: min(70vh, 560px);
  overflow: auto;
  padding: 10px 12px;
  border: 1px solid var(--rs-border, #ddd);
  border-radius: 6px;
  background: color-mix(in srgb, var(--rs-text) 4%, transparent);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
