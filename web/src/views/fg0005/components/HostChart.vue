<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { init, type ECharts } from 'echarts/core'
import { retainChartTooltipScroll } from '@/chart/tooltipScroll'
import type { ChartOption } from '../composables/charts'

const props = defineProps<{
  option: ChartOption
  height?: string
}>()

const el = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null
let observer: ResizeObserver | null = null
let releaseTooltipScroll: (() => void) | null = null

/** 刷新会整图重画。把当前图例开关带回去，关掉的线不会自己出现。 */
function keepLegend(chart: ECharts, option: ChartOption): ChartOption {
  const current = chart.getOption()?.legend
  if (!Array.isArray(current) || option == null || typeof option !== 'object' || Array.isArray(option)) return option
  const selected: Record<string, boolean> = {}
  for (const item of current) {
    const picked = (item as { selected?: Record<string, boolean> }).selected
    if (picked) Object.assign(selected, picked)
  }
  if (Object.keys(selected).length === 0) return option
  const legend = (option as { legend?: unknown }).legend
  if (legend == null || typeof legend !== 'object') return option
  const keep = (item: object) => ({ ...item, selected })
  return {
    ...option,
    legend: Array.isArray(legend) ? legend.map((item) => keep(item as object)) : keep(legend as object),
  }
}

function render() {
  if (!chart) return
  chart.setOption(keepLegend(chart, props.option), true)
}

onMounted(() => {
  releaseTooltipScroll = retainChartTooltipScroll()
  if (!el.value) return
  chart = init(el.value)
  render()
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(el.value)
})

watch(() => props.option, render)

onBeforeUnmount(() => {
  releaseTooltipScroll?.()
  releaseTooltipScroll = null
  observer?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div ref="el" class="fg-chart" :style="{ height: height || '240px' }" />
</template>

<style scoped>
.fg-chart {
  width: 100%;
}
</style>
