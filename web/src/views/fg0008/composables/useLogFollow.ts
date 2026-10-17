import { onUnmounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import {
  closeLogView,
  logOutput,
  openLogView,
  touchLogView,
  type LogViewChunk,
  type LogViewSession,
} from '../api'

/**
 * 应用日志跟读。启动/重启时把 catalina 末尾接到操作区，不另开弹窗。
 */
export function useLogFollow() {
  const text = ref('')
  const fileName = ref('')
  const live = ref(false)
  const closed = ref(false)
  let sessionId = ''
  let offset = 0
  let alive = false
  let generation = 0
  let pollTimer = 0
  let touchTimer = 0
  let releaseTimer = 0

  function clearTimers() {
    if (pollTimer) {
      window.clearTimeout(pollTimer)
      pollTimer = 0
    }
    if (touchTimer) {
      window.clearInterval(touchTimer)
      touchTimer = 0
    }
    if (releaseTimer) {
      window.clearTimeout(releaseTimer)
      releaseTimer = 0
    }
  }

  async function closeSession() {
    const id = sessionId
    sessionId = ''
    if (!id) return
    try {
      await closeLogView(id)
    } catch {
      // 关闭失败不打扰；会话会按空闲回收
    }
  }

  /**
   * 打开一路跟读。后一次调用会关掉前一路。
   * @param instId - 日志文件实例
   * @param name - 展示用文件名
   */
  async function start(instId: string, name: string) {
    const gen = ++generation
    alive = false
    live.value = false
    clearTimers()
    await closeSession()
    if (gen !== generation) return
    alive = true
    text.value = ''
    fileName.value = name
    closed.value = false
    offset = 0
    live.value = true
    try {
      const res = await openLogView(instId)
      const next = isApiSuccess(res) ? parseApiData<LogViewSession>(res) : null
      if (gen !== generation || !alive) {
        if (next?.logViewId) {
          try {
            await closeLogView(next.logViewId)
          } catch {
            // 页面已离开时把刚打开的会话关掉
          }
        }
        return
      }
      if (!isApiSuccess(res)) {
        live.value = false
        text.value = getApiMessage(res, '打开跟读失败')
        return
      }
      if (!next?.logViewId) {
        live.value = false
        text.value = '未返回跟读会话'
        return
      }
      sessionId = next.logViewId
      if (next.fileName) fileName.value = next.fileName
      schedulePoll(gen)
      touchTimer = window.setInterval(() => {
        if (sessionId && alive && gen === generation) void touchLogView(sessionId)
      }, 20000)
    } catch (err) {
      if (gen !== generation) return
      live.value = false
      text.value = getHttpErrorMessage(err, '打开跟读失败')
    }
  }

  /**
   * 回执到达后再跟一会儿，然后断开会话并留下已读文本。
   * @param ms - 延迟毫秒
   */
  function releaseSoon(ms = 4000) {
    const gen = generation
    if (releaseTimer) window.clearTimeout(releaseTimer)
    releaseTimer = window.setTimeout(() => {
      releaseTimer = 0
      if (gen !== generation) return
      void halt(true)
    }, ms)
  }

  /**
   * 停止跟读。
   * @param keepText - 为 true 时保留已显示的日志
   */
  async function halt(keepText: boolean) {
    const gen = ++generation
    alive = false
    live.value = false
    clearTimers()
    await closeSession()
    if (gen !== generation) return
    if (!keepText) {
      text.value = ''
      fileName.value = ''
      closed.value = false
    }
  }

  function schedulePoll(gen: number) {
    if (pollTimer) window.clearTimeout(pollTimer)
    if (!alive || gen !== generation || !sessionId) return
    pollTimer = window.setTimeout(() => void poll(gen), 200)
  }

  async function poll(gen: number) {
    const id = sessionId
    if (!id || !alive || gen !== generation) return
    const res = await logOutput(id, offset)
    if (!alive || gen !== generation) return
    if (!isApiSuccess(res)) {
      schedulePoll(gen)
      return
    }
    const chunk = parseApiData<LogViewChunk>(res)
    if (chunk) {
      if (chunk.text) {
        text.value += chunk.text
        if (text.value.length > 600_000) {
          text.value = text.value.slice(text.value.length - 500_000)
        }
        offset = chunk.offset
      } else {
        offset = chunk.offset
      }
      if (chunk.closed) {
        closed.value = true
        live.value = false
        return
      }
    }
    schedulePoll(gen)
  }

  onUnmounted(() => {
    void halt(false)
  })

  return { text, fileName, live, closed, start, releaseSoon, stop: () => halt(false) }
}
