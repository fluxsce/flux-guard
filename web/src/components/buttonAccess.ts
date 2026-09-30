import { useSessionStore } from '@/stores/session'

/**
 * 工具条和右键按模块号加按钮键判断，例如 fg0022 + add → fg0022:add。
 * 写了 code 时用完整权限码。模块没有登记该动作时保持显示。
 */
export function useButtonAllowed() {
  const session = useSessionStore()
  return (moduleId: string | undefined, key: string, code?: string) => {
    if (code) return session.allowButton(code)
    if (!moduleId || !key) return true
    return session.allowAction(moduleId, key)
  }
}
