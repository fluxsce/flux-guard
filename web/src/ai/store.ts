import { computed, ref, shallowRef } from 'vue'
import { defineStore } from 'pinia'
import { allTools, getTool, sceneHandler } from './catalog'
import { FgAiError } from './error'
import { toolVisible } from './permission'
import { redact } from './redact'
import type { FgAiAuditEvent, FgAiContextSource, FgAiContextView, FgAiInvokeOptions } from './types'
import { toolSchema, validateArgs } from './validate'

const AUDIT_LIMIT = 50

function viewContext(source: FgAiContextSource): FgAiContextView {
  return {
    moduleId: source.moduleId,
    title: source.title,
    snapshot: redact(source.snapshot()) as Record<string, unknown>,
  }
}

/**
 * 全平台助手入口。
 * 工具目录在启动时登记，不依赖页面是否打开。
 * 页面只提供当前场景。写操作要确认，返回值会去掉口令和令牌。
 */
export const useAiStore = defineStore('ai', () => {
  // 页面里的 watchEffect 会调用 attach。读写都走这份普通 Map，避免效果订阅到 contexts 后再被自己的写入重新触发。
  let registry = new Map<string, FgAiContextSource>()
  const contexts = shallowRef(registry)
  const activeModuleId = shallowRef('')
  const audits = ref<FgAiAuditEvent[]>([])

  const active = computed(() => contexts.value.get(activeModuleId.value) ?? null)

  function publish(next: Map<string, FgAiContextSource>) {
    registry = next
    contexts.value = next
  }

  /**
   * 登记当前打开的页面场景。
   * @param source - 模块号、标题和快照
   */
  function attach(source: FgAiContextSource) {
    const next = new Map(registry)
    next.set(source.moduleId, source)
    publish(next)
  }

  /**
   * 页面关闭时去掉场景。
   * @param moduleId - 模块号
   */
  function detach(moduleId: string) {
    if (!registry.has(moduleId)) return
    const next = new Map(registry)
    next.delete(moduleId)
    publish(next)
  }

  /**
   * 标记助手正在看的模块。
   * @param moduleId - 路由上的模块号
   */
  function setActive(moduleId: string) {
    activeModuleId.value = moduleId
  }

  /**
   * 读取页面场景。不传模块号时读当前页。
   * @param moduleId - 模块号
   * @returns 场景；页面未打开时为 null
   */
  function context(moduleId?: string): FgAiContextView | null {
    const source = contexts.value.get(moduleId || activeModuleId.value)
    return source ? viewContext(source) : null
  }

  /**
   * 当前用户可见的工具，已换成模型用的参数结构。
   * @returns 工具说明
   */
  function tools() {
    return allTools().filter(toolVisible).map(toolSchema)
  }

  function pushAudit(event: FgAiAuditEvent) {
    audits.value = [...audits.value, event].slice(-AUDIT_LIMIT)
  }

  /**
   * 按工具全名调用。页面关着时，platform 工具仍然走模块接口。
   * @param name - 如 fg0002.register
   * @param input - 与参数声明一致的对象
   * @param options - 写操作确认
   * @returns 去掉敏感字段后的结果
   */
  async function invoke(name: string, input?: unknown, options?: FgAiInvokeOptions) {
    const tool = getTool(name)
    if (!tool || !toolVisible(tool)) throw new FgAiError('没有这个工具或当前账号不可用')
    if (tool.risk === 'write' && options?.confirm !== true) {
      throw new FgAiError(`工具 ${name} 会改数据，调用时要带确认`)
    }
    const args = validateArgs(tool, input)
    const execute = (tool.scope ?? 'platform') === 'scene' ? sceneHandler(name) : tool.execute
    if (!execute) throw new FgAiError('需要先打开对应页面')
    try {
      const result = redact(await execute(args))
      pushAudit({ at: new Date().toISOString(), name, moduleId: tool.moduleId, risk: tool.risk, ok: true })
      return result
    } catch (err) {
      const message = err instanceof Error ? err.message : '调用失败'
      pushAudit({ at: new Date().toISOString(), name, moduleId: tool.moduleId, risk: tool.risk, ok: false, message })
      throw err
    }
  }

  return { activeModuleId, active, audits, attach, detach, setActive, context, tools, invoke }
})
