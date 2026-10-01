<script setup lang="ts">
import { RsCard, RsTooltip } from '@/ui'
import type { KafkaGroupRow, KafkaTopicRow, PointDetail } from '../api'

defineProps<{
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
  const where = item.part != null && item.part >= 0
    ? (item.topic ? `${item.topic}-${item.part}` : String(item.part))
    : item.topic
  return where ? `${where} · ${item.maxLag}` : String(item.maxLag)
}
</script>

<template>
  <RsCard v-if="panelCode === 'broker'" class="fg-monitor" title="Broker" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自集群元数据。连通是从这里再拨一次该 Broker。日志是这台机器上的日志目录占用。" aria-label="Broker 说明" />
    </template>
    <p v-if="detail.clusterId" class="fg-note">集群 {{ detail.clusterId }}</p>
    <p v-if="detail.brokerNote" class="fg-note">{{ detail.brokerNote }}</p>
    <p v-if="!detail.brokers?.length" class="fg-note">还没有 Broker。元数据里没有公告节点时，这里是空的。</p>
    <table v-else class="fg-table">
      <thead>
        <tr>
          <th>编号</th>
          <th>地址</th>
          <th>角色</th>
          <th>连通</th>
          <th class="fg-num">日志</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.brokers" :key="item.id">
          <td>{{ item.id }}</td>
          <td>{{ item.host }}:{{ item.port }}</td>
          <td>{{ item.controller ? '控制器' : 'Broker' }}</td>
          <td :data-level="item.up ? undefined : 'alert'">{{ item.up ? '可达' : '不可达' }}</td>
          <td class="fg-num">{{ bytes(item.logBytes) }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
  <RsCard v-else-if="panelCode === 'topic'" class="fg-monitor" title="Topic" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="健康的内部主题不列出。主题很多时，优先保留离线和未同步的。" aria-label="Topic 说明" />
    </template>
    <p v-if="detail.topicNote" class="fg-note">{{ detail.topicNote }}</p>
    <p v-if="!detail.topics?.length" class="fg-note">还没有 Topic。集群里没有主题时，这里是空的。</p>
    <table v-else class="fg-table">
      <thead>
        <tr>
          <th>名称</th>
          <th class="fg-num">分区</th>
          <th class="fg-num">
            <RsTooltip icon content="分区配置的副本数。各分区不一样时显示范围。">副本</RsTooltip>
          </th>
          <th class="fg-num">
            <RsTooltip icon content="没有可用 Leader 的分区数。">离线</RsTooltip>
          </th>
          <th class="fg-num">
            <RsTooltip icon content="副本没有跟上 Leader 的分区数。">未同步</RsTooltip>
          </th>
          <th class="fg-num">
            <RsTooltip icon content="Leader 不在首选副本上的分区数。">首选偏离</RsTooltip>
          </th>
          <th class="fg-num">
            <RsTooltip icon content="同步副本数低于 min.insync.replicas 的分区。主题有单独配置时按主题算。">低于最小 ISR</RsTooltip>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.topics" :key="item.name">
          <td>{{ topicName(item) }}</td>
          <td class="fg-num">{{ item.partitions }}</td>
          <td class="fg-num">{{ replicas(item) }}</td>
          <td class="fg-num" :data-level="item.offline > 0 ? 'alert' : undefined">{{ item.offline }}</td>
          <td class="fg-num" :data-level="item.under > 0 ? 'watch' : undefined">{{ item.under }}</td>
          <td class="fg-num" :data-level="item.skew ? 'watch' : undefined">{{ item.skew || 0 }}</td>
          <td class="fg-num" :data-level="item.belowMin ? 'alert' : undefined">{{ item.belowMin || 0 }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
  <RsCard v-else-if="panelCode === 'group'" class="fg-monitor" title="消费组" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="延迟是这个组各分区还没消费的条数合计。最慢分区是其中落后最多的一个。" aria-label="消费组说明" />
    </template>
    <p v-if="detail.groupNote" class="fg-note">{{ detail.groupNote }}</p>
    <p v-if="!detail.groups?.length" class="fg-note">没有消费组，或协调者暂时读不到。</p>
    <table v-else class="fg-table">
      <thead>
        <tr>
          <th>消费组</th>
          <th>状态</th>
          <th class="fg-num">成员</th>
          <th class="fg-num">
            <RsTooltip icon content="这个组还没消费的消息条数。">延迟</RsTooltip>
          </th>
          <th>最慢分区</th>
          <th>主题</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.groups" :key="item.name">
          <td>{{ item.name }}</td>
          <td :data-level="item.state === 'PreparingRebalance' || item.state === 'CompletingRebalance' || item.state === 'Dead' ? 'watch' : undefined">{{ stateText(item.state) }}</td>
          <td class="fg-num">{{ item.members ?? '—' }}</td>
          <td class="fg-num" :data-level="item.lag >= 10000 ? 'alert' : item.lag >= 1000 ? 'watch' : undefined">{{ lagText(item.lag) }}</td>
          <td>{{ slowText(item) }}</td>
          <td>{{ topicsText(item) }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
</template>

<style scoped>
.fg-monitor {
  flex: none;
  align-self: stretch;
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
.fg-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.fg-table th,
.fg-table td {
  padding: 6px 10px;
  text-align: left;
  vertical-align: top;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-table th {
  color: var(--rs-text-secondary, #4d647c);
  font-weight: 600;
}
.fg-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.fg-num :deep(.rs-tooltip--icon) {
  justify-content: flex-end;
  width: 100%;
}
td[data-level='alert'] {
  color: #e23b55;
}
td[data-level='watch'] {
  color: #c49212;
}
</style>
