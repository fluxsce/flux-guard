import { createApi, http } from '@/api/request'
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

/** 代下任务。 */
export interface FileFetchJob {
  jobId: string
  instId?: string
  fileName?: string
  status: string
  message?: string
  mode?: string
}

/** 发起代下（可拨则短拨，否则出站推送）。 */
export function requestFileFetch(instId: string) {
  return fg0008.post('/requestFileFetch', { instId })
}

/** 查询代下状态。 */
export function queryFileFetch(jobId: string) {
  return fg0008.get('/queryFileFetch', { jobId })
}

/** 下载已就绪的暂存文件。 */
export async function downloadFileFetch(jobId: string) {
  const res = await http.get(requestPathHelper.join(apiBase, '/downloadFileFetch'), {
    params: { jobId },
    responseType: 'blob',
    showLoading: true,
  })
  return res as { data: Blob; headers: Record<string, string> }
}
