<script setup lang="ts">
import { ref, watch } from 'vue'
import { RsTerminal, type RsTerminalExpose } from '@/ui'

const props = defineProps<{
  /** 已读到的日志全文。只把新增部分写入终端；前面被截掉时整屏重绘。 */
  text: string
  /** 还没有正文时盖在终端上的说明。 */
  placeholder?: string
  ariaLabel?: string
}>()

const term = ref<RsTerminalExpose | null>(null)
let written = ''

function bind(el: unknown) {
  const api = el as RsTerminalExpose | null
  if (!api || typeof api.write !== 'function') {
    term.value = null
    written = ''
    return
  }
  term.value = api
  void paint()
}

/** 按全文增量写入。xterm 在视口贴底时自动跟读，上滚后停在原处。 */
async function paint() {
  const api = term.value
  if (!api) return
  const next = props.text || ''
  if (!next) {
    if (written) api.clear()
    written = ''
    return
  }
  if (written && next.startsWith(written)) {
    const delta = next.slice(written.length)
    if (delta) api.write(delta)
    written = next
    return
  }
  api.clear()
  api.write(next)
  written = next
  await api.fit()
}

watch(() => props.text, () => void paint())
</script>

<template>
  <RsTerminal
    :ref="bind"
    class="fg-log-term"
    :input-enabled="false"
    :cursor-blink="false"
    :convert-eol="true"
    :zebra-stripes="true"
    :search-enabled="true"
    :scrollback="8000"
    :right-click-selects-word="false"
    wheel-scroll-modifier="none"
    :overlay="text ? '' : placeholder || ''"
    :aria-label="ariaLabel || '日志'"
  />
</template>

<style scoped>
.fg-log-term {
  flex: 1;
  min-height: 0;
  height: 100%;
}
</style>
