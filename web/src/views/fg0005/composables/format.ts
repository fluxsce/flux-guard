import type { HostTcp } from '../api'

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

export function fixed(value: unknown, digits: number) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return n.toFixed(digits)
}

/** 磁盘延迟：亚毫秒常见，不能一律保留 1 位小数，否则全显示成 0.0 ms。 */
export function latency(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  if (n === 0) return '0 ms'
  if (n < 0.001) return `${(n * 1000).toFixed(2)} µs`
  if (n < 0.1) return `${n.toFixed(3)} ms`
  if (n < 10) return `${n.toFixed(2)} ms`
  return `${n.toFixed(1)} ms`
}

export function rate(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB/s`
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KB/s`
  return `${n.toFixed(0)} B/s`
}

export function bytes(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  if (n >= 1024 ** 3) return `${(n / 1024 ** 3).toFixed(1)} GB`
  if (n >= 1024 ** 2) return `${(n / 1024 ** 2).toFixed(1)} MB`
  if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${n.toFixed(0)} B`
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

export function tcpStates(tcp: HostTcp) {
  return [
    { state: 'ESTABLISHED', count: tcp.establishedCount },
    { state: 'LISTEN', count: tcp.listenCount },
    { state: 'TIME_WAIT', count: tcp.timeWaitCount },
    { state: 'CLOSE_WAIT', count: tcp.closeWaitCount },
    { state: 'SYN_SENT', count: tcp.synSentCount },
    { state: 'SYN_RECV', count: tcp.synRecvCount },
    { state: 'FIN_WAIT', count: tcp.finWaitCount },
    { state: 'LAST_ACK', count: tcp.lastAckCount },
    { state: 'CLOSING', count: tcp.closingCount },
    { state: 'UDP', count: tcp.udpInUseCount },
  ]
}
