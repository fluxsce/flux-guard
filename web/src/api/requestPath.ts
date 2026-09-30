/**
 * 请求路径辅助。
 * 负责模块前缀拼接，以及从 /fluxguard/api/fgxxxx 读取模块编码。
 * 路径约定变化只改本文件，不要写回 request.ts。
 */

/** 管理端接口前缀。带尾斜杠便于切分，改根路径只改这一处。 */
export const API_PREFIX = '/fluxguard/api/'

export class RequestPathHelper {
  /** 拼接模块前缀与子路径，避免出现 // 或漏 /。 */
  join(baseURL: string, path = ''): string {
    if (!path) {
      return baseURL
    }
    return `${baseURL.replace(/\/$/, '')}/${path.replace(/^\//, '')}`
  }

  /** 模块前缀，例如 modulePrefix('fg0002') 得到 /fluxguard/api/fg0002。登录传 auth。 */
  modulePrefix(moduleName: string): string {
    return this.join(API_PREFIX, moduleName)
  }

  /**
   * 读取模块编码：/fluxguard/api/ 后的第一段（fg0002、auth）。
   * 模块身份以 createApi 前缀为准；拦截器只给未带 meta.module 的裸 request 做同样切分。
   */
  readModule(url?: string): string | undefined {
    if (!url) {
      return undefined
    }
    const path = this.takePathname(url)
    const start = path.indexOf(API_PREFIX)
    if (start < 0) {
      return undefined
    }
    const moduleSeg = path.slice(start + API_PREFIX.length).split('/')[0]
    if (!moduleSeg || (!moduleSeg.startsWith('fg') && moduleSeg !== 'auth')) {
      return undefined
    }
    return moduleSeg
  }

  /** 取 pathname，去掉 query/hash。 */
  takePathname(url: string): string {
    const withoutQuery = url.split('?')[0].split('#')[0]
    if (withoutQuery.includes('://')) {
      try {
        return new URL(withoutQuery).pathname
      } catch {
        return withoutQuery
      }
    }
    return withoutQuery.startsWith('/') ? withoutQuery : `/${withoutQuery}`
  }
}

export const requestPathHelper = new RequestPathHelper()

export function moduleApiPrefix(moduleName: string): string {
  return requestPathHelper.modulePrefix(moduleName)
}
