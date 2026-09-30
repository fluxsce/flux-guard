import { useAiStore } from './store'
import type { FgAiAuditEvent, FgAiContextView, FgAiInvokeOptions, FgAiToolSchema } from './types'

/**
 * 停靠位的 DOM id。后期助手挂到这里，现在保持为空。
 */
export const FG_AI_DOCK_ID = 'fg-ai-dock'

/**
 * 助手和页外自动化共用的入口。
 * tools 是全平台目录，context 是当前页，invoke 执行已登记工具。
 */
export interface FgAiBridge {
  tools: () => FgAiToolSchema[]
  context: (moduleId?: string) => FgAiContextView | null
  invoke: (name: string, args?: unknown, options?: FgAiInvokeOptions) => Promise<unknown>
  audits: () => FgAiAuditEvent[]
}

declare global {
  interface Window {
    __fgAi?: FgAiBridge
  }
}

/**
 * 在 Pinia 装好之后挂上 window.__fgAi。
 */
export function installAiBridge() {
  const ai = useAiStore()
  window.__fgAi = {
    tools: () => ai.tools(),
    context: (moduleId?: string) => ai.context(moduleId),
    invoke: (name, args, options) => ai.invoke(name, args, options),
    audits: () => ai.audits,
  }
}
