import { computed, onMounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useButtonAllowed } from '@/components/buttonAccess'
import type { FgFormField, FgSearchField } from '@/components/form/types'
import type { FgGridColumn } from '@/components/grid/types'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { rsConfirm, useRsToast } from '@/ui'
import {
  continueRelease,
  ignoreApp,
  queryApps,
  queryInstances,
  queryRelease,
  queryReleases,
  releaseApp,
  renameApp,
  rollbackRelease,
  type AppInstance,
  type AppRelease,
  type AppRow,
} from '../api'

export const moduleId = 'fg0008'

export const appSearchFields: FgSearchField[] = [
  { field: 'appName', label: '应用名', placeholder: '请输入应用名' },
]

export const appColumns: FgGridColumn[] = [
  { key: 'appName', title: '应用', minWidth: 160 },
  { key: 'artifactKind', title: '制品', width: 90 },
  { key: 'instanceCount', title: '实例', width: 80 },
  { key: 'versionText', title: '版本', minWidth: 120 },
  { key: 'onlineCount', title: '在线', width: 80 },
]

export const instColumns: FgGridColumn[] = [
  { key: 'pick', title: '', width: 48 },
  { key: 'hostName', title: '机器', minWidth: 120 },
  { key: 'ipAddress', title: '地址', minWidth: 120 },
  { key: 'instanceName', title: '实例', minWidth: 120 },
  { key: 'modeCode', title: '类型', width: 80 },
  { key: 'runtimeKind', title: '启停', width: 90 },
  { key: 'artifactKind', title: '制品', width: 80 },
  { key: 'versionText', title: '版本', minWidth: 100 },
  { key: 'artifactPath', title: '路径', minWidth: 180 },
  { key: 'commandAddress', title: '命令端口', minWidth: 140 },
  { key: 'tomcatVersion', title: 'Tomcat', minWidth: 100 },
  { key: 'runningFlag', title: '进程', width: 72 },
  { key: 'onlineFlag', title: '代理', width: 72 },
]

export const renameFields: FgFormField[] = [
  { field: 'appName', label: '显示名', type: 'input', required: true },
]

/** 列表版本展示：混版本时加「等」。 */
export function versionText(row: AppRow) {
  if (!row.versionText) return '—'
  return row.mixedVersion === 'Y' ? row.versionText + ' 等' : row.versionText
}

/** 发布状态文案。 */
export function statusText(status: string) {
  if (status === 'done') return '完成'
  if (status === 'failed') return '失败'
  if (status === 'waiting') return '待取'
  if (status === 'dialing') return '拨号'
  if (status === 'running') return '执行中'
  return status
}

/** 发布状态对应标签色。 */
export function statusVariant(status: string): 'success' | 'danger' | 'warning' | 'default' {
  if (status === 'done') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'waiting' || status === 'dialing' || status === 'running') return 'warning'
  return 'default'
}

/** 制品种类显示名。 */
export function kindLabel(value: string) {
  const map: Record<string, string> = {
    war: 'WAR',
    jar: 'JAR',
    node: 'Node',
    script: '脚本',
    static: '静态',
    runtime: '运行时',
    dir: '目录',
    log: '日志',
  }
  return map[value] || value
}

/** 发布步骤显示名。 */
export function stepLabel(name: string) {
  const map: Record<string, string> = {
    download: '下载制品',
    replace: '写入目标',
    rollback: '恢复备份',
    restart: '重启实例',
    startup: '启动探测',
    list: '列出目录',
    push: '推送文件',
  }
  return map[name] || name
}

/** 是否可从该发布回滚（正向更新且有成功备份）。 */
export function canRollbackRelease(row: AppRelease | null) {
  if (!row || row.purpose === 'rollback') return false
  return (row.items ?? []).some(
    (item) =>
      item.status === 'done' &&
      (item.steps ?? []).some((step) => step.name === 'replace' && step.status === 'done' && !!step.backupPath),
  )
}

/** 应用列表与详情的查询、发布、改名与忽略。 */
export function useAppCatalog() {
  const toast = useRsToast()
  const allowed = useButtonAllowed()
  const apps = ref<AppRow[]>([])
  const instances = ref<AppInstance[]>([])
  const releases = ref<AppRelease[]>([])
  const appName = ref('')
  const current = ref<AppRow | null>(null)
  const picked = ref<string[]>([])
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref('')
  const kind = ref('war')
  const version = ref('')
  const canaryCount = ref(1)
  const file = ref<File | null>(null)
  const releasing = ref<AppRelease | null>(null)
  const renameOpen = ref(false)
  const continueBusy = ref(false)

  const canEdit = computed(() => allowed(moduleId, 'edit'))
  const canRelease = computed(() => allowed(moduleId, 'release'))

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
        static: '静态资源 / 压缩包',
      }
      options.push({ value: sameKind.value, label: labels[sameKind.value] || sameKind.value })
    }
    if (canRuntime.value) options.push({ value: 'runtime', label: 'Tomcat 版本升级' })
    return options
  })

  const listToolbarButtons = computed<FgToolbarButton[]>(() => [
    {
      key: 'search',
      label: '刷新',
      icon: 'rotate-ccw',
      code: 'fg0008:search',
      aiRisk: 'read',
      onClick: () => void loadApps(),
    },
  ])

  /** 按应用名查询列表。 */
  async function loadApps(name?: string) {
    if (name !== undefined) appName.value = name
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

  /** 搜索表单提交。 */
  function onSearch(values: Record<string, unknown>) {
    void loadApps(String(values.appName ?? '').trim())
  }

  /** 重置搜索条件并重载。 */
  function onReset() {
    void loadApps('')
  }

  /** 打开应用详情并加载实例与发布记录。 */
  async function openApp(row: AppRow) {
    current.value = row
    picked.value = []
    releasing.value = null
    kind.value = row.artifactKind || 'war'
    version.value = ''
    file.value = null
    await refreshDetail()
  }

  /** 刷新当前应用的实例与发布历史。 */
  async function refreshDetail() {
    const row = current.value
    if (!row) return
    detailLoading.value = true
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
    } finally {
      detailLoading.value = false
    }
  }

  /** 退回应用列表。 */
  function backToList() {
    current.value = null
    instances.value = []
    releases.value = []
    releasing.value = null
    picked.value = []
  }

  /** 勾选或取消实例。 */
  function toggle(id: string) {
    picked.value = picked.value.includes(id) ? picked.value.filter((item) => item !== id) : [...picked.value, id]
  }

  /** 勾选全部在线实例。 */
  function pickOnline() {
    picked.value = instances.value.filter((row) => row.onlineFlag === 'Y').map((row) => row.instId)
  }

  /** 选择上传文件。 */
  function onFile(event: Event) {
    const input = event.target as HTMLInputElement
    file.value = input.files?.[0] ?? null
  }

  /** 下发更新到已选实例（默认金丝雀 1 台）。 */
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
    body.set('canaryCount', String(canaryCount.value || 1))
    body.set('file', file.value)
    try {
      const res = await releaseApp(body)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '更新失败'))
        return
      }
      const next = parseApiData<AppRelease>(res)
      if (!next) {
        toast.error('更新未返回发布信息')
        return
      }
      releasing.value = next
      const left = next.pendingCount ?? 0
      toast.success(left > 0 ? `已下发首批 ${(next.items ?? []).length} 台，待继续 ${left} 台` : '已下发')
      void poll()
      const relRes = await queryReleases(current.value.appCode)
      if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '更新失败'))
    }
  }

  /** 继续下发当前发布的下一批。 */
  async function continueBatch() {
    const id = releasing.value?.releaseId
    if (!id || !canRelease.value || continueBusy.value) return
    continueBusy.value = true
    try {
      const res = await continueRelease(id)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '继续下发失败'))
        return
      }
      const next = parseApiData<AppRelease>(res)
      if (!next) return
      releasing.value = next
      toast.success(`已继续下发，剩余 ${next.pendingCount ?? 0} 台`)
      void poll()
      if (current.value) {
        const relRes = await queryReleases(current.value.appCode)
        if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
      }
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '继续下发失败'))
    } finally {
      continueBusy.value = false
    }
  }

  /** 按当前查看的发布回滚已成功实例（默认金丝雀 1 台）。 */
  async function submitRollback() {
    const row = releasing.value
    if (!row || !canRelease.value || !canRollbackRelease(row)) return
    const ok = await rsConfirm.confirm({
      title: '回滚发布',
      description: `将把「${row.versionText}」已成功实例恢复到换包前的备份。默认先回滚 ${canaryCount.value || 1} 台。`,
      confirmText: '回滚',
      teleportTo: `#${moduleId}`,
    })
    if (!ok) return
    try {
      const res = await rollbackRelease(row.releaseId, [], canaryCount.value || 1)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '回滚失败'))
        return
      }
      const next = parseApiData<AppRelease>(res)
      if (!next) {
        toast.error('回滚未返回发布信息')
        return
      }
      releasing.value = next
      const left = next.pendingCount ?? 0
      toast.success(left > 0 ? `已回滚首批 ${(next.items ?? []).length} 台，待继续 ${left} 台` : '已下发回滚')
      void poll()
      if (current.value) {
        const relRes = await queryReleases(current.value.appCode)
        if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
      }
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '回滚失败'))
    }
  }

  /** 轮询当前发布进度。 */
  async function poll() {
    const id = releasing.value?.releaseId
    if (!id) return
    const res = await queryRelease(id)
    if (!isApiSuccess(res)) return
    const next = parseApiData<AppRelease>(res)
    if (!next) return
    releasing.value = next
    const pending = (next.items ?? []).some(
      (item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running',
    )
    if (pending) window.setTimeout(() => void poll(), 2000)
  }

  /** 查看某次历史发布的进度。 */
  async function openRelease(row: AppRelease) {
    releasing.value = row
    void poll()
  }

  /** 提交改名。 */
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

  /** 忽略当前应用并回列表。 */
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
    backToList()
    void loadApps()
  }

  /** 按实例编号找可读名。 */
  function instanceLabel(instId: string) {
    const row = instances.value.find((item) => item.instId === instId)
    if (!row) return instId
    const host = row.hostName || '—'
    const name = row.instanceName || row.instId
    return `${host} / ${name}`
  }

  onMounted(() => void loadApps())

  return {
    apps,
    instances,
    releases,
    appName,
    current,
    picked,
    loading,
    detailLoading,
    error,
    kind,
    version,
    canaryCount,
    file,
    releasing,
    renameOpen,
    continueBusy,
    canEdit,
    canRelease,
    sameKind,
    canRuntime,
    kindOptions,
    listToolbarButtons,
    loadApps,
    onSearch,
    onReset,
    openApp,
    refreshDetail,
    backToList,
    toggle,
    pickOnline,
    onFile,
    submitRelease,
    continueBatch,
    submitRollback,
    openRelease,
    submitRename,
    ignoreCurrent,
    instanceLabel,
  }
}
