<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { init, type ECharts } from 'echarts/core'
import type { ChartOption } from '../composables/charts'

const props = defineProps<{ option: ChartOption; height?: string }>()
const el = ref<HTMLDivElement | null>(null)
let chart: ECharts | null = null
let observer: ResizeObserver | null = null

function render() {
  chart?.setOption(props.option, true)
}

onMounted(() => {
  if (!el.value) return
  chart = init(el.value)
  render()
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(el.value)
})

watch(() => props.option, render)

onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div ref="el" class="fg-chart" :style="{ height: height || '280px' }" />
</template>

<style scoped>
.fg-chart {
  width: 100%;
}
</style>
