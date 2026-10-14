import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0071 = createApi(moduleApiPrefix('fg0071'))

export interface AppRow {
  appCode: string
  appName: string
  artifactKind: string
  instanceCount: number
  versionText: string
  mixedVersion: string
  onlineCount: number
}

export interface AppInstance {
  instId: string
  agentId: string
  appCode: string
  machineId: string
  hostName: string
  instanceName: string
  runtimeKind: string
  artifactKind: string
  versionText: string
  tomcatVersion: string
  runningFlag: string
  onlineFlag: string
  reportedTime: string
}

export interface ReleaseItem {
  commandId: string
  instId: string
  agentId: string
  status: string
  message?: string
}

export interface AppRelease {
  releaseId: string
  appCode: string
  artifactKind: string
  versionText: string
  items: ReleaseItem[]
}

export function queryApps(appName = '') {
  return fg0071.get('/queryApps', { appName })
}

export function queryInstances(appCode: string) {
  return fg0071.get('/queryInstances', { appCode })
}

export function renameApp(appCode: string, appName: string) {
  return fg0071.post('/rename', { appCode, appName })
}

export function ignoreApp(appCode: string) {
  return fg0071.post('/ignore', { appCode })
}

export function releaseApp(body: FormData) {
  return fg0071.post('/release', body)
}

export function queryRelease(releaseId: string) {
  return fg0071.get('/queryRelease', { releaseId })
}
