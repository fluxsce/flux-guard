const SECRET_KEY = /password|token|secret|credential/i

/**
 * 去掉结果里的口令、令牌和密钥字段。环境变量的 value 由各工具自己裁剪。
 * @param value - 工具返回值
 * @returns 可以交给助手的数据
 */
export function redact(value: unknown): unknown {
  return walk(value, '')
}

function walk(value: unknown, key: string): unknown {
  if (key && SECRET_KEY.test(key)) return undefined
  if (Array.isArray(value)) return value.map((item) => walk(item, ''))
  if (!value || typeof value !== 'object') return value
  const out: Record<string, unknown> = {}
  for (const [name, item] of Object.entries(value as Record<string, unknown>)) {
    if (SECRET_KEY.test(name)) continue
    const next = walk(item, name)
    if (next !== undefined) out[name] = next
  }
  return out
}
