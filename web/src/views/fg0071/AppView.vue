<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgGridColumn } from '@/components/grid/types'
import { RsButton, RsTag, useRsToast } from '@/ui'
import {
  ignoreApp,
  queryApps,
  queryInstances,
  queryRelease,
  releaseApp,
  renameApp,
  type AppInstance,
  type AppRelease,
  type AppRow,
} from './api'

const moduleId = 'fg0071'
const toast = useRsToast()
const apps = ref<AppRow[]>([])
const instances = ref<AppInstance[]>([])
const appName = ref('')
const current = ref<AppRow | null>(null)
const picked = ref<string[]>([])
const loading = ref(false)
const error = ref('')
const kind = ref('war')
const version = ref('')
const file = ref<File | null>(null)
const releasing = ref<AppRelease | null>(null)

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
  { key: 'runtimeKind', title: '运行方式', width: 110 },
  { key: 'artifactKind', title: '制品', width: 90 },
  { key: 'versionText', title: '版本', minWidth: 110 },
  { key: 'tomcatVersion', title: 'Tomcat', minWidth: 110 },
  { key: 'runningFlag', title: '进程', width: 80 },
  { key: 'onlineFlag', title: '代理', width: 80 },
]

const sameKind = computed(() => {
  const rows = instances.value.filter((row) => picked.value.includes(row.instId))
  if (!rows.length) return ''
  const kind = rows[0].artifactKind
  return rows.every((row) => row.artifactKind === kind) ? kind : ''
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
  try {
    const res = await queryInstances(row.appCode)
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, '加载实例失败'))
      return
    }
    instances.value = parseApiData<AppInstance[]>(res) ?? []
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '加载实例失败'))
  }
}

function toggle(id: string) {
  picked.value = picked.value.includes(id) ? picked.value.filter((item) => item !== id) : [...picked.value, id]
}

function onFile(event: Event) {
  const input = event.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
}

async function submitRelease() {
  if (!current.value) return
  if (!picked.value.length) {
    toast.error('请选择实例')
    return
  }
  if (kind.value !== 'runtime' && kind.value !== sameKind.value) {
    toast.error('请选择同一种制品')
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
  releasing.value = next as AppRelease
  const pending = next.items.some((item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running')
  if (pending) window.setTimeout(() => void poll(), 2000)
}

async function renameCurrent() {
  if (!current.value) return
  const name = window.prompt('显示名', current.value.appName)
  if (!name || !name.trim()) return
  const res = await renameApp(current.value.appCode, name.trim())
  if (!isApiSuccess(res)) {
    toast.error(getApiMessage(res, '改名失败'))
    return
  }
  current.value = { ...current.value, appName: name.trim() }
  void loadApps()
}

async function ignoreCurrent() {
  if (!current.value) return
  if (!window.confirm('忽略后这个应用不再出现在列表里。')) return
  const res = await ignoreApp(current.value.appCode)
  if (!isApiSuccess(res)) {
    toast.error(getApiMessage(res, '忽略失败'))
    return
  }
  current.value = null
  instances.value = []
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
          <span>{{ current.appCode }}</span>
          <RsButton size="sm" @click="renameCurrent">改名</RsButton>
          <RsButton size="sm" @click="ignoreCurrent">忽略</RsButton>
        </div>
      </template>
      <FgGrid :data="instances" :columns="instColumns" row-key="instId" :module-id="moduleId">
        <template #pick="{ row }">
          <input type="checkbox" :checked="picked.includes(instOf(row).instId)" @change="toggle(instOf(row).instId)" />
        </template>
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
      <form class="fg-app-release" @submit.prevent="submitRelease">
        <select v-model="kind">
          <option value="war">WAR</option>
          <option value="jar">JAR</option>
          <option value="node">Node</option>
          <option value="script">脚本</option>
          <option value="static">静态资源</option>
          <option value="runtime">版本升级</option>
        </select>
        <input v-model="version" placeholder="目标版本" />
        <input type="file" @change="onFile" />
        <RsButton type="submit" size="sm">更新</RsButton>
      </form>
      <ul v-if="releasing" class="fg-app-progress">
        <li v-for="item in releasing.items" :key="item.commandId">
          {{ item.instId }} {{ statusText(item.status) }} {{ item.message }}
        </li>
      </ul>
    </FgCatalog>
  </div>
</template>

<style scoped>
.fg-app-search,
.fg-app-release {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px 12px;
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
.fg-app-progress {
  margin: 0;
  padding: 0 16px 12px;
  font-size: 12px;
}
</style>
