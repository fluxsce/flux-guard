import { createApi } from '@/api/request'
import { moduleApiPrefix, requestPathHelper } from '@/api/requestPath'

const fg0008 = createApi(moduleApiPrefix('fg0008'))
const apiBase = moduleApiPrefix('fg0008')

/** 一套运行时（appside 的 instance），左侧导航主键。 */
export interface AppSite {
  agentId: string
  instanceName: string
  hostName: string
  ipAddress?: string
  modeCode: string
  runtimeKind: string
  basePath?: string
  /** 人工备注，便于区分同名多机运行时。 */
  noteText?: string
  fileCount: number
  onlineFlag: string
  commandAddress?: string
  tomcatVersion?: string
  reportedTime?: string
}

/** 应用列表行（兼容旧接口）。 */
export interface AppRow {
  appCode: string
  appName: string
  artifactKind: string
  instanceCount: number
  versionText: string
  mixedVersion: string
  onlineCount: number
}

/** 一套运行时下的制品文件 / 应用实例。 */
export interface AppInstance {
  instId: string
  agentId: string
  appCode: string
  machineId: string
  hostName: string
  ipAddress?: string
  instanceName: string
  runtimeKind: string
  modeCode: string
  artifactKind: string
  artifactPath?: string
  versionText: string
  tomcatVersion: string
  commandAddress?: string
  runningFlag: string
  onlineFlag: string
  reportedTime: string
}

/** 发布步骤（目标/备份路径），不是 CI 流水线。 */
export interface ReleaseStep {
  name: string
  status: string
  targetPath?: string
  backupPath?: string
  detail?: string
}

/** 发布明细。 */
export interface ReleaseItem {
  commandId: string
  instId: string
  agentId: string
  status: string
  message?: string
  steps?: ReleaseStep[]
}

/** 一次发布或回滚。 */
export interface AppRelease {
  releaseId: string
  appCode: string
  artifactKind: string
  versionText: string
  fileName?: string
  addTime?: string
  addWho?: string
  pendingCount?: number
  batchSize?: number
  purpose?: string
  sourceReleaseId?: string
  items?: ReleaseItem[]
}

/** 查询运行时套件。 */
export function querySites(keyword = '') {
  return fg0008.get('/querySites', { keyword })
}

/** 查询一套运行时下的制品文件。 */
export function querySiteFiles(agentId: string, instanceName: string) {
  return fg0008.get('/querySiteFiles', { agentId, instanceName })
}

/** 查询应用列表。 */
export function queryApps(appName = '') {
  return fg0008.get('/queryApps', { appName })
}

/** 查询应用实例。 */
export function queryInstances(appCode: string) {
  return fg0008.get('/queryInstances', { appCode })
}

/** 查询最近发布记录。 */
export function queryReleases(appCode: string) {
  return fg0008.get('/queryReleases', { appCode })
}

/** 修改应用显示名。 */
export function renameApp(appCode: string, appName: string) {
  return fg0008.post('/rename', { appCode, appName })
}

/** 修改运行时简短描述。 */
export function updateSiteNote(agentId: string, instanceName: string, noteText: string) {
  return fg0008.post('/updateSiteNote', { agentId, instanceName, noteText })
}

/** 忽略应用。 */
export function ignoreApp(appCode: string) {
  return fg0008.post('/ignore', { appCode })
}

/** 下发更新。 */
export function releaseApp(body: FormData) {
  return fg0008.post('/release', body)
}

/** 继续下发同一发布的下一批实例。 */
export function continueRelease(releaseId: string) {
  return fg0008.post('/continueRelease', { releaseId })
}

/** 按一次已成功发布的备份回滚。 */
export function rollbackRelease(releaseId: string, instIds: string[] = [], canaryCount = 1) {
  return fg0008.post('/rollback', { releaseId, instIds, canaryCount })
}

/** 查询一次发布进度。 */
export function queryRelease(releaseId: string) {
  return fg0008.get('/queryRelease', { releaseId })
}

/** 运行时启停命令视图。 */
export interface RuntimeCommand {
  commandId: string
  agentId: string
  action: string
  status: string
  message?: string
  steps?: ReleaseStep[]
  addTime?: string
}

/** 下发启动 / 停止 / 重启。 */
export function runtimeControl(agentId: string, instanceName: string, op: 'start' | 'stop' | 'restart') {
  return fg0008.post('/runtimeControl', { agentId, instanceName, op })
}

/** 查询一条命令进度。 */
export function queryCommand(commandId: string) {
  return fg0008.get('/queryCommand', { commandId })
}

/** 查询一套运行时最近的启动、停止、重启。 */
export function queryRuntimeCommands(agentId: string, instanceName: string) {
  return fg0008.get<RuntimeCommand[]>('/queryRuntimeCommands', { agentId, instanceName }, { showLoading: false })
}

/** 代下任务。 */
export interface FileFetchJob {
  jobId: string
  instId?: string
  fileName?: string
  status: string
  message?: string
  mode?: string
  bytesReceived?: number
  bytesTotal?: number
  phase?: string
}

/** 浏览器侧代下进度（拉取暂存 + 本机保存）。 */
export interface DownloadProgress {
  fileName: string
  phase: 'request' | 'queued' | 'transferring' | 'saving' | 'done' | 'failed'
  mode?: string
  percent: number
  bytesReceived: number
  bytesTotal: number
  message: string
}

/** 发起代下（可拨则短拨，否则出站推送）。 */
export function requestFileFetch(instId: string) {
  return fg0008.post('/requestFileFetch', { instId }, undefined, { showLoading: false })
}

/** 查询代下状态。 */
export function queryFileFetch(jobId: string) {
  return fg0008.get('/queryFileFetch', { jobId }, { showLoading: false })
}

/**
 * 构造代下暂存的浏览器直链（走会话 Cookie，由浏览器流式落盘，避免 1GiB 级进 JS 堆）。
 * @param jobId - 代下任务号
 */
export function downloadFileFetchURL(jobId: string): string {
  const path = requestPathHelper.join(apiBase, '/downloadFileFetch')
  const query = new URLSearchParams({ jobId: jobId.trim() }).toString()
  const base = String(import.meta.env.VITE_API_BASE_URL || '').replace(/\/$/, '')
  return `${base}${path}?${query}`
}

/** 打开日志跟读后的会话摘要（不含绝对路径）。 */
export interface LogViewSession {
  logViewId: string
  agentId: string
  instId: string
  fileName: string
}

/** 跟读增量块。 */
export interface LogViewChunk {
  text: string
  offset: number
  closed: boolean
}

/** 打开一路应用日志跟读（经代理会话隧道）。 */
export function openLogView(instId: string) {
  return fg0008.post<LogViewSession>('/openLogView', { instId }, undefined, { showLoading: false })
}

/** 拉取跟读增量。 */
export function logOutput(logViewId: string, offset: number) {
  return fg0008.get<LogViewChunk>('/logOutput', { logViewId, offset }, { showLoading: false })
}

/** 关闭跟读。 */
export function closeLogView(logViewId: string) {
  return fg0008.post<{ logViewId: string }>('/closeLogView', { logViewId }, undefined, { showLoading: false })
}

/** 跟读保活。 */
export function touchLogView(logViewId: string) {
  return fg0008.post<{ logViewId: string }>('/touchLogView', { logViewId }, undefined, { showLoading: false })
}

/**
 * 触发浏览器原生下载（流式写本机，不经过 axios blob）。
 * @param jobId - 代下任务号
 * @param fileName - 建议文件名（最终以 Content-Disposition 为准）
 */
export function triggerFileFetchDownload(jobId: string, fileName?: string) {
  const a = document.createElement('a')
  a.href = downloadFileFetchURL(jobId)
  a.rel = 'noopener'
  if (fileName) {
    a.download = fileName
  }
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  a.remove()
}
