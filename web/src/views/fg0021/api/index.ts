import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const fg0021 = createApi(moduleApiPrefix('fg0021'))

export interface User {
  userId: string
  userName: string
  realName: string
  email: string
  mobile: string
  gender: number
  roleIds: string[]
  roleNames: string
  statusFlag: string
  tenantAdminFlag: string
  userExpireDate: string
  lastLoginTime: string
  lastLoginIp: string
  activeFlag: string
  noteText: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
}

export interface UserInput {
  userId: string
  userName: string
  password: string
  realName: string
  email: string
  mobile: string
  gender: number
  statusFlag: string
  tenantAdminFlag: string
  userExpireDate: string
  activeFlag: string
  noteText: string
}

export interface UserRole {
  roleId: string
  roleName: string
  roleDescription: string
  checked?: boolean
}

export function queryUsers(params: Record<string, string | number>): Promise<ApiResult<PageData<User>>> {
  return fg0021.get<PageData<User>>('/queryUsers', params)
}

/** 新增用户。data 与列表中的一行相同。 */
export function createUser(body: UserInput): Promise<ApiResult<User>> {
  return fg0021.post<User>('/createUser', body)
}

/** 修改用户。data 与列表中的一行相同。 */
export function updateUser(body: UserInput): Promise<ApiResult<User>> {
  return fg0021.post<User>('/updateUser', body)
}

export function deleteUser(userId: string) {
  return fg0021.post('/deleteUser', { userId })
}

export function queryUserRoles(userId: string) {
  return fg0021.get('/queryUserRoles', { userId })
}

/** 保存用户角色。data 为带角色名的这一行，与列表项相同。 */
export function saveUserRoles(userId: string, roleIds: string[]): Promise<ApiResult<User>> {
  return fg0021.post<User>('/saveUserRoles', { userId, roleIds })
}
