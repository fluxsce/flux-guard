import { computed, onMounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useButtonAllowed } from '@/components/buttonAccess'
import { rsConfirm, useRsToast } from '@/ui'
import {
  continueRelease,
  downloadFileFetch,
  queryFileFetch,
  queryRelease,
  queryReleases,
  querySiteFiles,
  querySites,
  releaseApp,
  requestFileFetch,
  rollbackRelease,
  type AppInstance,
  type AppRelease,
  type AppSite,
  type FileFetchJob,
} from '../api'
import { canRollbackRelease, kindLabel, moduleId, statusText, statusVariant, stepLabel } from './useAppCatalog'

export { kindLabel, moduleId, statusText, statusVariant, stepLabel, canRollbackRelease }

/** 文件名：优先路径末段，否则应用编码。 */
export function fileNameOf(row: AppInstance) {
  const path = (row.artifactPath || '').replace(/\\/g, '/')
  const base = path.split('/').filter(Boolean).pop()
  return base || row.appCode || '—'
}

/** 扩展名（小写，不含点）。 */
export function fileExtOf(row: AppInstance) {
  const name = fileNameOf(row)
  const i = name.lastIndexOf('.')
  if (i < 0) return ''
  return name.slice(i + 1).toLowerCase()
}

/** 文件类型预览：图标字母 + 说明。 */
export function fileTypePreview(row: AppInstance): { badge: string; label: string } {
  const kind = row.artifactKind
  const ext = fileExtOf(row)
  if (kind === 'dir') return { badge: 'DIR', label: '目录' }
  if (kind === 'log' || ext === 'log' || ext === 'txt' || ext === 'out') return { badge: 'LOG', label: '日志 / 文本' }
  if (kind === 'war' || ext === 'war') return { badge: 'WAR', label: 'Web 应用包' }
  if (kind === 'jar' || ext === 'jar') return { badge: 'JAR', label: 'Java 包' }
  if (kind === 'static' || ext === 'zip') return { badge: 'ZIP', label: '压缩包 / 静态' }
  if (kind === 'node') return { badge: 'NODE', label: 'Node 应用' }
  if (kind === 'script') return { badge: 'SH', label: '脚本应用' }
  return { badge: (ext || kind || '?').toUpperCase().slice(0, 4), label: kindLabel(kind) }
}

/** 模式显示名。 */
export function modeLabel(mode: string) {
  const map: Record<string, string> = {
    files: '制品目录',
    tomcat: 'Tomcat',
    node: 'Node',
    script: '脚本',
    jar: 'JAR',
    static: '静态',
  }
  return map[mode] || mode || '—'
}

function fileNameFromDisposition(header?: string) {
  if (!header) return ''
  const star = /filename\*=(?:UTF-8''|utf-8'')([^;]+)/i.exec(header)
  if (star?.[1]) {
    try {
      return decodeURIComponent(star[1].trim().replace(/^"+|"+$/g, ''))
    } catch {
      /* fall through */
    }
  }
  const plain = /filename="([^"]+)"|filename=([^;]+)/i.exec(header)
  const raw = (plain?.[1] || plain?.[2] || '').trim()
  if (!raw) return ''
  try {
    return decodeURIComponent(raw)
  } catch {
    return raw
  }
}

/** 应用管理：左运行时 · 右文件仓 · 下操作日志。 */
export function useAppWorkbench() {
  const toast = useRsToast()
  const allowed = useButtonAllowed()
  const sites = ref<AppSite[]>([])
  const files = ref<AppInstance[]>([])
  const releases = ref<AppRelease[]>([])
  const keyword = ref('')
  const current = ref<AppSite | null>(null)
  const picked = ref<string[]>([])
  const loading = ref(false)
  const filesLoading = ref(false)
  const error = ref('')
  const version = ref('')
  const canaryCount = ref(1)
  const file = ref<File | null>(null)
  const releasing = ref<AppRelease | null>(null)
  const continueBusy = ref(false)
  const downloadBusy = ref(false)
  const logLines = ref<string[]>([])

  const canRelease = computed(() => allowed(moduleId, 'release'))

  const selectedFiles = computed(() => files.value.filter((row) => picked.value.includes(row.instId)))

  const releaseKind = computed(() => {
    const rows = selectedFiles.value
    if (!rows.length) return ''
    const kind = rows[0].artifactKind
    return rows.every((row) => row.artifactKind === kind) ? kind : ''
  })

  const releaseAppCode = computed(() => {
    const rows = selectedFiles.value
    if (!rows.length) return ''
    const code = rows[0].appCode
    return rows.every((row) => row.appCode === code) ? code : ''
  })

  function pushLog(line: string) {
    const stamp = new Date().toLocaleTimeString()
    logLines.value = [`[${stamp}] ${line}`, ...logLines.value].slice(0, 200)
  }

  async function loadSites(kw?: string) {
    if (kw !== undefined) keyword.value = kw
    loading.value = true
    error.value = ''
    try {
      const res = await querySites(keyword.value.trim())
      if (!isApiSuccess(res)) {
        error.value = getApiMessage(res, '加载失败')
        return
      }
      sites.value = parseApiData<AppSite[]>(res) ?? []
      if (current.value) {
        const still = sites.value.find(
          (row) => row.agentId === current.value?.agentId && row.instanceName === current.value?.instanceName,
        )
        if (!still) {
          current.value = null
          files.value = []
          picked.value = []
        } else {
          current.value = still
        }
      }
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
    } finally {
      loading.value = false
    }
  }

  async function openSite(row: AppSite) {
    current.value = row
    picked.value = []
    releasing.value = null
    version.value = ''
    file.value = null
    pushLog(`打开运行时 ${row.instanceName}（${modeLabel(row.modeCode)}）`)
    await refreshFiles()
  }

  async function refreshFiles() {
    const site = current.value
    if (!site) return
    filesLoading.value = true
    try {
      const res = await querySiteFiles(site.agentId, site.instanceName)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '加载文件失败'))
        return
      }
      files.value = parseApiData<AppInstance[]>(res) ?? []
      const codes = [...new Set(files.value.map((row) => row.appCode))]
      if (codes.length === 1) {
        const relRes = await queryReleases(codes[0])
        releases.value = isApiSuccess(relRes) ? (parseApiData<AppRelease[]>(relRes) ?? []) : []
      } else {
        releases.value = []
      }
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '加载文件失败'))
    } finally {
      filesLoading.value = false
    }
  }

  function toggle(id: string) {
    picked.value = picked.value.includes(id) ? picked.value.filter((item) => item !== id) : [...picked.value, id]
  }

  /** 勾选给定实例（由表头「当前目录全选」传入）。 */
  function pickAll(instIds?: string[]) {
    if (instIds?.length) {
      picked.value = [...instIds]
      return
    }
    picked.value = files.value.filter((row) => row.artifactKind !== 'dir').map((row) => row.instId)
  }

  function clearPick() {
    picked.value = []
  }

  /** 右键或工具栏：按给定行代下（可多文件顺序拉取）。 */
  async function downloadRows(rows: AppInstance[]) {
    const list = rows.filter((row) => row.artifactKind !== 'dir')
    if (!list.length) {
      toast.error(rows.some((row) => row.artifactKind === 'dir')
        ? '目录请点进去再选文件下载（整目录打包后续再做）'
        : '请先勾选要下载的文件')
      return
    }
    if (downloadBusy.value) {
      toast.error('正在下载，请稍候')
      return
    }
    const offline = list.filter((row) => row.onlineFlag !== 'Y')
    if (offline.length) {
      toast.error(`代理离线，无法代下：${offline.map(fileNameOf).join('、')}`)
      return
    }
    downloadBusy.value = true
    let ok = 0
    try {
      for (const row of list) {
        const name = await fetchOneFile(row)
        if (name) ok += 1
      }
      if (ok > 0) {
        toast.success(ok === 1 ? `已下载 ${ok} 个文件` : `已下载 ${ok} / ${list.length} 个文件`)
      }
    } finally {
      downloadBusy.value = false
    }
  }

  async function downloadSelected() {
    await downloadRows(selectedFiles.value)
  }

  /** 右键单行下载。 */
  async function downloadOne(instId: string) {
    const row = files.value.find((item) => item.instId === instId)
    if (!row) {
      toast.error('文件不存在或已刷新')
      return
    }
    await downloadRows([row])
  }

  async function fetchOneFile(row: AppInstance): Promise<string> {
    const label = fileNameOf(row)
    try {
      pushLog(`开始代下 ${label}`)
      const start = await requestFileFetch(row.instId)
      if (!isApiSuccess(start)) {
        toast.error(getApiMessage(start, `代下失败：${label}`))
        pushLog(`代下失败 ${label}：${getApiMessage(start, '代下失败')}`)
        return ''
      }
      let job = parseApiData<FileFetchJob>(start)
      if (!job?.jobId) {
        toast.error(`未返回任务号：${label}`)
        return ''
      }
      if (job.mode === 'push') {
        pushLog(`中央拨不通，已改为代理出站推送（${label} · ${job.jobId}）`)
      } else if (job.status === 'ready') {
        pushLog(`短拨已就绪 ${label}`)
      }
      for (let i = 0; i < 90 && job.status === 'waiting'; i++) {
        await new Promise((r) => window.setTimeout(r, 2000))
        const st = await queryFileFetch(job.jobId)
        if (!isApiSuccess(st)) continue
        job = parseApiData<FileFetchJob>(st) ?? job
        if (job.status === 'failed') break
        if (job.status === 'waiting' && i % 5 === 4) {
          pushLog(`等待代理推送 ${label}… ${job.message || ''}`.trim())
        }
      }
      if (job.status !== 'ready') {
        const msg = job.message || '代下超时或失败'
        toast.error(`${label}：${msg}`)
        pushLog(`代下失败 ${label}：${msg}`)
        return ''
      }
      const res = await downloadFileFetch(job.jobId)
      const blob = res.data
      if (blob.type && blob.type.includes('json')) {
        const text = await blob.text()
        let msg = '下载失败'
        try {
          const parsed = JSON.parse(text) as { error?: string; message?: string }
          msg = parsed.error || parsed.message || msg
        } catch {
          /* ignore */
        }
        toast.error(`${label}：${msg}`)
        pushLog(`下载失败 ${label}：${msg}`)
        return ''
      }
      const name =
        fileNameFromDisposition(res.headers?.['content-disposition'] || res.headers?.['Content-Disposition']) ||
        job.fileName ||
        label
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = name
      a.click()
      URL.revokeObjectURL(url)
      pushLog(`已下载 ${name}`)
      return name
    } catch (err) {
      toast.error(getHttpErrorMessage(err, `下载失败：${label}`))
      pushLog(`下载失败 ${label}：${getHttpErrorMessage(err, '下载失败')}`)
      return ''
    }
  }

  function onPickFile(next: File | null) {
    file.value = next
    if (next) {
      pushLog(`已选择上传文件 ${next.name}`)
    }
  }

  async function submitRelease() {
    if (!current.value || !canRelease.value) return
    if (!picked.value.length) {
      toast.error('请选择要更新的文件')
      return
    }
    if (!releaseAppCode.value || !releaseKind.value) {
      toast.error('请勾选同一应用的同一种制品（一次更新一个应用）')
      return
    }
    if (releaseKind.value === 'dir' || releaseKind.value === 'log') {
      toast.error('目录与日志请用下载，不能走换包上传')
      return
    }
    if (!version.value.trim() || !file.value) {
      toast.error('请填写版本并选择本地文件')
      return
    }
    const body = new FormData()
    body.set('appCode', releaseAppCode.value)
    body.set('kind', releaseKind.value)
    body.set('version', version.value.trim())
    body.set('instIds', picked.value.join(','))
    body.set('canaryCount', String(canaryCount.value || 1))
    body.set('file', file.value)
    try {
      const res = await releaseApp(body)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '更新失败'))
        pushLog(`更新失败：${getApiMessage(res, '更新失败')}`)
        return
      }
      const next = parseApiData<AppRelease>(res)
      releasing.value = next
      const left = next?.pendingCount ?? 0
      const msg =
        left > 0 ? `已下发首批 ${(next?.items ?? []).length} 台，待继续 ${left} 台` : `已下发更新 ${releaseAppCode.value}`
      toast.success(msg)
      pushLog(msg)
      void poll()
      const relRes = await queryReleases(releaseAppCode.value)
      if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '更新失败'))
      pushLog(`更新失败：${getHttpErrorMessage(err, '更新失败')}`)
    }
  }

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
      const msg = `已继续下发，剩余 ${next.pendingCount ?? 0} 台`
      toast.success(msg)
      pushLog(msg)
      void poll()
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '继续下发失败'))
    } finally {
      continueBusy.value = false
    }
  }

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
      releasing.value = next
      const left = next?.pendingCount ?? 0
      const msg = left > 0 ? `已回滚首批 ${(next?.items ?? []).length} 台，待继续 ${left} 台` : '已下发回滚'
      toast.success(msg)
      pushLog(msg)
      void poll()
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '回滚失败'))
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
    for (const item of next.items ?? []) {
      if (item.status === 'done' || item.status === 'failed') {
        const label = files.value.find((f) => f.instId === item.instId)?.appCode || item.instId
        pushLog(`${label} · ${statusText(item.status)}${item.message ? ` · ${item.message}` : ''}`)
      }
    }
    const pending = (next.items ?? []).some(
      (item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running',
    )
    if (pending) {
      window.setTimeout(() => void poll(), 2000)
    } else {
      void refreshFiles()
    }
  }

  async function openRelease(row: AppRelease) {
    releasing.value = row
    pushLog(`查看发布 ${row.versionText}（${row.releaseId}）`)
    void poll()
  }

  function siteKey(row: AppSite) {
    return `${row.agentId}::${row.instanceName}`
  }

  onMounted(() => void loadSites())

  return {
    sites,
    files,
    releases,
    keyword,
    current,
    picked,
    loading,
    filesLoading,
    error,
    version,
    canaryCount,
    file,
    releasing,
    continueBusy,
    downloadBusy,
    logLines,
    canRelease,
    selectedFiles,
    releaseKind,
    releaseAppCode,
    loadSites,
    openSite,
    refreshFiles,
    toggle,
    pickAll,
    clearPick,
    downloadSelected,
    downloadOne,
    onPickFile,
    submitRelease,
    continueBatch,
    submitRollback,
    openRelease,
    siteKey,
    fileNameOf,
    fileTypePreview,
    modeLabel,
  }
}
