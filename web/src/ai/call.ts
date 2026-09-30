import { getApiMessage, isApiSuccess } from '@/api/request'
import type { ApiResult } from '@/api/result'
import { FgAiError } from './error'

/**
 * 把管理端响应收成数据。失败时抛出接口文案。
 * 模块 get/post 往往没有泛型，所以这里接受 unknown，由调用方写明 T。
 * @param result - 模块接口返回
 * @param fallback - 没有错误文案时的说明
 * @returns data；成功但没有 data 时为 undefined
 */
export async function unwrap<T = unknown>(result: ApiResult<unknown>, fallback: string): Promise<T | undefined> {
  if (!isApiSuccess(result)) throw new FgAiError(getApiMessage(result, fallback))
  return result.data as T | undefined
}
