import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const api = createApi(moduleApiPrefix('fg0007'))

export interface MetricDef {
  code: string
  name: string
  unit?: string
}

export interface Kind {
  kindCode: string
  kindName: string
  defaultPort: number
  metricTable: string
  sortOrder: number
  noteText?: string
  metrics: MetricDef[]
}

export interface ComponentConn {
  componentId: string
  kindCode: string
  kindName: string
  componentName: string
  address: string
  userName: string
  secretSet: string
  databaseName: string
  optionText: string
  intervalSec: number
  statusFlag: string
  upFlag: string
  versionText: string
  roleName: string
  errorText: string
  summaryText: string
  probedTime: string
  noteText: string
  addTime: string
  editTime: string
}

export interface ComponentInput {
  componentId: string
  kindCode: string
  componentName: string
  address: string
  userName: string
  secret: string
  databaseName: string
  optionText: string
  intervalSec: number
  statusFlag: string
  noteText: string
}

export interface RedisDb {
  name: string
  keys: number
  expires: number
}

export interface RedisSlow {
  usec: number
  command: string
}

export interface KafkaTopicRow {
  name: string
  partitions: number
  offline: number
  under: number
}

export interface KafkaGroupRow {
  name: string
  lag: number
  topic: string
}

export interface PointDetail {
  dbs?: RedisDb[]
  slow?: RedisSlow[]
  topics?: KafkaTopicRow[]
  groups?: KafkaGroupRow[]
  groupNote?: string
}

export interface Point {
  observedTime: string
  upFlag: string
  versionText: string
  roleName: string
  errorText: string
  summaryText: string
  values: Record<string, number>
  detail?: PointDetail
}

export interface Latest {
  componentId: string
  kindCode: string
  point: Point | null
}

export function queryCatalog() {
  return api.get<Kind[]>('/queryCatalog')
}

export function queryConns(params: Record<string, unknown>) {
  return api.get<PageData<ComponentConn>>('/queryConns', params)
}

export function createConn(body: ComponentInput) {
  return api.post<ComponentConn>('/createConn', body)
}

export function updateConn(body: ComponentInput) {
  return api.post<ComponentConn>('/updateConn', body)
}

export function deleteConn(componentId: string) {
  return api.post<{ deleted: boolean }>('/deleteConn', { componentId })
}

export interface ConnTrial {
  upFlag: string
  versionText: string
  roleName: string
  errorText: string
  summaryText: string
}

export function testConn(body: ComponentInput) {
  return api.post<ConnTrial>('/testConn', body)
}

export function probeConn(componentId: string) {
  return api.post<ComponentConn>('/probeConn', { componentId })
}

export interface SeriesRange {
  from: string
  to: string
}

export function querySeries(componentId: string, limit = 120, range?: SeriesRange | null) {
  const params: Record<string, string | number> = { componentId, limit }
  if (range?.from && range.to) {
    params.from = range.from
    params.to = range.to
  }
  return api.get<Point[]>('/querySeries', params)
}

export type { ApiResult }
