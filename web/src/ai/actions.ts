import type { FgAiRisk } from './types'

const READ_KEYS = new Set(['view', 'search', 'reset', 'refresh', 'query', 'open'])

/**
 * 按按钮键给出风险。查看和刷新是 read，其余默认 write。
 * @param key - 动作键
 * @param explicit - 按钮上写明的风险，有则优先
 * @returns read 或 write
 */
export function riskOf(key: string, explicit?: FgAiRisk): FgAiRisk {
  if (explicit) return explicit
  return READ_KEYS.has(key) ? 'read' : 'write'
}
