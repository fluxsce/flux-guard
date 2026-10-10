<script setup lang="ts">
import { computed } from 'vue'
import { RsCard, RsTable, RsTooltip, type RsTableColumn } from '@/ui'
import type { KafkaBrokerRow, KafkaGroupRow, KafkaPartitionRow, KafkaTopicRow, PointDetail } from '../api'

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

function topicsText(item: KafkaGroupRow) {
  if (!item.topic) return '—'
  if ((item.topicCount ?? 0) > 1) return `${item.topic} 等 ${item.topicCount} 个`
  return item.topic
}

function slowText(item: KafkaGroupRow) {
  if (item.maxLag == null || item.maxLag < 0) return '—'
  let where = item.topic || ''
  if (item.part != null && item.part >= 0) {
    where = item.topic ? `${item.topic}-${item.part}` : String(item.part)
  }
  if (!where) return String(item.maxLag)
  return `${where} · ${item.maxLag}`
}

function behindText(item: KafkaGroupRow) {
  if (item.behind == null || item.behind < 0) return '—'
  return String(item.behind)
}

function leaderText(id: number) {
  return id < 0 ? '—' : String(id)
}

function num(left: number | undefined, right: number | undefined) {
  return (left ?? -1) - (right ?? -1)
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
}

type PartRow = KafkaPartitionRow & {
  id: string
  leaderText: string
  isrText: string
}

type GroupRow = KafkaGroupRow & {
  stateLabel: string
  memberText: string
  lagLabel: string
  behindLabel: string
  slow: string
  topicsLabel: string
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

const topics = computed<TopicRow[]>(() =>
  (props.detail.topics ?? []).map((item) => ({
    ...item,
    label: topicName(item),
    replicaText: replicas(item),
    skewCount: item.skew || 0,
    belowCount: item.belowMin || 0,
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

const groups = computed<GroupRow[]>(() =>
  (props.detail.groups ?? []).map((item) => ({
    ...item,
    stateLabel: stateText(item.state),
    memberText: item.members == null ? '—' : String(item.members),
    lagLabel: lagText(item.lag),
    behindLabel: behindText(item),
    slow: slowText(item),
    topicsLabel: topicsText(item),
  })),
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
  { key: 'leaders', title: '负责分区', dataIndex: 'leaders', align: 'right', sortable: true, filterable: true },
  { key: 'replicas', title: '副本', dataIndex: 'replicas', align: 'right', sortable: true, filterable: true },
  {
    key: 'logText',
    title: '日志',
    dataIndex: 'logText',
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.logBytes, right.logBytes),
  },
]

const topicColumns: RsTableColumn<TopicRow>[] = [
  { key: 'label', title: '名称', dataIndex: 'label', filterable: true, sortable: true, ellipsis: true },
  { key: 'partitions', title: '分区', dataIndex: 'partitions', width: 72, align: 'right', sortable: true, filterable: true },
  {
    key: 'replicaText',
    title: '副本',
    dataIndex: 'replicaText',
    width: 88,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.replicas, right.replicas) || num(left.replicaMax, right.replicaMax),
  },
  { key: 'offline', title: '离线', dataIndex: 'offline', width: 72, align: 'right', sortable: true, filterable: true },
  { key: 'under', title: '未同步', dataIndex: 'under', width: 88, align: 'right', sortable: true, filterable: true },
  { key: 'skewCount', title: '首选偏离', dataIndex: 'skewCount', width: 96, align: 'right', sortable: true, filterable: true },
  { key: 'belowCount', title: '低于最小 ISR', dataIndex: 'belowCount', width: 120, align: 'right', sortable: true, filterable: true },
]

const partColumns: RsTableColumn<PartRow>[] = [
  { key: 'topic', title: '主题', dataIndex: 'topic', filterable: true, sortable: true, ellipsis: true },
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
  { key: 'problem', title: '问题', dataIndex: 'problem', width: 120, filterable: true, sortable: true },
]

const groupColumns: RsTableColumn<GroupRow>[] = [
  { key: 'name', title: '消费组', dataIndex: 'name', filterable: true, sortable: true, ellipsis: true },
  { key: 'stateLabel', title: '状态', dataIndex: 'stateLabel', width: 88, filterable: true, sortable: true },
  {
    key: 'memberText',
    title: '成员',
    dataIndex: 'memberText',
    width: 72,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.members, right.members),
  },
  {
    key: 'lagLabel',
    title: '延迟',
    dataIndex: 'lagLabel',
    width: 88,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.lag, right.lag),
  },
  {
    key: 'behindLabel',
    title: '落后分区',
    dataIndex: 'behindLabel',
    width: 96,
    align: 'right',
    filterable: true,
    sortable: true,
    sorter: (left, right) => num(left.behind, right.behind),
  },
  { key: 'slow', title: '最慢分区', dataIndex: 'slow', filterable: true, sortable: true },
  { key: 'topicsLabel', title: '主题', dataIndex: 'topicsLabel', filterable: true, sortable: true, ellipsis: true },
]
</script>

<template>
  <RsCard v-if="panelCode === 'broker'" class="fg-monitor" title="Broker" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自集群元数据。连通是从这里再拨一次该 Broker。负责分区是它当前担任 Leader 的数量。日志是这台机器上的日志目录占用。" aria-label="Broker 说明" />
    </template>
    <p v-if="detail.clusterId" class="fg-note">集群 {{ detail.clusterId }}</p>
    <p v-if="detail.brokerNote" class="fg-note">{{ detail.brokerNote }}</p>
    <p v-if="!brokers.length" class="fg-note">还没有 Broker。元数据里没有公告节点时，这里是空的。</p>
    <RsTable
      v-else
      :columns="brokerColumns"
      :data="brokers"
      row-key="id"
      size="sm"
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
  </RsCard>
  <RsCard v-else-if="panelCode === 'topic'" class="fg-monitor" title="Topic" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="健康的内部主题不列出。主题很多时，优先保留离线和未同步的。" aria-label="Topic 说明" />
    </template>
    <p v-if="detail.topicNote" class="fg-note">{{ detail.topicNote }}</p>
    <p v-if="!topics.length" class="fg-note">还没有 Topic。集群里没有主题时，这里是空的。</p>
    <RsTable
      v-else
      :columns="topicColumns"
      :data="topics"
      row-key="name"
      size="sm"
      cell-focus="none"
      null-label="—"
      :rounded="false"
      :virtual="false"
      :context-menu="false"
    >
      <template #header-replicaText>
        <RsTooltip icon content="分区配置的副本数。各分区不一样时显示范围。">副本</RsTooltip>
      </template>
      <template #header-offline>
        <RsTooltip icon content="没有可用 Leader 的分区数。">离线</RsTooltip>
      </template>
      <template #header-under>
        <RsTooltip icon content="副本没有跟上 Leader 的分区数。">未同步</RsTooltip>
      </template>
      <template #header-skewCount>
        <RsTooltip icon content="Leader 不在首选副本上的分区数。">首选偏离</RsTooltip>
      </template>
      <template #header-belowCount>
        <RsTooltip icon content="同步副本数低于 min.insync.replicas 的分区。主题有单独配置时按主题算。">低于最小 ISR</RsTooltip>
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
    </RsTable>
  </RsCard>
  <RsCard v-else-if="panelCode === 'partition'" class="fg-monitor" title="分区" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="只列出离线、未同步、低于最小 ISR 或首选副本偏离的分区。优先显示离线和未同步。" aria-label="分区说明" />
    </template>
    <p v-if="detail.partitionNote" class="fg-note">{{ detail.partitionNote }}</p>
    <p v-if="!partitions.length" class="fg-note">没有离线、未同步或首选偏离的分区。</p>
    <RsTable
      v-else
      :columns="partColumns"
      :data="partitions"
      row-key="id"
      size="sm"
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
  </RsCard>
  <RsCard v-else-if="panelCode === 'group'" class="fg-monitor" title="消费组" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="延迟是这个组各分区还没消费的条数合计。落后分区是延迟大于 0 的分区个数。最慢分区是其中落后最多的一个。" aria-label="消费组说明" />
    </template>
    <p v-if="detail.groupNote" class="fg-note">{{ detail.groupNote }}</p>
    <p v-if="!groups.length" class="fg-note">没有消费组，或协调者暂时读不到。</p>
    <RsTable
      v-else
      :columns="groupColumns"
      :data="groups"
      row-key="name"
      size="sm"
      cell-focus="none"
      null-label="—"
      :rounded="false"
      :virtual="false"
      :context-menu="false"
    >
      <template #header-lagLabel>
        <RsTooltip icon content="这个组还没消费的消息条数。">延迟</RsTooltip>
      </template>
      <template #header-behindLabel>
        <RsTooltip icon content="延迟大于 0 的分区个数。">落后分区</RsTooltip>
      </template>
      <template #stateLabel="{ row }">
        <span :data-level="stateLevel((row as GroupRow).state)">{{ (row as GroupRow).stateLabel }}</span>
      </template>
      <template #lagLabel="{ row }">
        <span :data-level="lagLevel((row as GroupRow).lag)">{{ (row as GroupRow).lagLabel }}</span>
      </template>
      <template #behindLabel="{ row }">
        <span :data-level="((row as GroupRow).behind ?? 0) > 0 ? 'watch' : undefined">{{ (row as GroupRow).behindLabel }}</span>
      </template>
    </RsTable>
  </RsCard>
</template>

<style scoped>
.fg-monitor {
  flex: none;
  align-self: stretch;
}
.fg-monitor :deep(.rs-table),
.fg-monitor :deep(.rs-table-shell) {
  height: auto;
  max-height: none;
  overflow: visible;
}
.fg-monitor :deep(.rs-table--content-visibility .rs-table__row) {
  content-visibility: visible;
}
.fg-note {
  margin: 0;
  padding: 12px 14px 0;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
.fg-note:last-child {
  padding-bottom: 14px;
}
span[data-level='alert'] {
  color: #e23b55;
}
span[data-level='watch'] {
  color: #c49212;
}
</style>
