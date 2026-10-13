import { GaugeChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { JavaCurve } from '../api'
import { bytes, clock, count, millis, percent } from './format'

use([CanvasRenderer, LineChart, GaugeChart, GridComponent, TooltipComponent, LegendComponent])

export type ChartOption = EChartsCoreOption

// 曲线不用红色。红色留给以后的阈值水平线。
const palette = ['#0878ad', '#5b6cff', '#1a9a62', '#c49212', '#148a96', '#6a4dff']

const trendGroups = [
  { title: '堆', names: ['堆已用', '堆已提交', '堆上限'] },
  { title: '分代', names: ['年轻代', '幸存区', '老年代'] },
  { title: '非堆', names: ['非堆已用', '非堆已提交', '元空间已用'] },
  { title: '堆外', names: ['直接内存', '直接内存上限', '映射内存', '代码缓存', '压缩类空间'] },
  { title: '线程', names: ['线程数', '运行线程', '阻塞线程', '等待线程', '守护线程', '峰值线程'] },
  { title: '进程 CPU', names: ['进程 CPU'] },
  { title: '类加载', names: ['已加载类', '已卸载类'] },
]

export interface NamedChart {
  title: string
  option: ChartOption
  help?: string
}

const chartHelp: Record<string, string> = {
  堆: '堆已用、已提交和上限。上限为 0 表示进程没有设置 -Xmx，这条线不会画出。',
  分代: '年轻代、幸存区和老年代的已用内存。G1 的多个分区会按代加总。',
  非堆: '非堆含元空间、代码缓存等。元空间上涨通常是类加载变多。',
  堆外: '直接内存对照上限看离溢出还有多远。映射内存、代码缓存长期为 0 时不画。',
  线程: '总数之外分开运行、堵在锁上和等待。阻塞、等待一直为 0 时不画。这里没有线程栈。',
  '进程 CPU': '本进程占整机 CPU 的比例。第一拍还没有基线时不画点。',
  类加载: '当前仍加载的类，以及进程启动以来卸载的类。卸载一直为 0 时不画。',
  'GC 次数': '各回收器自进程启动以来的累计回收次数。',
  'GC 耗时': '各回收器自进程启动以来的累计停顿时间。',
  'GC 频率': '相邻两次上报之间，累计次数的增量除以时间，单位是次/秒。',
  'GC 停顿次数': '两次上报之间实际发生的停顿次数，不是累计值。',
  'GC 最长停顿': '两次上报之间单次最长停顿。',
  'GC 区间耗时': '两次上报之间各次停顿的合计。',
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

function formatValue(unit: string, value: number) {
  if (!Number.isFinite(value)) return '—'
  if (unit === 'percent') return percent(value)
  if (unit === 'bytes') return bytes(value)
  if (unit === 'count') return count(value)
  if (unit === 'ms') return millis(value)
  if (unit === 'persec') return `${count(value)} /s`
  return String(value)
}

function tone(value: number) {
  if (value >= 85) return '#e23b55'
  if (value >= 70) return '#c49212'
  return '#1a9a62'
}

export function gaugeOption(name: string, value: number, caption = ''): ChartOption {
  const n = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0
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
        progress: { show: true, width: 12, roundCap: true, itemStyle: { color: tone(n) } },
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
          formatter: () => percent(n),
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

function lineOption(curves: JavaCurve[]): ChartOption {
  const times = [...new Set(curves.flatMap((curve) => curve.points.map((point) => point.observedTime)))].sort()
  const spanMs = chartSpanMs(times)
  const colors = ink()
  const unit = curves[0]?.unit ?? ''
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
          return `${row.marker ?? ''}${row.seriesName ?? ''} ${formatValue(curve?.unit ?? unit, Number(row.value))}`
        })
        const when = rows[0]?.axisValue ? clock(rows[0].axisValue) : ''
        return [when, ...body].filter(Boolean).join('<br/>')
      },
    },
    legend: { type: curves.length > 4 ? 'scroll' : 'plain', top: 0, textStyle: { color: colors.text, fontSize: 12 } },
    grid: { left: 72, right: 16, top: 36, bottom: 28 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: times,
      axisLabel: {
        color: colors.text,
        fontSize: 11,
        hideOverlap: true,
        formatter: (value: string) => axisTick(value, spanMs),
      },
      axisLine: { lineStyle: { color: colors.line } },
    },
    yAxis: {
      type: 'value',
      min: 0,
      axisLabel: { color: colors.text, fontSize: 11, formatter: (value: number) => formatValue(unit, value) },
      splitLine: { lineStyle: { color: colors.line } },
    },
    series: curves.map((curve) => {
      const map = new Map(curve.points.map((point) => [point.observedTime, point.value]))
      return {
        name: curve.name,
        type: 'line',
        smooth: false,
        showSymbol: times.length < 3,
        data: times.map((time) => map.get(time) ?? null),
      }
    }),
  }
}

export function trendCharts(curves: JavaCurve[]): NamedChart[] {
  const plain = curves.filter((curve) => !curve.group)
  const byName = new Map(plain.map((curve) => [curve.name, curve]))
  const charts: NamedChart[] = []
  for (const group of trendGroups) {
    const items = group.names
      .map((name) => byName.get(name))
      .filter((item): item is JavaCurve => !!item && item.points.length > 0)
    if (items.length === 0) continue
    charts.push({ title: group.title, option: lineOption(items), help: chartHelp[group.title] })
  }
  const grouped = new Map<string, JavaCurve[]>()
  for (const curve of curves) {
    if (!curve.group || curve.points.length === 0) continue
    const items = grouped.get(curve.group) ?? []
    items.push(curve)
    grouped.set(curve.group, items)
  }
  for (const [title, items] of grouped) {
    charts.push({ title, option: lineOption(items), help: chartHelp[title] })
  }
  return charts
}
