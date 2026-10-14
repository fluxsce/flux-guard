<script setup lang="ts">
import { computed } from 'vue'
import { RsButton, RsEmpty, RsTag } from '@/ui'
import type { AppInstance, AppRelease } from '../api'
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
}>()

const emit = defineEmits<{
  continue: []
  rollback: []
  'open-release': [row: AppRelease]
}>()

const showContinue = computed(() => (props.releasing?.pendingCount ?? 0) > 0)
const showRollback = computed(() => canRollbackRelease(props.releasing))
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
          description="选择运行时后，上传与换包步骤会显示在这里。"
        />
        <pre v-else>{{ logLines.join('\n') }}</pre>
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
                <RsTag size="sm" :variant="statusVariant(step.status)">{{ stepLabel(step.name) }}</RsTag>
                <span class="paths">
                  <template v-if="step.targetPath">目标 {{ step.targetPath }}</template>
                  <template v-if="step.backupPath">
                    <template v-if="step.targetPath"> · </template>
                    备份 {{ step.backupPath }}
                  </template>
                  <template v-if="step.detail"> · {{ step.detail }}</template>
                </span>
              </li>
            </ol>
          </li>
        </ul>
      </div>

      <div v-if="releases.length" class="fg-op-history">
        <h4>最近发布</h4>
        <ul>
          <li v-for="item in releases" :key="item.releaseId">
            <button type="button" @click="emit('open-release', item)">
              {{ item.addTime || '—' }}
              <template v-if="item.purpose === 'rollback'"> · 回滚</template>
              · {{ kindLabel(item.artifactKind) }} · {{ item.versionText }}
            </button>
          </li>
        </ul>
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
  overflow: auto;
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(280px, 1.2fr) minmax(180px, 0.8fr);
  gap: 0;
}

.fg-op-log,
.fg-op-progress,
.fg-op-history {
  min-height: 0;
  padding: 10px 12px;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-op-history {
  border-right: 0;
}

.fg-op-empty {
  padding: 8px 0;
}

.fg-op-log pre {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.fg-op-progress h4,
.fg-op-history h4 {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--rs-text-secondary);
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
  gap: 8px;
  margin-top: 4px;
  border: 0;
  padding: 0;
}

.paths {
  color: var(--rs-text-secondary);
  word-break: break-all;
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
    grid-template-columns: 1fr;
  }

  .fg-op-log,
  .fg-op-progress,
  .fg-op-history {
    border-right: 0;
    border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  }
}
</style>
