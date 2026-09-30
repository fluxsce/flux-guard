/**
 * HTTP 传输层错误。
 * 仅网络中断、超时、非统一响应的 HTTP 失败会抛出本类。
 * 业务失败（含 403）走 ApiResult.ok=false，不使用本类。
 */
export class HttpTransportError extends Error {
  readonly name = 'HttpTransportError'
  readonly code: number
  readonly data: unknown
  readonly requestId: string
  readonly module?: string
  readonly url?: string

  constructor(options: {
    message: string
    code: number
    data?: unknown
    requestId?: string
    module?: string
    url?: string
  }) {
    super(options.message)
    this.code = options.code
    this.data = options.data
    this.requestId = options.requestId ?? ''
    this.module = options.module
    this.url = options.url
  }
}

/** 请求被取消。catch 里用 isHttpCancelledError 判断，不要当网络故障提示。 */
export class HttpCancelledError extends Error {
  readonly name = 'HttpCancelledError'
  readonly cancelled = true as const

  constructor(message = '请求已取消') {
    super(message)
  }
}

export function isHttpTransportError(error: unknown): error is HttpTransportError {
  return error instanceof HttpTransportError
}

export function isHttpCancelledError(error: unknown): error is HttpCancelledError {
  if (error instanceof HttpCancelledError) {
    return true
  }
  return (
    typeof error === 'object' &&
    error !== null &&
    'cancelled' in error &&
    (error as { cancelled?: unknown }).cancelled === true
  )
}

/** 从 catch 里取出可展示文案。取消返回空串。 */
export function getHttpErrorMessage(error: unknown, fallback: string): string {
  if (isHttpCancelledError(error)) {
    return ''
  }
  if (isHttpTransportError(error) && error.message) {
    return error.message
  }
  if (error instanceof Error && error.message) {
    return error.message
  }
  return fallback
}
