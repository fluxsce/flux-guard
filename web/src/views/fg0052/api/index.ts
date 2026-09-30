import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const api = createApi(moduleApiPrefix('fg0052'))

/** 一条代理操作日志。COMMAND 来自命令回执，SHELL 来自终端账本。 */
export interface AgentLog {
  source: string
  logId: string
  agentId: string
  agentName: string
  userId: string
  userName: string
  eventType: string
  statusFlag: string
  deliverMode: string
  intervalSec: number
  detailText: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
}

/**
 * 分页查询本租户代理命令与终端账本。
 * @param params - 页码、每页条数和筛选条件
 * @returns 一页日志
 */
export function queryLogs(params: Record<string, string | number>): Promise<ApiResult<PageData<AgentLog>>> {
  return api.get<PageData<AgentLog>>('/queryLogs', params)
}

/**
 * 按来源和业务主键读取一行。
 * @param source - COMMAND 或 SHELL
 * @param logId - 命令号或账本号
 * @returns 日志详情
 */
export function getLog(source: string, logId: string): Promise<ApiResult<AgentLog>> {
  return api.get<AgentLog>('/getLog', { source, logId })
}
