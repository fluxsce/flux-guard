import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0024 = createApi(moduleApiPrefix('fg0024'))

export interface WebTimeout {
  requestTimeoutSeconds: number
  sessionExpireHours: number
  defaultPageSize: number
  maxPageSize: number
  currentVersion: number
  editable: boolean
}

export type EnvVarKind = 'text' | 'number' | 'bool' | 'secret'

export interface EnvVarItem {
  name: string
  kind: EnvVarKind
  value: string
  hasValue: boolean
  note: string
}

export interface EnvVars {
  items: EnvVarItem[]
  currentVersion: number
}

export interface IngestToken {
  configured: boolean
  statusFlag: string
  hint: string
  currentVersion: number
  agentToken?: string
}

export interface Settings {
  editable: boolean
  webTimeout: WebTimeout
  envVars: EnvVars
  ingestToken: IngestToken
}

export interface EnvVarInput {
  name: string
  originalName?: string
  kind: EnvVarKind
  value: string
  note: string
}

export function querySettings() {
  return fg0024.get('/querySettings')
}

export function revealIngestToken() {
  return fg0024.get('/queryIngestToken', undefined, { showLoading: false })
}

export function saveSettings(body: Omit<WebTimeout, 'editable'> & { groupCode?: string }) {
  return fg0024.post('/saveSettings', { groupCode: 'webTimeout', ...body })
}

export function saveEnvVars(currentVersion: number, items: EnvVarInput[]) {
  return fg0024.post('/saveSettings', { groupCode: 'envVars', currentVersion, items })
}

export function saveIngestToken(currentVersion: number, action: 'rotate' | 'enable' | 'disable') {
  return fg0024.post('/saveSettings', { groupCode: 'ingestToken', currentVersion, action })
}
