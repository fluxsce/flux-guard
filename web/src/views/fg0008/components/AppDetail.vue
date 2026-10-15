<script setup lang="ts">
import { computed } from 'vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsButton, RsTag } from '@/ui'
import type { AppInstance, AppRelease, AppRow } from '../api'
import {
  canRollbackRelease,
  instColumns,
  kindLabel,
  moduleId,
  statusText,
  statusVariant,
  stepLabel,
} from '../composables/useAppCatalog'

const props = defineProps<{
  app: AppRow
  instances: AppInstance[]
  releases: AppRelease[]
  releasing: AppRelease | null
  picked: string[]
  kind: string
  version: string
  canaryCount: number
  kindOptions: { value: string; label: string }[]
  canEdit: boolean
  canRelease: boolean
  continueBusy?: boolean
  loading?: boolean
  instanceLabel: (instId: string) => string
}>()

const emit = defineEmits<{
  back: []
  rename: []
  ignore: []
  'pick-online': []
  refresh: []
  toggle: [instId: string]
  'update:kind': [value: string]
  'update:version': [value: string]
  'update:canaryCount': [value: number]
  file: [event: Event]
  release: []
  continue: []
  rollback: []
  'open-release': [row: AppRelease]
}>()

const showContinue = computed(() => (props.releasing?.pendingCount ?? 0) > 0)
const showRollback = computed(() => canRollbackRelease(props.releasing))

const title = computed(() => props.app.appName)

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'back', label: '返回', icon: 'arrow-left', aiRisk: 'read', onClick: () => emit('back') },
  {
    key: 'edit',
    label: '改名',
    icon: 'pencil',
    code: 'fg0008:edit',
    aiRisk: 'write',
    show: props.canEdit,
    onClick: () => emit('rename'),
  },
  {
    key: 'ignore',
    label: '忽略',
    icon: 'ban',
    code: 'fg0008:edit',
    aiRisk: 'write',
    show: props.canEdit,
    onClick: () => emit('ignore'),
  },
  {
    key: 'pick',
    label: '全选在线',
    icon: 'check',
    aiRisk: 'read',
    onClick: () => emit('pick-online'),
  },
  {
    key: 'view',
    label: '刷新',
    icon: 'rotate-ccw',
    code: 'fg0008:view',
    aiRisk: 'read',
    onClick: () => emit('refresh'),
  },
])

function instOf(row: unknown) {
  return row as AppInstance
}
</script>

<template>
  <section class="fg-app-detail">
    <FgToolbar plain wrap :title="title" :buttons="toolbarButtons" :module-id="moduleId">
      <template #after>
        <span class="fg-app-code">{{ app.appCode }}</span>
      </template>
    </FgToolbar>
    <div class="fg-app-detail-body">
      <p class="fg-note">
        制品 {{ kindLabel(app.artifactKind) }} · 实例 {{ app.instanceCount }} · 在线 {{ app.onlineCount }}
        · 版本 {{ app.versionText || '—' }}
      </p>
      <FgGrid
        class="fg-app-inst-grid"
        :data="instances"
        :columns="instColumns"
        :loading="loading"
        row-key="instId"
        :module-id="moduleId"
      >
        <template #pick="{ row }">
          <input
            type="checkbox"
            :checked="picked.includes(instOf(row).instId)"
            @change="emit('toggle', instOf(row).instId)"
          />
        </template>
        <template #modeCode="{ row }">{{ instOf(row).modeCode || '—' }}</template>
        <template #ipAddress="{ row }">{{ instOf(row).ipAddress || '—' }}</template>
        <template #artifactPath="{ row }">{{ instOf(row).artifactPath || '—' }}</template>
        <template #commandAddress="{ row }">{{ instOf(row).commandAddress || '—' }}</template>
        <template #runningFlag="{ row }">
          <RsTag size="sm" :variant="instOf(row).runningFlag === 'Y' ? 'success' : 'default'">
            {{ instOf(row).runningFlag === 'Y' ? '在跑' : '已停' }}
          </RsTag>
        </template>
        <template #onlineFlag="{ row }">
          <RsTag size="sm" :variant="instOf(row).onlineFlag === 'Y' ? 'success' : 'default'">
            {{ instOf(row).onlineFlag === 'Y' ? '在线' : '离线' }}
          </RsTag>
        </template>
      </FgGrid>

      <section v-if="canRelease" class="fg-panel fg-app-release-panel">
        <h3 class="fg-section-title">下发更新</h3>
        <form class="fg-app-release" @submit.prevent="emit('release')">
          <label class="fg-field">
            <span>种类</span>
            <select :value="kind" @change="emit('update:kind', ($event.target as HTMLSelectElement).value)">
              <option v-for="item in kindOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </label>
          <label class="fg-field grow">
            <span>目标版本</span>
            <input
              :value="version"
              placeholder="例如 1.2.0"
              @input="emit('update:version', ($event.target as HTMLInputElement).value)"
            />
          </label>
          <label class="fg-field">
            <span>金丝雀台数</span>
            <input
              type="number"
              min="1"
              max="32"
              :value="canaryCount"
              @input="emit('update:canaryCount', Number(($event.target as HTMLInputElement).value) || 1)"
            />
          </label>
          <label class="fg-field grow">
            <span>制品文件</span>
            <input type="file" @change="emit('file', $event)" />
          </label>
          <RsButton type="submit" size="sm">更新</RsButton>
        </form>
        <p class="fg-hint">
          先勾选同一种制品的实例；默认金丝雀先更新 1 台，确认后再继续下一批。Tomcat 可选运行时升级。已选
          {{ picked.length }} 台。
        </p>
      </section>

      <section v-if="releasing?.items?.length || showContinue" class="fg-panel">
        <div class="fg-prog-title-row">
          <h3 class="fg-section-title">
            {{ releasing?.purpose === 'rollback' ? '当前回滚进度' : '当前发布进度' }}
            <template v-if="releasing?.pendingCount"> · 待继续 {{ releasing.pendingCount }} 台</template>
          </h3>
          <div class="fg-prog-actions">
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
        </div>
        <ul class="fg-app-progress">
          <li v-for="item in releasing?.items ?? []" :key="item.commandId" class="fg-prog-item">
            <div class="fg-prog-head">
              <span class="fg-prog-name">{{ instanceLabel(item.instId) }}</span>
              <RsTag size="sm" :variant="statusVariant(item.status)">{{ statusText(item.status) }}</RsTag>
              <span class="fg-prog-msg">{{ item.message || '—' }}</span>
            </div>
            <ol v-if="item.steps?.length" class="fg-prog-steps">
              <li v-for="(step, index) in item.steps" :key="`${item.commandId}-${index}`">
                <div class="fg-step-line">
                  <RsTag size="sm" :variant="statusVariant(step.status)">{{ stepLabel(step.name) }}</RsTag>
                  <span class="fg-step-paths">
                    <template v-if="step.targetPath">目标 {{ step.targetPath }}</template>
                    <template v-if="step.backupPath">
                      <template v-if="step.targetPath"> · </template>
                      备份 {{ step.backupPath }}
                    </template>
                  </span>
                </div>
                <pre v-if="step.detail" class="fg-step-detail">{{ step.detail }}</pre>
              </li>
            </ol>
          </li>
        </ul>
      </section>

      <section v-if="releases.length" class="fg-panel">
        <h3 class="fg-section-title">最近发布</h3>
        <ul class="fg-app-history">
          <li v-for="item in releases" :key="item.releaseId">
            <button type="button" class="fg-app-history-link" @click="emit('open-release', item)">
              {{ item.addTime || '—' }} ·
              <template v-if="item.purpose === 'rollback'">回滚 · </template>
              {{ kindLabel(item.artifactKind) }} · {{ item.versionText }}
            </button>
            <span class="fg-app-file">{{ item.fileName || '—' }}</span>
          </li>
        </ul>
      </section>
    </div>
  </section>
</template>

<style scoped>
.fg-app-detail {
  flex: 1;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--fg-panel-bg, var(--rs-surface));
}

.fg-app-code {
  opacity: 0.7;
  font-size: 12px;
  margin-left: 8px;
}

.fg-app-detail-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0 0 24px;
  display: flex;
  flex-direction: column;
}

.fg-note {
  margin: 0;
  padding: 8px 16px 0;
  color: var(--rs-text-secondary);
  font-size: 13px;
}

.fg-app-inst-grid {
  flex: 0 0 auto;
  min-height: 200px;
  max-height: 42vh;
}

.fg-panel {
  margin: 12px 16px 0;
  padding: 12px 14px;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface-elevated, var(--rs-surface));
  box-shadow: var(--fg-panel-shadow);
}

.fg-section-title {
  margin: 0 0 10px;
  font-size: 13px;
  font-weight: 600;
}

.fg-prog-title-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  margin-bottom: 4px;
}

.fg-prog-title-row .fg-section-title {
  margin-bottom: 0;
}

.fg-prog-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.fg-app-release {
  display: flex;
  gap: 10px;
  align-items: flex-end;
  flex-wrap: wrap;
}

.fg-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 140px;
}

.fg-field.grow {
  flex: 1 1 180px;
}

.fg-field span {
  font-size: 12px;
  color: var(--rs-text-secondary);
}

.fg-field input,
.fg-field select {
  height: 32px;
  padding: 0 8px;
  border: 1px solid var(--rs-border);
  border-radius: 6px;
  background: transparent;
  color: inherit;
}

.fg-hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--rs-text-secondary);
}

.fg-app-progress,
.fg-app-history {
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 13px;
}

.fg-prog-item,
.fg-app-history li {
  padding: 8px 0;
  border-top: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-prog-item:first-child,
.fg-app-history li:first-child {
  border-top: 0;
  padding-top: 0;
}

.fg-prog-head,
.fg-app-history li {
  display: flex;
  gap: 12px;
  align-items: center;
}

.fg-prog-name {
  min-width: 160px;
  font-weight: 500;
}

.fg-prog-msg {
  color: var(--rs-text-secondary);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-prog-steps {
  margin: 8px 0 0;
  padding: 0 0 0 8px;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fg-prog-steps li {
  display: flex;
  flex-direction: column;
  gap: 4px;
  border: 0;
  padding: 0;
}

.fg-step-line {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.fg-step-paths {
  flex: 1;
  min-width: 0;
  color: var(--rs-text-secondary);
  font-size: 12px;
  word-break: break-all;
  line-height: 1.45;
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

.fg-app-history-link {
  border: 0;
  background: transparent;
  color: var(--rs-primary, #2563eb);
  cursor: pointer;
  padding: 0;
  text-align: left;
}

.fg-app-file {
  color: var(--rs-text-secondary);
  font-size: 12px;
}
</style>
