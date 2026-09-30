import { messageOfCode } from '@/api/errorText'

/**
 * 管理端统一响应。与 server/web/routes.Result 对应。
 * 业务失败也 resolve 为本结构，页面用 isApiSuccess / getApiMessage / parseApiData。
 * 查询的 data 是列表或详情。新增和修改的 data 必须是与列表同一行的记录，用 placeSavedRow 写回表格。
 * 失败时 code 是稳定错误码，error 是默认中文。getApiMessage 按 code 取当前语言。
 */
export interface ApiResult<T = unknown> {
  ok: boolean
  data?: T
  code?: string
  error?: string
}

/** 业务是否成功。 */
export function isApiSuccess(result: ApiResult | null | undefined): boolean {
  return result?.ok === true
}

/** 失败文案。有错误码时用当前语言，否则用 error，再否则用兜底。 */
export function getApiMessage(result: ApiResult | null | undefined, fallback: string): string {
  const server = result && typeof result.error === 'string' && result.error.trim() ? result.error : fallback
  return messageOfCode(result?.code, server)
}

/** 取出 data。失败或没有数据时返回兜底。 */
export function parseApiData<T>(result: ApiResult | null | undefined, fallback?: T): T | undefined {
  if (!isApiSuccess(result)) {
    return fallback
  }
  if (result?.data === undefined || result.data === null) {
    return fallback
  }
  return result.data as T
}
