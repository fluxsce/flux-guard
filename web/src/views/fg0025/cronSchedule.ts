/** 把五段周期收成表单上的点选。规则与 server/job/policy 的 ParseCron 一致。 */

const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec']
const DOW = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']
const WEEK = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

export type CronChoice = 'any' | 'values' | 'range' | 'step'

export interface CronFieldState {
  choice: CronChoice
  values: number[]
  from: number
  to: number
  start: number
  end: number
  step: number
}

export interface CronScheduleState {
  minute: CronFieldState
  hour: CronFieldState
  day: CronFieldState
  month: CronFieldState
  week: CronFieldState
}

export interface CronSpec {
  key: keyof CronScheduleState
  label: string
  every: string
  unit: string
  min: number
  max: number
  names: string[]
  zeroBased: boolean
  aliasSunday: boolean
  order: number[]
  labelOf: (n: number) => string
}

function sequence(min: number, max: number) {
  const out: number[] = []
  for (let n = min; n <= max; n++) out.push(n)
  return out
}

export const cronSpecs: CronSpec[] = [
  {
    key: 'minute',
    label: '分钟',
    every: '每一分钟',
    unit: '分钟',
    min: 0,
    max: 59,
    names: [],
    zeroBased: false,
    aliasSunday: false,
    order: sequence(0, 59),
    labelOf: (n) => String(n).padStart(2, '0'),
  },
  {
    key: 'hour',
    label: '小时',
    every: '每小时',
    unit: '小时',
    min: 0,
    max: 23,
    names: [],
    zeroBased: false,
    aliasSunday: false,
    order: sequence(0, 23),
    labelOf: (n) => `${String(n).padStart(2, '0')} 点`,
  },
  {
    key: 'day',
    label: '日期',
    every: '每天',
    unit: '天',
    min: 1,
    max: 31,
    names: [],
    zeroBased: false,
    aliasSunday: false,
    order: sequence(1, 31),
    labelOf: (n) => `${n} 日`,
  },
  {
    key: 'month',
    label: '月份',
    every: '每月',
    unit: '个月',
    min: 1,
    max: 12,
    names: MONTHS,
    zeroBased: false,
    aliasSunday: false,
    order: sequence(1, 12),
    labelOf: (n) => `${n} 月`,
  },
  {
    key: 'week',
    label: '星期',
    every: '不限星期',
    unit: '天',
    min: 0,
    max: 6,
    names: DOW,
    zeroBased: true,
    aliasSunday: true,
    order: [1, 2, 3, 4, 5, 6, 0],
    labelOf: (n) => WEEK[n] || String(n),
  },
]

export const cronChoices: { value: CronChoice; label: string }[] = [
  { value: 'any', label: '全部' },
  { value: 'values', label: '指定' },
  { value: 'range', label: '区间' },
  { value: 'step', label: '每隔' },
]

export function defaultSchedule(): CronScheduleState {
  return parseSchedule('0 2 * * *')
}

export function parseSchedule(expr: string): CronScheduleState {
  const fields = expr.trim().split(/\s+/).filter(Boolean)
  const tokens = fields.length === 5 ? fields : ['0', '2', '*', '*', '*']
  return {
    minute: parseToken(tokens[0], cronSpecs[0]),
    hour: parseToken(tokens[1], cronSpecs[1]),
    day: parseToken(tokens[2], cronSpecs[2]),
    month: parseToken(tokens[3], cronSpecs[3]),
    week: parseToken(tokens[4], cronSpecs[4]),
  }
}

export function buildSchedule(state: CronScheduleState) {
  return cronSpecs.map((spec) => buildToken(state[spec.key], spec)).join(' ')
}

export function fieldSummary(state: CronFieldState, spec: CronSpec) {
  if (state.choice === 'any') return spec.every
  if (state.choice === 'range') return `从 ${spec.labelOf(state.from)} 到 ${spec.labelOf(state.to)}`
  if (state.choice === 'step') return `从 ${spec.labelOf(state.start)} 到 ${spec.labelOf(state.end)}，每隔 ${state.step} ${spec.unit}`
  const span = spec.max - spec.min + 1
  if (!state.values.length) return '请至少选一个'
  if (state.values.length >= span) return spec.every
  if (state.values.length > 3) return `已选 ${state.values.length} 个`
  return state.values.map((n) => spec.labelOf(n)).join('、')
}

function buildToken(state: CronFieldState, spec: CronSpec) {
  if (state.choice === 'any') return '*'
  if (state.choice === 'range') {
    const from = clamp(Math.min(state.from, state.to), spec.min, spec.max)
    const to = clamp(Math.max(state.from, state.to), spec.min, spec.max)
    return from === to ? String(from) : `${from}-${to}`
  }
  if (state.choice === 'step') {
    const start = clamp(Math.min(state.start, state.end), spec.min, spec.max)
    const end = clamp(Math.max(state.start, state.end), spec.min, spec.max)
    const step = clamp(state.step, 1, spec.max)
    if (start === spec.min && end === spec.max) return `*/${step}`
    if (start === end) return String(start)
    return `${start}-${end}/${step}`
  }
  const picked = [...new Set(state.values.map((n) => clamp(n, spec.min, spec.max)))].sort((a, b) => a - b)
  if (!picked.length) return String(spec.min)
  if (picked.length === spec.max - spec.min + 1) return '*'
  return picked.join(',')
}

function blank(spec: CronSpec): CronFieldState {
  return {
    choice: 'any',
    values: [spec.min],
    from: spec.min,
    to: spec.max,
    start: spec.min,
    end: spec.max,
    step: spec.key === 'minute' ? 15 : 1,
  }
}

function matchStep(token: string) {
  const matched = /^(?:\*|(\d+)-(\d+))\/(\d+)$/.exec(token)
  if (!matched) return null
  const step = Number(matched[3])
  if (!Number.isFinite(step) || step <= 0) return null
  if (matched[1] == null) return { start: NaN, end: NaN, step, star: true }
  return { start: Number(matched[1]), end: Number(matched[2]), step, star: false }
}

function parseToken(raw: string, spec: CronSpec): CronFieldState {
  const base = blank(spec)
  const token = normalizeNames(raw, spec.names, spec.zeroBased)
  const values = expand(token, spec)
  if (!values.length) return { ...base, choice: 'values', values: [spec.min] }
  if (token === '*') return { ...base, values }
  const step = matchStep(token)
  if (step) {
    const start = step.star ? spec.min : step.start
    const end = step.star ? spec.max : step.end
    if (start >= spec.min && end <= spec.max && start <= end) {
      return { ...base, choice: 'step', values, start, end, step: step.step, from: start, to: end }
    }
  }
  const range = /^(\d+)-(\d+)$/.exec(token)
  if (range) {
    const from = Number(range[1])
    const to = Number(range[2])
    if (from >= spec.min && to <= spec.max && from <= to) {
      return { ...base, choice: 'range', values, from, to, start: from, end: to }
    }
  }
  const single = /^\d+$/.test(token) ? Number(token) : null
  if (single != null && single >= spec.min && single <= spec.max) {
    return { ...base, choice: 'values', values: [single], from: single, to: single, start: single, end: single }
  }
  return { ...base, choice: 'values', values, from: values[0], to: values[values.length - 1], start: values[0], end: values[values.length - 1] }
}

function expand(token: string, spec: CronSpec) {
  const limit = spec.aliasSunday ? 7 : spec.max
  const bits = Array<boolean>(limit + 1).fill(false)
  if (!token) return []
  try {
    for (const raw of token.split(',')) {
      fill(bits, raw.trim(), spec.min, limit)
    }
  } catch {
    return []
  }
  if (spec.aliasSunday && bits[7]) bits[0] = true
  const out: number[] = []
  for (let n = spec.min; n <= spec.max; n++) if (bits[n]) out.push(n)
  return out
}

function fill(bits: boolean[], part: string, min: number, max: number) {
  let step = 1
  let base = part
  const slash = part.indexOf('/')
  if (slash >= 0) {
    base = part.slice(0, slash)
    step = Number(part.slice(slash + 1))
    if (!Number.isInteger(step) || step <= 0) throw new Error('步长无效')
  }
  let start = min
  let end = max
  if (base !== '*') {
    const dash = base.indexOf('-')
    if (dash >= 0) {
      start = Number(base.slice(0, dash))
      end = Number(base.slice(dash + 1))
      if (!Number.isInteger(start) || !Number.isInteger(end) || start > end) throw new Error('区间无效')
    } else {
      start = Number(base)
      end = start
      if (!Number.isInteger(start)) throw new Error('数字无效')
    }
  }
  if (start < min || end > max) throw new Error('超出范围')
  for (let n = start; n <= end; n += step) bits[n] = true
}

function normalizeNames(field: string, names: string[], zeroBased: boolean) {
  let lower = field.toLowerCase()
  names.forEach((name, index) => {
    const n = zeroBased ? index : index + 1
    lower = lower.replaceAll(name, String(n))
  })
  return lower
}

function clamp(n: number, min: number, max: number) {
  if (!Number.isFinite(n)) return min
  return Math.min(max, Math.max(min, Math.trunc(n)))
}
