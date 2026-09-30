import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const api = createApi(moduleApiPrefix('fg0043'))

export interface AlertLog {
  alertLogId: string
  alertLevel: string
  alertType: string
  sceneCode: string
  alertTitle: string
  alertContent: string
  alertTimestamp: string
  channelId: string
  sendStatus: string
  sendTime: string
  sendResult: string
  sendError: string
  failCount: number
  alertExtra: string
  activeFlag: string
  noteText: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
}

export function queryLogs(params: Record<string, string | number>): Promise<ApiResult<PageData<AlertLog>>> {
  return api.get<PageData<AlertLog>>('/queryLogs', params)
}
