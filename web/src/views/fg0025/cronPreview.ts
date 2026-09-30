/** 五段 cron 预览。规则与 server/job/policy 的 ParseCron 一致：分 时 日 月 周，UTC。日和周都写了具体值时，命中任意一段即可。 */

const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec']
const DOW = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']

export interface CronPreview {
  expr: string
  summary: string
  fields: { label: string; token: string; text: string }[]
  next: string[]
  note: string
  error: string
}

export function previewCron(expr: string, now = new Date()): CronPreview {
  const text = expr.trim()
  const empty: CronPreview = { expr: text, summary: '', fields: [], next: [], note: '', error: '' }
  if (!text) {
    return { ...empty, error: '请填写五段表达式：分 时 日 月 周' }
  }
  try {
    const parsed = parseCron(text)
    const fields = describeFields(text, parsed)
    return {
      expr: text,
      summary: summaryOf(parsed),
      fields,
      next: upcoming(parsed, now, 5).map(formatUtc),
      note: !parsed.domAny && !parsed.dowAny ? '日和周都写了具体值，命中任意一段就会执行。' : '',
      error: '',
    }
  } catch (err) {
    return { ...empty, error: err instanceof Error ? err.message : '表达式无法识别' }
  }
}

interface Parsed {
  min: boolean[]
  hour: boolean[]
  dom: boolean[]
  month: boolean[]
  dow: boolean[]
  domAny: boolean
  dowAny: boolean
}

function parseCron(expr: string): Parsed {
  const fields = expr.split(/\s+/).filter(Boolean)
  if (fields.length !== 5) throw new Error('须为五段：分 时 日 月 周')
  return {
    min: parseField(fields[0], 0, 59, '分钟'),
    hour: parseField(fields[1], 0, 23, '小时'),
    dom: parseField(fields[2], 1, 31, '日'),
    month: parseField(replaceNames(fields[3], MONTHS, false), 1, 12, '月'),
    dow: parseDow(fields[4]),
    domAny: fields[2] === '*',
    dowAny: fields[4] === '*',
  }
}

function parseDow(field: string) {
  const wide = parseField(replaceNames(field, DOW, true), 0, 7, '周')
  const out = wide.slice(0, 7)
  if (wide[7]) out[0] = true
  return out
}

function parseField(field: string, min: number, max: number, label: string) {
  const bits = Array<boolean>(60).fill(false)
  if (max >= bits.length) throw new Error(`${label}范围过大`)
  for (const raw of field.split(',')) {
    const part = raw.trim()
    if (!part) throw new Error(`${label}有空段`)
    fillPart(bits, part, min, max, label)
  }
  return bits
}

function fillPart(bits: boolean[], part: string, min: number, max: number, label: string) {
  let step = 1
  let base = part
  const slash = part.indexOf('/')
  if (slash >= 0) {
    base = part.slice(0, slash)
    const n = Number(part.slice(slash + 1))
    if (!Number.isInteger(n) || n <= 0) throw new Error(`${label}步长无效`)
    step = n
  }
  let start = min
  let end = max
  if (base !== '*') {
    const dash = base.indexOf('-')
    if (dash >= 0) {
      const a = Number(base.slice(0, dash))
      const b = Number(base.slice(dash + 1))
      if (!Number.isInteger(a) || !Number.isInteger(b) || a > b) throw new Error(`${label}区间无效`)
      start = a
      end = b
    } else {
      const n = Number(base)
      if (!Number.isInteger(n)) throw new Error(`${label}数字无效`)
      start = n
      end = n
    }
  }
  if (start < min || end > max) throw new Error(`${label}超出 ${min}-${max}`)
  for (let n = start; n <= end; n += step) bits[n] = true
}

function replaceNames(field: string, names: string[], zeroBased: boolean) {
  let lower = field.toLowerCase()
  names.forEach((name, i) => {
    lower = lower.replaceAll(name, String(zeroBased ? i : i + 1))
  })
  return lower
}

function picked(bits: boolean[], min: number, max: number) {
  const values: number[] = []
  for (let n = min; n <= max; n += 1) if (bits[n]) values.push(n)
  return values
}

function describeSpan(values: number[], min: number, max: number, every: string, one: (n: number) => string) {
  if (values.length === max - min + 1) return every
  if (values.length === 1) return one(values[0])
  const step = values[1] - values[0]
  const even = values.every((n, i) => i === 0 || n - values[i - 1] === step)
  if (even && step > 1 && values[0] === min && values[values.length - 1] + step > max) return `每 ${step}`
  if (even && step === 1) return `${one(values[0])}到${one(values[values.length - 1])}`
  if (even && step > 1) return `从${one(values[0])}起每 ${step}`
  return values.map(one).join('、')
}

function describeFields(expr: string, parsed: Parsed): CronPreview['fields'] {
  const tokens = expr.split(/\s+/)
  const minute = describeSpan(picked(parsed.min, 0, 59), 0, 59, '每分钟', (n) => `${n} 分`)
  const hour = describeSpan(picked(parsed.hour, 0, 23), 0, 23, '每小时', (n) => `${n} 点`)
  const day = describeSpan(picked(parsed.dom, 1, 31), 1, 31, '每天', (n) => `${n} 日`)
  const month = describeSpan(picked(parsed.month, 1, 12), 1, 12, '每月', (n) => `${n} 月`)
  const week = describeWeek(picked(parsed.dow, 0, 6))
  const texts = [
    minute === '每' ? '每分钟' : minute.startsWith('每 ') ? `${minute} 分钟` : minute,
    hour === '每' ? '每小时' : hour.startsWith('每 ') ? `${hour} 小时` : hour,
    day === '每' ? '每天' : day,
    month === '每' ? '每月' : month,
    week === '每' ? '每天' : week.startsWith('每 ') ? week.replace('每 ', '每隔 ') : week,
  ]
  return ['分', '时', '日', '月', '周'].map((label, i) => ({ label, token: tokens[i] || '', text: texts[i] }))
}

function summaryOf(parsed: Parsed) {
  const minute = picked(parsed.min, 0, 59)
  const hour = picked(parsed.hour, 0, 23)
  const clock = clockText(minute, hour)
  const month = describeSpan(picked(parsed.month, 1, 12), 1, 12, '', (n) => `${n} 月`)
  const day = describeSpan(picked(parsed.dom, 1, 31), 1, 31, '', (n) => `${n} 日`)
  const weekName = describeWeek(picked(parsed.dow, 0, 6))
  const week = weekName === '不限星期' ? '' : weekName
  if (parsed.domAny && parsed.dowAny) {
    if (clock.startsWith('每')) return month ? `${month} ${clock}（UTC）` : `${clock}（UTC）`
    return `${month || '每天'} ${clock}（UTC）`
  }
  if (!parsed.domAny && parsed.dowAny && !month) return `每月 ${day} ${clock}（UTC）`
  const parts = [month, parsed.domAny ? '' : day, parsed.dowAny ? '' : week].filter(Boolean)
  if (!parsed.domAny && !parsed.dowAny) return `${parts.join('，或 ')} ${clock}（UTC）`
  return `${parts.join(' ')} ${clock}（UTC）`
}

function describeWeek(values: number[]) {
  if (values.length === 7) return '不限星期'
  if (values.length === 1) return WEEK[values[0]] || String(values[0])
  const contiguous = values.every((n, i) => n === values[0] + i)
  if (contiguous) return `${WEEK[values[0]]}到${WEEK[values[values.length - 1]]}`
  return values.map((n) => WEEK[n] || String(n)).join('、')
}

function clockText(minute: number[], hour: number[]) {
  if (minute.length === 60 && hour.length === 24) return '每分钟'
  if (minute.length === 1 && hour.length === 1) {
    return `${pad(hour[0])}:${pad(minute[0])}`
  }
  if (minute.length === 1 && minute[0] === 0 && hour.length === 24) return '每小时整点'
  if (minute.length === 1 && hour.length === 24) return `每小时的第 ${minute[0]} 分`
  if (isEvery(minute, 0, 59) && hour.length === 24) return `每 ${minute[1] - minute[0]} 分钟`
  if (minute.length === 1) return `${hour.map((n) => `${n} 点`).join('、')} ${pad(minute[0])} 分`
  return `${hour.length === 24 ? '每小时' : hour.map((n) => `${n} 点`).join('、')}的 ${minute.join('、')} 分`
}

function isEvery(values: number[], min: number, max: number) {
  if (values.length < 2 || values[0] !== min) return false
  const step = values[1] - values[0]
  return values.every((n, i) => i === 0 || n - values[i - 1] === step) && values[values.length - 1] + step > max
}

function upcoming(parsed: Parsed, now: Date, count: number) {
  const start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()))
  const out: Date[] = []
  const limit = start.getTime() + 366 * 24 * 60 * 60 * 1000
  for (let t = start.getTime(); t < limit && out.length < count; t += 60 * 1000) {
    const at = new Date(t)
    if (matches(parsed, at)) out.push(at)
  }
  return out
}

function matches(parsed: Parsed, at: Date) {
  if (!parsed.min[at.getUTCMinutes()] || !parsed.hour[at.getUTCHours()] || !parsed.month[at.getUTCMonth() + 1]) return false
  const domOK = parsed.dom[at.getUTCDate()]
  const dowOK = parsed.dow[at.getUTCDay()]
  if (parsed.domAny && parsed.dowAny) return true
  if (parsed.domAny) return dowOK
  if (parsed.dowAny) return domOK
  return domOK || dowOK
}

function formatUtc(at: Date) {
  return `${at.getUTCFullYear()}-${pad(at.getUTCMonth() + 1)}-${pad(at.getUTCDate())} ${pad(at.getUTCHours())}:${pad(at.getUTCMinutes())} UTC ${WEEK[at.getUTCDay()]}`
}

function pad(n: number) {
  return String(n).padStart(2, '0')
}
