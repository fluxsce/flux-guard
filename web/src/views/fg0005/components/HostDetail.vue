<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsDatePicker, RsEmpty, RsInputNumber, RsSwitch, RsTable, RsTooltip, useRsToast, type RsDatePickerShortcut, type RsTableColumn } from '@/ui'
import type { HostDetail, HostMonitor, SeriesRange } from '../api'
import HostChart from './HostChart.vue'
import { gaugeCountOption, gaugeOption, snapshotCharts, trendCharts } from '../composables/charts'
import { bytes, clock, count, fixed, latency, percent, rate, tcpStates } from '../composables/format'
import { levelOptions, metricOptions, opOptions } from '../composables/useHostRules'

const props = defineProps<{
  host: HostMonitor
  detail: HostDetail | null
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

const title = computed(() => props.host.reportName || props.host.hostName || props.host.agentId)

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
  { key: 'refresh', label: '刷新', icon: 'rotate-ccw', code: 'fg0005:view', aiRisk: 'read', onClick: () => slideWindow() },
])

function latestValue(name: string) {
  const curve = props.curves.find((item) => item.name === name)
  const point = curve?.points[curve.points.length - 1]
  return typeof point?.value === 'number' ? point.value : undefined
}

function ratio(used: number | undefined, total: number | undefined) {
  if (used == null || total == null || total <= 0) return undefined
  return (used / total) * 100
}

function latestObserved() {
  for (const curve of props.curves) {
    const time = curve.points[curve.points.length - 1]?.observedTime
    if (time) return time
  }
  return ''
}

function pairCaption(used: number | undefined, total: number | undefined) {
  if (!total) return ''
  return `${bytes(used ?? 0)} / ${bytes(total)}`
}

function diskCaption() {
  const disks = props.detail?.disks ?? []
  if (disks.length === 0) return ''
  const fullest = [...disks].sort((left, right) => right.usedPercent - left.usedPercent)[0]
  const size = pairCaption(fullest.usedBytes, fullest.totalBytes)
  if (!size) return `${disks.length} 块盘`
  if (disks.length === 1) return `${fullest.mountPoint} ${size}`
  return `${disks.length} 块 · ${fullest.mountPoint} ${size}`
}

function fdCaption() {
  const fd = props.detail?.fd
  if (fd && fd.maximumCount > 0) return `${count(fd.allocatedCount)} / ${count(fd.maximumCount)}`
  if ((fd?.allocatedCount ?? props.host.fdAllocated) > 0) return '系统句柄'
  return ''
}

const gauges = computed(() => {
  const cores = latestValue('CPU核数')
  const cpuPercent = ratio(latestValue('占用核'), cores) ?? props.host.cpuUsagePercent
  const cpuCaption = cores ? `${fixed(latestValue('占用核') ?? (cpuPercent / 100) * cores, 1)} / ${fixed(cores, 0)} 核` : ''
  const memUsed = latestValue('内存已用') ?? props.host.memUsedBytes
  const memTotal = latestValue('内存总量') ?? props.host.memTotalBytes
  const memPercent = ratio(latestValue('内存已用'), latestValue('内存总量')) ?? props.host.memUsedPercent
  const memCaption = pairCaption(memUsed, memTotal)
  const fd = props.detail?.fd
  const fdAllocated = fd?.allocatedCount ?? props.host.fdAllocated
  const fdMaximum = fd?.maximumCount ?? 0
  const fdPercent = fdMaximum > 0 ? (fd?.usedPercent ?? props.host.fdUsedPercent) : props.host.fdUsedPercent
  // Linux file-max 往往远大于已用数，占比会显示成 0.0%。这时改显示已分配数。
  const fdGauge =
    fdMaximum > 0 && fdPercent >= 0.05
      ? gaugeOption('句柄', fdPercent, fdCaption())
      : gaugeCountOption('句柄', fdAllocated, fdCaption())
  const fullest = [...(props.detail?.disks ?? [])].sort((left, right) => right.usedPercent - left.usedPercent)[0]
  return [
    { option: gaugeOption('CPU', cpuPercent, cpuCaption) },
    { option: gaugeOption('内存', memPercent, memCaption) },
    { option: gaugeOption('磁盘', fullest?.usedPercent ?? props.host.diskUsedPercent, diskCaption()) },
    {
      option: fdGauge,
      help: '进程打开的文件描述符 / 句柄数。Linux 上限很大时仪表显示已分配数，占比够高才显示百分比。Windows 通常只有已分配数、无可用上限。',
    },
  ]
})

const trends = computed(() => trendCharts(props.curves))
const snapshots = computed(() => (props.detail ? snapshotCharts(props.detail) : []))
const alerts = computed(() => props.detail?.alerts ?? [])

function optionLabel(options: { label: string; value: string }[], value: string) {
  return options.find((item) => item.value === value)?.label ?? value
}

function alertText(item: NonNullable<HostDetail['alerts']>[number]) {
  const status = item.statusCode === 'firing' ? '告警' : item.statusCode === 'acked' ? '已确认' : '持续中'
  const name = item.ruleName || optionLabel(metricOptions, item.metricCode)
  const level = optionLabel(levelOptions, item.levelCode)
  return `${status} · ${level} · ${name} ${optionLabel(opOptions, item.compareOp)} ${item.thresholdValue}，当前 ${item.currentValue}`
}

function tableHeight(rows: number) {
  const header = 36
  const row = 34
  const pad = 2
  return Math.min(360, header + Math.max(rows, 1) * row + pad)
}

const diskColumns: RsTableColumn<HostDetail['disks'][number]>[] = [
  { key: 'mountPoint', title: '挂载点', dataIndex: 'mountPoint', ellipsis: true, sortable: true },
  { key: 'deviceName', title: '设备', dataIndex: 'deviceName', sortable: true },
  { key: 'usedPercent', title: '使用率', dataIndex: 'usedPercent', align: 'right', sortable: true, formatter: (value) => percent(value) },
  { key: 'inodeUsedPercent', title: 'inode', dataIndex: 'inodeUsedPercent', align: 'right', sortable: true, formatter: (value) => percent(value) },
  { key: 'usedBytes', title: '已用', dataIndex: 'usedBytes', align: 'right', sortable: true, formatter: (value) => bytes(value) },
  { key: 'totalBytes', title: '总量', dataIndex: 'totalBytes', align: 'right', sortable: true, formatter: (value) => bytes(value) },
]

const ioColumns: RsTableColumn<HostDetail['diskIo'][number]>[] = [
  { key: 'deviceName', title: '设备', dataIndex: 'deviceName', sortable: true },
  { key: 'readBytesPerSec', title: '读', dataIndex: 'readBytesPerSec', align: 'right', sortable: true, formatter: (value) => rate(value) },
  { key: 'writeBytesPerSec', title: '写', dataIndex: 'writeBytesPerSec', align: 'right', sortable: true, formatter: (value) => rate(value) },
  { key: 'utilPercent', title: '繁忙', dataIndex: 'utilPercent', align: 'right', sortable: true, formatter: (value) => percent(value) },
  { key: 'readLatencyMs', title: '读延迟', dataIndex: 'readLatencyMs', align: 'right', sortable: true, formatter: (value) => latency(value) },
  { key: 'writeLatencyMs', title: '写延迟', dataIndex: 'writeLatencyMs', align: 'right', sortable: true, formatter: (value) => latency(value) },
]

const netColumns: RsTableColumn<HostDetail['nets'][number]>[] = [
  { key: 'nicName', title: '网卡', dataIndex: 'nicName', sortable: true },
  { key: 'receiveBytesPerSec', title: '接收', dataIndex: 'receiveBytesPerSec', align: 'right', sortable: true, formatter: (value) => rate(value) },
  { key: 'transmitBytesPerSec', title: '发送', dataIndex: 'transmitBytesPerSec', align: 'right', sortable: true, formatter: (value) => rate(value) },
  { key: 'receiveErrs', title: '接收错误', dataIndex: 'receiveErrs', align: 'right', sortable: true, formatter: (value) => count(value) },
  { key: 'transmitErrs', title: '发送错误', dataIndex: 'transmitErrs', align: 'right', sortable: true, formatter: (value) => count(value) },
  { key: 'receiveDrop', title: '接收丢弃', dataIndex: 'receiveDrop', align: 'right', sortable: true, formatter: (value) => count(value) },
  { key: 'transmitDrop', title: '发送丢弃', dataIndex: 'transmitDrop', align: 'right', sortable: true, formatter: (value) => count(value) },
]

const procColumns: RsTableColumn<HostDetail['procs'][number]>[] = [
  { key: 'processName', title: '进程', dataIndex: 'processName', ellipsis: true, sortable: true },
  {
    key: 'processId',
    title: 'PID',
    dataIndex: 'processId',
    width: 88,
    align: 'right',
    sortable: true,
    sorter: (left, right) => Number(left.processId) - Number(right.processId),
  },
  { key: 'rssBytes', title: '内存', dataIndex: 'rssBytes', align: 'right', sortable: true, formatter: (value) => bytes(value) },
  { key: 'cpuPercent', title: 'CPU', dataIndex: 'cpuPercent', align: 'right', sortable: true, formatter: (value) => percent(value) },
]

const procIOColumns: RsTableColumn<HostDetail['procIos'][number]>[] = [
  { key: 'processName', title: '进程', dataIndex: 'processName', ellipsis: true, sortable: true },
  {
    key: 'processId',
    title: 'PID',
    dataIndex: 'processId',
    width: 88,
    align: 'right',
    sortable: true,
    sorter: (left, right) => Number(left.processId) - Number(right.processId),
  },
  { key: 'readBytesPerSec', title: '读', dataIndex: 'readBytesPerSec', align: 'right', sortable: true, formatter: (value) => rate(value) },
  { key: 'writeBytesPerSec', title: '写', dataIndex: 'writeBytesPerSec', align: 'right', sortable: true, formatter: (value) => rate(value) },
]
</script>

<template>
  <section class="fg-host-detail">
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
    <div class="fg-host-detail-body">
      <p class="fg-note">
        {{ host.onlineFlag === 'Y' ? '在线' : '离线' }} · {{ host.osName || '—' }} · {{ host.ipAddress || '—' }}
        · 内存 {{ bytes(latestValue('内存已用') ?? host.memUsedBytes) }} / {{ bytes(latestValue('内存总量') ?? host.memTotalBytes) }}
        · 负载 {{ fixed(latestValue('负载') ?? host.loadAvg1, 2) }}
        · {{ clock(latestObserved() || host.reportedTime) }}
      </p>
      <p v-if="loading && !detail" class="fg-note">正在加载</p>
      <ul v-if="alerts.length" class="fg-alerts">
        <li v-for="item in alerts" :key="item.incidentId" :class="'fg-alert-' + item.levelCode">{{ alertText(item) }}</li>
      </ul>
      <section class="fg-kpis">
        <article v-for="(item, index) in gauges" :key="index" class="fg-panel">
          <div v-if="item.help" class="fg-panel-help">
            <RsTooltip icon :content="item.help" aria-label="指标说明" />
          </div>
          <HostChart :option="item.option" height="168px" />
        </article>
      </section>
      <template v-if="detail">
        <h3 class="fg-section">趋势</h3>
        <RsEmpty v-if="trends.length === 0" title="还没有趋势数据" />
        <section v-else class="fg-charts">
          <article v-for="chart in trends" :key="chart.title" class="fg-panel">
            <h4>
              <RsTooltip v-if="chart.help" icon :content="chart.help">{{ chart.title }}</RsTooltip>
              <template v-else>{{ chart.title }}</template>
            </h4>
            <HostChart :option="chart.option" />
          </article>
        </section>
        <h3 class="fg-section">当前</h3>
        <section v-if="snapshots.length" class="fg-charts">
          <article v-for="chart in snapshots" :key="chart.title" class="fg-panel">
            <h4>
              <RsTooltip v-if="chart.help" icon :content="chart.help">{{ chart.title }}</RsTooltip>
              <template v-else>{{ chart.title }}</template>
            </h4>
            <HostChart :option="chart.option" />
          </article>
        </section>
        <p class="fg-note">
          TCP 占用 {{ count(detail.tcp.tcpInUseCount) }}，累计重传 {{ count(detail.tcp.retransmitCount) }}。
          已分配句柄 {{ count(detail.fd.allocatedCount) }}
          <template v-if="detail.fd.maximumCount > 0"> / {{ count(detail.fd.maximumCount) }}（{{ percent(detail.fd.usedPercent) }}）</template>
          。连接合计 {{ count(tcpStates(detail.tcp).reduce((sum, item) => sum + item.count, 0)) }}。
        </p>
        <h3 class="fg-section">进程读写</h3>
        <RsEmpty v-if="!detail.procIos || detail.procIos.length === 0" title="没有进程读写" />
        <RsTable
          v-else
          class="fg-detail-table"
          :columns="procIOColumns"
          :data="detail.procIos"
          row-key="processId"
          size="sm"
          cell-focus="none"
          null-label="—"
          :rounded="false"
          :context-menu="false"
          :height="tableHeight(detail.procIos.length)"
        />
        <h3 class="fg-section">进程明细</h3>
        <RsEmpty v-if="detail.procs.length === 0" title="没有进程占用" />
        <RsTable
          v-else
          class="fg-detail-table"
          :columns="procColumns"
          :data="detail.procs"
          row-key="processId"
          size="sm"
          cell-focus="none"
          null-label="—"
          :rounded="false"
          :context-menu="false"
          :height="tableHeight(detail.procs.length)"
        />
        <h3 class="fg-section">磁盘明细</h3>
        <RsEmpty v-if="detail.disks.length === 0" title="没有磁盘容量" />
        <RsTable
          v-else
          class="fg-detail-table"
          :columns="diskColumns"
          :data="detail.disks"
          row-key="mountPoint"
          size="sm"
          cell-focus="none"
          null-label="—"
          :rounded="false"
          :context-menu="false"
          :height="tableHeight(detail.disks.length)"
        />
        <h3 class="fg-section">磁盘 IO 明细</h3>
        <RsEmpty v-if="detail.diskIo.length === 0" title="没有磁盘 IO" />
        <RsTable
          v-else
          class="fg-detail-table"
          :columns="ioColumns"
          :data="detail.diskIo"
          row-key="deviceName"
          size="sm"
          cell-focus="none"
          null-label="—"
          :rounded="false"
          :context-menu="false"
          :height="tableHeight(detail.diskIo.length)"
        />
        <h3 class="fg-section">网卡明细</h3>
        <RsEmpty v-if="detail.nets.length === 0" title="没有网卡流量" />
        <RsTable
          v-else
          class="fg-detail-table"
          :columns="netColumns"
          :data="detail.nets"
          row-key="nicName"
          size="sm"
          cell-focus="none"
          null-label="—"
          :rounded="false"
          :context-menu="false"
          :height="tableHeight(detail.nets.length)"
        />
      </template>
    </div>
  </section>
</template>

<style scoped>
.fg-host-detail {
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

.fg-host-detail-body {
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

.fg-alerts {
  margin: 0 0 12px;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fg-alerts li {
  padding: 6px 10px;
  font-size: 13px;
  background: rgba(196, 146, 18, 0.14);
  color: var(--rs-text-primary, #102033);
}

.fg-alerts .fg-alert-critical {
  background: rgba(226, 59, 85, 0.14);
}

.fg-alerts .fg-alert-info {
  background: rgba(8, 120, 173, 0.12);
}

.fg-kpis,
.fg-charts {
  display: grid;
  gap: 12px;
}

.fg-kpis {
  grid-template-columns: repeat(4, minmax(0, 1fr));
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

.fg-panel-help {
  position: absolute;
  top: 8px;
  right: 10px;
  z-index: 1;
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

.fg-detail-table :deep(.rs-table-shell--ctx) {
  height: auto;
  flex: 0 0 auto;
}

@media (max-width: 1100px) {
  .fg-kpis,
  .fg-charts {
    grid-template-columns: 1fr;
  }
}
</style>
