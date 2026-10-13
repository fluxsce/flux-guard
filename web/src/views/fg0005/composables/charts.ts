import { BarChart, GaugeChart, LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { HostCurve, HostDetail } from '../api'
import { bytes, clock, count, fixed, latency, percent, rate, tcpStates } from './format'

use([CanvasRenderer, LineChart, BarChart, GaugeChart, GridComponent, TooltipComponent, LegendComponent])

export type ChartOption = EChartsCoreOption

// 曲线不用红色。红色留给以后的阈值水平线。
const palette = ['#0878ad', '#5b6cff', '#1a9a62', '#c49212', '#148a96', '#6a4dff']

const trendGroups = [
  { title: 'CPU', names: ['占用核', 'CPU核数'] },
  { title: 'CPU 构成', names: ['CPU用户', 'CPU系统', 'CPU等待', 'CPU中断', 'CPU软中断', 'CPU窃取'] },
  {
    title: '内存',
    names: ['内存已用', '内存总量', '内存可用', '内存缓存', '内存缓冲', '交换已用', '交换总量'],
  },
  { title: '负载', names: ['负载', '负载5', '负载15'] },
  { title: '进程', names: ['进程数', '运行', '睡眠', '僵尸', 'RSS合计'] },
  { title: '连接', names: ['ESTABLISHED', 'LISTEN', 'TIME_WAIT', 'CLOSE_WAIT', 'SYN_SENT', 'SYN_RECV', 'FIN_WAIT', 'LAST_ACK', 'CLOSING', 'UDP', '累计重传'] },
  { title: '句柄', names: ['句柄数', '句柄上限'] },
]

const asideCurves = new Set(['CPU核数'])

export interface NamedChart {
  title: string
  option: ChartOption
  /** 标题旁帮助：只给易歧义指标 */
  help?: string
}

const chartHelp: Record<string, string> = {
  'CPU 构成': '这一采集间隔里各类 CPU 时间的占比。用户是 CPU 在跑应用程序自己的代码（计算、压缩、编解码），系统是陷进内核的时间（系统调用、拷贝、调度）。等待是 CPU 空着等磁盘。用户高而系统不高，时间花在业务代码上。低优先级的 nice 不在这几条线里。',
  内存: '含物理内存与交换。交换是磁盘上的后备：内存不够时系统把闲置页挪过去，交换已用上升通常表示内存吃紧。',
  负载: '过去 1/5/15 分钟的平均可运行队列长度。持续高于 CPU 核数说明算力或等待偏紧。',
  句柄: '进程打开的文件描述符 / 句柄数。Linux 上限单独一轴，避免把已用数压成贴底的线；占比太低时仪表显示已分配数。Windows 通常只有已分配数、无可用上限。',
  磁盘吞吐: '同 iostat 的 rkB/s、wkB/s，这里按字节/秒显示。数值是相邻两次采集的平均（代理默认约 15 秒），不是 iostat 1 秒那一行。',
  '磁盘 IOPS': '同 iostat 的 r/s、w/s（已完成次数，不含 rrqm/wrqm）。同样是采集间隔内的平均。',
  磁盘延迟: '同 iostat 的 r_await、w_await（含排队），不是 svctm。这一间隔里次数很少时，一次慢 IO 会把平均值拉高。',
  磁盘繁忙: '同 iostat 的 %util（ΔIoTime / Δ墙钟）。采集间隔比 iostat 长时，短脉冲会被摊平。Windows 通常无 IoTime，会一直是 0%。',
  进程IO: '这一间隔里进程自己交给存储的读、写速度，单位与磁盘吞吐相同。Linux 把已退出子进程的历史读写加到父进程上，所以不列出 1 号进程和 systemd。上一拍见过的子进程会从父进程这一拍里减掉。只保留读或写靠前的进程，不能对应到某一块盘。读不到其他用户的进程时这张图是空的。',
  连接: 'TCP 各状态连接数与累计重传。ESTABLISHED 多为业务连接，TIME_WAIT / CLOSE_WAIT 偏高需留意。',
  连接状态: '当前时刻各 TCP 状态的连接数快照。',
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

function formatValue(unit: string, value: number) {
  if (!Number.isFinite(value)) return '—'
  if (unit === 'percent') return percent(value)
  if (unit === 'rate') return rate(value)
  if (unit === 'count' || unit === 'cap') return count(value)
  if (unit === 'bytes') return bytes(value)
  if (unit === 'cores') return `${Number.isInteger(value) ? String(value) : value.toFixed(1)} 核`
  if (unit === 'ms') return latency(value)
  if (unit === 'iops' || unit === 'persec') return `${count(value)} /s`
  return fixed(value, 2)
}

const healthy = '#1a9a62'
const warn = '#c49212'
const danger = '#e23b55'

function tone(value: number) {
  if (value >= 85) return danger
  if (value >= 70) return warn
  return healthy
}

function tooltipLayer() {
  const surface = 'var(--rs-surface-elevated)'
  const border = 'var(--rs-popover-border)'
  const text = 'var(--rs-text-primary)'
  const shadow = 'var(--rs-popover-shadow)'
  return {
    appendTo: 'body' as const,
    className: 'fg-chart-tooltip',
    // 鼠标可以移进浮层。默认 pointer-events 是 none，滚轮到不了指标列表。
    enterable: true,
    hideDelay: 400,
    backgroundColor: surface,
    borderColor: border,
    textStyle: { color: text, fontSize: 12 },
    extraCssText: `max-height:min(70vh,520px);overflow-y:auto;overscroll-behavior:contain;z-index:10000;background-color:${surface};color:${text};border-color:${border};box-shadow:${shadow};`,
  }
}

function axisTooltip(formatter: (params: unknown) => string) {
  return {
    ...tooltipLayer(),
    trigger: 'axis' as const,
    formatter,
  }
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
        center: ['50%', '54%'],
        progress: { show: true, width: 12, roundCap: true, itemStyle: { color: tone(n) } },
        axisLine: {
          lineStyle: {
            width: 12,
            color: [
              [0.7, 'rgba(26, 154, 98, 0.28)'],
              [0.85, 'rgba(196, 146, 18, 0.4)'],
              [1, 'rgba(226, 59, 85, 0.4)'],
            ],
          },
        },
        pointer: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        title: {
          offsetCenter: [0, '78%'],
          fontSize: 12,
          color: colors.text,
          formatter: caption ? `{name|${name}}\n{cap|${caption}}` : name,
          rich: {
            name: { fontSize: 12, color: colors.text, lineHeight: 16 },
            cap: { fontSize: 11, color: colors.title, lineHeight: 16, fontWeight: 600 },
          },
        },
        detail: {
          valueAnimation: true,
          offsetCenter: [0, '8%'],
          fontSize: 22,
          fontWeight: 600,
          color: colors.title,
          formatter: (current: number) => `${current.toFixed(1)}%`,
        },
        data: [{ value: Number(n.toFixed(1)), name }],
      },
    ],
  }
}

/** 无上限的计数（如 Windows 系统句柄），中间显示绝对数量，不伪造成百分比。 */
export function gaugeCountOption(name: string, value: number, caption = ''): ChartOption {
  const n = Number.isFinite(value) ? Math.max(0, value) : 0
  const colors = ink()
  return {
    series: [
      {
        type: 'gauge',
        startAngle: 210,
        endAngle: -30,
        min: 0,
        max: Math.max(n, 1),
        radius: '96%',
        center: ['50%', '54%'],
        progress: { show: true, width: 12, roundCap: true, itemStyle: { color: healthy } },
        axisLine: { lineStyle: { width: 12, color: [[1, 'rgba(26, 154, 98, 0.22)']] } },
        pointer: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        title: {
          offsetCenter: [0, '78%'],
          fontSize: 12,
          color: colors.text,
          formatter: caption ? `{name|${name}}\n{cap|${caption}}` : name,
          rich: {
            name: { fontSize: 12, color: colors.text, lineHeight: 16 },
            cap: { fontSize: 11, color: colors.title, lineHeight: 16, fontWeight: 600 },
          },
        },
        detail: {
          valueAnimation: true,
          offsetCenter: [0, '8%'],
          fontSize: 22,
          fontWeight: 600,
          color: colors.title,
          formatter: () => count(n),
        },
        data: [{ value: n, name }],
      },
    ],
  }
}

export function trendCharts(curves: HostCurve[]): NamedChart[] {
  const plain = curves.filter((curve) => !curve.group)
  const byName = new Map(plain.map((curve) => [curve.name, curve]))
  const used = new Set<string>()
  const charts: NamedChart[] = []
  for (const group of trendGroups) {
    const items = group.names
      .map((name) => byName.get(name))
      .filter((item): item is HostCurve => !!item && item.points.length > 0)
    if (items.length === 0) continue
    for (const item of items) used.add(item.name)
    charts.push({ title: group.title, option: lineOption(items), help: chartHelp[group.title] })
  }
  for (const curve of plain) {
    if (used.has(curve.name) || asideCurves.has(curve.name) || curve.points.length === 0) continue
    charts.push({ title: curve.name, option: lineOption([curve]), help: chartHelp[curve.name] })
  }
  const disks = new Map<string, HostCurve[]>()
  for (const curve of curves) {
    if (!curve.group || curve.points.length === 0) continue
    const items = disks.get(curve.group) ?? []
    items.push(curve)
    disks.set(curve.group, items)
  }
  const diskCharts = [...disks.entries()].map(([title, items]) => ({
    title,
    option: lineOption(items),
    help: chartHelp[title],
  }))
  const anchor = charts.findIndex((chart) => chart.title === '连接')
  charts.splice(anchor < 0 ? charts.length : anchor, 0, ...diskCharts)
  return charts
}

export function snapshotCharts(detail: HostDetail): NamedChart[] {
  const charts: NamedChart[] = []
  if (detail.cores.length > 0) {
    charts.push({
      title: 'CPU 核心',
      option: barOption(
        detail.cores.map((core) => core.coreName),
        [{ name: '使用率', data: detail.cores.map((core) => core.usagePercent) }],
        'percent',
      ),
    })
  }
  if (detail.disks.length > 0) {
    charts.push({ title: '磁盘容量', option: capacityOption(detail.disks) })
  }
  if (detail.diskIo.length > 0) {
    charts.push({
      title: '磁盘 IO',
      help: chartHelp['磁盘吞吐'],
      option: barOption(
        detail.diskIo.map((io) => io.deviceName),
        [
          { name: '读', data: detail.diskIo.map((io) => io.readBytesPerSec) },
          { name: '写', data: detail.diskIo.map((io) => io.writeBytesPerSec) },
        ],
        'rate',
      ),
    })
  }
  if (detail.nets.length > 0) {
    charts.push({
      title: '网卡流量',
      option: barOption(
        detail.nets.map((nic) => nic.nicName),
        [
          { name: '接收', data: detail.nets.map((nic) => nic.receiveBytesPerSec) },
          { name: '发送', data: detail.nets.map((nic) => nic.transmitBytesPerSec) },
        ],
        'rate',
      ),
    })
  }
  const states = tcpStates(detail.tcp).filter((item) => item.count > 0)
  if (states.length > 0) {
    charts.push({
      title: '连接状态',
      help: chartHelp['连接状态'],
      option: barOption(
        states.map((item) => item.state),
        [{ name: '数量', data: states.map((item) => item.count) }],
        'count',
      ),
    })
  }
  if (detail.procs.length > 0) {
    charts.push({
      title: '进程 CPU',
      option: barOption(
        detail.procs.map((proc) => proc.processName),
        [{ name: 'CPU', data: detail.procs.map((proc) => proc.cpuPercent) }],
        'percent',
      ),
    })
    charts.push({
      title: '进程内存',
      option: barOption(
        detail.procs.map((proc) => proc.processName),
        [{ name: '内存', data: detail.procs.map((proc) => proc.rssBytes) }],
        'bytes',
      ),
    })
  }
  return charts
}

function lastText(curve: HostCurve) {
  const point = curve.points[curve.points.length - 1]
  if (!point) return curve.name
  return `${curve.name} ${formatValue(curve.unit, point.value)}`
}

function valueAxis(unit: string, colors: ReturnType<typeof ink>, split: boolean) {
  return {
    type: 'value' as const,
    min: unit === 'percent' || unit === 'bytes' || unit === 'cores' || unit === 'ms' || unit === 'iops' || unit === 'persec' || unit === 'count' || unit === 'cap' ? 0 : undefined,
    max: unit === 'percent' ? 100 : undefined,
    axisLabel: {
      color: colors.text,
      fontSize: 11,
      formatter: (value: number) => formatValue(unit, value),
    },
    splitLine: { show: split, lineStyle: { color: colors.line } },
  }
}

function chartSpanMs(times: string[]) {
  if (times.length < 2) return 0
  const start = Date.parse(times[0])
  const end = Date.parse(times[times.length - 1])
  if (!Number.isFinite(start) || !Number.isFinite(end)) return 0
  return Math.abs(end - start)
}

/** 三小时以内标到秒。采集大约 5 秒一次，只写到分钟时同一分钟里的点会对在同一个刻度上。 */
function axisTick(time: string, spanMs: number) {
  const text = clock(time)
  if (text.length < 19) return text
  if (spanMs > 0 && spanMs <= 3 * 60 * 60 * 1000) return text.slice(11, 19)
  return text.slice(5, 16)
}

function lineOption(curves: HostCurve[]): ChartOption {
  const times = [...new Set(curves.flatMap((curve) => curve.points.map((point) => point.observedTime)))].sort()
  const spanMs = chartSpanMs(times)
  const colors = ink()
  const units = [...new Set(curves.map((curve) => curve.unit))]
  const byName = new Map(curves.map((curve) => [curve.name, curve]))
  return {
    color: palette,
    tooltip: axisTooltip((params: unknown) => {
      const rows = (Array.isArray(params) ? params : [params]) as Array<{
        seriesName?: string
        value?: number | string | null
        axisValue?: string
        marker?: string
      }>
      const active = rows.filter((row) => {
        const value = Number(row.value)
        return Number.isFinite(value) && value !== 0
      })
      const shown = rows.length > 8 && active.length > 0 ? active : rows
      const body = shown.map((row) => {
        const curve = byName.get(row.seriesName ?? '')
        return `${row.marker ?? ''}${row.seriesName ?? ''} ${formatValue(curve?.unit ?? '', Number(row.value))}`
      })
      const hidden = rows.length - shown.length
      if (hidden > 0) body.push(`另有 ${hidden} 项为 0`)
      const when = rows[0]?.axisValue ? clock(rows[0].axisValue) : ''
      return [when, ...body].filter(Boolean).join('<br/>')
    }),
    legend: {
      type: curves.length > 4 ? 'scroll' : 'plain',
      top: 0,
      textStyle: { color: colors.text, fontSize: 12 },
      formatter: (name: string) => {
        const curve = byName.get(name)
        return curve ? lastText(curve) : name
      },
    },
    grid: { left: 72, right: units.length > 1 ? 56 : 16, top: 36, bottom: 28 },
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
    yAxis: units.map((unit, index) => valueAxis(unit, colors, index === 0)),
    series: curves.map((curve, index) => {
      const map = new Map(curve.points.map((point) => [point.observedTime, point.value]))
      const color = palette[index % palette.length]
      return {
        name: curve.name,
        type: 'line',
        yAxisIndex: Math.max(0, units.indexOf(curve.unit)),
        smooth: false,
        showSymbol: times.length < 3,
        connectNulls: false,
        data: times.map((time) => map.get(time) ?? null),
        areaStyle: curves.length === 1 ? { color, opacity: 0.12 } : undefined,
        lineStyle: { width: 2, color },
        itemStyle: { color },
      }
    }),
  }
}

function shortDevice(disk: { deviceName: string; mountPoint: string }) {
  let device = disk.deviceName.trim().replace(/^\/dev\//, '')
  const slash = device.lastIndexOf('/')
  if (slash >= 0) device = device.slice(slash + 1)
  return device || disk.mountPoint
}

function capacityOption(disks: HostDetail['disks']): ChartOption {
  const rows = [...disks].sort((left, right) => right.usedPercent - left.usedPercent)
  const colors = ink()
  const horizontal = rows.length > 4
  const counts = new Map<string, number>()
  for (const disk of rows) {
    const name = shortDevice(disk)
    counts.set(name, (counts.get(name) ?? 0) + 1)
  }
  const names = rows.map((disk) => {
    const name = shortDevice(disk)
    if ((counts.get(name) ?? 0) > 1 && disk.mountPoint && disk.mountPoint !== name) return `${name} ${disk.mountPoint}`
    return name
  })
  const valueAxis = {
    type: 'value' as const,
    min: 0,
    axisLabel: {
      color: colors.text,
      fontSize: 11,
      formatter: (value: number) => formatValue('bytes', value),
    },
    splitLine: { lineStyle: { color: colors.line } },
  }
  const categoryAxis = {
    type: 'category' as const,
    data: names,
    inverse: horizontal,
    axisLabel: { color: colors.text, fontSize: 11, interval: 0, width: horizontal ? 120 : undefined, overflow: 'truncate' },
  }
  return {
    tooltip: {
      ...tooltipLayer(),
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: unknown) => {
        const list = (Array.isArray(params) ? params : [params]) as Array<{ dataIndex?: number }>
        const index = list[0]?.dataIndex ?? 0
        const disk = rows[index]
        if (!disk) return ''
        const device = shortDevice(disk)
        const title = disk.mountPoint && disk.mountPoint !== device ? `${device} · ${disk.mountPoint}` : device
        return [
          title,
          `已用 ${bytes(disk.usedBytes)}（${percent(disk.usedPercent)}）`,
          `可用 ${bytes(disk.freeBytes)}`,
          `总量 ${bytes(disk.totalBytes)}`,
        ].join('<br/>')
      },
    },
    legend: { top: 0, textStyle: { color: colors.text, fontSize: 12 } },
    grid: { left: horizontal ? 96 : 64, right: 16, top: 32, bottom: horizontal ? 24 : 36 },
    xAxis: horizontal ? valueAxis : categoryAxis,
    yAxis: horizontal ? categoryAxis : valueAxis,
    series: [
      {
        name: '已用',
        type: 'bar',
        stack: 'cap',
        barMaxWidth: 18,
        data: rows.map((disk) => ({ value: disk.usedBytes, itemStyle: { color: tone(disk.usedPercent) } })),
      },
      {
        name: '可用',
        type: 'bar',
        stack: 'cap',
        barMaxWidth: 18,
        itemStyle: { color: 'rgba(8, 120, 173, 0.12)' },
        data: rows.map((disk) => disk.freeBytes),
      },
    ],
  }
}

function barOption(categories: string[], series: { name: string; data: number[] }[], unit: string): ChartOption {
  const colors = ink()
  const horizontal = categories.length > 6
  const valueAxis = {
    type: 'value' as const,
    max: unit === 'percent' ? 100 : undefined,
    axisLabel: {
      color: colors.text,
      fontSize: 11,
      formatter: (value: number) => formatValue(unit, value),
    },
    splitLine: { lineStyle: { color: colors.line } },
  }
  const categoryAxis = {
    type: 'category' as const,
    data: categories,
    axisLabel: {
      color: colors.text,
      fontSize: 11,
      interval: 0,
      rotate: !horizontal && categories.length > 4 ? 28 : 0,
    },
  }
  return {
    color: palette,
    tooltip: {
      ...tooltipLayer(),
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      valueFormatter: (value: unknown) => formatValue(unit, Number(value)),
    },
    legend: series.length > 1 ? { top: 0, textStyle: { color: colors.text, fontSize: 12 } } : undefined,
    grid: { left: horizontal ? 96 : 64, right: 16, top: series.length > 1 ? 32 : 16, bottom: horizontal ? 24 : 52 },
    xAxis: horizontal ? valueAxis : categoryAxis,
    yAxis: horizontal ? categoryAxis : valueAxis,
    series: series.map((item) => ({
      name: item.name,
      type: 'bar',
      barMaxWidth: 16,
      data: item.data,
    })),
  }
}
