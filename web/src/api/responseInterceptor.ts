/**
 * 响应拦截器。
 * 统一响应（含 HTTP 4xx/5xx）resolve 给页面；会话 401 弹窗且不落到业务。
 * 登录接口的 401 是口令错误，按业务失败返回。
 * 仅取消、超时、断网、非统一响应才 reject。
 */
import { HttpCancelledError, HttpTransportError } from '@/api/requestError'
import { requestHeaderHelper } from '@/api/requestHeader'
import { requestLoadingHelper } from '@/api/requestLoading'
import { requestPathHelper } from '@/api/requestPath'
import { requestPluginHelper } from '@/api/requestPlugin'
import type { InternalRequestConfig } from '@/api/requestTypes'
import type { ApiResult } from '@/api/result'
import { rsConfirm } from '@/ui'
import type { AxiosError, AxiosInstance, AxiosResponse } from 'axios'
import axios from 'axios'

export class ResponseInterceptorHelper {
  private is401DialogShowing = false

  attach(http: AxiosInstance): void {
    http.interceptors.response.use(
      (response: AxiosResponse) => {
        requestLoadingHelper.end((response.config as InternalRequestConfig).showLoading)
        requestPluginHelper.notifyFulfilled(response)
        if (response.config.responseType === 'blob') {
          return response
        }
        return response.data
      },
      (error: AxiosError) => {
        requestLoadingHelper.end((error.config as InternalRequestConfig | undefined)?.showLoading)
        requestPluginHelper.notifyRejected(error)

        if (axios.isCancel(error)) {
          return Promise.reject(new HttpCancelledError())
        }

        const status = error.response?.status
        const data = error.response?.data
        const reqConfig = error.config as InternalRequestConfig | undefined
        const requestId = requestHeaderHelper.readRequestId(reqConfig?.headers)

        if (status === 401 && !this.isLogin(reqConfig?.url)) {
          return this.handle401(this.readError(data, '未授权，请重新登录'))
        }

        if (error.response && this.isApiBody(data)) {
          return this.toBizFailure(data, this.statusFallback(status ?? 0))
        }

        let message = '网络异常，请稍后重试'
        if (error.response) {
          message = this.statusFallback(status ?? 0)
        } else if (error.request) {
          message = '服务器无响应，请检查网络连接'
        }

        const transportError = new HttpTransportError({
          message,
          code: status ?? -1,
          data,
          requestId,
          module: reqConfig?.meta?.module,
          url: reqConfig?.url,
        })

        if (import.meta.env.DEV) {
          console.warn('[http]', message, {
            requestId: transportError.requestId,
            module: transportError.module,
            url: transportError.url,
            status,
          })
        }

        return Promise.reject(transportError)
      },
    )
  }

  private isLogin(url?: string): boolean {
    if (!url) {
      return false
    }
    return requestPathHelper.takePathname(url).endsWith('/auth/login')
  }

  private isApiBody(data: unknown): data is Partial<ApiResult> {
    return !!data && typeof data === 'object' && ('ok' in data || 'error' in data)
  }

  private readError(data: unknown, fallback: string): string {
    if (!this.isApiBody(data) || typeof data.error !== 'string' || !data.error.trim()) {
      return fallback
    }
    return data.error
  }

  private toBizFailure(data: unknown, fallback: string): ApiResult {
    const body = this.isApiBody(data) ? data : {}
    const code = typeof body.code === 'string' ? body.code : undefined
    return {
      ok: false,
      data: body.data,
      code,
      error: this.readError(body, fallback),
    }
  }

  private statusFallback(status: number): string {
    switch (status) {
      case 400:
        return '请求参数错误'
      case 403:
        return '没有执行此操作的权限'
      case 404:
        return '请求资源不存在'
      case 500:
        return '服务器内部错误'
      default:
        return `请求失败(${status})`
    }
  }

  /** 清会话并弹窗。返回永不 settle 的 Promise，避免页面再提示一次。 */
  private handle401(message: string): Promise<never> {
    requestHeaderHelper.clearSession()
    void import('pinia').then(({ getActivePinia }) => {
      if (!getActivePinia()) return
      return import('@/stores/session').then(({ useSessionStore }) => {
        useSessionStore().reset()
      })
    })
    const loginPath = `${import.meta.env.BASE_URL.replace(/\/$/, '')}/login`
    if (this.is401DialogShowing || window.location.pathname === loginPath) {
      return new Promise(() => {})
    }
    this.is401DialogShowing = true
    void rsConfirm.error({
      title: '登录已失效',
      description: message,
      confirmText: '重新登录',
      confirmVariant: 'primary',
      tone: 'warning',
      showOverlay: false,
      onConfirm: () => {
        this.is401DialogShowing = false
        window.location.href = loginPath
      },
      onCancel: () => {
        this.is401DialogShowing = false
      },
    })
    return new Promise(() => {})
  }
}

export const responseInterceptorHelper = new ResponseInterceptorHelper()
