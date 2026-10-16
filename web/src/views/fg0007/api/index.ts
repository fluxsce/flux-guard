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

export interface Panel {
  panelCode: string
  panelName: string
  sortOrder: number
  terminalFlag: string
  listFlag: string
  metrics?: string[]
  sections?: string[]
}

export interface Kind {
  kindCode: string
  kindName: string
  defaultPort: number
  metricTable: string
  sortOrder: number
  noteText?: string
  metrics: MetricDef[]
  panels?: Panel[]
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
  headlines?: Record<string, number>
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
  at: number
  usec: number
  command: string
}

export interface KafkaBrokerRow {
  id: number
  host: string
  port: number
  controller: boolean
  up: boolean
  logBytes?: number
  leaders?: number
  replicas?: number
}

export interface KafkaPartitionRow {
  topic: string
  part: number
  leader: number
  isr: number
  replicas: number
  problem: string
}

export interface KafkaTopicRow {
  name: string
  partitions: number
  offline: number
  under: number
  skew?: number
  replicas?: number
  replicaMax?: number
  belowMin?: number
  internal?: boolean
}

export interface KafkaGroupLag {
  topic: string
  part: number
  lag: number
}

export interface KafkaOffsetRow {
  topic: string
  part: number
  current?: number
  end?: number
  lag: number
  consumerId?: string
  host?: string
  clientId?: string
}

export interface KafkaClient {
  clientId?: string
  host?: string
}

export interface KafkaTopicLink {
  topic: string
  parts: number
  lag: number
  maxLag?: number
  part?: number
  behind?: number
}

export interface KafkaGroupRow {
  name: string
  lag: number
  maxLag?: number
  part?: number
  topic: string
  topicCount?: number
  state?: string
  members?: number
  behind?: number
  coordinator?: string
  coordinatorId?: number
  assignor?: string
  clients?: KafkaClient[]
  clientNote?: string
  topics?: KafkaTopicLink[]
  topicLinkNote?: string
  offsets?: KafkaOffsetRow[]
  offsetNote?: string
  lags?: KafkaGroupLag[]
  lagNote?: string
}

export interface MongoMember {
  name: string
  state: string
  health: number
  lag: number
  self?: boolean
}

export interface MongoDatabase {
  name: string
  bytes: number
  empty?: boolean
}

export interface MongoRunning {
  op: string
  ns: string
  secs: number
  client?: string
}

export interface PointDetail {
  dbs?: RedisDb[]
  slow?: RedisSlow[]
  clusterId?: string
  brokers?: KafkaBrokerRow[]
  topics?: KafkaTopicRow[]
  partitions?: KafkaPartitionRow[]
  groups?: KafkaGroupRow[]
  groupNote?: string
  topicNote?: string
  partitionNote?: string
  brokerNote?: string
  setName?: string
  members?: MongoMember[]
  databases?: MongoDatabase[]
  running?: MongoRunning[]
  memberNote?: string
  dbNote?: string
  runningNote?: string
  source?: string
  minLatency?: number
  maxLatency?: number
  connections?: number
  received?: number
  sent?: number
  zxid?: string
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

export interface ZkNodeStat {
  path: string
  czxid?: number
  mzxid?: number
  ctime?: number
  mtime?: number
  version?: number
  cversion?: number
  aversion?: number
  dataLength?: number
  numChildren?: number
  ephemeralOwner?: number
  data?: string
  dataBase64?: string
}

export interface ZkChildren {
  path: string
  children?: string[]
}

/** 对一条 ZooKeeper 连接执行目录操作。写操作由终端按钮控制。 */
export function execZk(componentId: string, command: string, args: Record<string, string>) {
  return api.post<ZkNodeStat & ZkChildren>('/execConn', { componentId, command, args })
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
