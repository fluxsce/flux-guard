export function count(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString()
}

export function percent(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return `${n.toFixed(1)}%`
}

export function bytes(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  const sign = n < 0 ? '-' : ''
  const abs = Math.abs(n)
  if (abs >= 1024 ** 3) return `${sign}${(abs / 1024 ** 3).toFixed(1)} GB`
  if (abs >= 1024 ** 2) return `${sign}${(abs / 1024 ** 2).toFixed(1)} MB`
  if (abs >= 1024) return `${sign}${(abs / 1024).toFixed(1)} KB`
  return `${sign}${abs.toFixed(0)} B`
}

export function millis(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  if (n >= 1000) return `${(n / 1000).toFixed(1)} s`
  return `${n.toFixed(0)} ms`
}

/** 库存时间是 UTC。无时区的文本按 UTC 解析，再显示为本地时间。 */
function parseUtc(value: unknown) {
  const text = String(value ?? '').trim()
  if (!text) return null
  const zoned = /(?:Z|[+-]\d{2}:\d{2})$/.test(text)
  const date = new Date(zoned ? text : `${text.replace(' ', 'T')}Z`)
  if (Number.isNaN(date.getTime())) return null
  return date
}

export function clock(value: unknown) {
  const date = parseUtc(value)
  if (!date) {
    const text = String(value ?? '').trim()
    return text || '—'
  }
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
