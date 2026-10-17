<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { RsButton, RsEmpty, RsTag } from '@/ui'
import type { AppInstance, AppRelease, ReleaseStep, RuntimeCommand } from '../api'
import { catalinaStarted } from '../composables/startupLog'
import {
  canRollbackRelease,
  kindLabel,
  statusText,
  statusVariant,
  stepLabel,
} from '../composables/useAppWorkbench'

const props = defineProps<{
  logLines: string[]
  releasing: AppRelease | null
  releases: AppRelease[]
  files: AppInstance[]
  canRelease: boolean
  continueBusy?: boolean
  instanceLabel: (instId: string) => string
  tailText?: string
  tailName?: string
  tailLive?: boolean
  lifecycle?: RuntimeCommand | null
  runtimeHistory?: RuntimeCommand[]
}>()

const emit = defineEmits<{
  continue: []
  rollback: []
  'open-release': [row: AppRelease]
  'open-runtime': [row: RuntimeCommand]
}>()

const showContinue = computed(() => (props.releasing?.pendingCount ?? 0) > 0)
const showRollback = computed(() => canRollbackRelease(props.releasing))
const showTail = computed(() => !!props.tailLive || !!props.tailText)
const lifecycleAction = computed(() => {
  const action = props.lifecycle?.action || ''
  if (action.endsWith('stop')) return '停止'
  if (action.endsWith('start')) return '启动'
  if (action.endsWith('restart')) return '重启'
  return '启停'
})
const logStarted = computed(() => catalinaStarted(props.tailText || ''))
const tailRef = ref<HTMLPreElement | null>(null)
const stickTail = ref(true)

watch(
  () => props.tailText,
  async () => {
    await nextTick()
    const el = tailRef.value
    if (!stickTail.value || !el) return
    el.scrollTop = el.scrollHeight
  },
)

function onTailScroll() {
  const el = tailRef.value
  if (!el) return
  stickTail.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48
}

function stepLead(detail?: string) {
  return (detail || '').split('\n')[0]?.trim() || ''
}

function runtimeAction(action: string) {
  if (action.endsWith('stop')) return '停止'
  if (action.endsWith('restart')) return '重启'
  if (action.endsWith('start')) return '启动'
  return '启停'
}

function stepRest(step: ReleaseStep) {
  if (step.name === 'startup' && showTail.value) return ''
  const text = (step.detail || '').replace(/\r\n/g, '\n').trim()
  const split = text.indexOf('\n')
  if (split < 0) return ''
  return text.slice(split + 1).trim()
}
</script>

<template>
  <section class="fg-op-console">
    <header class="fg-op-head">
      <h3>操作日志</h3>
      <div class="fg-op-actions">
        <RsButton
          v-if="canRelease && showContinue"
          size="sm"
          :disabled="continueBusy"
          @click="emit('continue')"
        >
          继续下一批
        </RsButton>
        <RsButton
          v-if="canRelease && showRollback"
          size="sm"
          variant="ghost"
          @click="emit('rollback')"
        >
          回滚已成功实例
        </RsButton>
      </div>
    </header>

    <div class="fg-op-body">
      <div class="fg-op-log">
        <RsEmpty
          v-if="!logLines.length"
          class="fg-op-empty"
          title="暂无操作日志"
          description="选择运行时后，上传、换包、重启与启动探测结果会显示在这里。"
        />
        <pre v-else>{{ logLines.join('\n') }}</pre>
      </div>

      <div v-if="showTail" class="fg-op-tail">
        <h4>
          {{ tailName || '启动日志' }}
          <RsTag v-if="logStarted" size="sm" variant="success">Catalina.start 已出现</RsTag>
          <span v-else-if="tailLive" class="msg">滚动跟读</span>
          <span v-else class="msg">跟读已停</span>
        </h4>
        <pre ref="tailRef" @scroll="onTailScroll">{{ tailText || (tailLive ? '正在接入…' : '') }}</pre>
      </div>

      <div v-if="lifecycle" class="fg-op-progress">
        <h4>本次{{ lifecycleAction }} · {{ statusText(lifecycle.status) }}</h4>
        <p v-if="!lifecycle.steps?.length" class="fg-wait">
          <template v-if="lifecycleAction === '停止'">应用代理正在停止，做完才返回这一步的说明。</template>
          <template v-else>
            停止、启动和启动探测在应用代理上做完才一次返回。右侧是 catalina 原文，出现 Catalina.start 表示容器已写完启动。
          </template>
        </p>
        <p v-else-if="lifecycle.message" class="fg-wait">{{ lifecycle.message }}</p>
        <ol v-if="lifecycle.steps?.length">
          <li v-for="(step, index) in lifecycle.steps" :key="`${lifecycle.commandId}-${index}`">
            <div class="fg-step-line">
              <RsTag size="sm" :variant="statusVariant(step.status)">{{ stepLabel(step.name) }}</RsTag>
              <span class="paths">{{ stepLead(step.detail) || '—' }}</span>
            </div>
            <pre v-if="stepRest(step)" class="fg-step-detail">{{ stepRest(step) }}</pre>
          </li>
        </ol>
      </div>

      <div v-if="releasing?.items?.length" class="fg-op-progress">
        <h4>
          {{ releasing.purpose === 'rollback' ? '当前回滚' : '当前发布' }}
          · {{ releasing.versionText }}
          <template v-if="releasing.pendingCount"> · 待继续 {{ releasing.pendingCount }}</template>
        </h4>
        <ul>
          <li v-for="item in releasing.items" :key="item.commandId">
            <div class="fg-op-row">
              <span>{{ instanceLabel(item.instId) }}</span>
              <RsTag size="sm" :variant="statusVariant(item.status)">{{ statusText(item.status) }}</RsTag>
              <span class="msg">{{ item.message || '—' }}</span>
            </div>
            <ol v-if="item.steps?.length">
              <li v-for="(step, index) in item.steps" :key="`${item.commandId}-${index}`">
                <div class="fg-step-line">
                  <RsTag size="sm" :variant="statusVariant(step.status)">{{ stepLabel(step.name) }}</RsTag>
                  <span class="paths">
                    {{ stepLead(step.detail) }}
                    <template v-if="step.targetPath">
                      <template v-if="step.detail"> · </template>
                      目标 {{ step.targetPath }}
                    </template>
                    <template v-if="step.backupPath">
                      <template v-if="step.detail || step.targetPath"> · </template>
                      备份 {{ step.backupPath }}
                    </template>
                  </span>
                </div>
                <pre v-if="stepRest(step)" class="fg-step-detail">{{ stepRest(step) }}</pre>
              </li>
            </ol>
            <p v-else-if="item.status === 'waiting' || item.status === 'dialing' || item.status === 'running'" class="fg-wait">
              下载制品、写入目标、重启和启动探测在应用代理上做完才一次返回。需要重启时，右侧跟读 catalina。
            </p>
          </li>
        </ul>
      </div>

      <div v-if="runtimeHistory?.length || releases.length" class="fg-op-history">
        <template v-if="runtimeHistory?.length">
          <h4>最近启停</h4>
          <ul>
            <li v-for="item in runtimeHistory" :key="item.commandId">
              <button type="button" @click="emit('open-runtime', item)">
                {{ item.addTime || '—' }} · {{ runtimeAction(item.action) }} · {{ statusText(item.status) }}
              </button>
            </li>
          </ul>
        </template>
        <template v-if="releases.length">
          <h4 :class="{ 'fg-hist-next': runtimeHistory?.length }">最近更新</h4>
          <ul>
            <li v-for="item in releases" :key="item.releaseId">
              <button type="button" @click="emit('open-release', item)">
                {{ item.addTime || '—' }}
                <template v-if="item.purpose === 'rollback'"> · 回滚</template>
                · {{ kindLabel(item.artifactKind) }} · {{ item.versionText }}
              </button>
            </li>
          </ul>
        </template>
      </div>
    </div>
  </section>
</template>

<style scoped>
.fg-op-console {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  border-top: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface-elevated, var(--rs-surface));
}

.fg-op-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-op-head h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.fg-op-actions {
  display: flex;
  gap: 8px;
}

.fg-op-body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
}

.fg-op-log,
.fg-op-tail,
.fg-op-progress,
.fg-op-history {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  overflow: auto;
  padding: 10px 12px;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-op-tail {
  flex: 1.6 1 0;
  display: flex;
  flex-direction: column;
}

.fg-op-body > :last-child {
  border-right: 0;
}

.fg-op-empty {
  padding: 8px 0;
}

.fg-op-log pre,
.fg-op-tail pre {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.fg-op-tail pre {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.fg-op-tail h4 {
  display: flex;
  align-items: center;
  gap: 8px;
}

.fg-op-tail h4 .msg {
  font-weight: 400;
  color: var(--rs-text-secondary);
}

.fg-op-tail h4,
.fg-wait {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--rs-text-secondary);
}

.fg-op-progress h4,
.fg-op-history h4 {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--rs-text-secondary);
}

.fg-hist-next {
  margin-top: 14px;
}

.fg-op-progress ul,
.fg-op-history ul {
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 12px;
}

.fg-op-progress li + li,
.fg-op-history li + li {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-op-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.fg-op-row .msg {
  color: var(--rs-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-op-progress ol {
  margin: 6px 0 0;
  padding: 0;
  list-style: none;
}

.fg-op-progress ol li {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
  border: 0;
  padding: 0;
}

.fg-step-line {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}

.paths {
  color: var(--rs-text-secondary);
  word-break: break-all;
}

.fg-step-detail {
  margin: 0;
  padding: 6px 8px;
  max-height: 160px;
  overflow: auto;
  border-radius: 4px;
  background: var(--rs-bg-muted, rgba(0, 0, 0, 0.04));
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--rs-text-secondary);
}

.fg-op-history button {
  border: 0;
  background: transparent;
  color: var(--rs-primary, #2563eb);
  cursor: pointer;
  padding: 0;
  text-align: left;
  font-size: 12px;
}

@media (max-width: 1100px) {
  .fg-op-body {
    flex-direction: column;
    overflow: auto;
  }

  .fg-op-log,
  .fg-op-tail,
  .fg-op-progress,
  .fg-op-history {
    flex: 1 0 auto;
    border-right: 0;
    border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  }
}
</style>
