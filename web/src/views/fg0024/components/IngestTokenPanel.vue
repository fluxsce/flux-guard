<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsTag, useRsToast } from '@/ui'
import { revealIngestToken, type IngestToken } from '../api'

const props = defineProps<{
  token: IngestToken
  editable: boolean
  saving: boolean
}>()

const emit = defineEmits<{
  rotate: []
  toggle: []
}>()

const toast = useRsToast()
const revealed = ref(false)
const plain = ref('')
const reading = ref(false)

const display = computed(() => {
  if (!props.token.configured) return '—'
  if (revealed.value && plain.value) return plain.value
  return props.token.hint ? `····${props.token.hint}` : '—'
})

watch(
  () => [props.token.configured, props.token.hint, props.token.agentToken || ''] as const,
  ([configured, hint, agent]) => {
    if (agent) {
      plain.value = agent
      revealed.value = true
      return
    }
    if (!configured) {
      plain.value = ''
      revealed.value = false
      return
    }
    if (plain.value && hint && !plain.value.endsWith(hint)) {
      plain.value = ''
      revealed.value = false
    }
  },
  { immediate: true },
)

async function ensurePlain() {
  if (plain.value) return plain.value
  if (props.token.agentToken) {
    plain.value = props.token.agentToken
    return plain.value
  }
  reading.value = true
  try {
    const result = await revealIngestToken()
    if (!isApiSuccess(result)) {
      toast.error(getApiMessage(result, '无法读出令牌'))
      return ''
    }
    const data = parseApiData<IngestToken>(result)
    plain.value = data?.agentToken || ''
    if (!plain.value) toast.error('无法读出令牌，请轮换后再分发')
    return plain.value
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '无法读出令牌'))
    return ''
  } finally {
    reading.value = false
  }
}

async function toggleView() {
  if (revealed.value) {
    revealed.value = false
    return
  }
  const value = await ensurePlain()
  if (value) revealed.value = true
}

async function copyToken() {
  const value = await ensurePlain()
  if (!value) return
  try {
    await writeClipboard(value)
    toast.success('已复制')
  } catch {
    toast.error('复制失败')
  }
}

async function writeClipboard(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const el = document.createElement('textarea')
  el.value = text
  el.setAttribute('readonly', '')
  el.style.position = 'fixed'
  el.style.left = '-9999px'
  document.body.appendChild(el)
  el.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(el)
  if (!ok) throw new Error('copy failed')
}
</script>

<template>
  <div class="fg-ingest">
    <p class="fg-ingest-lead">主机、Java、数据库等代理共用这一把令牌。令牌只证明允许向本租户上报。</p>
    <dl class="fg-ingest-meta">
      <div>
        <dt>状态</dt>
        <dd>
          <RsTag size="sm" :variant="token.configured && token.statusFlag !== 'N' ? 'success' : 'default'">
            {{ !token.configured ? '未签发' : token.statusFlag === 'N' ? '已停用' : '启用' }}
          </RsTag>
        </dd>
      </div>
      <div>
        <dt>令牌</dt>
        <dd class="fg-token-line">
          <code class="fg-token">{{ display }}</code>
          <template v-if="token.configured">
            <RsButton variant="secondary" size="sm" :disabled="reading" @click="toggleView">
              {{ revealed ? '隐藏' : '查看' }}
            </RsButton>
            <RsButton variant="secondary" size="sm" :disabled="reading" @click="copyToken">复制</RsButton>
          </template>
        </dd>
      </div>
    </dl>
    <div v-if="editable" class="fg-ingest-actions">
      <RsButton variant="primary" size="md" :disabled="saving" @click="emit('rotate')">
        {{ token.configured ? '轮换令牌' : '签发令牌' }}
      </RsButton>
      <RsButton
        v-if="token.configured"
        variant="secondary"
        size="md"
        :disabled="saving"
        @click="emit('toggle')"
      >
        {{ token.statusFlag === 'N' ? '启用' : '停用' }}
      </RsButton>
    </div>
  </div>
</template>

<style scoped>
.fg-ingest {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 36rem;
}
.fg-ingest-lead {
  margin: 0;
  color: var(--rs-muted, var(--rs-text-secondary));
  font-size: 13px;
  line-height: 1.6;
}
.fg-ingest-meta {
  display: grid;
  grid-template-columns: 8rem 1fr;
  gap: 12px 16px;
  margin: 0;
}
.fg-ingest-meta div {
  display: contents;
}
.fg-ingest-meta dt {
  margin: 0;
  color: var(--rs-muted, var(--rs-text-secondary));
  font-size: 13px;
  line-height: 28px;
}
.fg-ingest-meta dd {
  margin: 0;
  font-size: 13px;
}
.fg-token-line {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}
.fg-token {
  margin: 0;
  padding: 4px 8px;
  border-radius: 8px;
  background: var(--rs-surface-muted, var(--rs-surface));
  font-family: ui-monospace, monospace;
  font-size: 13px;
  word-break: break-all;
}
.fg-ingest-actions {
  display: flex;
  gap: 8px;
}
</style>
