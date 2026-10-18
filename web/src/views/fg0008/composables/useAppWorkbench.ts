import { computed, onMounted, onUnmounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useButtonAllowed } from '@/components/buttonAccess'
import { rsConfirm, useRsToast } from '@/ui'
import {
  continueRelease,
  queryCommand,
  queryRuntimeCommands,
  queryFileFetch,
  triggerFileFetchDownload,
  queryRelease,
  queryReleases,
  querySiteFiles,
  querySites,
  releaseApp,
  requestFileFetch,
  rollbackRelease,
  runtimeControl,
  type AppInstance,
  type AppRelease,
  type AppSite,
  type DownloadProgress,
  type FileFetchJob,
  type RuntimeCommand,
} from '../api'
import { startupLogOf } from './startupLog'
import { useLogFollow } from './useLogFollow'
import { canRollbackRelease, kindLabel, moduleId, statusText, statusVariant, stepLabel } from './useAppCatalog'

export { kindLabel, moduleId, statusText, statusVariant, stepLabel, canRollbackRelease }

/** 启动探测详情的第一行是结论，其余是日志摘录。 */
function startupExcerpt(detail?: string) {
  const text = (detail || '').replace(/\r\n/g, '\n').trim()
  const split = text.indexOf('\n')
  if (split < 0) return ''
  return text.slice(split + 1).trim()
}

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

/** 从文件名推断应用编号、种类与版本（对齐代理 AppCode / FileKind）。 */
export function inferArtifactFromFile(fileName: string, cwd = '') {
  const base = fileName.split(/[/\\]/).pop() || fileName
  const dot = base.lastIndexOf('.')
  const ext = dot >= 0 ? base.slice(dot).toLowerCase() : ''
  const stem = dot >= 0 ? base.slice(0, dot) : base
  let appCode = stem
  let version = ''
  const hash = stem.indexOf('##')
  if (hash >= 0) {
    appCode = stem.slice(0, hash)
    version = stem.slice(hash + 2)
  }
  let kind = ''
  switch (ext) {
    case '.war':
      kind = 'war'
      break
    case '.jar':
      kind = 'jar'
      break
    case '.zip':
      kind = 'static'
      break
    default:
      kind = ''
  }
  const prefix = cwd.replace(/\\/g, '/').replace(/^\/+|\/+$/g, '')
  if (prefix && appCode) {
    appCode = `${prefix}/${appCode}`
  }
  return { appCode, kind, version }
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
  const downloadProgress = ref<DownloadProgress | null>(null)
  const logLines = ref<string[]>([])
  const follow = useLogFollow()
  const lifecycle = ref<RuntimeCommand | null>(null)
  const runtimeHistory = ref<RuntimeCommand[]>([])
  /** 页面卸载后停止代下轮询与发布 poll，避免空跑请求。 */
  let pageAlive = true
  let downloadAbort: AbortController | null = null

  const canRelease = computed(() => allowed(moduleId, 'release'))

  function sleep(ms: number, signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
      if (signal?.aborted) {
        reject(new DOMException('aborted', 'AbortError'))
        return
      }
      const timer = window.setTimeout(() => {
        signal?.removeEventListener('abort', onAbort)
        resolve()
      }, ms)
      const onAbort = () => {
        window.clearTimeout(timer)
        reject(new DOMException('aborted', 'AbortError'))
      }
      signal?.addEventListener('abort', onAbort, { once: true })
    })
  }

  function formatBytes(n: number) {
    if (!n || n < 0) return '0 B'
    if (n < 1024) return `${n} B`
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
    return `${(n / (1024 * 1024)).toFixed(1)} MB`
  }

  function jobPercent(job: FileFetchJob) {
    const total = job.bytesTotal || 0
    const got = job.bytesReceived || 0
    if (total > 0) return Math.min(99, Math.round((got / total) * 100))
    return 0
  }

  function setDownloadProgress(next: DownloadProgress | null) {
    downloadProgress.value = next
  }

  function patchDownloadProgress(partial: Partial<DownloadProgress>) {
    const cur = downloadProgress.value
    if (!cur) return
    downloadProgress.value = { ...cur, ...partial }
  }

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
    void follow.stop()
    lifecycle.value = null
    runtimeHistory.value = []
    pushLog(`打开运行时 ${row.instanceName}（${modeLabel(row.modeCode)}）`)
    await refreshFiles()
    await loadRuntimeHistory()
  }

  function onSiteNoteSaved(row: AppSite) {
    const i = sites.value.findIndex(
      (item) => item.agentId === row.agentId && item.instanceName === row.instanceName,
    )
    if (i >= 0) sites.value[i] = { ...sites.value[i], noteText: row.noteText || '' }
    if (
      current.value &&
      current.value.agentId === row.agentId &&
      current.value.instanceName === row.instanceName
    ) {
      current.value = { ...current.value, noteText: row.noteText || '' }
    }
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
      // 日志 / 目录不参与发版；Tomcat 同时上报 logs 时仍按 WAR 等制品查最近发布。
      const codes = [
        ...new Set(
          files.value
            .filter((row) => {
              const kind = row.artifactKind
              return kind !== 'dir' && kind !== 'log' && kind !== 'runtime'
            })
            .map((row) => row.appCode)
            .filter(Boolean),
        ),
      ]
      if (codes.length >= 1) {
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
    downloadAbort?.abort()
    downloadAbort = new AbortController()
    const signal = downloadAbort.signal
    downloadBusy.value = true
    let ok = 0
    try {
      for (const row of list) {
        if (!pageAlive || signal.aborted) break
        const name = await fetchOneFile(row, signal)
        if (name) ok += 1
      }
      if (ok > 0 && pageAlive) {
        toast.success(ok === 1 ? `已下载 ${ok} 个文件` : `已下载 ${ok} / ${list.length} 个文件`)
      }
    } finally {
      downloadBusy.value = false
      setDownloadProgress(null)
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

  async function fetchOneFile(row: AppInstance, signal: AbortSignal): Promise<string> {
    const label = fileNameOf(row)
    let lastLogPct = -1
    let lastWaitLogAt = 0
    try {
      if (signal.aborted) return ''
      setDownloadProgress({
        fileName: label,
        phase: 'request',
        percent: 0,
        bytesReceived: 0,
        bytesTotal: 0,
        message: '发起代下…',
      })
      pushLog(`开始代下 ${label}`)
      const start = await requestFileFetch(row.instId)
      if (signal.aborted) return ''
      if (!isApiSuccess(start)) {
        const msg = getApiMessage(start, '代下失败')
        toast.error(`代下失败：${label}`)
        pushLog(`代下失败 ${label}：${msg}`)
        patchDownloadProgress({ phase: 'failed', message: msg })
        return ''
      }
      let job = parseApiData<FileFetchJob>(start)
      if (!job?.jobId) {
        toast.error(`未返回任务号：${label}`)
        patchDownloadProgress({ phase: 'failed', message: '未返回任务号' })
        return ''
      }
      if (job.mode === 'push') {
        pushLog(`中央拨不通，已改为代理出站推送（${label} · ${job.jobId}）`)
        patchDownloadProgress({
          phase: 'queued',
          mode: 'push',
          message: '等待代理出站推送…',
        })
      } else if (job.status === 'ready') {
        pushLog(`短拨已就绪 ${label}`)
      } else {
        pushLog(`短拨拉取中 ${label}${job.bytesTotal ? ` · ${formatBytes(job.bytesTotal)}` : ''}`)
        patchDownloadProgress({
          phase: 'transferring',
          mode: 'dial',
          bytesTotal: job.bytesTotal || 0,
          message: '中央短拨拉取中…',
        })
      }

      // 1GiB 级传输可能数十分钟；与中央 AppFileDownloadTimeout（60m）对齐。
      const startedAt = Date.now()
      const deadline = startedAt + 60 * 60 * 1000
      while (Date.now() < deadline && job.status === 'waiting') {
        await sleep(2000, signal)
        if (signal.aborted || !pageAlive) {
          pushLog(`代下已停止 ${label}`)
          return ''
        }
        const st = await queryFileFetch(job.jobId)
        if (signal.aborted || !pageAlive) {
          pushLog(`代下已停止 ${label}`)
          return ''
        }
        if (!isApiSuccess(st)) continue
        job = parseApiData<FileFetchJob>(st) ?? job
        if (job.status === 'failed') break

        const pct = jobPercent(job)
        const phase =
          job.phase === 'transferring' || (job.bytesReceived || 0) > 0 ? 'transferring' : 'queued'
        const sizeText =
          job.bytesTotal && job.bytesTotal > 0
            ? `${formatBytes(job.bytesReceived || 0)} / ${formatBytes(job.bytesTotal)}`
            : job.bytesReceived
              ? formatBytes(job.bytesReceived)
              : ''
        const msg =
          phase === 'queued'
            ? job.message || (job.mode === 'push' ? '等待代理出站推送…' : '准备拉取…')
            : `传输中${sizeText ? ` · ${sizeText}` : ''}${pct ? ` · ${pct}%` : ''}`
        patchDownloadProgress({
          phase,
          mode: job.mode,
          percent: pct,
          bytesReceived: job.bytesReceived || 0,
          bytesTotal: job.bytesTotal || 0,
          message: msg,
        })

        // 有百分比时按 10% 打点；否则每 10 秒写一条等待日志。
        if (pct > 0 && pct >= lastLogPct + 10) {
          lastLogPct = pct - (pct % 10)
          pushLog(`代下进度 ${label} · ${pct}%${sizeText ? ` · ${sizeText}` : ''}`)
        } else if (Date.now() - lastWaitLogAt >= 10000) {
          lastWaitLogAt = Date.now()
          const elapsed = Math.round((Date.now() - startedAt) / 1000)
          pushLog(
            `${phase === 'queued' ? '等待推送' : '传输中'} ${label} · ${elapsed}s${sizeText ? ` · ${sizeText}` : ''}${job.message ? ` · ${job.message}` : ''}`,
          )
        }
      }
      if (signal.aborted || !pageAlive) {
        pushLog(`代下已停止 ${label}`)
        return ''
      }
      if (job.status !== 'ready') {
        const msg = job.message || '代下超时或失败'
        toast.error(`${label}：${msg}`)
        pushLog(`代下失败 ${label}：${msg}`)
        patchDownloadProgress({ phase: 'failed', message: msg })
        return ''
      }

      const name = job.fileName || label
      const total = job.bytesTotal || job.bytesReceived || 0
      patchDownloadProgress({
        phase: 'saving',
        percent: 100,
        bytesReceived: total,
        bytesTotal: total,
        message: '已交给浏览器流式保存（不占用页面内存）…',
      })
      pushLog(
        `暂存已就绪，浏览器流式下载 ${name}${total ? ` · ${formatBytes(total)}` : ''}`,
      )
      // Cookie 会话鉴权；浏览器直链流式落盘，避免 axios blob 把整文件堆进内存。
      triggerFileFetchDownload(job.jobId, name)
      pushLog(`已开始本机保存 ${name}${total ? ` · ${formatBytes(total)}` : ''}`)
      patchDownloadProgress({
        phase: 'done',
        percent: 100,
        bytesReceived: total,
        bytesTotal: total,
        message: '已开始本机保存',
        fileName: name,
      })
      return name
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') {
        pushLog(`代下已停止 ${label}`)
        return ''
      }
      const msg = getHttpErrorMessage(err, '下载失败')
      toast.error(`${msg}：${label}`)
      pushLog(`下载失败 ${label}：${msg}`)
      patchDownloadProgress({ phase: 'failed', message: msg })
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
      if (!next) {
        toast.error('更新未返回发布信息')
        return
      }
      releasing.value = next
      const left = next.pendingCount ?? 0
      const msg =
        left > 0 ? `已下发首批 ${(next.items ?? []).length} 台，待继续 ${left} 台` : `已下发更新 ${releaseAppCode.value}`
      toast.success(msg)
      pushLog(msg)
      pushLog('处理顺序：上传到中央 → 代理下载并校验 → 写入目标并备份 → 需要时重启 → 启动探测')
      void poll()
      const relRes = await queryReleases(releaseAppCode.value)
      if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '更新失败'))
      pushLog(`更新失败：${getHttpErrorMessage(err, '更新失败')}`)
    }
  }

  /** 当前站点是否支持进程启停（files / static 无进程）。 */
  const canLifecycle = computed(() => {
    const mode = current.value?.modeCode || ''
    return mode !== '' && mode !== 'files' && mode !== 'static'
  })

  async function submitLifecycle(op: 'start' | 'stop' | 'restart') {
    if (!current.value || !canRelease.value || !canLifecycle.value) return
    const site = current.value
    const label = op === 'start' ? '启动' : op === 'stop' ? '停止' : '重启'
    const ok = await rsConfirm.confirm({
      title: `${label}运行时`,
      description: `将对「${site.instanceName}」执行${label}（按代理本机 systemd / Windows 服务 / docker / 命令配置）。`,
      confirmText: label,
      teleportTo: `#${moduleId}`,
    })
    if (!ok) return
    try {
      const res = await runtimeControl(site.agentId, site.instanceName, op)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, `${label}失败`))
        pushLog(`${label}失败：${getApiMessage(res, `${label}失败`)}`)
        return
      }
      const cmd = parseApiData<RuntimeCommand>(res)
      if (!cmd?.commandId) {
        toast.error(`${label}未返回命令`)
        return
      }
      pushLog(`已下发${label} ${site.instanceName}`)
      toast.success(`已下发${label}`)
      lifecycle.value = {
        commandId: cmd.commandId,
        agentId: site.agentId,
        action: op === 'start' ? 'app.runtime.start' : op === 'stop' ? 'app.runtime.stop' : 'app.runtime.restart',
        status: 'running',
        addTime: cmd.addTime || new Date().toISOString(),
      }
      if (op !== 'stop') void followStartupLog(site.instanceName)
      void pollCommand(cmd.commandId, label)
    } catch (err) {
      toast.error(getHttpErrorMessage(err, `${label}失败`))
      pushLog(`${label}失败：${getHttpErrorMessage(err, `${label}失败`)}`)
    }
  }

  async function pollCommand(commandId: string, label: string) {
    if (!pageAlive) return
    const res = await queryCommand(commandId)
    if (!pageAlive || !isApiSuccess(res)) return
    const cmd = parseApiData<RuntimeCommand>(res)
    if (!cmd) return
    lifecycle.value = cmd
    if (cmd.status === 'done' || cmd.status === 'failed') {
      pushLog(`${label} · ${statusText(cmd.status)}${cmd.message ? ` · ${cmd.message}` : ''}`)
      const startup = (cmd.steps ?? []).find((step) => step.name === 'startup')
      const verdict = (startup?.detail || '').split('\n')[0]?.trim()
      if (verdict) pushLog(verdict)
      // 右侧已在跟读原文时，不再把同一段日志贴进左侧事件。
      if (!follow.text.value) {
        const excerpt = startupExcerpt(startup?.detail)
        if (excerpt) pushLog(excerpt)
      }
      follow.releaseSoon()
      if (cmd.status === 'done') void refreshFiles()
      void loadRuntimeHistory()
      return
    }
    window.setTimeout(() => void pollCommand(commandId, label), 2000)
  }

  /** 在当前运行时下新增制品（不勾选已有文件）。 */
  async function submitCreate(appCode: string, kind: string) {
    if (!current.value || !canRelease.value) return
    appCode = appCode.trim()
    kind = kind.trim()
    if (!appCode || !kind) {
      toast.error('请填写应用编号与制品种类')
      return
    }
    if (kind === 'dir' || kind === 'log') {
      toast.error('目录与日志不能走新增上传')
      return
    }
    if (!version.value.trim() || !file.value) {
      toast.error('请填写版本并选择本地文件')
      return
    }
    const body = new FormData()
    body.set('purpose', 'create')
    body.set('agentId', current.value.agentId)
    body.set('instanceName', current.value.instanceName)
    body.set('appCode', appCode)
    body.set('kind', kind)
    body.set('version', version.value.trim())
    body.set('file', file.value)
    try {
      const res = await releaseApp(body)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '新增失败'))
        pushLog(`新增失败：${getApiMessage(res, '新增失败')}`)
        return
      }
      const next = parseApiData<AppRelease>(res)
      if (!next) {
        toast.error('新增未返回发布信息')
        return
      }
      releasing.value = next
      const msg = `已下发新增 ${appCode}`
      toast.success(msg)
      pushLog(msg)
      pushLog('处理顺序：上传到中央 → 代理下载并校验 → 写入目标 → 需要时重启 → 启动探测')
      void poll()
      const relRes = await queryReleases(appCode)
      if (isApiSuccess(relRes)) releases.value = parseApiData<AppRelease[]>(relRes) ?? []
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '新增失败'))
      pushLog(`新增失败：${getHttpErrorMessage(err, '新增失败')}`)
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
      if (!next) {
        toast.error('回滚未返回发布信息')
        return
      }
      releasing.value = next
      const left = next.pendingCount ?? 0
      const msg = left > 0 ? `已回滚首批 ${(next.items ?? []).length} 台，待继续 ${left} 台` : '已下发回滚'
      toast.success(msg)
      pushLog(msg)
      pushLog('处理顺序：把备份移回目标 → 需要时重启 → 启动探测')
      void poll()
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '回滚失败'))
    }
  }

  const loggedCommands = new Set<string>()
  let releaseFollowId = ''

  function logReleaseSteps(label: string, item: { steps?: { name: string; status: string; detail?: string; targetPath?: string; backupPath?: string }[] }) {
    for (const step of item.steps ?? []) {
      const lead = (step.detail || '').split('\n')[0]?.trim()
      const paths = [
        step.targetPath ? `目标 ${step.targetPath}` : '',
        step.backupPath ? `备份 ${step.backupPath}` : '',
      ].filter(Boolean)
      const text = [stepLabel(step.name), statusText(step.status), lead, ...paths].filter(Boolean).join(' · ')
      pushLog(`${label} · ${text}`)
    }
  }

  function followReleaseLogs(releaseId: string) {
    if (releaseFollowId === releaseId) return
    const mode = current.value?.modeCode || ''
    if (mode === '' || mode === 'files' || mode === 'static') return
    releaseFollowId = releaseId
    if (current.value) void followStartupLog(current.value.instanceName)
  }

  async function poll() {
    if (!pageAlive) return
    const id = releasing.value?.releaseId
    if (!id) return
    const res = await queryRelease(id)
    if (!pageAlive || !isApiSuccess(res)) return
    const next = parseApiData<AppRelease>(res)
    if (!next) return
    releasing.value = next
    for (const item of next.items ?? []) {
      if ((item.status === 'done' || item.status === 'failed') && !loggedCommands.has(item.commandId)) {
        loggedCommands.add(item.commandId)
        const label = files.value.find((f) => f.instId === item.instId)?.appCode || item.instId
        pushLog(`${label} · ${statusText(item.status)}${item.message ? ` · ${item.message}` : ''}`)
        logReleaseSteps(label, item)
      }
    }
    const pending = (next.items ?? []).some(
      (item) => item.status === 'waiting' || item.status === 'dialing' || item.status === 'running',
    )
    if (pending) {
      followReleaseLogs(id)
      window.setTimeout(() => void poll(), 2000)
    } else {
      if (releaseFollowId === id) follow.releaseSoon()
      void refreshFiles()
    }
  }

  async function openRelease(row: AppRelease) {
    releasing.value = row
    pushLog(`查看发布 ${row.versionText}（${row.releaseId}）`)
    void poll()
  }

  let historyTimer = 0

  function scheduleHistoryRefresh() {
    if (historyTimer || !pageAlive) return
    const pending = runtimeHistory.value.some(
      (row) => row.status === 'running' || row.status === 'waiting' || row.status === 'dialing',
    )
    if (!pending) return
    historyTimer = window.setTimeout(() => {
      historyTimer = 0
      if (!pageAlive) return
      void loadRuntimeHistory()
    }, 3000)
  }

  async function loadRuntimeHistory() {
    const site = current.value
    if (!site) {
      runtimeHistory.value = []
      return
    }
    try {
      const res = await queryRuntimeCommands(site.agentId, site.instanceName)
      if (!isApiSuccess(res)) return
      runtimeHistory.value = parseApiData<RuntimeCommand[]>(res) ?? []
      scheduleHistoryRefresh()
    } catch {
      // 历史列表失败不挡住当前操作
    }
  }

  async function openRuntimeCommand(row: RuntimeCommand) {
    try {
      const res = await queryCommand(row.commandId)
      if (!isApiSuccess(res)) {
        toast.error(getApiMessage(res, '加载启停记录失败'))
        return
      }
      const cmd = parseApiData<RuntimeCommand>(res)
      if (!cmd) return
      const name = lifecycleName(cmd.action)
      pushLog(`查看${name} ${cmd.addTime || row.addTime || ''}`)
      if (!loggedCommands.has(cmd.commandId)) {
        loggedCommands.add(cmd.commandId)
        logReleaseSteps(name, cmd)
      }
    } catch (err) {
      toast.error(getHttpErrorMessage(err, '加载启停记录失败'))
    }
  }

  function lifecycleName(action: string) {
    if (action.endsWith('stop')) return '停止'
    if (action.endsWith('restart')) return '重启'
    if (action.endsWith('start')) return '启动'
    return '启停'
  }

  function followStartupLog(instanceName: string) {
    const row = startupLogOf(files.value)
    if (!row) {
      pushLog(`${instanceName} 文件列表里没有 catalina 日志，完成后只显示探测结论`)
      return
    }
    const name = fileNameOf(row)
    pushLog(`跟读启动日志 ${name}`)
    void follow.start(row.instId, name)
  }

  function siteKey(row: AppSite) {
    return `${row.agentId}::${row.instanceName}`
  }

  onMounted(() => void loadSites())
  onUnmounted(() => {
    pageAlive = false
    if (historyTimer) window.clearTimeout(historyTimer)
    downloadAbort?.abort()
    downloadAbort = null
  })

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
    downloadProgress,
    logLines,
    lifecycle,
    runtimeHistory,
    tailText: follow.text,
    tailName: follow.fileName,
    tailLive: follow.live,
    canRelease,
    canLifecycle,
    selectedFiles,
    releaseKind,
    releaseAppCode,
    loadSites,
    openSite,
    onSiteNoteSaved,
    refreshFiles,
    toggle,
    pickAll,
    clearPick,
    downloadSelected,
    downloadOne,
    onPickFile,
    submitRelease,
    submitCreate,
    submitLifecycle,
    continueBatch,
    submitRollback,
    openRelease,
    openRuntimeCommand,
    siteKey,
    fileNameOf,
    fileTypePreview,
    modeLabel,
  }
}
