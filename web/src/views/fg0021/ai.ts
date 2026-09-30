import { defineTools } from '@/ai/catalog'
import { unwrap } from '@/ai/call'
import { argNumber, argString, argStrings } from '@/ai/validate'
import { listParams, PAGE_SIZE_CEILING } from '@/api/pagination'
import { createUser, deleteUser, queryUserRoles, queryUsers, saveUserRoles, updateUser, type UserInput } from './api'

const flag = (name: string, description: string): { name: string; type: 'string'; description: string; enum: string[] } => ({
  name,
  type: 'string',
  description,
  enum: ['Y', 'N'],
})

function userBody(args: Record<string, unknown>): UserInput {
  return {
    userId: argString(args, 'userId'),
    userName: argString(args, 'userName'),
    password: argString(args, 'password'),
    realName: argString(args, 'realName'),
    email: argString(args, 'email'),
    mobile: argString(args, 'mobile'),
    gender: argNumber(args, 'gender', 0),
    statusFlag: argString(args, 'statusFlag') || 'Y',
    tenantAdminFlag: argString(args, 'tenantAdminFlag') || 'N',
    userExpireDate: argString(args, 'userExpireDate'),
    activeFlag: 'Y',
    noteText: argString(args, 'noteText'),
  }
}

const userFields = [
  { name: 'userId', type: 'string' as const, description: '用户 ID', required: true },
  { name: 'userName', type: 'string' as const, description: '用户自定义名称', required: true },
  { name: 'realName', type: 'string' as const, description: '姓名', required: true },
  { name: 'email', type: 'string' as const, description: '邮箱' },
  { name: 'mobile', type: 'string' as const, description: '手机' },
  { name: 'gender', type: 'number' as const, description: '性别：0 未知，1 男，2 女' },
  flag('statusFlag', '状态，Y 启用，N 停用'),
  flag('tenantAdminFlag', '是否租户管理员'),
  { name: 'userExpireDate', type: 'string' as const, description: '过期时间' },
  { name: 'noteText', type: 'string' as const, description: '备注' },
]

defineTools([
  {
    name: 'fg0021.query',
    moduleId: 'fg0021',
    title: '查询用户',
    description: '按页列出租户下的用户。返回 items 和 total。',
    risk: 'read',
    resourceCode: 'fg0021:search',
    parameters: [
      { name: 'page', type: 'number', description: '页码，从 1 开始' },
      { name: 'pageSize', type: 'number', description: '每页条数，最大 200' },
      { name: 'userName', type: 'string', description: '用户名，包含匹配' },
      { name: 'realName', type: 'string', description: '姓名，包含匹配' },
      flag('statusFlag', '状态'),
    ],
    execute: async (args) => unwrap(await queryUsers(listParams(argNumber(args, 'page', 1), argNumber(args, 'pageSize', PAGE_SIZE_CEILING), {
      userName: argString(args, 'userName'),
      realName: argString(args, 'realName'),
      statusFlag: argString(args, 'statusFlag'),
    })), '加载失败'),
  },
  {
    name: 'fg0021.create',
    moduleId: 'fg0021',
    title: '新增用户',
    description: '创建用户。口令只在本次调用中使用。',
    risk: 'write',
    resourceCode: 'fg0021:add',
    parameters: [{ name: 'password', type: 'string', description: '登录口令', required: true, secret: true }, ...userFields],
    execute: async (args) => unwrap(await createUser(userBody(args)), '保存失败'),
  },
  {
    name: 'fg0021.update',
    moduleId: 'fg0021',
    title: '修改用户',
    description: '修改用户资料。口令留空表示不改。',
    risk: 'write',
    resourceCode: 'fg0021:edit',
    parameters: [{ name: 'password', type: 'string', description: '新口令，留空则不修改', secret: true }, ...userFields],
    execute: async (args) => unwrap(await updateUser(userBody(args)), '保存失败'),
  },
  {
    name: 'fg0021.delete',
    moduleId: 'fg0021',
    title: '删除用户',
    description: '按用户 ID 删除用户。',
    risk: 'write',
    resourceCode: 'fg0021:delete',
    parameters: [{ name: 'userId', type: 'string', description: '用户 ID', required: true }],
    execute: async (args) => unwrap(await deleteUser(argString(args, 'userId')), '删除失败'),
  },
  {
    name: 'fg0021.queryRoles',
    moduleId: 'fg0021',
    title: '查询用户角色',
    description: '读取某个用户已勾选的角色。',
    risk: 'read',
    resourceCode: 'fg0021:grant',
    parameters: [{ name: 'userId', type: 'string', description: '用户 ID', required: true }],
    execute: async (args) => unwrap(await queryUserRoles(argString(args, 'userId')), '加载失败'),
  },
  {
    name: 'fg0021.saveRoles',
    moduleId: 'fg0021',
    title: '保存用户角色',
    description: '用角色 ID 列表覆盖该用户的角色。',
    risk: 'write',
    resourceCode: 'fg0021:grant',
    parameters: [
      { name: 'userId', type: 'string', description: '用户 ID', required: true },
      { name: 'roleIds', type: 'string[]', description: '角色 ID 列表', required: true },
    ],
    execute: async (args) => unwrap(await saveUserRoles(argString(args, 'userId'), argStrings(args, 'roleIds')), '保存失败'),
  },
  {
    name: 'fg0021.applySearch',
    moduleId: 'fg0021',
    title: '筛选用户列表',
    description: '把筛选条件应用到已经打开的用户页。页面未打开时不能调用。',
    risk: 'read',
    resourceCode: 'fg0021:search',
    scope: 'scene',
    parameters: [
      { name: 'userName', type: 'string', description: '用户名' },
      { name: 'realName', type: 'string', description: '姓名' },
      flag('statusFlag', '状态'),
    ],
  },
])
