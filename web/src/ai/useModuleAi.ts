import { onScopeDispose, watchEffect } from 'vue'
import { bindScene } from './catalog'
import { useAiStore } from './store'
import type { FgAiContextSource, FgAiTool } from './types'

/**
 * 页面挂载期间上报当前场景。
 * 工具本身在模块 ai.ts 里登记，这里只描述用户正在看的内容。
 * 需要随页面打开才能执行的工具，用 scene 绑定。
 * @param describe - 现算模块号、标题和快照
 * @param scene - 仅在本页打开时生效的工具执行函数，键是工具全名
 */
export function useModuleAi(
  describe: () => FgAiContextSource,
  scene?: () => Array<{ name: string; execute: NonNullable<FgAiTool['execute']> }>,
) {
  const ai = useAiStore()
  let current = ''
  let unbind: Array<() => void> = []
  const stop = watchEffect(() => {
    const source = describe()
    if (current && current !== source.moduleId) ai.detach(current)
    current = source.moduleId
    ai.attach(source)
    for (const stopBind of unbind) stopBind()
    unbind = (scene?.() ?? []).map((item) => bindScene(item.name, item.execute))
  })
  onScopeDispose(() => {
    stop()
    for (const stopBind of unbind) stopBind()
    unbind = []
    if (current) ai.detach(current)
    current = ''
  })
}
