import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const api = createApi(moduleApiPrefix('fg0051'))

/** 一条系统操作日志，对应审计表的一行。 */
export interface OperationLog {
  auditId: string
  userId: string
  userName: string
  action: string
  moduleCode: string
  targetType: string
  targetId: string
  targetName: string
  resourceCode: string
  requestPath: string
  requestMethod: string
  clientIP: string
  resultFlag: string
  detailText: string
  failReason: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
}

/**
 * 分页查询本租户操作日志。
 * @param params - 页码、每页条数和筛选条件
 * @returns 一页日志
 */
export function queryLogs(params: Record<string, string | number>): Promise<ApiResult<PageData<OperationLog>>> {
  return api.get<PageData<OperationLog>>('/queryLogs', params)
}

/**
 * 按审计主键读取一行。
 * @param auditId - 审计编码
 * @returns 日志详情
 */
export function getLog(auditId: string): Promise<ApiResult<OperationLog>> {
  return api.get<OperationLog>('/getLog', { auditId })
}
