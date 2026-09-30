import type { AxiosHeaderValue, AxiosRequestHeaders } from 'axios'
import { clearUserCookie } from '@/views/login/sessionCookie'

/** 会话令牌。登录写入，401 与退出时清除。 */
export const SESSION_TOKEN_KEY = 'fg_token'

export const TRACE_ID_HEADER = 'X-Trace-ID'
export const REQUEST_ID_HEADER = 'X-Request-ID'
export const TRACEPARENT_HEADER = 'traceparent'

/**
 * 请求头补丁。
 * 补会话令牌和追踪头。后续要补公共头，加在本类，不要写回 request.ts。
 */
export class RequestHeaderHelper {
  get(headers: AxiosRequestHeaders, name: string): string {
    return this.toHeaderString(headers.get(name))
  }

  set(headers: AxiosRequestHeaders, name: string, value: string): void {
    headers.set(name, value)
  }

  /** 已登录则带 Bearer。调用方已写 Authorization 时不覆盖。 */
  patchAuth(headers: AxiosRequestHeaders): void {
    if (this.get(headers, 'Authorization')) {
      return
    }
    const token = sessionStorage.getItem(SESSION_TOKEN_KEY)
    if (token) {
      this.set(headers, 'Authorization', `Bearer ${token}`)
    }
  }

  /** 发出前补齐追踪头。已有不覆盖。 */
  patchTrace(headers: AxiosRequestHeaders): string {
    const traceId = this.get(headers, TRACE_ID_HEADER)
    const requestId = this.get(headers, REQUEST_ID_HEADER)
    const id = traceId || requestId || this.createId()
    if (!traceId) {
      this.set(headers, TRACE_ID_HEADER, id)
    }
    if (!requestId) {
      this.set(headers, REQUEST_ID_HEADER, id)
    }
    if (!this.get(headers, TRACEPARENT_HEADER)) {
      this.set(headers, TRACEPARENT_HEADER, this.toTraceparent(id))
    }
    return id
  }

  readRequestId(headers?: AxiosRequestHeaders): string {
    if (!headers) {
      return ''
    }
    return this.get(headers, REQUEST_ID_HEADER) || this.get(headers, TRACE_ID_HEADER)
  }

  clearSession(): void {
    sessionStorage.removeItem(SESSION_TOKEN_KEY)
    clearUserCookie()
  }

  private toHeaderString(value: AxiosHeaderValue): string {
    if (typeof value === 'string') {
      return value
    }
    if (Array.isArray(value) && typeof value[0] === 'string') {
      return value[0]
    }
    return ''
  }

  private toTraceparent(requestId: string): string {
    const hex = requestId.replaceAll('-', '').toLowerCase()
    if (hex.length === 32 && /^[0-9a-f]+$/.test(hex)) {
      return `00-${hex}-${hex.slice(16)}-01`
    }
    const bytes = new Uint8Array(24)
    if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
      crypto.getRandomValues(bytes)
    }
    const full = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
    return `00-${full.slice(0, 32)}-${full.slice(32, 48)}-01`
  }

  private createId(): string {
    if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
      const bytes = new Uint8Array(16)
      crypto.getRandomValues(bytes)
      bytes[6] = (bytes[6] & 0x0f) | 0x40
      bytes[8] = (bytes[8] & 0x3f) | 0x80
      const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
      return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
    }
    return `fg-${Date.now().toString(16)}-${Math.random().toString(16).slice(2)}`
  }
}

export const requestHeaderHelper = new RequestHeaderHelper()
