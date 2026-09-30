/**
 * 工具风险。read 可直接调用；write 会改数据，调用时必须带确认。
 */
export type FgAiRisk = 'read' | 'write'

/**
 * 谁能在目录里看见这个工具。
 * signedIn 要已登录，anonymous 只在未登录时出现，any 两种都出现。
 */
export type FgAiAudience = 'signedIn' | 'anonymous' | 'any'

/**
 * platform 走模块接口，页面关着也能调用。
 * scene 只在对应页面打开、并绑定处理函数后才能调用。
 */
export type FgAiScope = 'platform' | 'scene'

/** 参数类型。与发给模型的 JSON Schema 一致。 */
export type FgAiParamType = 'string' | 'number' | 'boolean' | 'string[]'

/**
 * 工具参数。secret 为 true 的值只在调用时传入，不会写进审计。
 */
export interface FgAiParam {
  name: string
  type: FgAiParamType
  description: string
  required?: boolean
  enum?: string[]
  secret?: boolean
}

/**
 * 一个可被助手调用的工具。名称全平台唯一，形如 fg0002.register。
 * resourceCode 对齐按钮码，例如 fg0002:edit。真正拒绝仍由服务端按钮鉴权完成。
 */
export interface FgAiTool {
  name: string
  moduleId: string
  title: string
  description: string
  risk: FgAiRisk
  resourceCode?: string
  audience?: FgAiAudience
  scope?: FgAiScope
  parameters: FgAiParam[]
  execute?: (args: Record<string, unknown>) => Promise<unknown>
}

/**
 * 发给模型的工具说明，结构对齐常见 function calling。
 */
export interface FgAiToolSchema {
  name: string
  moduleId: string
  title: string
  description: string
  risk: FgAiRisk
  resourceCode?: string
  scope: FgAiScope
  parameters: {
    type: 'object'
    properties: Record<string, { type: string; description: string; enum?: string[] }>
    required: string[]
  }
}

/**
 * 当前打开的页面。只描述用户正在看的内容，不携带可执行逻辑。
 */
export interface FgAiContextSource {
  moduleId: string
  title: string
  snapshot: () => Record<string, unknown>
}

/**
 * 助手读到的当前页。
 */
export interface FgAiContextView {
  moduleId: string
  title: string
  snapshot: Record<string, unknown>
}

/**
 * 调用工具时的选项。write 必须 confirm 为 true。
 */
export interface FgAiInvokeOptions {
  confirm?: boolean
}

/**
 * 一次调用的审计。不记录参数，避免口令进入日志。
 */
export interface FgAiAuditEvent {
  at: string
  name: string
  moduleId: string
  risk: FgAiRisk
  ok: boolean
  message?: string
}
