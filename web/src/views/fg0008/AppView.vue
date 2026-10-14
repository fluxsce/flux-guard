<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useButtonAllowed } from '@/components/buttonAccess'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import type { FgFormField } from '@/components/form/types'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgGridColumn } from '@/components/grid/types'
import { RsButton, RsTag, rsConfirm, useRsToast } from '@/ui'
import {
  ignoreApp,
  queryApps,
  queryInstances,
  queryRelease,
  queryReleases,
  releaseApp,
  renameApp,
  type AppInstance,
  type AppRelease,
  type AppRow,
} from './api'

const moduleId = 'fg0008'
const toast = useRsToast()
const allowed = useButtonAllowed()
const apps = ref<AppRow[]>([])
const instances = ref<AppInstance[]>([])
const releases = ref<AppRelease[]>([])
const appName = ref('')
const current = ref<AppRow | null>(null)
const picked = ref<string[]>([])
const loading = ref(false)
const error = ref('')
const kind = ref('war')
const version = ref('')
const file = ref<File | null>(null)
const releasing = ref<AppRelease | null>(null)
const renameOpen = ref(false)

const canEdit = computed(() => allowed(moduleId, 'edit'))
const canRelease = computed(() => allowed(moduleId, 'release'))

const appColumns: FgGridColumn[] = [
  { key: 'appName', title: '应用', minWidth: 160 },
  { key: 'artifactKind', title: '制品', width: 90 },
  { key: 'instanceCount', title: '实例', width: 80 },
  { key: 'versionText', title: '版本', minWidth: 120 },
  { key: 'onlineCount', title: '在线', width: 80 },
]

const instColumns: FgGridColumn[] = [
  { key: 'pick', title: '', width: 48 },
  { key: 'hostName', title: '机器', minWidth: 140 },
  { key: 'instanceName', title: '实例', minWidth: 140 },
  { key: 'modeCode', title: '类型', width: 90 },
  { key: 'runtimeKind', title: '启停', width: 100 },
  { key: 'artifactKind', title: '制品', width: 90 },
  { key: 'versionText', title: '版本', minWidth: 110 },
  { key: 'tomcatVersion', title: 'Tomcat', minWidth: 110 },
  { key: 'runningFlag', title: '进程', width: 80 },
  { key: 'onlineFlag', title: '代理', width: 80 },
]

const renameFields: FgFormField[] = [
  { field: 'appName', label: '显示名', type: 'input', required: true },
]

const sameKind = computed(() => {
  const rows = instances.value.filter((row) => picked.value.includes(row.instId))
  if (!rows.length) return ''
  const value = rows[0].artifactKind
  return rows.every((row) => row.artifactKind === value) ? value : ''
})

const canRuntime = computed(() => {
  const rows = instances.value.filter((row) => picked.value.includes(row.instId))
  if (!rows.length) return false
  return rows.every((row) => {
    const mode = row.modeCode || (row.artifactKind === 'war' ? 'tomcat' : '')
    return mode === 'tomcat'
  })
})

const kindOptions = computed(() => {
  const options: { value: string; label: string }[] = []
  if (sameKind.value) {
    const labels: Record<string, string> = {
      war: 'WAR',
      jar: 'JAR',
      node: 'Node',
      script: '脚本',
      static: '静态资源',
    }
    options.push({ value: sameKind.value, label: labels[sameKind.value] || sameKind.value })
  }
  if (canRuntime.value) options.push({ value: 'runtime', label: 'Tomcat 版本升级' })
  return options
})

function versionText(row: AppRow) {
  if (!row.versionText) return '—'
  return row.mixedVersion === 'Y' ? row.versionText + ' 等' : row.versionText
}

async function loadApps() {
  loading.value = true
  error.value = ''
  try {
    const res = await queryApps(appName.value.trim())
    if (!isApiSuccess(res)) {
      error.value = getApiMessage(res, '加载失败')
      return
    }
    apps.value = parseApiData<AppRow[]>(res) ?? []
  } catch (err) {
    error.value = getHttpErrorMessage(err, '加载失败')
  } finally {
    loading.value = false
  }
}

async function openApp(row: AppRow) {
  current.value = row
  picked.value = []
  releasing.value = null
  kind.value = row.artifactKind || 'war'
  version.value = ''
  file.value = null
  try {
    const [instRes, relRes] = await Promise.all([queryInstances(row.appCode), queryReleases(row.appCode)])
    if (!isApiSuccess(instRes)) {
      toast.error(getApiMessage(instRes, '加载实例失败'))
      return
    }
    instances.value = parseApiData<AppInstance[]>(instRes) ?? []
    releases.value = isApiSuccess(relRes) ? (parseApiData<AppRelease[]>(relRes) ?? []) : []
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '加载实例失败'))
  }
}

function toggle(id: string) {
  picked.value = picked.value.includes(id) ? picked.value.filter((item) => item !== id) : [...picked.value, id]
}

function pickOnline() {
  picked.value = instances.value.filter((row) => row.onlineFlag === 'Y').map((row) => row.instId)
}

function onFile(event: Event) {
  const input = event.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
}

async function submitRelease() {
  if (!current.value || !canRelease.value) return
  if (!picked.value.length) {
    toast.error('请选择实例')
    return
  }
  if (kind.value !== 'runtime' && kind.value !== sameKind.value) {
    toast.error('请选择同一种制品')
    return
  }
  if (kind.value === 'runtime' && !canRuntime.value) {
    toast.error('版本升级只支持 Tomcat 实例')
    return
  }
  if (!version.value.trim() || !file.value) {
    toast.error('请填写版本并选择文件')
    return
  }
  const body = new FormData()
  body.set('appCode', current.value.appCode)
  body.set('kind', kind.value)
  body.set('version', version.value.trim())
  body.set('instIds', picked.value.join(','))
  body.set('file', file.value)
  try {
    const res = await releaseApp(body)
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '更新失败'))
      return
    }
    releasing.value = parseApiData<AppRelease>(res) ?? null
    toast.success('已下发')
    void poll()
    const relRes = await queryReleases(current.value.appCode)
    if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '更新失败'))
  }
}

async function poll() {
  const id = releasing.value?.releaseId
  if (!id) return
  const res = await queryRelease(id)
  if (!isApiSuccess(res)) return
  const next = parseApiData<AppRelease>(res)
  if (!next) return
  releasing.value = next
  const pending = (next.items ?? []).some((item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running')
  if (pending) window.setTimeout(() => void poll(), 2000)
}

async function openRelease(row: AppRelease) {
  releasing.value = row
  void poll()
}

async function submitRename(values: Record<string, unknown>) {
  if (!current.value) return
  const name = String(values.appName ?? '').trim()
  if (!name) {
    toast.error('请填写显示名')
    return
  }
  const res = await renameApp(current.value.appCode, name)
  if (!isApiSuccess(res)) {
    toast.error(getApiMessage(res, '改名失败'))
    return
  }
  current.value = { ...current.value, appName: name }
  renameOpen.value = false
  void loadApps()
}

async function ignoreCurrent() {
  if (!current.value || !canEdit.value) return
  const ok = await rsConfirm.confirm({
    title: '忽略应用',
    description: `忽略后「${current.value.appName}」不再出现在列表里。`,
    confirmText: '忽略',
    teleportTo: `#${moduleId}`,
  })
  if (!ok) return
  const res = await ignoreApp(current.value.appCode)
  if (!isApiSuccess(res)) {
    toast.error(getApiMessage(res, '忽略失败'))
    return
  }
  current.value = null
  instances.value = []
  releases.value = []
  void loadApps()
}

function instOf(row: unknown) {
  return row as AppInstance
}

function statusText(status: string) {
  if (status === 'done') return '完成'
  if (status === 'failed') return '失败'
  if (status === 'waiting') return '待取'
  if (status === 'dialing') return '拨号'
  if (status === 'running') return '执行中'
  return status
}

function kindLabel(value: string) {
  const map: Record<string, string> = {
    war: 'WAR',
    jar: 'JAR',
    node: 'Node',
    script: '脚本',
    static: '静态',
    runtime: '运行时',
  }
  return map[value] || value
}

onMounted(() => void loadApps())

useModuleAi(() => ({
  moduleId,
  title: '应用管理',
  snapshot: () => ({
    app: current.value?.appCode ?? '',
    picked: picked.value.length,
    release: releasing.value?.releaseId ?? '',
  }),
}))
</script>

<template>
  <div class="fg-page" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <FgCatalog v-if="!current">
      <template #search>
        <form class="fg-app-search" @submit.prevent="loadApps">
          <input v-model="appName" placeholder="应用名" />
          <RsButton type="submit" size="sm">查询</RsButton>
        </form>
      </template>
      <p v-if="error" class="fg-catalog-error">{{ error }}</p>
      <FgGrid :data="apps" :columns="appColumns" :loading="loading" row-key="appCode" :module-id="moduleId" @row-click="openApp($event)">
        <template #versionText="{ row }">{{ versionText(row as AppRow) }}</template>
      </FgGrid>
    </FgCatalog>
    <FgCatalog v-else>
      <template #search>
        <div class="fg-app-search">
          <RsButton size="sm" variant="ghost" @click="current = null">返回</RsButton>
          <strong>{{ current.appName }}</strong>
          <span class="fg-app-code">{{ current.appCode }}</span>
          <RsButton v-if="canEdit" size="sm" @click="renameOpen = true">改名</RsButton>
          <RsButton v-if="canEdit" size="sm" @click="ignoreCurrent">忽略</RsButton>
          <RsButton size="sm" variant="ghost" @click="pickOnline">全选在线</RsButton>
        </div>
      </template>
      <FgGrid :data="instances" :columns="instColumns" row-key="instId" :module-id="moduleId">
        <template #pick="{ row }">
          <input type="checkbox" :checked="picked.includes(instOf(row).instId)" @change="toggle(instOf(row).instId)" />
        </template>
        <template #modeCode="{ row }">{{ instOf(row).modeCode || '—' }}</template>
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
      <form v-if="canRelease" class="fg-app-release" @submit.prevent="submitRelease">
        <select v-model="kind">
          <option v-for="item in kindOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select>
        <input v-model="version" placeholder="目标版本" />
        <input type="file" @change="onFile" />
        <RsButton type="submit" size="sm">更新</RsButton>
      </form>
      <ul v-if="releasing?.items?.length" class="fg-app-progress">
        <li v-for="item in releasing.items" :key="item.commandId">
          {{ item.instId }} {{ statusText(item.status) }} {{ item.message }}
        </li>
      </ul>
      <div v-if="releases.length" class="fg-app-history">
        <h4>最近发布</h4>
        <ul>
          <li v-for="item in releases" :key="item.releaseId">
            <button type="button" class="fg-app-history-link" @click="openRelease(item)">
              {{ item.addTime || '—' }} · {{ kindLabel(item.artifactKind) }} · {{ item.versionText }}
            </button>
            <span>{{ item.fileName }}</span>
          </li>
        </ul>
      </div>
    </FgCatalog>
    <FgDataFormModal
      v-model:open="renameOpen"
      title="修改显示名"
      mode="edit"
      :fields="renameFields"
      :initial-data="{ appName: current?.appName ?? '' }"
      :module-id="moduleId"
      save-action="edit"
      :teleport-to="`#${moduleId}`"
      :auto-close="false"
      width="420px"
      @submit="submitRename"
    />
  </div>
</template>

<style scoped>
.fg-app-search,
.fg-app-release {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px 12px;
  flex-wrap: wrap;
}
.fg-app-code {
  opacity: 0.7;
  font-size: 12px;
}
.fg-app-search input,
.fg-app-release input,
.fg-app-release select {
  height: 32px;
  padding: 0 8px;
  border: 1px solid var(--rs-border);
  border-radius: 6px;
  background: transparent;
  color: inherit;
}
.fg-app-progress,
.fg-app-history {
  margin: 0;
  padding: 0 16px 12px;
  font-size: 12px;
}
.fg-app-history h4 {
  margin: 8px 0;
  font-size: 13px;
  font-weight: 600;
}
.fg-app-history ul {
  margin: 0;
  padding: 0;
  list-style: none;
}
.fg-app-history li {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 4px 0;
}
.fg-app-history-link {
  border: 0;
  background: transparent;
  color: var(--rs-primary, #2563eb);
  cursor: pointer;
  padding: 0;
}
</style>
