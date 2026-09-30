import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'

// 目录名是 login。接口前缀仍是 /fluxguard/api/auth，与后端登记的前缀一致。
const loginApi = createApi(moduleApiPrefix('auth'))

export interface SessionUser {
  userId: string
  userName: string
  realName: string
  tenantId: string
  /** 租户管理员，Y 或 N。菜单仍按角色授权返回，前端用它放行按钮。 */
  tenantAdminFlag?: string
  expireAt?: string
}

export interface Menu {
  resourceCode: string
  resourceName: string
  resourcePath: string
  parentResourceId: string
  groupName: string
  sortOrder: number
  iconClass?: string
  /** 当前用户已授权的按钮编码。 */
  buttons?: string[]
  /** 该模块登记的按钮动作，如 view、edit。 */
  actions?: string[]
}

export interface LoginCaptcha {
  captchaId: string
  captchaCode: string
}

export interface CaptchaTicket {
  captchaId: string
  image: string
  expireAt: number
}

export function login(userId: string, password: string, captcha?: LoginCaptcha) {
  return loginApi.post('/login', {
    userId,
    password,
    ...(captcha ? { captchaId: captcha.captchaId, captchaCode: captcha.captchaCode } : {}),
  })
}

export function captcha() {
  return loginApi.post<CaptchaTicket>('/captcha', { type: 'random' }, undefined, { showLoading: false })
}

export function logout() {
  return loginApi.post('/logout')
}

export function me() {
  return loginApi.get('/me')
}

export interface SelfProfile {
  userId: string
  userName: string
  realName: string
  email: string
  mobile: string
  gender: number
}

export interface SelfProfileInput {
  realName: string
  email: string
  mobile: string
  gender: number
  currentPassword?: string
  newPassword?: string
}

export function profile() {
  return loginApi.get('/profile')
}

export function saveProfile(body: SelfProfileInput) {
  return loginApi.post('/profile', body)
}

export function menus() {
  return loginApi.get('/menus')
}
