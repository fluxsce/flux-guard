/** 前端可读的用户上下文。登录票在 HttpOnly 的 FG_SESSION 里，不放进这枚 Cookie。 */
export const USER_COOKIE = 'fg_user'

export interface CachedUser {
  userId: string
  userName: string
  realName: string
  tenantId: string
  tenantAdminFlag?: string
  expireAt?: string
}

export function readUserCookie(): CachedUser | null {
  if (typeof document === 'undefined') return null
  const raw = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${USER_COOKIE}=`))
  if (!raw) return null
  try {
    const user = JSON.parse(decodeURIComponent(raw.slice(USER_COOKIE.length + 1))) as CachedUser
    if (!user?.userId || accountExpired(user.expireAt)) {
      clearUserCookie()
      return null
    }
    return user
  } catch {
    clearUserCookie()
    return null
  }
}

export function writeUserCookie(user: CachedUser): void {
  if (typeof document === 'undefined' || !user.userId) return
  const value = encodeURIComponent(JSON.stringify({
    userId: user.userId,
    userName: user.userName,
    realName: user.realName,
    tenantId: user.tenantId,
    tenantAdminFlag: user.tenantAdminFlag === 'Y' ? 'Y' : 'N',
    expireAt: user.expireAt || '',
  }))
  document.cookie = `${USER_COOKIE}=${value}; Path=/; SameSite=Lax`
}

export function clearUserCookie(): void {
  if (typeof document === 'undefined') return
  document.cookie = `${USER_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`
}

function accountExpired(expireAt?: string): boolean {
  if (!expireAt) return false
  const text = expireAt.includes('T') ? expireAt : `${expireAt.replace(' ', 'T')}Z`
  const at = Date.parse(text)
  return Number.isFinite(at) && at <= Date.now()
}
