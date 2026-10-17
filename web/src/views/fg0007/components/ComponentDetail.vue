<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import FgToolbar from '@/components/toolbar/FgToolbar.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { getApiMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsDatePicker, RsInputNumber, RsSwitch, RsTabs, useRsToast, type RsDatePickerShortcut, type RsTabItem } from '@/ui'
import { useSessionStore } from '@/stores/session'
import KafkaBrowser from './KafkaBrowser.vue'
import KafkaMonitor from './KafkaMonitor.vue'
import MetricSections from './MetricSections.vue'
import MongoBrowser from './MongoBrowser.vue'
import MongoMonitor from './MongoMonitor.vue'
import RedisBrowser from './RedisBrowser.vue'
import RedisMonitor from './RedisMonitor.vue'
import TerminalPane from './TerminalPane.vue'
import ZkBrowser from './ZkBrowser.vue'
import ZkMonitor from './ZkMonitor.vue'
import { querySeries, type ComponentConn, type Panel, type Point, type PointDetail, type SeriesRange } from '../api'
import { inspect } from '../composables/boards'
import { layoutOf } from '../composables/forms'
import { panelIcon, sectionsFor } from '../composables/panels'

const props = defineProps<{
  conn: ComponentConn
  panels: Panel[]
}>()
const emit = defineEmits<{ back: []; refresh: [] }>()

const toast = useRsToast()
const session = useSessionStore()
const panel = ref('overview')
const points = ref<Point[]>([])
const error = ref('')
const range = ref({ start: '', end: '' })
const rollingMinutes = ref<number | null>(60)
const auto = ref(true)
const intervalSec = ref(15)
let timer = 0
let rejectedRange = ''
let shortcutWrite = false

const latest = computed(() => points.value[points.value.length - 1] ?? null)
const board = computed(() => inspect(props.conn, latest.value))
const visiblePanels = computed(() =>
  props.panels.filter((item) => item.terminalFlag !== 'Y' || session.allowButton('fg0007:shell')),
)
const tabItems = computed<RsTabItem[]>(() =>
  visiblePanels.value.map((item) => ({ value: item.panelCode, label: item.panelName, icon: panelIcon(item.panelCode) })),
)
const activePanel = computed(() => visiblePanels.value.find((item) => item.panelCode === panel.value) ?? visiblePanels.value[0] ?? null)
const panelSections = computed(() => sectionsFor(props.panels, activePanel.value?.panelCode ?? 'overview', board.value.sections))
const terminal = computed(() => activePanel.value?.terminalFlag === 'Y')
const detail = computed<PointDetail>(() => latest.value?.detail ?? {})
const layout = computed(() => layoutOf(props.conn.kindCode, props.conn.optionText, props.conn.userName, props.conn.secretSet).label)
const ranged = computed(() => {
  const kind = props.conn.kindCode
  return kind === 'redis' || kind === 'mongo' || kind === 'kafka' || kind === 'zk'
})
const kafkaSignals = ['Broker', '离线分区', '未同步分区', '低于最小 ISR', '最大分区延迟', '再均衡']
const signalCards = computed(() => {
  if (props.conn.kindCode !== 'kafka' || terminal.value) return []
  const want = new Set(kafkaSignals)
  return board.value.sections.flatMap((section) => section.cards).filter((card) => want.has(card.label))
})
const chartSections = computed(() => {
  if (props.conn.kindCode !== 'kafka' || terminal.value) return panelSections.value
  if ((activePanel.value?.panelCode ?? 'overview') === 'overview') return board.value.sections
  return panelSections.value
})
const kafkaDesk = computed(() => {
  if (props.conn.kindCode !== 'kafka' || terminal.value) return false
  const code = activePanel.value?.panelCode ?? ''
  return code === 'broker' || code === 'topic' || code === 'group' || code === 'partition'
})

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

async function load(window?: SeriesRange | null) {
  const limit = ranged.value ? 4000 : 120
  const result = await querySeries(props.conn.componentId, limit, ranged.value ? window : null)
  if (!isApiSuccess(result)) {
    error.value = getApiMessage(result, '曲线加载失败')
    return
  }
  error.value = ''
  points.value = parseApiData<Point[]>(result) ?? []
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
  void load(next)
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
  if (!ranged.value || !auto.value) return
  const seconds = Math.min(300, Math.max(5, Number(intervalSec.value) || 15))
  timer = window.setInterval(() => slideWindow(), seconds * 1000)
}

watch(range, () => {
  if (!ranged.value) return
  if (shortcutWrite) shortcutWrite = false
  else rollingMinutes.value = null
  publish()
}, { deep: true })

watch([auto, intervalSec], arm)

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'back', label: '返回', icon: 'arrow-left', aiRisk: 'read', onClick: () => emit('back') },
  {
    key: 'refresh',
    label: '刷新',
    icon: 'rotate-ccw',
    code: 'fg0007:view',
    aiRisk: 'read',
    onClick: () => {
      if (props.conn.kindCode === 'kafka' || props.conn.kindCode === 'zk') emit('refresh')
      else slideWindow()
    },
  },
])

const signalNames: Record<string, string> = {
  Broker: 'Broker',
  离线分区: '离线',
  未同步分区: '未同步',
  '低于最小 ISR': '最小 ISR',
  再均衡: '再均衡',
  最大分区延迟: '最大延迟',
}

function signalName(label: string) {
  return signalNames[label] || label
}

watch(visiblePanels, (list) => {
  if (!list.some((item) => item.panelCode === panel.value)) panel.value = list[0]?.panelCode ?? 'overview'
})

watch(() => props.conn.kindCode, () => {
  panel.value = 'overview'
})

watch(() => props.conn.componentId, () => {
  if (ranged.value) publish()
  else void load()
})
watch(() => props.conn.probedTime, () => {
  if (ranged.value) publish()
  else void load()
})

onMounted(() => {
  if (!ranged.value) {
    void load()
    return
  }
  shortcutWrite = true
  range.value = rangeEndingNow(60)
  arm()
})

onUnmounted(() => {
  window.clearInterval(timer)
})
</script>

<template>
  <section class="fg-detail">
    <FgToolbar v-if="ranged" plain :buttons="toolbarButtons">
      <template #title>
        <span class="fg-desk-title">
          <strong>{{ conn.componentName }}</strong>
          <span class="fg-desk-meta">{{ conn.kindName || conn.kindCode }} · {{ layout }} · {{ conn.address }}</span>
        </span>
      </template>
      <template #right>
        <div v-if="!terminal" class="fg-monitor-bar">
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
    <header v-else class="fg-detail-bar">
      <RsButton size="sm" variant="ghost" icon="arrow-left" @click="emit('back')">返回</RsButton>
      <div class="fg-detail-id">
        <strong>{{ conn.componentName }}</strong>
        <span class="fg-chip">{{ conn.kindName || conn.kindCode }}</span>
        <span class="fg-meta">{{ layout }}</span>
        <span class="fg-addr">{{ conn.address }}</span>
      </div>
      <RsButton size="sm" variant="secondary" icon="rotate-ccw" @click="emit('refresh')">刷新</RsButton>
    </header>
    <RsTabs v-if="tabItems.length" v-model="panel" class="fg-panels" :items="tabItems" size="sm" panelless />
    <div class="fg-detail-body" :class="{ 'fg-detail-body--desk': kafkaDesk }">
      <p v-if="board.finding && !terminal" class="fg-finding" :data-level="board.level">{{ board.finding }}</p>
      <p v-else-if="!points.length && !terminal" class="fg-note">还没有采集点。保存后会按间隔拨号，也可以在列表里立即采集。</p>
      <p v-if="error && !terminal" class="fg-finding" data-level="alert">{{ error }}</p>
      <div v-if="signalCards.length" class="fg-signals">
        <span v-for="item in signalCards" :key="item.label" class="fg-signal" :data-level="item.level">
          <span class="fg-signal__name">{{ signalName(item.label) }}</span>
          <strong>{{ item.value }}</strong>
        </span>
      </div>
      <ZkBrowser v-if="terminal && conn.kindCode === 'zk'" :component-id="conn.componentId" />
      <MongoBrowser v-else-if="terminal && conn.kindCode === 'mongo'" :component-id="conn.componentId" />
      <KafkaBrowser v-else-if="terminal && conn.kindCode === 'kafka'" :component-id="conn.componentId" />
      <RedisBrowser v-else-if="terminal && conn.kindCode === 'redis'" :component-id="conn.componentId" />
      <TerminalPane v-else-if="terminal" :kind-code="conn.kindCode" :kind-name="conn.kindName || conn.kindCode" />
      <template v-else>
        <MetricSections
          v-if="chartSections.length && !kafkaDesk"
          :sections="chartSections"
          :points="points"
          :show-cards="conn.kindCode !== 'kafka'"
        />
        <RedisMonitor v-if="conn.kindCode === 'redis'" :panel-code="activePanel?.panelCode ?? ''" :detail="detail" />
        <KafkaMonitor v-else-if="conn.kindCode === 'kafka'" :panel-code="activePanel?.panelCode ?? ''" :detail="detail" />
        <MongoMonitor v-else-if="conn.kindCode === 'mongo'" :panel-code="activePanel?.panelCode ?? ''" :detail="detail" :component-id="conn.componentId" />
        <ZkMonitor v-else-if="conn.kindCode === 'zk'" :panel-code="activePanel?.panelCode ?? ''" :detail="detail" />
      </template>
    </div>
  </section>
</template>

<style scoped>
.fg-detail {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  background: var(--fg-panel-bg, var(--rs-surface));
}
.fg-desk-title {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  max-width: 100%;
}
.fg-desk-title strong {
  flex: none;
  font-weight: 650;
}
.fg-desk-meta {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 12px;
  font-weight: 400;
}
.fg-monitor-bar {
  display: inline-flex;
  flex: 0 0 auto;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px 12px;
}
.fg-range {
  width: 26.5rem;
  flex: 0 0 26.5rem;
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
.fg-interval {
  width: 72px;
  flex: 0 0 72px;
}
.fg-unit {
  color: var(--rs-text-secondary);
  font-size: 13px;
}
.fg-detail-bar {
  display: flex;
  flex: none;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface);
}
.fg-detail-id {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.fg-detail-id strong {
  font-size: 15px;
  font-weight: 650;
}
.fg-chip {
  flex: none;
  padding: 1px 8px;
  font-size: 12px;
  line-height: 18px;
  color: var(--rs-on-info-container, #08445e);
  background: var(--rs-info-container, #d7f2fb);
}
.fg-meta,
.fg-addr {
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
  white-space: nowrap;
}
.fg-addr {
  font-variant-numeric: tabular-nums;
}
.fg-panels {
  flex: none;
  padding: 0 16px;
  background: var(--rs-surface);
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-detail-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  padding: 12px 16px 24px;
  overflow: auto;
}
.fg-detail-body--desk {
  overflow: hidden;
  gap: 8px;
  padding-top: 8px;
  padding-bottom: 12px;
}
.fg-detail-body--desk > * {
  flex: none;
}
.fg-detail-body--desk .fg-kafka {
  flex: 1 1 auto;
  min-height: 0;
}
.fg-signals {
  display: flex;
  flex: none;
  align-items: stretch;
  min-width: 0;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface);
}
.fg-signal {
  display: inline-flex;
  flex: 1 1 0;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
  padding: 5px 10px;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
  white-space: nowrap;
}
.fg-signal:last-child {
  border-right: 0;
}
.fg-signal__name {
  overflow: hidden;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 12px;
  text-overflow: ellipsis;
}
.fg-signal strong {
  flex: none;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  font-weight: 650;
}
.fg-signal[data-level='alert'] strong {
  color: #e23b55;
}
.fg-signal[data-level='watch'] strong {
  color: #c49212;
}
.fg-finding,
.fg-note {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
}
.fg-finding {
  padding: 8px 12px;
  background: var(--rs-surface);
  box-shadow: inset 3px 0 0 var(--rs-text-secondary), inset 0 0 0 1px var(--rs-border);
}
.fg-finding[data-level='alert'] {
  color: var(--rs-on-danger-container, #8f1d30);
  background: var(--rs-danger-container, #ffe4e8);
  box-shadow: inset 3px 0 0 var(--rs-danger);
}
.fg-finding[data-level='watch'] {
  color: var(--rs-on-warning-container, #7a5608);
  background: var(--rs-warning-container, #fff4d6);
  box-shadow: inset 3px 0 0 var(--rs-warning);
}
.fg-note,
.fg-finding[data-level='ok'] {
  color: var(--rs-text-secondary, #4d647c);
}
</style>
