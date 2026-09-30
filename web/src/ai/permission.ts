import { useSessionStore } from '@/stores/session'
import type { FgAiTool } from './types'

type Checker = (tool: FgAiTool) => boolean

let checker: Checker | null = null

/**
 * 替换客户端的工具可见性判断。
 * 默认按已授权模块过滤。按钮码进入会话后，用这个接上更细的判断。
 * 服务端按钮鉴权始终生效，这里只决定目录里展示哪些工具。
 * @param next - 返回 true 表示当前用户可以看见并尝试调用
 * @returns 恢复上一个判断
 */
export function registerAiPermission(next: Checker) {
  const previous = checker
  checker = next
  return () => {
    checker = previous
  }
}

/**
 * 当前用户是否可以在目录里看到该工具。
 * @param tool - 已登记工具
 * @returns 是否可见
 */
export function toolVisible(tool: FgAiTool) {
  const session = useSessionStore()
  const audience = tool.audience ?? 'signedIn'
  if (audience === 'anonymous' && session.user) return false
  if (audience === 'signedIn' && !session.user) return false
  if (checker) return checker(tool)
  if (!tool.resourceCode) return true
  if (session.allowModule(tool.resourceCode)) return true
  return session.allowModule(tool.moduleId)
}
