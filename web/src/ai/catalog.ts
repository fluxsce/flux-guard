import type { FgAiTool } from './types'

const tools = new Map<string, FgAiTool>()
const sceneHandlers = new Map<string, NonNullable<FgAiTool['execute']>>()

const NAME = /^[a-z][a-z0-9]*\.[a-z][a-zA-Z0-9]*$/

/**
 * 登记一个模块的工具。名称重复时抛错，扩展包不能悄悄盖掉已有工具。
 * 新模块在自己的 ai.ts 里调用，再由 ai/modules.ts 引入。
 * @param items - 该模块的工具
 */
export function defineTools(items: FgAiTool[]) {
  for (const item of items) {
    if (!NAME.test(item.name)) throw new Error(`工具名必须是模块.动作：${item.name}`)
    const moduleId = item.name.slice(0, item.name.indexOf('.'))
    if (item.moduleId !== moduleId) throw new Error(`工具 ${item.name} 的模块号不一致`)
    if (item.scope !== 'scene' && !item.execute) throw new Error(`工具 ${item.name} 缺少执行函数`)
    if (tools.has(item.name)) throw new Error(`工具名重复：${item.name}`)
    tools.set(item.name, item)
  }
}

/**
 * 页面打开时绑定 scene 工具的执行函数。页面关闭时调用返回的函数解绑。
 * @param name - 工具全名
 * @param execute - 当前页上的处理函数
 * @returns 解绑
 */
export function bindScene(name: string, execute: NonNullable<FgAiTool['execute']>) {
  const tool = tools.get(name)
  if (!tool || (tool.scope ?? 'platform') !== 'scene') {
    throw new Error(`没有可绑定的场景工具 ${name}`)
  }
  sceneHandlers.set(name, execute)
  return () => {
    if (sceneHandlers.get(name) === execute) sceneHandlers.delete(name)
  }
}

/**
 * 按全名取工具。
 * @param name - 工具全名
 * @returns 工具；未登记时为 undefined
 */
export function getTool(name: string) {
  return tools.get(name)
}

/**
 * 全部已登记工具，未按登录态过滤。
 * @returns 工具列表
 */
export function allTools() {
  return [...tools.values()]
}

/**
 * 取出场景工具当前绑定的执行函数。
 * @param name - 工具全名
 * @returns 处理函数；页面未打开时为 undefined
 */
export function sceneHandler(name: string) {
  return sceneHandlers.get(name)
}
