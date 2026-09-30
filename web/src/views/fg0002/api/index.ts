import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

const fg0002 = createApi(moduleApiPrefix('fg0002'))

export interface Agent {
  agentId: string
  agentName: string
  agentCode?: string
  machineId?: string
  hostName: string
  ipAddress: string
  commandAddress?: string
  onlineFlag: string
  statusFlag: string
  lastSeenTime?: string
}

export function queryAgents() {
  return fg0002.get('/queryAgents')
}

export function registerAgent(agentName: string, hostName: string) {
  return fg0002.post('/registerAgent', { agentName, hostName })
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
