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
  opid?: number
}

/** 终端里列出的一个集合。bytes 是存储占用。 */
export interface MongoCollection {
  name: string
  type?: string
  bytes: number
}

/** 集合存储统计，来自 collStats。 */
export interface MongoCollStats {
  database: string
  collection: string
  ns: string
  count: number
  size: number
  avgObjSize: number
  storageSize: number
  nindexes: number
  totalIndexSize: number
  capped?: boolean
}

/** 集合上的一个索引。 */
export interface MongoIndex {
  name: string
  key: string
  unique?: boolean
  sparse?: boolean
  hidden?: boolean
  ttl?: number
  partial?: boolean
}

/** 库列表。 */
export interface MongoDatabaseList {
  databases?: MongoDatabase[]
  note?: string
}

/** 某个库下的集合列表。 */
export interface MongoCollectionList {
  database: string
  collections?: MongoCollection[]
  note?: string
}

/** 某个集合的索引列表。 */
export interface MongoIndexList {
  database: string
  collection: string
  indexes?: MongoIndex[]
  note?: string
}

/** 从库复制线程。不是从库时没有这一项。 */
export interface MysqlReplica {
  io?: string
  sql?: string
  source?: string
  ioError?: string
  sqlError?: string
}

/** 已执行至少 1 秒、且不是 Sleep 的会话。 */
export interface MysqlSession {
  id: number
  user?: string
  host?: string
  db?: string
  command?: string
  time: number
  state?: string
  info?: string
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
  mysqlReplica?: MysqlReplica
  mysqlSessions?: MysqlSession[]
  mysqlSessionNote?: string
  mysqlPoolBytes?: number
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
  pzxid?: number
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

/** 对一条 MongoDB 连接执行库、集合、索引、查询或终止操作。写操作由终端按钮控制。 */
export function execMongo<T>(componentId: string, command: string, args: Record<string, string>) {
  return api.post<T>('/execConn', { componentId, command, args })
}

/** 一次文档查询的结果。documents 里每条是 JSON 文本。 */
export interface MongoFindResult {
  documents?: string[]
  note?: string
}

/** 带条件的文档计数。 */
export interface MongoCountResult {
  count: number
}

/** explain 的执行摘要。 */
export interface MongoExplainResult {
  nReturned: number
  docsExamined: number
  keysExamined: number
  millis: number
  stage?: string
  index?: string
}

/** system.profile 里的一条历史慢操作。 */
export interface MongoProfileRow {
  op: string
  ns: string
  millis: number
  client?: string
  plan?: string
}

/** 某个库的历史慢日志。 */
export interface MongoProfileResult {
  entries?: MongoProfileRow[]
  note?: string
}

/** 终端里列出的一个 Kafka 主题。 */
export interface KafkaTopicLive {
  name: string
  partitions: number
  replicas: number
  internal?: boolean
}

/** 主题上可看、可改的几项配置。空字符串表示还是 Broker 默认。 */
export interface KafkaTopicConfig {
  name: string
  retentionMs: string
  retentionBytes: string
  cleanup: string
  minIsr: string
  maxMessageBytes: string
}

/** 终端里列出的一个消费组。 */
export interface KafkaGroupLive {
  name: string
  state?: string
}

/** 对一条 Kafka 连接执行主题和消费组操作。写操作由终端按钮控制。 */
export function execKafka<T>(componentId: string, command: string, args: Record<string, string>) {
  return api.post<T>('/execConn', { componentId, command, args })
}

/** 终端里的一个 Redis 库。 */
export interface RedisDatabaseLive {
  name: string
  keys: number
  expires: number
}

/** databases 的返回。集群只有 db0，note 说明查找会走每个主节点。 */
export interface RedisDatabaseList {
  databases?: RedisDatabaseLive[]
  note?: string
}

/** 一页 SCAN 结果。cursor 为 0 时后面没有了。 */
export interface RedisScanResult {
  database: string
  keys?: string[]
  cursor: string
  note?: string
}

/** 哈希字段或有序集合成员。 */
export interface RedisField {
  field: string
  value?: string
  score?: string
}

/** 一个键的类型、过期时间和截断后的内容。 */
export interface RedisKeyView {
  name: string
  type: string
  ttl: number
  count?: number
  text?: string
  items?: string[]
  fields?: RedisField[]
  note?: string
}

/** 对一条 Redis 连接执行键浏览、删除或过期时间。写操作由终端按钮控制。 */
export function execRedis<T>(componentId: string, command: string, args: Record<string, string>) {
  return api.post<T>('/execConn', { componentId, command, args })
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
