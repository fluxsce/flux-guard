import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0002 = createApi(moduleApiPrefix('fg0002'))

export interface Agent {
  agentId: string
  agentName: string
  agentCode?: string
  machineId?: string
  agentVersion?: string
  goos?: string
  goarch?: string
  hostName: string
  ipAddress: string
  commandAddress?: string
  onlineFlag: string
  statusFlag: string
  lastSeenTime?: string
}

/** 一次代理自更新的汇总。 */
export interface AgentRelease {
  releaseId: string
  agentCode: string
  versionText: string
  fileName?: string
  goos?: string
  goarch?: string
  pendingCount?: number
  batchSize?: number
  addTime?: string
  addWho?: string
  openFlag?: string
  items: AgentReleaseItem[]
}

/** 一台代理上的升级命令状态与版本对照。 */
export interface AgentReleaseItem {
  commandId: string
  agentId: string
  agentName?: string
  status: string
  message?: string
  agentVersion?: string
  versionMatched?: string
}

/** 代理列表上展示的最近升级状态。 */
export interface AgentUpgradeStatus {
  agentId: string
  releaseId: string
  versionText: string
  status: string
  versionMatched?: string
  message?: string
}

export function queryAgents() {
  return fg0002.get('/queryAgents')
}

export function registerAgent(agentName: string, hostName: string) {
  return fg0002.post('/registerAgent', { agentName, hostName })
}

/**
 * 只改代理显示名。身份键不变，后续上报不会冲掉。
 * @param agentId - 代理编号
 * @param agentName - 新显示名
 */
export function renameAgent(agentId: string, agentName: string) {
  return fg0002.post<Agent>('/renameAgent', { agentId, agentName })
}

/**
 * 物理删除选中的采集代理及其身份、终端、命令等关联行。
 * @param agentIds - 代理编号列表
 */
export function deleteAgents(agentIds: string[]) {
  return fg0002.post<{ deleted: number }>('/deleteAgents', { agentIds })
}

export interface CommandView {
  commandId: string
  agentId: string
  action: string
  status: string
  deliverMode?: string
  message?: string
}

export function dispatchCommand(agentId: string, action: string, intervalSec?: number) {
  return fg0002.post<CommandView>('/dispatchCommand', { agentId, action, intervalSec }, undefined, { timeout: 30000 })
}

/** 控制台下发代理自更新的请求体；安装包取自中央 agents/。 */
export interface UpgradeAgentsInput {
  agentCode: string
  version: string
  goos: string
  goarch: string
  agentIds: string[]
  canaryCount: number
}

/**
 * 使用中央 agents/ 包装并向选中代理下发自更新。
 * @param body - 种类、版本、平台与目标代理
 */
export function upgradeAgents(body: UpgradeAgentsInput) {
  return fg0002.post<AgentRelease>('/upgradeAgents', body, undefined, { timeout: 120000 })
}

/**
 * 列出本租户最近的代理升级（不含各台明细）。
 * @param agentCode - 可选种类过滤
 * @param limit - 条数，默认 20
 */
export function queryAgentReleases(agentCode?: string, limit = 20) {
  const params: Record<string, string | number> = { limit }
  if (agentCode) params.agentCode = agentCode
  return fg0002.get<AgentRelease[]>('/queryAgentReleases', params, { showLoading: false })
}

/**
 * 查询一次代理升级的命令进度。
 * @param releaseId - 发布号
 */
export function queryAgentRelease(releaseId: string) {
  return fg0002.get<AgentRelease>('/queryAgentRelease', { releaseId }, { showLoading: false })
}

/**
 * 查询各代理最近一次仍需关注的升级状态。
 */
export function queryAgentUpgradeStatuses() {
  return fg0002.get<AgentUpgradeStatus[]>('/queryAgentUpgradeStatuses', undefined, { showLoading: false })
}

/**
 * 继续下发同一发布中尚未升级的代理批次。
 * @param releaseId - 发布号
 */
export function continueAgentRelease(releaseId: string) {
  return fg0002.post<AgentRelease>('/continueAgentRelease', { releaseId })
}

export function queryCommand(commandId: string) {
  return fg0002.get<CommandView>('/queryCommand', { commandId })
}

/** 一条带回执的历史命令。 */
export interface CommandHistory extends CommandView {
  intervalSec?: number
  addTime?: string
  addWho?: string
}

/**
 * 按代理列出最近命令执行记录。
 * @param agentId - 代理编码
 */
export function queryCommands(agentId: string) {
  return fg0002.get<CommandHistory[]>('/queryCommands', { agentId })
}

/** 终端会话账本的一行。 */
export interface ShellAudit {
  auditId: string
  shellId: string
  agentId: string
  userId: string
  userName: string
  eventType: string
  detailText?: string
  addTime: string
}

/**
 * 按代理列出终端开闭与回车命令行。
 * @param agentId - 代理编码
 */
export function queryShellAudits(agentId: string) {
  return fg0002.get<ShellAudit[]>('/queryShellAudits', { agentId })
}

export interface ShellSession {
  shellId: string
  agentId: string
  userId: string
  userName: string
  shellTitle: string
  shellNo: number
  statusFlag: string
}

export function openShell(agentId: string) {
  return fg0002.post<ShellSession>('/openShell', { agentId })
}

export function queryShells(agentId: string) {
  return fg0002.get<ShellSession[]>('/queryShells', { agentId }, { showLoading: false })
}

export function closeShell(shellId: string) {
  return fg0002.post<{ shellId: string }>('/closeShell', { shellId })
}

export interface ShellChunk {
  text: string
  offset: number
  closed: boolean
}

const quiet = { showLoading: false }

export function shellInput(shellId: string, text: string) {
  return fg0002.post<{ shellId: string }>('/shellInput', { shellId, text }, undefined, quiet)
}

export function shellOutput(shellId: string, offset: number) {
  return fg0002.get<ShellChunk>('/shellOutput', { shellId, offset }, quiet)
}

export function shellResize(shellId: string, cols: number, rows: number) {
  return fg0002.post<{ shellId: string }>('/shellResize', { shellId, cols, rows }, undefined, quiet)
}
