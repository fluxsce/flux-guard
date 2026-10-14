<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsDatePicker, RsEmpty, RsInputNumber, RsSwitch, RsTable, RsTooltip, useRsToast, type RsDatePickerShortcut, type RsTableColumn } from '@/ui'
import type { JavaApp, JavaBeat, JavaCollector, JavaDetail, JavaPause, JavaRestart, SeriesRange } from '../api'
import JavaChart from './JavaChart.vue'
import { gaugeOption, trendCharts } from '../composables/charts'
import { bytes, clock, count, millis, percent } from '../composables/format'

const props = defineProps<{
  app: JavaApp
  detail: JavaDetail | null
  curves: Parameters<typeof trendCharts>[0]
  restarts: JavaRestart[]
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

const trends = computed(() => trendCharts(props.curves, (props.restarts ?? []).map((item) => item.observedTime)))
const collectors = computed(() => props.detail?.collectors ?? [])
const previous = computed(() => props.detail?.previous ?? null)
const beat = computed(() => props.detail?.beat ?? null)
const pauses = computed(() => beat.value?.pauses ?? [])

function knownNumber(value: unknown) {
  const n = Number(value)
  return Number.isFinite(n) ? n : Number.NaN
}

function known(value: unknown, render: (n: number) => string) {
  const n = knownNumber(value)
  if (!Number.isFinite(n) || n < 0) return '—'
  return render(n)
}

function overheadText(value: number | undefined) {
  const n = knownNumber(value)
  if (!Number.isFinite(n) || n < 0) return '—'
  return `${n.toFixed(3)}%`
}

function deadlockText(row: JavaBeat | null) {
  if (!row || row.deadlockCount < 0) return '未采集'
  return count(row.deadlockCount)
}

function pauseMeasure(row: JavaPause, value: unknown, render: (n: number) => string) {
  if (row.pauseCount <= 0 && knownNumber(value) <= 0) return '—'
  return known(value, render)
}

function tableHeight(rows: number) {
  return Math.min(360, 36 + Math.max(rows, 1) * 34 + 2)
}

const pauseColumns: RsTableColumn<JavaPause>[] = [
  { key: 'collectorName', title: '回收器', dataIndex: 'collectorName', ellipsis: true, sortable: true },
  { key: 'pauseCount', title: '停顿次数', dataIndex: 'pauseCount', align: 'right', sortable: true, formatter: (value: unknown) => known(value, count) },
  { key: 'pauseMaxMs', title: '最长', dataIndex: 'pauseMaxMs', align: 'right', sortable: true, formatter: (value: unknown, row: JavaPause) => pauseMeasure(row, value, millis) },
  { key: 'pauseP50Ms', title: 'p50', dataIndex: 'pauseP50Ms', align: 'right', sortable: true, formatter: (value: unknown) => known(value, millis) },
  { key: 'pauseP99Ms', title: 'p99', dataIndex: 'pauseP99Ms', align: 'right', sortable: true, formatter: (value: unknown) => known(value, millis) },
  { key: 'pauseTotalMs', title: '合计', dataIndex: 'pauseTotalMs', align: 'right', sortable: true, formatter: (value: unknown, row: JavaPause) => pauseMeasure(row, value, millis) },
  { key: 'pauseCause', title: '最长一次的原因', dataIndex: 'pauseCause', ellipsis: true },
  { key: 'heapBeforeBytes', title: '最长停顿前', dataIndex: 'heapBeforeBytes', align: 'right', formatter: (value: unknown) => known(value, bytes) },
  { key: 'heapAfterBytes', title: '最长停顿后', dataIndex: 'heapAfterBytes', align: 'right', formatter: (value: unknown) => known(value, bytes) },
  { key: 'promotedBytes', title: '区间晋升', dataIndex: 'promotedBytes', align: 'right', formatter: (value: unknown) => known(value, bytes) },
  { key: 'cycleCount', title: '周期次数', dataIndex: 'cycleCount', align: 'right', formatter: (value: unknown) => known(value, count) },
  { key: 'cycleTimeMs', title: '周期耗时', dataIndex: 'cycleTimeMs', align: 'right', formatter: (value: unknown) => known(value, millis) },
]

const gcColumns: RsTableColumn<JavaCollector>[] = [
  { key: 'collectorName', title: '回收器', dataIndex: 'collectorName', ellipsis: true, sortable: true },
  { key: 'collectionCount', title: '累计次数', dataIndex: 'collectionCount', align: 'right', sortable: true, formatter: (value: unknown) => count(value) },
  { key: 'collectionTimeMs', title: '累计时间', dataIndex: 'collectionTimeMs', align: 'right', sortable: true, formatter: (value: unknown) => millis(value) },
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
        {{ app.onlineFlag === 'Y' ? '在线' : '离线' }} · {{ app.javaVersion || '—' }} · {{ app.mainClass || '—' }} · 进程 {{ app.processId || '—' }}
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
      <h3 class="fg-section">这一拍</h3>
      <p class="fg-note">
        最近一次成功上报。小于 0 的项显示为 —，不会当成 0。停顿开销是本区间 STW 合计除以距离上次成功上报的墙钟，保留三位小数，达到 10.000% 才和曲线上的红线一起提示。上报失败的那一拍会并进下一次成功上报，不会被丢掉，也不会把开销算小。
        p99 是这一拍的最近秩，判断看最长停顿（200ms 关注，500ms 严重）和开销，不看 p99。
        最长停顿前、最长停顿后只描述这一拍里最长的那一次停顿。区间晋升是年轻代回收里老年代增加量的合计；其中有一次测不到，整段就是 —。
        死锁 0 表示调用成功且没有死锁。线程池队列没有标准 JVM 指标，这里不估算。
        下面「累计时间」是 MXBean 的 collectionTime，含周期，不是停顿。
      </p>
      <p v-if="beat" class="fg-note">
        停顿开销 {{ overheadText(beat.overheadPercent) }}
        <template v-if="beat.overheadTime"> · {{ clock(beat.overheadTime) }}</template>
        · 死锁线程 {{ deadlockText(beat) }}
        <template v-if="beat.deadlockTime"> · {{ clock(beat.deadlockTime) }}</template>
      </p>
      <p v-if="beat && beat.deadlockCount > 0 && beat.deadlockText" class="fg-finding" data-level="alert">{{ beat.deadlockText }}</p>
      <RsEmpty v-if="detail && pauses.length === 0" title="这一拍没有停顿或周期" />
      <RsTable
        v-else-if="pauses.length > 0"
        class="fg-detail-table"
        :columns="pauseColumns"
        :data="pauses"
        row-key="collectorName"
        size="sm"
        cell-focus="none"
        null-label="—"
        :rounded="false"
        :context-menu="false"
        :height="tableHeight(pauses.length)"
      />
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
      <h3 class="fg-section">本进程启动以来</h3>
      <p class="fg-note">次数和耗时从这次 JVM 启动累计。应用重启后从 0 再计，曲线在重启处断开，频率不跨代相减。</p>
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
      <template v-if="previous">
        <h3 class="fg-section">上一次进程</h3>
        <p class="fg-note">
          启动 {{ clock(previous.startTime) }}
          · 结束于 {{ clock(previous.closedTime) }}
          · 结束时累计 {{ count(previous.collectionCount) }} 次，{{ millis(previous.collectionTimeMs) }}
          <template v-if="previous.processId"> · 进程 {{ previous.processId }}</template>
        </p>
        <RsTable
          v-if="previous.collectors.length > 0"
          class="fg-detail-table"
          :columns="gcColumns"
          :data="previous.collectors"
          row-key="collectorName"
          size="sm"
          cell-focus="none"
          null-label="—"
          :rounded="false"
          :context-menu="false"
          :height="tableHeight(previous.collectors.length)"
        />
      </template>
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
