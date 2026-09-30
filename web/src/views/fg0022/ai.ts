import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString, argStrings } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { createRole, deleteRole, queryResources, queryRoles, saveRoleResources, updateRole, type RoleInput } from './api'

const flag = (name: string, description: string) => ({
  name,
  type: 'string' as const,
  description,
  enum: ['Y', 'N'],
})

function roleBody(args: Record<string, unknown>): RoleInput {
  return {
    roleId: argString(args, 'roleId'),
    roleName: argString(args, 'roleName'),
    roleDescription: argString(args, 'roleDescription'),
    roleStatus: argString(args, 'roleStatus') || 'Y',
    dataScope: argString(args, 'dataScope'),
    activeFlag: 'Y',
    noteText: argString(args, 'noteText'),
  }
}

const roleFields = [
  { name: 'roleId', type: 'string' as const, description: '角色编码', required: true },
  { name: 'roleName', type: 'string' as const, description: '角色名称', required: true },
  { name: 'roleDescription', type: 'string' as const, description: '说明' },
  flag('roleStatus', '状态，Y 启用，N 停用'),
  { name: 'dataScope', type: 'string' as const, description: '数据范围' },
  { name: 'noteText', type: 'string' as const, description: '备注' },
]

defineTools([
  {
    name: 'fg0022.query',
    moduleId: 'fg0022',
    title: '查询角色',
    description: '按页列出角色。返回 items 和 total。',
    risk: 'read',
    resourceCode: 'fg0022:search',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'roleName', type: 'string', description: '角色名，包含匹配' },
      { name: 'roleStatus', type: 'string', description: '角色状态' },
      flag('builtInFlag', '是否内置'),
    ],
    execute: async (args) => unwrap(await queryRoles(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      roleName: argString(args, 'roleName'),
      roleStatus: argString(args, 'roleStatus'),
      builtInFlag: argString(args, 'builtInFlag'),
    })), '加载失败'),
  },
  {
    name: 'fg0022.create',
    moduleId: 'fg0022',
    title: '新增角色',
    description: '创建角色。',
    risk: 'write',
    resourceCode: 'fg0022:add',
    parameters: roleFields,
    execute: async (args) => unwrap(await createRole(roleBody(args)), '保存失败'),
  },
  {
    name: 'fg0022.update',
    moduleId: 'fg0022',
    title: '修改角色',
    description: '修改角色。',
    risk: 'write',
    resourceCode: 'fg0022:edit',
    parameters: roleFields,
    execute: async (args) => unwrap(await updateRole(roleBody(args)), '保存失败'),
  },
  {
    name: 'fg0022.delete',
    moduleId: 'fg0022',
    title: '删除角色',
    description: '按角色编码删除。内置角色由服务端拒绝。',
    risk: 'write',
    resourceCode: 'fg0022:delete',
    parameters: [{ name: 'roleId', type: 'string', description: '角色编码', required: true }],
    execute: async (args) => unwrap(await deleteRole(argString(args, 'roleId')), '删除失败'),
  },
  {
    name: 'fg0022.queryResources',
    moduleId: 'fg0022',
    title: '查询角色授权',
    description: '读取某个角色的资源树和勾选状态。',
    risk: 'read',
    resourceCode: 'fg0022:grant',
    parameters: [{ name: 'roleId', type: 'string', description: '角色编码', required: true }],
    execute: async (args) => unwrap(await queryResources(argString(args, 'roleId')), '加载失败'),
  },
  {
    name: 'fg0022.saveResources',
    moduleId: 'fg0022',
    title: '保存角色授权',
    description: '用资源 ID 列表覆盖该角色的授权。',
    risk: 'write',
    resourceCode: 'fg0022:grant',
    parameters: [
      { name: 'roleId', type: 'string', description: '角色编码', required: true },
      { name: 'resourceIds', type: 'string[]', description: '资源 ID 列表', required: true },
    ],
    execute: async (args) => unwrap(await saveRoleResources(argString(args, 'roleId'), argStrings(args, 'resourceIds')), '保存失败'),
  },
  {
    name: 'fg0022.applySearch',
    moduleId: 'fg0022',
    title: '筛选角色列表',
    description: '把筛选条件应用到已经打开的角色页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0022:search',
    scope: 'scene',
    parameters: [
      { name: 'roleName', type: 'string', description: '角色名称' },
      flag('roleStatus', '状态'),
      flag('builtInFlag', '是否内置'),
    ],
  },
])
