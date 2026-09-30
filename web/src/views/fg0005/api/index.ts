import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { PageData } from '@/api/pagination'

const fg0005 = createApi(moduleApiPrefix('fg0005'))

export interface HostMonitor {
  agentId: string
  hostName: string
  reportName: string
  ipAddress: string
  osName: string
  cpuUsagePercent: number
  loadAvg1: number
  memUsedPercent: number
  memUsedBytes: number
  memTotalBytes: number
  diskUsedPercent: number
  diskReadBytesPerSec: number
  diskWriteBytesPerSec: number
  diskUtilPercent: number
  netReceiveBytesPerSec: number
  netTransmitBytesPerSec: number
  tcpEstablished: number
  tcpTimeWait: number
  tcpCloseWait: number
  fdAllocated: number
  fdUsedPercent: number
  procCount: number
  zombieCount: number
  reportedTime: string
}

export interface HostDisk {
  mountPoint: string
  deviceName: string
  totalBytes: number
  usedBytes: number
  freeBytes: number
  usedPercent: number
  inodeUsedPercent: number
}

export interface HostDiskIO {
  deviceName: string
  readBytesPerSec: number
  writeBytesPerSec: number
  readLatencyMs: number
  writeLatencyMs: number
  utilPercent: number
}

export interface HostNet {
  nicName: string
  receiveBytesPerSec: number
  transmitBytesPerSec: number
  receiveErrs: number
  transmitErrs: number
  receiveDrop: number
  transmitDrop: number
}

export interface HostCore {
  coreName: string
  usagePercent: number
}

export interface HostProc {
  processId: string
  processName: string
  rssBytes: number
  cpuPercent: number
}

export interface HostTcp {
  establishedCount: number
  listenCount: number
  timeWaitCount: number
  closeWaitCount: number
  synSentCount: number
  synRecvCount: number
  finWaitCount: number
  lastAckCount: number
  closingCount: number
  tcpInUseCount: number
  udpInUseCount: number
  retransmitCount: number
}

export interface HostFd {
  allocatedCount: number
  maximumCount: number
  usedPercent: number
}

export interface HostAlert {
  incidentId: string
  ruleName: string
  metricCode: string
  levelCode: string
  statusCode: string
  currentValue: number
  thresholdValue: number
  compareOp: string
  firstFireTime: string
}

export interface HostDetail {
  disks: HostDisk[]
  diskIo: HostDiskIO[]
  nets: HostNet[]
  cores: HostCore[]
  procs: HostProc[]
  tcp: HostTcp
  fd: HostFd
  alerts: HostAlert[]
}

export function queryHosts(params: Record<string, string | number>) {
  return fg0005.get<PageData<HostMonitor>>('/queryHosts', params)
}

export function queryHostDetail(agentId: string) {
  return fg0005.get('/queryHostDetail', { agentId })
}

export interface CurvePoint {
  observedTime: string
  value: number
}

export interface HostCurve {
  name: string
  unit: string
  group?: string
  points: CurvePoint[]
}

export interface HostSeries {
  curves: HostCurve[]
}

export interface SeriesRange {
  from: string
  to: string
}

export function queryHostSeries(agentId: string, range?: SeriesRange | null) {
  const params: Record<string, string> = { agentId }
  if (range?.from && range.to) {
    params.from = range.from
    params.to = range.to
  }
  return fg0005.get('/queryHostSeries', params)
}

export interface Rule {
  ruleId: string
  ruleName: string
  metricCode: string
  compareOp: string
  thresholdValue: number
  durationSec: number
  levelCode: string
  agentId: string
  channelId: string
  silenceStart: string
  silenceEnd: string
  waitMinutes: number
  escalateChannelId: string
  statusFlag: string
  activeFlag: string
  noteText: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
  currentVersion: number
  oprSeqFlag: string
}

export interface RuleInput {
  ruleId: string
  ruleName: string
  metricCode: string
  compareOp: string
  thresholdValue: number
  durationSec: number
  levelCode: string
  agentId: string
  channelId: string
  silenceStart: string
  silenceEnd: string
  waitMinutes: number
  escalateChannelId: string
  statusFlag: string
  activeFlag: string
  noteText: string
}

export function queryRules(params: Record<string, string | number>) {
  return fg0005.get<PageData<Rule>>('/queryRules', params)
}

export function createRule(body: RuleInput) {
  return fg0005.post<Rule>('/createRule', body)
}

export function updateRule(body: RuleInput) {
  return fg0005.post<Rule>('/updateRule', body)
}

export function deleteRule(ruleId: string) {
  return fg0005.post('/deleteRule', { ruleId })
}
