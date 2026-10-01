import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { use, type EChartsCoreOption } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import type { MetricDef, Point } from '../api'

use([CanvasRenderer, LineChart, GridComponent, TooltipComponent, LegendComponent])

export type ChartOption = EChartsCoreOption

const palette = ['#0878ad', '#5b6cff', '#1a9a62', '#c49212', '#e23b55', '#6a4dff']

function ink() {
  if (typeof document === 'undefined') {
    return {
      text: '#4d647c',
      line: 'rgba(8, 120, 173, 0.16)',
      title: '#102033',
      surface: '#ffffff',
      border: '#c5d6e4',
    }
  }
  const style = getComputedStyle(document.documentElement)
  return {
    text: style.getPropertyValue('--rs-text-secondary').trim() || '#4d647c',
    title: style.getPropertyValue('--rs-text-primary').trim() || '#102033',
    line: style.getPropertyValue('--fg-grid-line').trim() || 'rgba(8, 120, 173, 0.16)',
    surface: style.getPropertyValue('--rs-surface-elevated').trim() || '#ffffff',
    border: style.getPropertyValue('--rs-border').trim() || '#c5d6e4',
  }
}

function bytesOf(value: number) {
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${Math.round(value)} B`
}

function formatMetric(metric: MetricDef, value: unknown) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
  if (metric.unit === 'byte') return bytesOf(value)
  if (metric.unit === '%') return `${value.toFixed(1)}%`
  if (metric.unit === 'ratio') return value.toFixed(2)
  if (metric.unit === 'KB/s') return `${value.toFixed(1)} KB/s`
  if (metric.unit === '/s') return `${value.toFixed(2)}/s`
  if (metric.unit === 'ms') return `${Math.round(value)} ms`
  if (metric.unit === 's' && value >= 3600) return `${(value / 3600).toFixed(1)} 小时`
  if (metric.unit === 's') return `${Math.round(value)} s`
  if (Number.isInteger(value)) return String(value)
  return value.toFixed(1)
}

function clockLabel(value: unknown) {
  const text = String(value ?? '')
  const matched = text.match(/(\d{2}:\d{2}:\d{2})/)
  return matched?.[1] ?? text
}

function pointValue(metric: MetricDef, point: Point) {
  const value = point.values?.[metric.code]
  if (typeof value !== 'number' || !Number.isFinite(value)) return null
  if (value < 0) return null
  if (metric.code === 'maxMemoryBytes' && value === 0) return null
  return value
}

function percentCeiling(metrics: MetricDef[], points: Point[]) {
  let peak = 100
  for (const metric of metrics) {
    if (metric.unit !== '%') continue
    for (const point of points) {
      const value = pointValue(metric, point)
      if (value != null && value > peak) peak = value
    }
  }
  if (peak <= 100) return 100
  return Math.ceil(peak / 10) * 10
}

function seriesName(metric: MetricDef) {
  if (metric.unit && metric.unit !== 'byte') return `${metric.name} (${metric.unit})`
  return metric.name
}

export function seriesOption(metrics: MetricDef[], points: Point[]): ChartOption {
  const colors = ink()
  const times = points.map((item) => item.observedTime)
  const bytes = metrics.length > 0 && metrics.every((metric) => metric.unit === 'byte')
  const percentOnly = metrics.length > 0 && metrics.every((metric) => metric.unit === '%')
  const split = metrics.some((metric) => metric.unit === '%') && metrics.some((metric) => metric.unit !== '%')
  const sparse = points.length <= 8
  const showLegend = metrics.length > 1
  const axisText = { color: colors.text, fontSize: 11 }

  const countAxis = {
    type: 'value' as const,
    min: 0,
    axisLabel: bytes ? { ...axisText, formatter: (value: number) => bytesOf(value) } : axisText,
    splitLine: { lineStyle: { color: colors.line } },
  }
  const percentAxis = {
    type: 'value' as const,
    min: 0,
    max: percentCeiling(metrics, points),
    axisLabel: { ...axisText, formatter: (value: number) => `${value}%` },
    splitLine: { show: false },
  }

  return {
    color: palette,
    tooltip: {
      trigger: 'axis',
      confine: true,
      backgroundColor: colors.surface,
      borderColor: colors.border,
      textStyle: { color: colors.title, fontSize: 12 },
      axisPointer: { type: 'line', lineStyle: { color: colors.line } },
    },
    legend: {
      show: showLegend,
      top: 0,
      left: 0,
      itemWidth: 10,
      itemHeight: 8,
      itemGap: 14,
      textStyle: { color: colors.text, fontSize: 12 },
    },
    grid: { left: 8, right: split ? 12 : 8, top: showLegend ? 32 : 12, bottom: 4, containLabel: true },
    xAxis: {
      type: 'category',
      data: times,
      boundaryGap: true,
      axisLabel: { ...axisText, formatter: clockLabel, hideOverlap: true },
      axisTick: { alignWithLabel: true },
      axisLine: { lineStyle: { color: colors.line } },
    },
    yAxis: split ? [countAxis, percentAxis] : percentOnly ? percentAxis : countAxis,
    series: metrics.map((metric) => ({
      name: seriesName(metric),
      type: 'line',
      yAxisIndex: split && metric.unit === '%' ? 1 : 0,
      showSymbol: sparse,
      symbolSize: 7,
      connectNulls: true,
      data: points.map((item) => pointValue(metric, item)),
      tooltip: {
        valueFormatter: (value: unknown) => formatMetric(metric, value),
      },
    })),
  }
}
