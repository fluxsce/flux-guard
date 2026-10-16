/**
 * 全局 HTTP 客户端入口。
 *
 * 契约（新模块只走这一条）：
 * 1. 业务 JSON 用 createApi(moduleApiPrefix('fgxxxx'))，得到 ApiResult。
 * 2. 页面用 isApiSuccess / getApiMessage / parseApiData。
 * 3. 后端已给出 { ok, error } 的 HTTP 4xx/5xx 会 resolve 为 ok=false，不当异常抛。
 * 4. 仅网络中断、超时、取消才 reject。会话 401 在拦截器弹窗。登录口令错误除外。
 * 5. blob 下载才用 http。FormData 仍走 createApi，拦截器会去掉 JSON Content-Type。请求前扩展走 registerHttpPlugin。
 * 6. 取消使用 RequestConfig.signal。
 */
import { requestInterceptorHelper } from '@/api/requestInterceptor'
import { requestPathHelper } from '@/api/requestPath'
import type { ModuleApi, RequestConfig, RequestMethod } from '@/api/requestTypes'
import { responseInterceptorHelper } from '@/api/responseInterceptor'
import type { ApiResult } from '@/api/result'
import axios from 'axios'

export {
  HttpCancelledError,
  HttpTransportError,
  getHttpErrorMessage,
  isHttpCancelledError,
  isHttpTransportError,
} from '@/api/requestError'
export { registerHttpPlugin, registerHttpResponsePlugin } from '@/api/requestPlugin'
export type { HttpRequestPlugin, HttpResponsePlugin } from '@/api/requestPlugin'
export type { InternalRequestConfig, ModuleApi, RequestConfig, RequestMeta, RequestMethod } from '@/api/requestTypes'
export { getApiMessage, isApiSuccess, parseApiData } from '@/api/result'
export type { ApiResult } from '@/api/result'

/** 与服务端环境设置默认的接口超时 120 秒一致。登录和当前用户接口会按实际设置覆盖。 */
const defaultRequestTimeoutMs = 120_000

const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  timeout: defaultRequestTimeoutMs,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json; charset=UTF-8',
  },
})

requestInterceptorHelper.attach(http)
responseInterceptorHelper.attach(http)

export function updateTimeout(timeout: number): void {
  if (timeout > 0) {
    http.defaults.timeout = timeout
  }
}

export function getCurrentTimeout(): number {
  return http.defaults.timeout || defaultRequestTimeoutMs
}

function isWriteMethod(method?: string): boolean {
  const upper = String(method || 'GET').toUpperCase()
  return upper !== 'GET' && upper !== 'DELETE' && upper !== 'HEAD' && upper !== 'OPTIONS'
}

function mergeRequestConfig(base: RequestConfig = {}, extra: RequestConfig = {}): RequestConfig {
  return {
    showLoading: true,
    ...base,
    ...extra,
    meta: {
      ...base.meta,
      ...extra.meta,
    },
  }
}

export function request<T = unknown>(options: RequestConfig): Promise<ApiResult<T>>
export function request<T = unknown>(
  method: RequestMethod,
  url: string,
  data?: unknown,
  params?: unknown,
  config?: RequestConfig,
): Promise<ApiResult<T>>
export function request<T = unknown>(
  methodOrOptions: RequestMethod | RequestConfig,
  url?: string,
  data?: unknown,
  params?: unknown,
  config?: RequestConfig,
): Promise<ApiResult<T>> {
  if (typeof methodOrOptions === 'object') {
    return http(mergeRequestConfig(methodOrOptions)) as Promise<ApiResult<T>>
  }

  const method = methodOrOptions
  const requestConfig: RequestConfig = mergeRequestConfig({ method, url }, config)
  if (isWriteMethod(method)) {
    requestConfig.data = data
    requestConfig.params = params
  } else {
    requestConfig.params = { ...(params as object), ...config?.params }
  }
  return http(requestConfig) as Promise<ApiResult<T>>
}

export function createApi(baseURL: string, defaultConfig: RequestConfig = {}): ModuleApi {
  const defaults = mergeRequestConfig(
    {
      meta: { module: requestPathHelper.readModule(baseURL) },
    },
    defaultConfig,
  )

  const send = <T = unknown>(
    method: RequestMethod,
    path = '',
    data?: unknown,
    params?: unknown,
    config?: RequestConfig,
  ): Promise<ApiResult<T>> => {
    return request<T>(method, requestPathHelper.join(baseURL, path), data, params, mergeRequestConfig(defaults, config))
  }

  return {
    request: (method, path, data, params, config) => send(method, path, data, params, config),
    get: (path, params, config) => send('GET', path, undefined, params, config),
    post: (path, data, params, config) => send('POST', path, data, params, config),
    put: (path, data, config) => send('PUT', path, data, undefined, config),
    delete: (path, params, config) => send('DELETE', path, undefined, params, config),
    patch: (path, data, config) => send('PATCH', path, data, undefined, config),
    getURL: (path = '') => requestPathHelper.join(baseURL, path),
  } satisfies ModuleApi
}

/** 原始 axios 实例。业务 JSON 请用 request / createApi。 */
export { http }
export default http
