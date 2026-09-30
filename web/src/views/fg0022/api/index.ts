import { createApi } from '@/api/request'
import { moduleApiPrefix } from '@/api/requestPath'
import type { ApiResult } from '@/api/result'
import type { PageData } from '@/api/pagination'

const fg0022 = createApi(moduleApiPrefix('fg0022'))

export interface Role {
  roleId: string
  roleName: string
  roleDescription: string
  roleStatus: string
  builtInFlag: string
  dataScope: string
  activeFlag: string
  noteText: string
  addTime: string
  addWho: string
  editTime: string
  editWho: string
}

export interface Resource {
  resourceId: string
  resourceName: string
  resourceCode: string
  resourceType: string
  checked?: boolean
  children?: Resource[]
}

export interface RoleInput {
  roleId: string
  roleName: string
  roleDescription: string
  roleStatus: string
  dataScope: string
  activeFlag: string
  noteText: string
}

export function queryRoles(params: Record<string, string | number>): Promise<ApiResult<PageData<Role>>> {
  return fg0022.get<PageData<Role>>('/queryRoles', params)
}

/** 按角色编码取当前这一行。查看和修改都先用它，不用列表里的旧数据。 */
export function getRole(roleId: string): Promise<ApiResult<Role>> {
  return fg0022.get<Role>('/getRole', { roleId })
}

/** 新增角色。data 与列表中的一行相同。 */
export function createRole(body: RoleInput): Promise<ApiResult<Role>> {
  return fg0022.post<Role>('/createRole', body)
}

/** 修改角色。data 与列表中的一行相同。 */
export function updateRole(body: RoleInput): Promise<ApiResult<Role>> {
  return fg0022.post<Role>('/updateRole', body)
}

export function deleteRole(roleId: string) {
  return fg0022.post('/deleteRole', { roleId })
}

export function queryResources(roleId: string) {
  return fg0022.get('/queryResources', { roleId })
}

export function saveRoleResources(roleId: string, resourceIds: string[]) {
  return fg0022.post('/saveRoleResources', { roleId, resourceIds })
}
