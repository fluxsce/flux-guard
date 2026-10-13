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

export interface JavaGeneration {
  processId: string
  startTime: string
  openedTime: string
  closedTime?: string
  collectionCount: number
  collectionTimeMs: number
  collectors: JavaCollector[]
}

export interface JavaPause {
  collectorName: string
  pauseCount: number
  pauseMaxMs: number
  pauseTotalMs: number
  pauseP50Ms: number
  pauseP99Ms: number
  heapBeforeBytes: number
  heapAfterBytes: number
  promotedBytes: number
  pauseCause?: string
  cycleCount: number
  cycleTimeMs: number
  observedTime: string
}

export interface JavaBeat {
  overheadPercent: number
  overheadTime?: string
  deadlockCount: number
  deadlockText?: string
  deadlockTime?: string
  pauses: JavaPause[]
}

export interface JavaDetail {
  findings?: string[]
  findingLevel?: string
  beat?: JavaBeat | null
  collectors: JavaCollector[]
  previous?: JavaGeneration | null
}

export interface JavaRestart {
  observedTime: string
  startTime?: string
  processId?: string
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
  restarts?: JavaRestart[]
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
