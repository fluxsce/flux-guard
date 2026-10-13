import { GaugeChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, MarkLineComponent, TooltipComponent } from 'echarts/components'
import { use, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { JavaCurve } from '../api'
import { bytes, clock, count, millis, percent } from './format'

use([CanvasRenderer, LineChart, GaugeChart, GridComponent, TooltipComponent, LegendComponent, MarkLineComponent])

export type ChartOption = EChartsCoreOption

// 曲线不用红色。红色只用于和 judge.go 相同的预警线。
const palette = ['#0878ad', '#5b6cff', '#1a9a62', '#c49212', '#148a96', '#6a4dff']
const alertInk = '#e23b55'

const trendGroups = [
  { title: '堆', names: ['堆已用', '堆已提交', '堆上限'] },
  { title: '堆使用率', names: ['堆使用率'] },
  { title: '分代', names: ['年轻代', '幸存区', '老年代'] },
  { title: '非堆', names: ['非堆已用', '非堆已提交', '元空间已用'] },
  { title: '堆外', names: ['直接内存', '直接内存上限', '映射内存', '代码缓存', '压缩类空间'] },
  { title: '线程', names: ['线程数', '运行线程', '阻塞线程', '等待线程', '守护线程', '峰值线程', '死锁线程'] },
  { title: '进程 CPU', names: ['进程 CPU'] },
  { title: '类加载', names: ['已加载类', '已卸载类'] },
]

export interface NamedChart {
  title: string
  option: ChartOption
  help?: string
}

const chartHelp: Record<string, string> = {
  堆: '堆已用、已提交和上限，单位是字节。没有正的上限（未设置 -Xmx）时不画上限，使用率也不记成 0%。使用率在旁边单独的图上，不在这张字节图上画 85% 线。',
  堆使用率: '已用除以上限，按一位小数画点，和仪表、判断是同一个数。没有 -Xmx 时不画，也不记成 0%。红色虚线 85%：达到 85.0% 才提示堆贴着上限。',
  分代: '年轻代、幸存区和老年代的已用内存。G1 的多个分区会按代加总。',
  非堆: '非堆含元空间、代码缓存等。元空间上涨通常是类加载变多。',
  堆外: '直接内存对照上限看离溢出还有多远。映射内存、代码缓存长期为 0 时不画。',
  线程: '总数之外分开运行、堵在锁上和等待。死锁来自 findDeadlockedThreads，0 表示调用成功且没有死锁；采集不到时不画点。一直为 0 的阻塞、等待和死锁不画。线程池队列没有标准 JVM 指标，这里不估算。栈只在检测到死锁时附在这一拍里。',
  '进程 CPU': '本进程占整机 CPU 的比例。第一拍还没有基线时不画点。',
  类加载: '当前仍加载的类，以及进程启动以来卸载的类。卸载一直为 0 时不画。',
  'GC 次数': '各回收器自本次进程启动以来的累计回收次数。进程重启后从 0 再计，曲线在重启处断开，不把上一代的次数连过来。',
  'GC 累计时间': '各回收器自本次进程启动以来的 MXBean collectionTime。ZGC、Shenandoah 的周期也算在里面，所以这不是停顿，不能拿来做停顿预警。停顿看最长停顿和开销。进程重启后重新累计。',
  'GC 频率': '相邻两次上报之间，同一进程累计次数的增量除以时间，单位是次/秒。进程重启的那一拍不画点，避免把次数回落算成 0。',
  'GC 停顿次数': '两次上报之间实际发生的 STW 停顿次数，不是累计值。周期不计入。',
  'GC 最长停顿': '两次上报之间单次最长 STW 停顿。红色虚线 200ms 为关注、500ms 为严重，与页面判断相同。p99 不参与判断。',
  'GC 区间耗时': '两次上报之间各次 STW 停顿的合计。周期不计入。',
  'GC p99': '这一拍停顿时长的最近秩，ceil(0.99×次数) 对应的那一次。不是全天分位，也不触发判断。没有停顿时不画。',
  'GC 开销': '本区间 STW 停顿合计除以距离上一拍的实际墙钟时间。第一拍没有分母时不画，也不记成 0。周期不计入。红色虚线 10% 与判断相同，数值保留三位小数。',
  'GC 周期': '并发周期或非停顿通知的耗时。不计入停顿次数、最长停顿和开销，也不触发停顿判断。',
}

function ink() {
  if (typeof document === 'undefined') {
    return { text: '#4d647c', line: 'rgba(8, 120, 173, 0.16)', title: '#102033' }
  }
  const style = getComputedStyle(document.documentElement)
  return {
    text: style.getPropertyValue('--rs-text-secondary').trim() || '#4d647c',
    title: style.getPropertyValue('--rs-text-primary').trim() || '#102033',
    line: style.getPropertyValue('--fg-grid-line').trim() || 'rgba(8, 120, 173, 0.16)',
  }
}

/** 指标浮层可移入并滚动，超出视口时不再把滚轮交给背后的页面。 */
function tooltipLayer() {
  const surface = 'var(--rs-surface-elevated)'
  const border = 'var(--rs-popover-border)'
  const text = 'var(--rs-text-primary)'
  const shadow = 'var(--rs-popover-shadow)'
  return {
    appendTo: 'body' as const,
    className: 'fg-chart-tooltip',
    enterable: true,
    hideDelay: 400,
    backgroundColor: surface,
    borderColor: border,
    textStyle: { color: text, fontSize: 12 },
    extraCssText: `max-height:min(70vh,520px);overflow-y:auto;overscroll-behavior:contain;z-index:10000;background-color:${surface};color:${text};border-color:${border};box-shadow:${shadow};`,
  }
}

function formatValue(unit: string, value: number, digits = 1) {
  if (!Number.isFinite(value)) return '—'
  if (unit === 'percent') return digits === 1 ? percent(value) : `${value.toFixed(digits)}%`
  if (unit === 'bytes') return bytes(value)
  if (unit === 'count') return count(value)
  if (unit === 'ms') return millis(value)
  if (unit === 'persec') return `${count(value)} /s`
  return String(value)
}

// 与 judge.go 的 heapAlertPercent 一致。按一位小数着色，低于 85.0 保持绿色。
function tone(value: number) {
  const shown = Number(value.toFixed(1))
  if (shown >= 85) return alertInk
  return '#1a9a62'
}

function alertMarks(title: string) {
  if (title === '堆使用率') return [{ value: 85, label: '预警 85%' }]
  if (title === 'GC 最长停顿') return [{ value: 200, label: '预警 200ms' }, { value: 500, label: '严重 500ms' }]
  if (title === 'GC 开销') return [{ value: 10, label: '预警 10%' }]
  return []
}

export function gaugeOption(name: string, value: number, caption = ''): ChartOption {
  const unknown = !Number.isFinite(value) || value < 0
  const n = unknown ? 0 : Math.max(0, Math.min(100, value))
  const colors = ink()
  return {
    series: [
      {
        type: 'gauge',
        startAngle: 210,
        endAngle: -30,
        min: 0,
        max: 100,
        radius: '96%',
        center: ['50%', '58%'],
        progress: { show: !unknown, width: 12, roundCap: true, itemStyle: { color: tone(n) } },
        axisLine: { lineStyle: { width: 12, color: [[1, 'rgba(8, 120, 173, 0.12)']] } },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        pointer: { show: false },
        anchor: { show: false },
        title: { offsetCenter: [0, '18%'], fontSize: 12, color: colors.text },
        detail: {
          valueAnimation: false,
          offsetCenter: [0, '-8%'],
          fontSize: 22,
          fontWeight: 600,
          color: colors.title,
          formatter: () => (unknown ? '—' : percent(n)),
        },
        data: [{ value: n, name: caption ? `${name}\n${caption}` : name }],
      },
    ],
  }
}

function chartSpanMs(times: string[]) {
  if (times.length < 2) return 0
  const start = Date.parse(times[0])
  const end = Date.parse(times[times.length - 1])
  if (!Number.isFinite(start) || !Number.isFinite(end)) return 0
  return Math.abs(end - start)
}

/** 三小时以内标到秒。只写到分钟时，同一分钟里的点会对在同一个刻度上。 */
function axisTick(time: string, spanMs: number) {
  const text = clock(time)
  if (text.length < 19) return text
  if (spanMs > 0 && spanMs <= 3 * 60 * 60 * 1000) return text.slice(11, 19)
  return text.slice(5, 16)
}

function timelineSlots(times: string[], restarts: string[]) {
  const breaks = [...new Set(restarts.filter(Boolean))].sort()
  const slots: Array<{ time: string; gap: boolean }> = []
  let used = 0
  for (const time of times) {
    let pending = false
    while (used < breaks.length && breaks[used] <= time) {
      pending = slots.some((slot) => !slot.gap)
      used += 1
    }
    if (pending) slots.push({ time: `\u0000${used}`, gap: true })
    slots.push({ time, gap: false })
  }
  return slots
}

function restartMarks(times: string[], restarts: string[]) {
  const marks: string[] = []
  for (const restart of restarts) {
    if (!restart || restart <= times[0]) continue
    const hit = times.find((time) => time >= restart)
    if (hit && !marks.includes(hit)) marks.push(hit)
  }
  return marks
}

function lineOption(curves: JavaCurve[], restarts: string[] = [], title = ''): ChartOption {
  const times = [...new Set(curves.flatMap((curve) => curve.points.map((point) => point.observedTime)))].sort()
  const slots = timelineSlots(times, restarts)
  const marks = restartMarks(times, restarts)
  const spanMs = chartSpanMs(times)
  const colors = ink()
  const unit = curves[0]?.unit ?? ''
  const percentDigits = title === 'GC 开销' ? 3 : 1
  const alerts = alertMarks(title)
  const byName = new Map(curves.map((curve) => [curve.name, curve]))
  return {
    color: palette,
    tooltip: {
      ...tooltipLayer(),
      trigger: 'axis',
      formatter: (params: unknown) => {
        const rows = (Array.isArray(params) ? params : [params]) as Array<{
          seriesName?: string
          value?: number | string | null
          axisValue?: string
          marker?: string
        }>
        const body = rows.map((row) => {
          const curve = byName.get(row.seriesName ?? '')
          return `${row.marker ?? ''}${row.seriesName ?? ''} ${formatValue(curve?.unit ?? unit, Number(row.value), percentDigits)}`
        })
        const axisValue = String(rows[0]?.axisValue ?? '')
        if (axisValue.startsWith('\u0000')) return ''
        const when = axisValue ? clock(axisValue) : ''
        return [when, ...body].filter(Boolean).join('<br/>')
      },
    },
    legend: { type: curves.length > 4 ? 'scroll' : 'plain', top: 0, textStyle: { color: colors.text, fontSize: 12 } },
    grid: { left: 72, right: 16, top: 36, bottom: 28 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: slots.map((slot) => slot.time),
      axisLabel: {
        color: colors.text,
        fontSize: 11,
        hideOverlap: true,
        formatter: (value: string) => (value.startsWith('\u0000') ? '' : axisTick(value, spanMs)),
      },
      axisLine: { lineStyle: { color: colors.line } },
    },
    yAxis: {
      type: 'value',
      min: 0,
      axisLabel: { color: colors.text, fontSize: 11, formatter: (value: number) => formatValue(unit, value) },
      splitLine: { lineStyle: { color: colors.line } },
    },
    series: curves.map((curve, index) => {
      const map = new Map(curve.points.map((point) => [point.observedTime, point.value]))
      return {
        name: curve.name,
        type: 'line',
        smooth: false,
        connectNulls: false,
        showSymbol: times.length < 3,
        data: slots.map((slot) => (slot.gap ? null : map.get(slot.time) ?? null)),
        markLine: index === 0 && (marks.length > 0 || alerts.length > 0) ? {
          symbol: 'none',
          silent: true,
          data: [
            ...marks.map((time) => ({
              xAxis: time,
              label: { formatter: '重启', color: colors.text, fontSize: 11 },
              lineStyle: { type: 'dashed' as const, color: colors.text },
            })),
            ...alerts.map((item) => ({
              yAxis: item.value,
              label: { formatter: item.label, color: alertInk, fontSize: 11 },
              lineStyle: { type: 'dashed' as const, color: alertInk },
            })),
          ],
        } : undefined,
      }
    }),
  }
}

export function trendCharts(curves: JavaCurve[], restarts: string[] = []): NamedChart[] {
  const plain = curves.filter((curve) => !curve.group)
  const byName = new Map(plain.map((curve) => [curve.name, curve]))
  const charts: NamedChart[] = []
  for (const group of trendGroups) {
    const items = group.names
      .map((name) => byName.get(name))
      .filter((item): item is JavaCurve => !!item && item.points.length > 0)
    if (items.length === 0) continue
    charts.push({ title: group.title, option: lineOption(items, restarts, group.title), help: chartHelp[group.title] })
  }
  const grouped = new Map<string, JavaCurve[]>()
  for (const curve of curves) {
    if (!curve.group || curve.points.length === 0) continue
    const items = grouped.get(curve.group) ?? []
    items.push(curve)
    grouped.set(curve.group, items)
  }
  for (const [title, items] of grouped) {
    charts.push({ title, option: lineOption(items, restarts, title), help: chartHelp[title] })
  }
  return charts
}
