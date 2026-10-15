<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgGridColumn, FgGridMenuItem } from '@/components/grid/types'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsBreadcrumb, RsTag, useRsToast } from '@/ui'
import type { AppInstance, AppSite, DownloadProgress } from '../api'
import { breadcrumbOf, entriesInDir, relativeToBase, type FileEntry } from '../composables/fileTree'
import { fileTypePreview, kindLabel, modeLabel, moduleId } from '../composables/useAppWorkbench'
import LogViewDialog from './LogViewDialog.vue'
import UploadDialog, { type UploadMode } from './UploadDialog.vue'

const props = defineProps<{
  site: AppSite
  files: AppInstance[]
  picked: string[]
  loading?: boolean
  downloadBusy?: boolean
  downloadProgress?: DownloadProgress | null
  canRelease: boolean
  canLifecycle?: boolean
  version: string
  canaryCount: number
  releaseKind: string
  releaseAppCode: string
}>()

const emit = defineEmits<{
  refresh: []
  'pick-all': [instIds: string[]]
  'clear-pick': []
  download: []
  'download-one': [instId: string]
  toggle: [instId: string]
  'update:version': [value: string]
  'update:canaryCount': [value: number]
  'pick-file': [file: File | null]
  release: []
  create: [payload: { appCode: string; kind: string }]
  lifecycle: [op: 'start' | 'stop' | 'restart']
}>()

const toast = useRsToast()
const cwd = ref('')
const uploadOpen = ref(false)
const uploadMode = ref<UploadMode>('update')
const logOpen = ref(false)
const logInstId = ref('')
const logFileName = ref('')

watch(
  () => props.site.agentId + props.site.instanceName,
  () => {
    cwd.value = ''
  },
)

const title = computed(() => props.site.instanceName)
const crumbs = computed(() =>
  breadcrumbOf(cwd.value, props.site.basePath).map((c) => ({
    key: c.path || 'root',
    label: c.label,
    to: c.path === '' ? '#' : `#${c.path}`,
  })),
)
const progressPercent = computed(() => {
  const p = props.downloadProgress
  if (!p) return 0
  if (p.percent > 0) return Math.min(100, p.percent)
  if (p.phase === 'done') return 100
  if (p.phase === 'saving') return 90
  if (p.phase === 'transferring') return 35
  if (p.phase === 'queued') return 15
  if (p.phase === 'request') return 5
  return 0
})
const progressIndeterminate = computed(() => {
  const p = props.downloadProgress
  if (!p || p.phase === 'done' || p.phase === 'failed') return false
  return !(p.bytesTotal > 0 && p.percent > 0)
})
const entries = computed(() => entriesInDir(props.files, props.site.basePath, cwd.value))
/** 勾选列用行 key；业务侧仍用 instId。 */
const selectedKeys = computed(() =>
  entries.value
    .filter((e) => e.kind === 'file' && e.row && props.picked.includes(e.row.instId))
    .map((e) => e.key),
)
const canUpload = computed(() => {
  if (!props.picked.length || !props.releaseAppCode || !props.releaseKind) return false
  if (props.releaseKind === 'dir' || props.releaseKind === 'log') return false
  return true
})

const columns: FgGridColumn[] = [
  { key: 'type', title: '类型', width: 72 },
  { key: 'name', title: '名称', minWidth: 160, ellipsis: true },
  { key: 'appCode', title: '应用', width: 120, ellipsis: true },
  { key: 'versionText', title: '版本', width: 100 },
  { key: 'path', title: '路径', minWidth: 180, ellipsis: true },
  { key: 'online', title: '代理', width: 80 },
]

function canPreviewLog(row: FileEntry) {
  if (row.kind !== 'file' || !row.row || row.row.onlineFlag !== 'Y') return false
  const kind = String(row.row.artifactKind || '').toLowerCase()
  if (kind === 'log') return true
  const name = (row.name || row.row.artifactPath || '').toLowerCase()
  return name.endsWith('.log') || name.endsWith('.txt') || name.endsWith('.out')
}

const menuItems = computed<FgGridMenuItem[]>(() => [
  {
    key: 'open',
    label: '打开',
    icon: 'folder',
    disabled: (row: FileEntry) => row.kind !== 'dir',
  },
  {
    key: 'view-log',
    label: '查看',
    icon: 'eye',
    code: 'fg0008:view',
    disabled: (row: FileEntry) => !canPreviewLog(row),
  },
  {
    key: 'download',
    label: '下载',
    icon: 'download',
    code: 'fg0008:view',
    disabled: (row: FileEntry) =>
      row.kind !== 'file' || !row.row || props.downloadBusy || row.row.onlineFlag !== 'Y',
  },
])

/** 优先 catalina.out，否则 logs 下常见启动日志。 */
const startupLogRow = computed(() => {
  const logs = props.files.filter((row) => {
    if (row.onlineFlag !== 'Y') return false
    const kind = String(row.artifactKind || '').toLowerCase()
    const path = (row.artifactPath || '').replace(/\\/g, '/').toLowerCase()
    const name = (row.appCode || path.split('/').pop() || '').toLowerCase()
    if (kind === 'log') return path.includes('/logs/') || name.endsWith('.log') || name.endsWith('.out')
    return name.endsWith('.log') || name.endsWith('.out') || name.endsWith('.txt')
  })
  if (!logs.length) return null
  const catalina = logs.find((row) => {
    const path = (row.artifactPath || '').replace(/\\/g, '/').toLowerCase()
    return path.endsWith('/catalina.out') || path.includes('/catalina.')
  })
  return catalina || logs[0]
})

function openStartupLog() {
  const row = startupLogRow.value
  if (!row) {
    toast.error('未找到启动日志，请先刷新或进入 logs 目录')
    return
  }
  logInstId.value = row.instId
  logFileName.value = fileNameFromRow(row)
  logOpen.value = true
}

function fileNameFromRow(row: AppInstance) {
  const path = (row.artifactPath || '').replace(/\\/g, '/')
  return path.split('/').filter(Boolean).pop() || row.appCode || '日志'
}

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  {
    key: 'upload',
    label: '上传更新',
    icon: 'upload',
    code: 'fg0008:release',
    aiRisk: 'write',
    show: props.canRelease,
    onClick: () => openUpload(),
  },
  {
    key: 'upload-create',
    label: '上传新增',
    icon: 'plus',
    code: 'fg0008:release',
    aiRisk: 'write',
    show: props.canRelease,
    onClick: () => openCreate(),
  },
  {
    key: 'start',
    label: '启动',
    icon: 'play',
    code: 'fg0008:release',
    aiRisk: 'write',
    show: !!props.canRelease && !!props.canLifecycle,
    onClick: () => emit('lifecycle', 'start'),
  },
  {
    key: 'stop',
    label: '停止',
    icon: 'square',
    code: 'fg0008:release',
    aiRisk: 'write',
    show: !!props.canRelease && !!props.canLifecycle,
    onClick: () => emit('lifecycle', 'stop'),
  },
  {
    key: 'restart',
    label: '重启',
    icon: 'refresh-cw',
    code: 'fg0008:release',
    aiRisk: 'write',
    show: !!props.canRelease && !!props.canLifecycle,
    onClick: () => emit('lifecycle', 'restart'),
  },
  {
    key: 'startup-log',
    label: '启动日志',
    icon: 'scroll-text',
    code: 'fg0008:view',
    aiRisk: 'read',
    show: !!startupLogRow.value,
    onClick: () => openStartupLog(),
  },
  {
    key: 'download',
    label: props.downloadBusy ? '下载中…' : '下载',
    icon: 'download',
    code: 'fg0008:view',
    aiRisk: 'read',
    show: props.picked.length > 0 || props.downloadBusy,
    disabled: props.downloadBusy,
    onClick: () => emit('download'),
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

function openUpload() {
  if (!props.picked.length) {
    toast.error('请先勾选要更新的文件')
    return
  }
  if (!canUpload.value) {
    toast.error('请勾选同一应用的可更新制品（目录与日志请用下载）')
    return
  }
  uploadMode.value = 'update'
  uploadOpen.value = true
}

function openCreate() {
  uploadMode.value = 'create'
  uploadOpen.value = true
}

function rowSelectable(row: FileEntry) {
  return row.kind === 'file' && !!row.row
}

function onSelectedKeys(keys: string[]) {
  const instIds = entries.value
    .filter((e) => keys.includes(e.key) && e.kind === 'file' && e.row)
    .map((e) => e.row!.instId)
  if (!instIds.length) emit('clear-pick')
  else emit('pick-all', instIds)
}

function enterDir(relDir: string) {
  cwd.value = relDir
  emit('clear-pick')
}

function onCrumb(item: { key?: string; to?: string }, event: MouseEvent) {
  event.preventDefault()
  const path = item.key === 'root' ? '' : (item.key || '')
  enterDir(path)
}

function onRowClick(row: FileEntry) {
  if (row.kind === 'dir') enterDir(row.relDir)
}

function onRowDblclick(row: FileEntry) {
  if (row.kind === 'dir') {
    enterDir(row.relDir)
    return
  }
  if (canPreviewLog(row) && row.row) openLog(row)
}

function openLog(row: FileEntry) {
  if (!row.row) return
  logInstId.value = row.row.instId
  logFileName.value = row.name || row.row.artifactPath || '日志'
  logOpen.value = true
}

function onMenuClick(payload: { key: string; row: FileEntry }) {
  const row = payload.row
  if (!row) return
  if (payload.key === 'open' && row.kind === 'dir') {
    enterDir(row.relDir)
    return
  }
  if (payload.key === 'view-log' && row.row) {
    openLog(row)
    return
  }
  if (payload.key === 'download' && row.row) {
    emit('download-one', row.row.instId)
  }
}

function onSubmitUpload() {
  emit('release')
  uploadOpen.value = false
}

function onSubmitCreate(payload: { appCode: string; kind: string }) {
  emit('create', payload)
  uploadOpen.value = false
}

function entryOf(row: unknown) {
  return row as FileEntry
}

function typePreview(entry: FileEntry) {
  if (entry.kind === 'dir') return { badge: 'DIR', label: '目录', kind: 'dir' }
  if (!entry.row) return { badge: '?', label: '—', kind: '' }
  const preview = fileTypePreview(entry.row)
  return { ...preview, kind: entry.row.artifactKind }
}

function displayPath(entry: FileEntry) {
  if (entry.kind === 'dir') return entry.relDir || '—'
  if (!entry.row) return entry.relDir || '—'
  return relativeToBase(entry.row.artifactPath, props.site.basePath) || entry.name
}
</script>

<template>
  <section class="fg-file-browser">
    <FgToolbar plain wrap :title="title" :buttons="toolbarButtons" :module-id="moduleId">
      <template #after>
        <span class="fg-file-sub">{{ modeLabel(site.modeCode) }}</span>
      </template>
    </FgToolbar>

    <div class="fg-base-row" :title="site.basePath || '未上报 base 目录'">
      <span class="fg-base-label">Base</span>
      <code class="fg-base-path">{{ site.basePath || '—' }}</code>
    </div>

    <RsBreadcrumb class="fg-crumbs" :items="crumbs" @click="onCrumb" />

    <div v-if="downloadProgress" class="fg-dl-progress" :data-phase="downloadProgress.phase">
      <div class="fg-dl-head">
        <strong>{{ downloadProgress.fileName }}</strong>
        <span>{{ downloadProgress.message }}</span>
      </div>
      <div
        class="fg-dl-track"
        :class="{ indeterminate: progressIndeterminate }"
        role="progressbar"
        :aria-valuenow="progressPercent"
        aria-valuemin="0"
        aria-valuemax="100"
      >
        <div class="fg-dl-bar" :style="{ width: `${progressPercent}%` }" />
      </div>
    </div>

    <FgGrid
      class="fg-file-grid"
      :data="entries"
      :columns="columns"
      :loading="loading"
      row-key="key"
      selectable
      :selected-row-keys="selectedKeys"
      :row-selectable="rowSelectable"
      :module-id="moduleId"
      :menu-items="menuItems"
      @update:selected-row-keys="onSelectedKeys"
      @row-click="onRowClick"
      @row-dblclick="onRowDblclick"
      @menu-click="onMenuClick"
    >
      <template #type="{ row }">
        <span
          class="fg-file-badge"
          :data-kind="typePreview(entryOf(row)).kind"
          :title="typePreview(entryOf(row)).label"
        >{{ typePreview(entryOf(row)).badge }}</span>
      </template>
      <template #name="{ row }">
        <span class="fg-file-name">
          {{ entryOf(row).kind === 'dir' ? `${entryOf(row).name}/` : entryOf(row).name }}
        </span>
      </template>
      <template #appCode="{ row }">{{ entryOf(row).row?.appCode || '—' }}</template>
      <template #versionText="{ row }">{{ entryOf(row).row?.versionText || '—' }}</template>
      <template #path="{ row }">
        <span :title="entryOf(row).row?.artifactPath || displayPath(entryOf(row))">
          {{ displayPath(entryOf(row)) }}
        </span>
      </template>
      <template #online="{ row }">
        <RsTag
          v-if="entryOf(row).row"
          size="sm"
          :variant="entryOf(row).row!.onlineFlag === 'Y' ? 'success' : 'default'"
        >
          {{ entryOf(row).row!.onlineFlag === 'Y' ? '在线' : '离线' }}
        </RsTag>
        <span v-else class="fg-muted">—</span>
      </template>
    </FgGrid>

    <p class="fg-dir-hint">
      右键打开 / 查看日志 / 下载。已选 {{ picked.length }} 个
      <template v-if="releaseAppCode"> · {{ releaseAppCode }}</template>
      <template v-if="releaseKind"> · {{ kindLabel(releaseKind) }}</template>
      。
    </p>

    <UploadDialog
      v-if="canRelease"
      v-model:open="uploadOpen"
      :mode="uploadMode"
      :cwd="cwd"
      :picked-count="picked.length"
      :release-app-code="releaseAppCode"
      :release-kind="releaseKind"
      :version="version"
      :canary-count="canaryCount"
      :can-submit="uploadMode === 'create' ? true : canUpload && !!version.trim()"
      @update:version="emit('update:version', $event)"
      @update:canary-count="emit('update:canaryCount', $event)"
      @pick-file="emit('pick-file', $event)"
      @submit="onSubmitUpload"
      @create="onSubmitCreate"
    />
    <LogViewDialog
      v-model:open="logOpen"
      :inst-id="logInstId"
      :file-name="logFileName"
      :module-id="moduleId"
    />
  </section>
</template>

<style scoped>
.fg-file-browser {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  background: var(--fg-panel-bg, var(--rs-surface));
}

.fg-file-sub {
  margin-left: 10px;
  font-size: 12px;
  color: var(--rs-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-base-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 8px 12px 0;
}

.fg-base-label {
  flex: 0 0 auto;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--rs-text-secondary);
}

.fg-base-path {
  min-width: 0;
  margin: 0;
  padding: 2px 8px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.4;
  color: var(--rs-text);
  background: color-mix(in srgb, var(--rs-text-secondary) 10%, transparent);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-crumbs {
  padding: 6px 12px 0;
  font-size: 12px;
  overflow: hidden;
}

.fg-crumbs :deep(a),
.fg-crumbs :deep(span) {
  max-width: 280px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-dl-progress {
  margin: 8px 12px 0;
  padding: 8px 10px;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  border-radius: 6px;
  background: color-mix(in srgb, var(--rs-primary, #2563eb) 6%, transparent);
}

.fg-dl-progress[data-phase='failed'] {
  background: color-mix(in srgb, var(--rs-danger, #b91c1c) 8%, transparent);
}

.fg-dl-progress[data-phase='done'] {
  background: color-mix(in srgb, var(--rs-success, #15803d) 8%, transparent);
}

.fg-dl-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 6px 12px;
  margin-bottom: 6px;
  font-size: 12px;
}

.fg-dl-head strong {
  font-weight: 600;
}

.fg-dl-head span {
  color: var(--rs-text-secondary);
}

.fg-dl-track {
  position: relative;
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: color-mix(in srgb, var(--rs-text-secondary) 18%, transparent);
}

.fg-dl-bar {
  height: 100%;
  border-radius: inherit;
  background: var(--rs-primary, #2563eb);
  transition: width 0.25s ease;
}

.fg-dl-track.indeterminate .fg-dl-bar {
  width: 36% !important;
  animation: fg-dl-slide 1.2s ease-in-out infinite;
}

@keyframes fg-dl-slide {
  0% {
    transform: translateX(-120%);
  }
  100% {
    transform: translateX(320%);
  }
}

.fg-file-grid {
  flex: 1;
  min-height: 0;
  margin-top: 4px;
}

.fg-dir-hint {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin: 0;
  padding: 8px 12px 10px;
  font-size: 12px;
  color: var(--rs-text-secondary);
  line-height: 1.45;
  border-top: 1px solid var(--fg-panel-border, var(--rs-border));
}

.fg-file-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 40px;
  height: 22px;
  padding: 0 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: #0f172a;
  background: #e2e8f0;
}

.fg-file-badge[data-kind='war'] {
  background: #dbeafe;
  color: #1d4ed8;
}

.fg-file-badge[data-kind='jar'] {
  background: #ffedd5;
  color: #c2410c;
}

.fg-file-badge[data-kind='static'],
.fg-file-badge[data-kind='node'] {
  background: #dcfce7;
  color: #15803d;
}

.fg-file-badge[data-kind='script'] {
  background: #f3e8ff;
  color: #7e22ce;
}

.fg-file-badge[data-kind='dir'] {
  background: #fef3c7;
  color: #b45309;
}

.fg-file-badge[data-kind='log'] {
  background: #e2e8f0;
  color: #475569;
}

.fg-file-name {
  font-weight: 500;
}

.fg-muted {
  color: var(--rs-text-secondary);
  font-size: 12px;
}
</style>
