/**
 * HTTP 插件。
 * 请求插件只能改发出去的 config；响应插件只旁路通知，不能改 ApiResult 返回值。
 */
import type { InternalRequestConfig } from '@/api/requestTypes'
import type { AxiosError, AxiosResponse } from 'axios'

export interface HttpRequestPlugin {
  name: string
  apply(config: InternalRequestConfig): void | Promise<void>
}

export interface HttpResponsePlugin {
  name: string
  onFulfilled?(response: AxiosResponse): void
  onRejected?(error: AxiosError): void
}

export class RequestPluginHelper {
  private readonly requestPlugins: HttpRequestPlugin[] = []
  private readonly responsePlugins: HttpResponsePlugin[] = []

  registerRequest(plugin: HttpRequestPlugin): void {
    if (this.requestPlugins.some((item) => item.name === plugin.name)) {
      return
    }
    this.requestPlugins.push(plugin)
  }

  registerResponse(plugin: HttpResponsePlugin): void {
    if (this.responsePlugins.some((item) => item.name === plugin.name)) {
      return
    }
    this.responsePlugins.push(plugin)
  }

  async applyRequest(config: InternalRequestConfig): Promise<void> {
    for (const plugin of this.requestPlugins) {
      await plugin.apply(config)
    }
  }

  notifyFulfilled(response: AxiosResponse): void {
    for (const plugin of this.responsePlugins) {
      try {
        plugin.onFulfilled?.(response)
      } catch (error) {
        if (import.meta.env.DEV) {
          console.warn('[http plugin]', plugin.name, error)
        }
      }
    }
  }

  notifyRejected(error: AxiosError): void {
    for (const plugin of this.responsePlugins) {
      try {
        plugin.onRejected?.(error)
      } catch (pluginError) {
        if (import.meta.env.DEV) {
          console.warn('[http plugin]', plugin.name, pluginError)
        }
      }
    }
  }
}

export const requestPluginHelper = new RequestPluginHelper()

export function registerHttpPlugin(plugin: HttpRequestPlugin): void {
  requestPluginHelper.registerRequest(plugin)
}

export function registerHttpResponsePlugin(plugin: HttpResponsePlugin): void {
  requestPluginHelper.registerResponse(plugin)
}
