import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0008 = createApi(moduleApiPrefix('fg0008'))

/** 应用列表行。 */
export interface AppRow {
  appCode: string
  appName: string
  artifactKind: string
  instanceCount: number
  versionText: string
  mixedVersion: string
  onlineCount: number
}

/** 应用实例。 */
export interface AppInstance {
  instId: string
  agentId: string
  appCode: string
  machineId: string
  hostName: string
  instanceName: string
  runtimeKind: string
  modeCode: string
  artifactKind: string
  versionText: string
  tomcatVersion: string
  runningFlag: string
  onlineFlag: string
  reportedTime: string
}

/** 发布明细。 */
export interface ReleaseItem {
  commandId: string
  instId: string
  agentId: string
  status: string
  message?: string
}

/** 一次发布。 */
export interface AppRelease {
  releaseId: string
  appCode: string
  artifactKind: string
  versionText: string
  fileName?: string
  addTime?: string
  addWho?: string
  items?: ReleaseItem[]
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

/** 查询一次发布进度。 */
export function queryRelease(releaseId: string) {
  return fg0008.get('/queryRelease', { releaseId })
}
