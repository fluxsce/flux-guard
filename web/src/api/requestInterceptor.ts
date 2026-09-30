/**
 * 请求拦截器：发出前补齐模块标签、令牌、追踪头、插件和 loading。
 * 后续要加请求前逻辑改本类，不要写回 request.ts。
 */
import { requestHeaderHelper } from '@/api/requestHeader'
import { requestLoadingHelper } from '@/api/requestLoading'
import { requestPathHelper } from '@/api/requestPath'
import { requestPluginHelper } from '@/api/requestPlugin'
import type { InternalRequestConfig } from '@/api/requestTypes'
import type { AxiosError, AxiosInstance } from 'axios'

export class RequestInterceptorHelper {
  attach(http: AxiosInstance): void {
    http.interceptors.request.use(
      async (cfg: InternalRequestConfig) => {
        cfg.showLoading ??= true
        cfg.meta ??= {}
        cfg.meta.module ??= requestPathHelper.readModule(cfg.url)
        requestHeaderHelper.patchAuth(cfg.headers)
        requestHeaderHelper.patchTrace(cfg.headers)
        await requestPluginHelper.applyRequest(cfg)
        if (cfg.showLoading) {
          requestLoadingHelper.begin()
        }
        return cfg
      },
      (error: AxiosError) => {
        requestLoadingHelper.end((error.config as InternalRequestConfig | undefined)?.showLoading)
        return Promise.reject(error)
      },
    )
  }
}

export const requestInterceptorHelper = new RequestInterceptorHelper()
