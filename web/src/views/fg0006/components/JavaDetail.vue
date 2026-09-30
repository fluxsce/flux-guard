<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsDatePicker, RsEmpty, RsInputNumber, RsSwitch, RsTable, RsTooltip, useRsToast, type RsDatePickerShortcut, type RsTableColumn } from '@/ui'
import type { JavaApp, JavaCollector, JavaDetail, SeriesRange } from '../api'
import JavaChart from './JavaChart.vue'
import { gaugeOption, trendCharts } from '../composables/charts'
import { bytes, clock, count, millis, percent } from '../composables/format'

const props = defineProps<{
  app: JavaApp
  detail: JavaDetail | null
  curves: Parameters<typeof trendCharts>[0]
  loading: boolean
}>()

const emit = defineEmits<{ back: []; refresh: [range: SeriesRange] }>()

const toast = useRsToast()
const range = ref({ start: '', end: '' })
const rollingMinutes = ref<number | null>(60)
const auto = ref(true)
const intervalSec = ref(15)
let timer = 0
let rejectedRange = ''
let shortcutWrite = false

const title = computed(() => props.app.appName || props.app.agentId)

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function utcSql(date: Date) {
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`
}

function localSql(date: Date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function localSqlToUtc(text: string) {
  const matched = text.trim().match(/^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?/)
  if (!matched) return ''
  const date = new Date(
    Number(matched[1]),
    Number(matched[2]) - 1,
    Number(matched[3]),
    Number(matched[4]),
    Number(matched[5]),
    Number(matched[6] ?? 0),
  )
  if (Number.isNaN(date.getTime())) return ''
  return utcSql(date)
}

function rangeEndingNow(minutes: number) {
  const to = new Date()
  const from = new Date(to.getTime() - minutes * 60 * 1000)
  return { start: localSql(from), end: localSql(to) }
}

function applyRolling(minutes: number) {
  shortcutWrite = true
  rollingMinutes.value = minutes
  return rangeEndingNow(minutes)
}

const rangeShortcuts: RsDatePickerShortcut[] = [
  { label: '近 15 分钟', value: () => applyRolling(15) },
  { label: '近 1 小时', value: () => applyRolling(60) },
  { label: '近 6 小时', value: () => applyRolling(360) },
  { label: '近 24 小时', value: () => applyRolling(24 * 60) },
  { label: '近 7 天', value: () => applyRolling(7 * 24 * 60) },
]

function currentRange(): SeriesRange | null {
  const source = rollingMinutes.value ? rangeEndingNow(rollingMinutes.value) : range.value
  const from = localSqlToUtc(source.start)
  const to = localSqlToUtc(source.end)
  if (!from || !to || from >= to) return null
  return { from, to }
}

function publish() {
  const next = currentRange()
  if (!next) return
  const from = Date.parse(`${next.from.replace(' ', 'T')}Z`)
  const to = Date.parse(`${next.to.replace(' ', 'T')}Z`)
  const key = `${next.from}|${next.to}`
  if (to - from > 7 * 24 * 60 * 60 * 1000) {
    if (rejectedRange !== key) {
      rejectedRange = key
      toast.error('时间段不能超过 7 天')
    }
    return
  }
  rejectedRange = ''
  emit('refresh', next)
}

function slideWindow() {
  if (!rollingMinutes.value) {
    publish()
    return
  }
  shortcutWrite = true
  range.value = rangeEndingNow(rollingMinutes.value)
}

function arm() {
  window.clearInterval(timer)
  timer = 0
  if (!auto.value) return
  const seconds = Math.min(300, Math.max(5, Number(intervalSec.value) || 15))
  timer = window.setInterval(() => slideWindow(), seconds * 1000)
}

watch(range, () => {
  if (shortcutWrite) shortcutWrite = false
  else rollingMinutes.value = null
  publish()
}, { deep: true })

watch([auto, intervalSec], arm)

onMounted(() => {
  shortcutWrite = true
  range.value = rangeEndingNow(60)
  arm()
})

onUnmounted(() => {
  window.clearInterval(timer)
})

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'back', label: '返回', icon: 'arrow-left', aiRisk: 'read', onClick: () => emit('back') },
  { key: 'refresh', label: '刷新', icon: 'rotate-ccw', code: 'fg0006:view', aiRisk: 'read', onClick: () => slideWindow() },
])

const findingText = computed(() => {
  const lines = props.detail?.findings?.length ? props.detail.findings : props.app.findings
  return (lines ?? []).join('；')
})

const findingLevel = computed(() => props.detail?.findingLevel || props.app.findingLevel || 'ok')

const heapCaption = computed(() => {
  if (props.app.heapMaxBytes <= 0) return bytes(props.app.heapUsedBytes)
  return `${bytes(props.app.heapUsedBytes)} / ${bytes(props.app.heapMaxBytes)}`
})

const gauges = computed(() => [
  { option: gaugeOption('堆', props.app.heapUsedPercent, heapCaption.value) },
])

const trends = computed(() => trendCharts(props.curves))
const collectors = computed(() => props.detail?.collectors ?? [])

function tableHeight(rows: number) {
  return Math.min(360, 36 + Math.max(rows, 1) * 34 + 2)
}

const gcColumns: RsTableColumn<JavaCollector>[] = [
  { key: 'collectorName', title: '回收器', dataIndex: 'collectorName', ellipsis: true, sortable: true },
  { key: 'collectionCount', title: '累计次数', dataIndex: 'collectionCount', align: 'right', sortable: true, formatter: (value) => count(value) },
  { key: 'collectionTimeMs', title: '累计耗时', dataIndex: 'collectionTimeMs', align: 'right', sortable: true, formatter: (value) => millis(value) },
]
</script>

<template>
  <section class="fg-java-detail">
    <FgToolbar plain wrap :title="title" :buttons="toolbarButtons">
      <template #after>
        <div class="fg-monitor-bar">
          <RsDatePicker
            v-model="range"
            class="fg-range"
            range
            with-time
            with-seconds
            value-format="string"
            size="sm"
            :shortcuts="rangeShortcuts"
          />
          <label class="fg-auto">
            <RsSwitch v-model="auto" size="sm" />
            <span>自动刷新</span>
            <RsInputNumber v-model="intervalSec" class="fg-interval" :min="5" :max="300" :disabled="!auto" size="sm" />
            <span class="fg-unit">秒</span>
          </label>
        </div>
      </template>
    </FgToolbar>
    <div class="fg-java-detail-body">
      <p class="fg-note">
        {{ app.javaVersion || '—' }} · {{ app.mainClass || '—' }} · 进程 {{ app.processId || '—' }}
        · {{ app.hostName || '—' }} {{ app.ipAddress || '' }}
        · 启动 {{ clock(app.startTime) }}
        · {{ clock(app.reportedTime) }}
      </p>
      <p v-if="findingText" class="fg-finding" :data-level="findingLevel">{{ findingText }}</p>
      <p v-if="loading && !detail" class="fg-note">正在加载</p>
      <section class="fg-kpis">
        <article v-for="(item, index) in gauges" :key="index" class="fg-panel">
          <JavaChart :option="item.option" height="168px" />
        </article>
      </section>
      <h3 class="fg-section">趋势</h3>
      <RsEmpty v-if="trends.length === 0" title="还没有趋势数据" />
      <section v-else class="fg-charts">
        <article v-for="chart in trends" :key="chart.title" class="fg-panel">
          <h4>
            <RsTooltip v-if="chart.help" icon :content="chart.help">{{ chart.title }}</RsTooltip>
            <template v-else>{{ chart.title }}</template>
          </h4>
          <JavaChart :option="chart.option" />
        </article>
      </section>
      <h3 class="fg-section">回收器</h3>
      <RsEmpty v-if="collectors.length === 0" title="没有回收器数据" />
      <RsTable
        v-else
        class="fg-detail-table"
        :columns="gcColumns"
        :data="collectors"
        row-key="collectorName"
        size="sm"
        cell-focus="none"
        null-label="—"
        :rounded="false"
        :context-menu="false"
        :height="tableHeight(collectors.length)"
      />
    </div>
  </section>
</template>

<style scoped>
.fg-finding {
  margin: 0 0 12px;
  font-size: 14px;
  line-height: 1.5;
}

.fg-finding[data-level='alert'] {
  color: #e23b55;
}

.fg-finding[data-level='watch'] {
  color: #c49212;
}

.fg-finding[data-level='ok'] {
  color: var(--rs-text-secondary);
}

.fg-java-detail {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--fg-panel-bg, var(--rs-surface));
}

.fg-monitor-bar {
  display: inline-flex;
  flex: 0 0 auto;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px 12px;
  width: max-content;
  max-width: 100%;
}

.fg-range {
  width: 340px;
  max-width: 42vw;
  flex: 0 1 340px;
}

.fg-auto {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  color: var(--rs-text-secondary);
  font-size: 13px;
  white-space: nowrap;
}

.fg-auto :deep(.fg-interval) {
  width: 72px;
  flex: 0 0 72px;
}

.fg-unit {
  color: var(--rs-text-secondary);
  font-size: 13px;
}

.fg-java-detail-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px 16px 24px;
}

.fg-section {
  margin: 18px 0 8px;
  font-size: 13px;
  font-weight: 600;
}

.fg-note {
  margin: 0 0 12px;
  color: var(--rs-text-secondary);
  font-size: 13px;
}

.fg-kpis,
.fg-charts {
  display: grid;
  gap: 12px;
}

.fg-kpis {
  grid-template-columns: minmax(0, 280px);
}

.fg-charts {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.fg-panel {
  position: relative;
  min-width: 0;
  padding: 8px 10px 4px;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface-elevated, var(--rs-surface));
  box-shadow: var(--fg-panel-shadow);
}

.fg-panel h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--rs-text-secondary);
}

.fg-detail-table {
  margin-bottom: 4px;
}

@media (max-width: 960px) {
  .fg-charts {
    grid-template-columns: 1fr;
  }
}
</style>
