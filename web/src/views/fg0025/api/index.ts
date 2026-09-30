import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0025 = createApi(moduleApiPrefix('fg0025'))

export interface JobRow {
  code: string
  title: string
  description: string
  kind: string
  source?: string
  scope?: string
  enabled: boolean
  mode?: string
  intervalMinutes: number
  runAt?: string
  cron?: string
  timeoutSeconds?: number
  retryCount?: number
  deletable?: boolean
  lastRunAt?: string
  lastOk?: boolean
  lastDurationMs?: number
  lastResult?: string
  lastError?: string
  lastTrigger?: string
  nextStart?: string
  ranAtLeast?: boolean
}

export interface JobKind {
  kind: string
  title: string
  description: string
}

export interface JobList {
  editable: boolean
  items: JobRow[]
  kinds?: JobKind[]
}

export interface ArchiveJob {
  code: string
  title: string
  description: string
  kind: string
  source?: string
  scope?: string
  enabled: boolean
  mode: string
  intervalMinutes: number
  runAt?: string
  cron?: string
  timeoutSeconds?: number
  retryCount?: number
  auditLogDays: number
  opsLogDays: number
  alertDays: number
  sampleDays: number
  params?: string
  currentVersion: number
  editable: boolean
  deletable?: boolean
  lastRunAt?: string
  lastOk?: boolean
  lastDurationMs?: number
  lastResult?: string
  lastError?: string
  lastTrigger?: string
  nextStart?: string
  ranAtLeast?: boolean
}

export function queryJobs() {
  return fg0025.get('/queryJobs')
}

export function getJob(code: string) {
  return fg0025.get('/getJob', { code })
}

export interface JobLog {
  logId: string
  trigger: string
  startTime: string
  endTime: string
  durationMs: number
  ok: boolean
  result?: string
  errorText?: string
}

export interface JobLogList {
  code: string
  title: string
  items: JobLog[]
}

export function queryJobLogs(code: string) {
  return fg0025.get('/queryJobLogs', { code })
}

export function saveArchiveJob(body: Omit<ArchiveJob, 'editable' | 'title' | 'description' | 'kind' | 'source' | 'scope' | 'lastRunAt' | 'lastOk' | 'lastDurationMs' | 'lastResult' | 'lastError' | 'lastTrigger' | 'ranAtLeast'>) {
  return fg0025.post('/saveArchiveJob', body)
}

export function saveJob(body: Record<string, unknown>) {
  return fg0025.post('/saveJob', body)
}

export function deleteJob(code: string) {
  return fg0025.post('/deleteJob', { code })
}

export function runJob(code: string) {
  return fg0025.post('/runJob', { code })
}
