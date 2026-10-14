<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgGridColumn, FgGridMenuItem } from '@/components/grid/types'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsBreadcrumb, RsTag, useRsToast } from '@/ui'
import type { AppInstance, AppSite } from '../api'
import { breadcrumbOf, entriesInDir, type FileEntry } from '../composables/fileTree'
import { fileTypePreview, kindLabel, modeLabel, moduleId } from '../composables/useAppWorkbench'
import UploadDialog from './UploadDialog.vue'

const props = defineProps<{
  site: AppSite
  files: AppInstance[]
  picked: string[]
  loading?: boolean
  downloadBusy?: boolean
  canRelease: boolean
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
}>()

const toast = useRsToast()
const cwd = ref('')
const uploadOpen = ref(false)

watch(
  () => props.site.agentId + props.site.instanceName,
  () => {
    cwd.value = ''
  },
)

const title = computed(() => props.site.instanceName)
const crumbs = computed(() =>
  breadcrumbOf(cwd.value).map((c) => ({
    key: c.path || 'root',
    label: c.label,
    to: c.path === '' ? '#' : `#${c.path}`,
  })),
)
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

const menuItems = computed<FgGridMenuItem[]>(() => [
  {
    key: 'open',
    label: '打开',
    icon: 'folder',
    disabled: (row: FileEntry) => row.kind !== 'dir',
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

function onMenuClick(payload: { key: string; row: FileEntry }) {
  const row = payload.row
  if (!row) return
  if (payload.key === 'open' && row.kind === 'dir') {
    enterDir(row.relDir)
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

function entryOf(row: unknown) {
  return row as FileEntry
}

function typePreview(entry: FileEntry) {
  if (entry.kind === 'dir') return { badge: 'DIR', label: '目录', kind: 'dir' }
  if (!entry.row) return { badge: '?', label: '—', kind: '' }
  const preview = fileTypePreview(entry.row)
  return { ...preview, kind: entry.row.artifactKind }
}
</script>

<template>
  <section class="fg-file-browser">
    <FgToolbar plain wrap :title="title" :buttons="toolbarButtons" :module-id="moduleId">
      <template #after>
        <span class="fg-file-sub">
          {{ modeLabel(site.modeCode) }}
          <template v-if="site.basePath"> · {{ site.basePath }}</template>
        </span>
      </template>
    </FgToolbar>

    <RsBreadcrumb class="fg-crumbs" :items="crumbs" @click="onCrumb" />

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
        {{ entryOf(row).row?.artifactPath || entryOf(row).relDir || '—' }}
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
      右键复制 / 打开 / 下载。已选 {{ picked.length }} 个
      <template v-if="releaseAppCode"> · {{ releaseAppCode }}</template>
      <template v-if="releaseKind"> · {{ kindLabel(releaseKind) }}</template>
      。
    </p>

    <UploadDialog
      v-if="canRelease"
      v-model:open="uploadOpen"
      :picked-count="picked.length"
      :release-app-code="releaseAppCode"
      :release-kind="releaseKind"
      :version="version"
      :canary-count="canaryCount"
      :can-submit="canUpload && !!version.trim()"
      @update:version="emit('update:version', $event)"
      @update:canary-count="emit('update:canaryCount', $event)"
      @pick-file="emit('pick-file', $event)"
      @submit="onSubmitUpload"
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

.fg-crumbs {
  padding: 8px 12px 0;
  font-size: 12px;
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
