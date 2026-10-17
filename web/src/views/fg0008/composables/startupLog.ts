import type { AppInstance } from '../api'

function logPath(row: AppInstance) {
  return (row.artifactPath || '').replace(/\\/g, '/').toLowerCase()
}

function logName(row: AppInstance) {
  const path = logPath(row)
  return (row.appCode || path.split('/').pop() || '').toLowerCase()
}

/**
 * 选出用于跟读的启动日志。
 * 优先 catalina.out（标准输出），其次其它 catalina，再退到任意在线日志。
 * @param files - 当前运行时文件列表
 * @returns 日志实例；没有可跟读文件时为 null
 */
export function startupLogOf(files: AppInstance[]): AppInstance | null {
  const logs = files.filter((row) => {
    if (row.onlineFlag !== 'Y') return false
    const path = logPath(row)
    const name = logName(row)
    const kind = String(row.artifactKind || '').toLowerCase()
    if (kind === 'log') return path.includes('/logs/') || name.endsWith('.log') || name.endsWith('.out')
    return name.endsWith('.log') || name.endsWith('.out') || name.endsWith('.txt')
  })
  if (!logs.length) return null
  const out = logs.find((row) => logName(row) === 'catalina.out' || logPath(row).endsWith('/catalina.out'))
  if (out) return out
  const catalina = logs.find((row) => {
    const path = logPath(row)
    const name = logName(row)
    return name.includes('catalina') || path.includes('/catalina')
  })
  return catalina || logs[0]
}

/**
 * 跟读里是否已经出现 Catalina.start 完成行。
 * 与代理判定同一条规则：JULI 的类名.方法名是稳定标记，译文只是兜底。堆栈 Catalina.start( 不算。
 * @param text - catalina 跟读正文
 */
export function catalinaStarted(text: string) {
  if (!text) return false
  return text.split('\n').some((line) => catalinaStartLine(line))
}

function catalinaStartLine(line: string) {
  const mark = 'org.apache.catalina.startup.Catalina.start'
  const at = line.indexOf(mark)
  if (at >= 0) {
    const next = line.charAt(at + mark.length)
    if ((!next || next === ' ' || next === '\t') && !lineSevere(line)) return true
  }
  if (line.includes('Server startup in') || line.toLowerCase().includes('server startup [')) return true
  return line.includes('毫秒后服务器启动') || line.includes('毫秒内启动')
}

function lineSevere(line: string) {
  return (
    line.includes('SEVERE:') ||
    line.includes('SEVERE [') ||
    line.includes('FATAL [') ||
    line.includes(' 严重 ') ||
    line.includes('严重 [') ||
    line.includes('致命 [') ||
    line.toLowerCase().includes('lifecycleexception') ||
    line.toLowerCase().includes('address already in use') ||
    line.includes('BindException') ||
    line.includes('地址已在使用')
  )
}
