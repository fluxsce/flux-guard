/**
 * HTTP 客户端共享类型。
 * 业务模块一般只使用 RequestConfig；拦截器内部才用 InternalRequestConfig。
 */
import type { ApiResult } from '@/api/result'
import type { AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios'

export type RequestMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | 'HEAD' | 'OPTIONS'

/** 单次请求的业务元数据。不发给后端，只给拦截器、插件、排障用。 */
export interface RequestMeta {
  /** 模块编码，如 fg0002、auth */
  module?: string
}

/** 业务请求配置。取消使用继承来的 signal（AbortSignal）。 */
export interface RequestConfig extends AxiosRequestConfig {
  /** 是否计入顶栏进度，默认 true */
  showLoading?: boolean
  /** 模块标签等，不发给后端 */
  meta?: RequestMeta
}

/** 拦截器内部配置。 */
export interface InternalRequestConfig extends InternalAxiosRequestConfig {
  showLoading?: boolean
  meta?: RequestMeta
}

/** 模块级 API。由 createApi 返回，JSON 一律按 ApiResult 处理。T 是 data 的行或列表类型。 */
export interface ModuleApi {
  request<T = unknown>(
    method: RequestMethod,
    path?: string,
    data?: unknown,
    params?: unknown,
    config?: RequestConfig,
  ): Promise<ApiResult<T>>
  get<T = unknown>(path?: string, params?: unknown, config?: RequestConfig): Promise<ApiResult<T>>
  post<T = unknown>(path?: string, data?: unknown, params?: unknown, config?: RequestConfig): Promise<ApiResult<T>>
  put<T = unknown>(path?: string, data?: unknown, config?: RequestConfig): Promise<ApiResult<T>>
  delete<T = unknown>(path?: string, params?: unknown, config?: RequestConfig): Promise<ApiResult<T>>
  patch<T = unknown>(path?: string, data?: unknown, config?: RequestConfig): Promise<ApiResult<T>>
  getURL(path?: string): string
}
