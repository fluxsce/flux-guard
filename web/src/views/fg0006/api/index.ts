import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { PageData } from '@/api/pagination'

const fg0006 = createApi(moduleApiPrefix('fg0006'))

export interface JavaApp {
  agentId: string
  appName: string
  mainClass: string
  processId: string
  javaVersion: string
  startTime: string
  hostName: string
  ipAddress: string
  heapUsedPercent: number
  heapUsedBytes: number
  heapMaxBytes: number
  nonHeapUsedBytes: number
  threadCount: number
  daemonCount: number
  collectionCount: number
  collectionTimeMs: number
  reportedTime: string
  findings?: string[]
  findingLevel?: string
}

export interface JavaCollector {
  collectorName: string
  collectionCount: number
  collectionTimeMs: number
  observedTime: string
}

export interface JavaDetail {
  findings?: string[]
  findingLevel?: string
  collectors: JavaCollector[]
}

export interface CurvePoint {
  observedTime: string
  value: number
}

export interface JavaCurve {
  name: string
  unit: string
  group?: string
  points: CurvePoint[]
}

export interface JavaSeries {
  curves: JavaCurve[]
}

export interface SeriesRange {
  from: string
  to: string
}

export function queryApps(params: Record<string, string | number>) {
  return fg0006.get<PageData<JavaApp>>('/queryApps', params)
}

export function queryDetail(agentId: string) {
  return fg0006.get('/queryDetail', { agentId })
}

export function querySeries(agentId: string, range?: SeriesRange | null) {
  const params: Record<string, string> = { agentId }
  if (range?.from && range.to) {
    params.from = range.from
    params.to = range.to
  }
  return fg0006.get('/querySeries', params)
}
