<script setup lang="ts">
import { computed } from 'vue'
import { RsTable, RsTooltip, type RsTableColumn } from '@/ui'
import type { KafkaBrokerRow, KafkaGroupRow, KafkaOffsetRow, KafkaPartitionRow, KafkaTopicRow, PointDetail } from '../api'

const props = defineProps<{
  panelCode: string
  detail: PointDetail
}>()

function lagText(lag: number) {
  return lag < 0 ? '—' : String(lag)
}

function bytes(value: number | undefined) {
  if (value == null || value < 0) return '—'
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${Math.round(value)} B`
}

function replicas(item: KafkaTopicRow) {
  if (!item.replicas) return '—'
  if (item.replicaMax && item.replicaMax !== item.replicas) return `${item.replicas}–${item.replicaMax}`
  return String(item.replicas)
}

function topicName(item: KafkaTopicRow) {
  return item.internal ? `${item.name} · 内部` : item.name
}

const states: Record<string, string> = {
  Stable: '稳定',
  Empty: '空闲',
  Dead: '失效',
  PreparingRebalance: '再均衡',
  CompletingRebalance: '再均衡',
}

function stateText(state?: string) {
  if (!state) return '—'
  return states[state] || state
}

function leaderText(id: number) {
  return id < 0 ? '—' : String(id)
}

function num(left: number | undefined, right: number | undefined) {
  return (left ?? -1) - (right ?? -1)
}

function hostText(host?: string) {
  if (!host) return ''
  return host.startsWith('/') ? host.slice(1) : host
}

type BrokerRow = KafkaBrokerRow & {
  address: string
  role: string
  link: string
  logText: string
}

type TopicRow = KafkaTopicRow & {
  label: string
  replicaText: string
  skewCount: number
  belowCount: number
  faults: string
}

type PartRow = KafkaPartitionRow & {
  id: string
  leaderText: string
  isrText: string
}

type ConsumeRow = {
  id: string
  group: string
  state?: string
  stateLabel: string
  topic: string
  partText: string
  currentText: string
  endText: string
  lag: number
  lagLabel: string
  consumerId: string
  host: string
  clientId: string
}

const brokers = computed<BrokerRow[]>(() =>
  (props.detail.brokers ?? []).map((item) => ({
    ...item,
    leaders: item.leaders ?? 0,
    replicas: item.replicas ?? 0,
    address: `${item.host}:${item.port}`,
    role: item.controller ? '控制器' : 'Broker',
    link: item.up ? '可达' : '不可达',
    logText: bytes(item.logBytes),
  })),
)

const partitions = computed<PartRow[]>(() =>
  (props.detail.partitions ?? []).map((item) => ({
    ...item,
    id: `${item.topic}-${item.part}`,
    leaderText: leaderText(item.leader),
    isrText: `${item.isr}/${item.replicas}`,
  })),
)

function faultText(name: string) {
  const rows = partitions.value.filter((row) => row.topic === name)
  if (!rows.length) return '—'
  const shown = rows.slice(0, 4).map((row) => `${row.part} ${row.problem}`)
  if (rows.length > shown.length) return `${shown.join('，')} 等 ${rows.length} 个`
  return shown.join('，')
}

const topics = computed<TopicRow[]>(() =>
  (props.detail.topics ?? []).map((item) => ({
    ...item,
    label: topicName(item),
    replicaText: replicas(item),
    skewCount: item.skew || 0,
    belowCount: item.belowMin || 0,
    faults: faultText(item.name),
  })),
)

function offsetText(value: number | undefined) {
  if (value == null || value < 0) return '—'
  return String(value)
}

function offsetsOf(item: KafkaGroupRow): KafkaOffsetRow[] {
  if (item.offsets?.length) return item.offsets
  if (item.lags?.length) {
    return item.lags.map((row) => ({ topic: row.topic, part: row.part, lag: row.lag }))
  }
  if (item.topic) {
    return [{ topic: item.topic, part: item.part ?? -1, lag: item.maxLag ?? item.lag }]
  }
  return []
}

function consumeRow(item: KafkaGroupRow, row: KafkaOffsetRow): ConsumeRow {
  const part = row.part ?? -1
  return {
    id: `${item.name}\u0000${row.topic}\u0000${part}`,
    group: item.name,
    state: item.state,
    stateLabel: stateText(item.state),
    topic: row.topic || '—',
    partText: part < 0 ? '—' : String(part),
    currentText: offsetText(row.current),
    endText: offsetText(row.end),
    lag: row.lag,
    lagLabel: lagText(row.lag),
    consumerId: row.consumerId || '—',
    host: hostText(row.host) || '—',
    clientId: row.clientId || '—',
  }
}

const linkNote = computed(() => {
  const notes = (props.detail.groups ?? [])
    .flatMap((item) => [item.offsetNote, item.topicLinkNote])
    .filter((item): item is string => Boolean(item))
  return [...new Set(notes)].join(' ')
})

const consumes = computed<ConsumeRow[]>(() =>
  (props.detail.groups ?? []).flatMap((item) => {
    const rows = offsetsOf(item)
    if (!rows.length) {
      return [consumeRow(item, { topic: '—', part: -1, lag: item.lag })]
    }
    return rows.map((row) => consumeRow(item, row))
  }),
)

function leaderLevel(item: BrokerRow) {
  const rows = brokers.value
  if (rows.length < 2) return undefined
  const counts = rows.map((row) => row.leaders ?? 0)
  const sum = counts.reduce((total, value) => total + value, 0)
  if (sum <= 0) return undefined
  const leaders = item.leaders ?? 0
  if (leaders === Math.max(...counts) && leaders > (sum / rows.length) * 1.5) return 'watch'
  return undefined
}

function problemLevel(problem: string) {
  if (problem === '离线' || problem === '低于最小 ISR') return 'alert'
  if (problem) return 'watch'
  return undefined
}

function faultLevel(text: string) {
  if (text.includes('离线') || text.includes('低于最小 ISR')) return 'alert'
  if (text !== '—') return 'watch'
  return undefined
}

function stateLevel(state?: string) {
  if (state === 'PreparingRebalance' || state === 'CompletingRebalance' || state === 'Dead') return 'watch'
  return undefined
}

function lagLevel(lag: number) {
  if (lag >= 10000) return 'alert'
  if (lag >= 1000) return 'watch'
  return undefined
}

const brokerColumns: RsTableColumn<BrokerRow>[] = [
  { key: 'id', title: '编号', dataIndex: 'id', width: 72, align: 'right', sortable: true, filterable: true },
  { key: 'address', title: '地址', dataIndex: 'address', filterable: true, sortable: true },
  { key: 'role', title: '角色', dataIndex: 'role', width: 96, filterable: true, sortable: true },
  { key: 'link', title: '连通', dataIndex: 'link', width: 88, filterable: true, sortable: true },
  { key: 'leaders', title: '负责分区', dataIndex: 'leaders', width: 96, align: 'right', sortable: true, filterable: true },
  { key: 'replicas', title: '副本', dataIndex: 'replicas', width: 80, align: 'right', sortable: true, filterable: true },
  {
    key: 'logText',
    title: '日志',
    dataIndex: 'logText',
    minWidth: 96,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.logBytes, right.logBytes),
  },
]

const topicColumns: RsTableColumn<TopicRow>[] = [
  { key: 'label', title: '主题', dataIndex: 'label', width: 220, filterable: true, sortable: true, ellipsis: true },
  { key: 'partitions', title: '分区', dataIndex: 'partitions', width: 72, align: 'right', sortable: true, filterable: true },
  {
    key: 'replicaText',
    title: '副本因子',
    dataIndex: 'replicaText',
    width: 88,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.replicas, right.replicas) || num(left.replicaMax, right.replicaMax),
  },
  { key: 'offline', title: '离线分区', dataIndex: 'offline', width: 88, align: 'right', sortable: true, filterable: true },
  { key: 'under', title: '未同步', dataIndex: 'under', width: 80, align: 'right', sortable: true, filterable: true },
  { key: 'skewCount', title: '首选偏离', dataIndex: 'skewCount', width: 96, align: 'right', sortable: true, filterable: true },
  { key: 'belowCount', title: '低于最小 ISR', dataIndex: 'belowCount', width: 120, align: 'right', sortable: true, filterable: true },
  { key: 'faults', title: '异常分区', dataIndex: 'faults', minWidth: 160, filterable: true, sortable: true, ellipsis: true },
]

const partColumns: RsTableColumn<PartRow>[] = [
  { key: 'topic', title: '主题', dataIndex: 'topic', width: 220, filterable: true, sortable: true, ellipsis: true },
  { key: 'part', title: '分区', dataIndex: 'part', width: 72, align: 'right', sortable: true, filterable: true },
  {
    key: 'leaderText',
    title: 'Leader',
    dataIndex: 'leaderText',
    width: 88,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.leader, right.leader),
  },
  { key: 'isrText', title: 'ISR', dataIndex: 'isrText', width: 88, align: 'right', filterable: true, sortable: true },
  { key: 'problem', title: '问题', dataIndex: 'problem', minWidth: 120, filterable: true, sortable: true },
]

const consumeColumns: RsTableColumn<ConsumeRow>[] = [
  { key: 'group', title: '消费组', dataIndex: 'group', width: 120, filterable: true, sortable: true, ellipsis: true },
  { key: 'stateLabel', title: '状态', dataIndex: 'stateLabel', width: 80, filterable: true, sortable: true },
  { key: 'topic', title: '主题', dataIndex: 'topic', width: 180, filterable: true, sortable: true, ellipsis: true },
  { key: 'partText', title: '分区', dataIndex: 'partText', width: 64, align: 'right', filterable: true, sortable: true },
  { key: 'currentText', title: '当前位点', dataIndex: 'currentText', width: 120, align: 'right', filterable: true, sortable: true },
  { key: 'endText', title: '日志末端', dataIndex: 'endText', width: 120, align: 'right', filterable: true, sortable: true },
  {
    key: 'lagLabel',
    title: '延迟',
    dataIndex: 'lagLabel',
    width: 72,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.lag, right.lag),
  },
  { key: 'consumerId', title: '消费者', dataIndex: 'consumerId', width: 220, filterable: true, sortable: true, ellipsis: true },
  { key: 'host', title: '主机', dataIndex: 'host', width: 140, filterable: true, sortable: true, ellipsis: true },
  { key: 'clientId', title: '客户端', dataIndex: 'clientId', minWidth: 140, filterable: true, sortable: true, ellipsis: true },
]
</script>

<template>
  <div v-if="panelCode === 'broker'" class="fg-kafka">
    <p v-if="detail.clusterId" class="fg-note">集群 {{ detail.clusterId }}</p>
    <p v-if="detail.brokerNote" class="fg-note">{{ detail.brokerNote }}</p>
    <p v-if="!brokers.length" class="fg-note">还没有 Broker。元数据里没有公告节点时，这里是空的。</p>
    <div v-else class="fg-table">
      <RsTable
        :columns="brokerColumns"
        :data="brokers"
        row-key="id"
        size="sm"
        fill
        resizable
        cell-focus="none"
        null-label="—"
        :rounded="false"
        :virtual="false"
        :context-menu="false"
      >
        <template #header-leaders>
          <RsTooltip icon content="这台 Broker 当前担任 Leader 的分区数。明显多于其它节点时标出来。">负责分区</RsTooltip>
        </template>
        <template #header-replicas>
          <RsTooltip icon content="副本落在这台 Broker 上的分区数，含它担任 Leader 的分区。">副本</RsTooltip>
        </template>
        <template #link="{ row }">
          <span :data-level="(row as BrokerRow).up ? undefined : 'alert'">{{ (row as BrokerRow).link }}</span>
        </template>
        <template #leaders="{ row }">
          <span :data-level="leaderLevel(row as BrokerRow)">{{ (row as BrokerRow).leaders ?? 0 }}</span>
        </template>
      </RsTable>
    </div>
  </div>
  <div v-else-if="panelCode === 'topic'" class="fg-kafka">
    <p v-if="detail.topicNote" class="fg-note">{{ detail.topicNote }}</p>
    <p v-if="detail.partitionNote" class="fg-note">{{ detail.partitionNote }}</p>
    <p v-if="!topics.length" class="fg-note">还没有 Topic。集群里没有主题时，这里是空的。</p>
    <div v-else class="fg-table">
      <RsTable
        :columns="topicColumns"
        :data="topics"
        row-key="name"
        size="sm"
        fill
        resizable
        cell-focus="none"
        null-label="—"
        :rounded="false"
        :virtual="false"
        :context-menu="false"
      >
        <template #header-partitions>
          <RsTooltip icon content="这个主题的分区个数。">分区</RsTooltip>
        </template>
        <template #header-replicaText>
          <RsTooltip icon content="分区配置的副本因子。各分区不一样时显示范围。">副本因子</RsTooltip>
        </template>
        <template #header-offline>
          <RsTooltip icon content="没有可用 Leader 的分区数。">离线分区</RsTooltip>
        </template>
        <template #header-under>
          <RsTooltip icon content="同步副本数小于副本因子的分区数。">未同步</RsTooltip>
        </template>
        <template #header-skewCount>
          <RsTooltip icon content="Leader 不在首选副本上的分区数。">首选偏离</RsTooltip>
        </template>
        <template #header-belowCount>
          <RsTooltip icon content="同步副本数低于 min.insync.replicas 的分区。主题有单独配置时按主题算。">低于最小 ISR</RsTooltip>
        </template>
        <template #header-faults>
          <RsTooltip icon content="离线、未同步、低于最小 ISR 或首选副本偏离的分区号。">异常分区</RsTooltip>
        </template>
        <template #offline="{ row }">
          <span :data-level="(row as TopicRow).offline > 0 ? 'alert' : undefined">{{ (row as TopicRow).offline }}</span>
        </template>
        <template #under="{ row }">
          <span :data-level="(row as TopicRow).under > 0 ? 'watch' : undefined">{{ (row as TopicRow).under }}</span>
        </template>
        <template #skewCount="{ row }">
          <span :data-level="(row as TopicRow).skewCount > 0 ? 'watch' : undefined">{{ (row as TopicRow).skewCount }}</span>
        </template>
        <template #belowCount="{ row }">
          <span :data-level="(row as TopicRow).belowCount > 0 ? 'alert' : undefined">{{ (row as TopicRow).belowCount }}</span>
        </template>
        <template #faults="{ row }">
          <span :data-level="faultLevel((row as TopicRow).faults)">{{ (row as TopicRow).faults }}</span>
        </template>
      </RsTable>
    </div>
  </div>
  <div v-else-if="panelCode === 'group'" class="fg-kafka">
    <p v-if="detail.groupNote" class="fg-note">{{ detail.groupNote }}</p>
    <p v-if="linkNote" class="fg-note">{{ linkNote }}</p>
    <p v-if="!consumes.length" class="fg-note">没有消费组，或协调者暂时读不到。</p>
    <div v-else class="fg-table">
      <RsTable
        :columns="consumeColumns"
        :data="consumes"
        row-key="id"
        size="sm"
        fill
        resizable
        cell-focus="none"
        null-label="—"
        :rounded="false"
        :virtual="false"
        :context-menu="false"
      >
        <template #header-currentText>
          <RsTooltip icon content="这个组在该分区已提交的 offset。">当前位点</RsTooltip>
        </template>
        <template #header-endText>
          <RsTooltip icon content="该分区当前的最新 offset。">日志末端</RsTooltip>
        </template>
        <template #header-lagLabel>
          <RsTooltip icon content="日志末端减去当前位点。位点对齐时是 0。">延迟</RsTooltip>
        </template>
        <template #header-consumerId>
          <RsTooltip icon content="分配到这个分区的成员编号。">消费者</RsTooltip>
        </template>
        <template #header-host>
          <RsTooltip icon content="该成员连上来的地址。">主机</RsTooltip>
        </template>
        <template #header-clientId>
          <RsTooltip icon content="该成员的 client.id。">客户端</RsTooltip>
        </template>
        <template #stateLabel="{ row }">
          <span :data-level="stateLevel((row as ConsumeRow).state)">{{ (row as ConsumeRow).stateLabel }}</span>
        </template>
        <template #lagLabel="{ row }">
          <span :data-level="lagLevel((row as ConsumeRow).lag)">{{ (row as ConsumeRow).lagLabel }}</span>
        </template>
      </RsTable>
    </div>
  </div>
  <div v-else-if="panelCode === 'partition'" class="fg-kafka">
    <p v-if="detail.partitionNote" class="fg-note">{{ detail.partitionNote }}</p>
    <p v-if="!partitions.length" class="fg-note">没有离线、未同步或首选偏离的分区。</p>
    <div v-else class="fg-table">
      <RsTable
        :columns="partColumns"
        :data="partitions"
        row-key="id"
        size="sm"
        fill
        resizable
        cell-focus="none"
        null-label="—"
        :rounded="false"
        :virtual="false"
        :context-menu="false"
      >
        <template #header-isrText>
          <RsTooltip icon content="同步副本数 / 配置副本数。">ISR</RsTooltip>
        </template>
        <template #problem="{ row }">
          <span :data-level="problemLevel((row as PartRow).problem)">{{ (row as PartRow).problem }}</span>
        </template>
      </RsTable>
    </div>
  </div>
</template>

<style scoped>
.fg-kafka {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
  min-height: 0;
  height: 100%;
}
.fg-table {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  width: 100%;
  min-width: 0;
  min-height: 0;
}
.fg-note {
  margin: 0;
  padding: 0 0 8px;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
span[data-level='alert'] {
  color: #e23b55;
}
span[data-level='watch'] {
  color: #c49212;
}
</style>
