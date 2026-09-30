import { FgAiError } from './error'
import type { FgAiParam, FgAiTool, FgAiToolSchema } from './types'

/**
 * 把工具定义收成模型可消费的参数结构。
 * @param tool - 已登记工具
 * @returns function calling 用的说明
 */
export function toolSchema(tool: FgAiTool): FgAiToolSchema {
  const properties: FgAiToolSchema['parameters']['properties'] = {}
  const required: string[] = []
  for (const param of tool.parameters) {
    properties[param.name] = {
      type: param.type === 'string[]' ? 'array' : param.type,
      description: param.description,
      ...(param.enum ? { enum: param.enum } : {}),
    }
    if (param.required) required.push(param.name)
  }
  return {
    name: tool.name,
    moduleId: tool.moduleId,
    title: tool.title,
    description: tool.description,
    risk: tool.risk,
    resourceCode: tool.resourceCode,
    scope: tool.scope ?? 'platform',
    parameters: { type: 'object', properties, required },
  }
}

/**
 * 按工具参数声明校验并裁剪入参。未声明的字段丢掉。
 * @param tool - 已登记工具
 * @param input - 调用方传入的参数
 * @returns 只含已声明字段的参数
 */
export function validateArgs(tool: FgAiTool, input: unknown): Record<string, unknown> {
  const source = input && typeof input === 'object' && !Array.isArray(input) ? (input as Record<string, unknown>) : {}
  const args: Record<string, unknown> = {}
  for (const param of tool.parameters) {
    const value = source[param.name]
    if (value === undefined || value === null || value === '') {
      if (param.required) throw new FgAiError(`缺少参数 ${param.name}`)
      continue
    }
    args[param.name] = checkParam(param, value)
  }
  return args
}

function checkParam(param: FgAiParam, value: unknown): unknown {
  if (param.enum && !param.enum.includes(String(value))) {
    throw new FgAiError(`参数 ${param.name} 必须是 ${param.enum.join('、')}`)
  }
  if (param.type === 'string') {
    if (typeof value !== 'string') throw new FgAiError(`参数 ${param.name} 必须是字符串`)
    return value
  }
  if (param.type === 'number') {
    if (typeof value !== 'number' || !Number.isFinite(value)) throw new FgAiError(`参数 ${param.name} 必须是数字`)
    return value
  }
  if (param.type === 'boolean') {
    if (typeof value !== 'boolean') throw new FgAiError(`参数 ${param.name} 必须是布尔值`)
    return value
  }
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) {
    throw new FgAiError(`参数 ${param.name} 必须是字符串数组`)
  }
  return value
}

/**
 * 读取已经校验过的字符串参数。
 * @param args - validateArgs 的结果
 * @param name - 参数名
 * @returns 字符串，没有时为空
 */
export function argString(args: Record<string, unknown>, name: string) {
  return typeof args[name] === 'string' ? args[name] : ''
}

/**
 * 读取已经校验过的数字参数。
 * @param args - validateArgs 的结果
 * @param name - 参数名
 * @param fallback - 未传时的值
 * @returns 数字
 */
export function argNumber(args: Record<string, unknown>, name: string, fallback: number) {
  return typeof args[name] === 'number' ? args[name] : fallback
}

/**
 * 读取已经校验过的字符串数组。
 * @param args - validateArgs 的结果
 * @param name - 参数名
 * @returns 字符串数组
 */
export function argStrings(args: Record<string, unknown>, name: string) {
  return Array.isArray(args[name]) ? (args[name] as string[]) : []
}
