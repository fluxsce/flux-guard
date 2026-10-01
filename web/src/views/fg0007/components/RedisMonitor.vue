<script setup lang="ts">
import { computed } from 'vue'
import { RsCard, RsTable, RsTooltip, type RsTableColumn } from '@/ui'
import type { PointDetail, RedisDb, RedisSlow } from '../api'

const props = defineProps<{
  panelCode: string
  detail: PointDetail
}>()

function ms(usec: number) {
  if (usec >= 1000) return `${(usec / 1000).toFixed(1)} ms`
  return `${usec} μs`
}

function when(sec: number) {
  if (!Number.isFinite(sec) || sec <= 0) return '—'
  const date = new Date(sec * 1000)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function kindOf(command: string) {
  const word = command.trim().split(/\s+/, 1)[0]
  return word ? word.toUpperCase() : '—'
}

type SlowRow = RedisSlow & { id: string; atText: string; kind: string }

const slowRows = computed<SlowRow[]>(() =>
  (props.detail.slow ?? []).map((item, index) => ({
    ...item,
    id: String(index),
    atText: when(item.at),
    kind: kindOf(item.command),
  })),
)

const dbColumns: RsTableColumn<RedisDb>[] = [
  { key: 'name', title: '库', dataIndex: 'name', width: 88 },
  { key: 'keys', title: '键', dataIndex: 'keys', align: 'right' },
  { key: 'expires', title: '将过期', dataIndex: 'expires', align: 'right' },
]

const slowColumns: RsTableColumn<SlowRow>[] = [
  { key: 'atText', title: '时间', dataIndex: 'atText', width: 168 },
  { key: 'kind', title: '类型', dataIndex: 'kind', width: 96 },
  { key: 'usec', title: '耗时', dataIndex: 'usec', width: 96 },
  { key: 'command', title: '命令', dataIndex: 'command' },
]
</script>

<template>
  <RsCard v-if="panelCode === 'keys'" class="fg-monitor" title="各库键数量" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="按实例配置列出每个库。没有键的库也显示，数量是 0。集群只有 db0。" aria-label="各库键数量说明" />
    </template>
    <p v-if="!detail.dbs?.length" class="fg-note">还没有各库键数量。INFO 的 Keyspace 段为空时，这里也是空的。</p>
    <RsTable
      v-else
      :columns="dbColumns"
      :data="detail.dbs"
      row-key="name"
      size="sm"
      cell-focus="none"
      null-label="—"
      :rounded="false"
      :virtual="false"
      :context-menu="false"
    >
      <template #header-expires>
        <RsTooltip icon content="这个库里设了过期时间、还没到期的键。">将过期</RsTooltip>
      </template>
    </RsTable>
  </RsCard>
  <RsCard v-else-if="panelCode === 'slow'" class="fg-monitor" title="最近慢操作" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自 SLOWLOG，最多列出最近 128 条。耗时达到 100 毫秒会标红。" aria-label="最近慢操作说明" />
    </template>
    <p v-if="!slowRows.length" class="fg-note">慢操作日志是空的，或实例没有打开 SLOWLOG。</p>
    <RsTable
      v-else
      :columns="slowColumns"
      :data="slowRows"
      row-key="id"
      size="sm"
      cell-focus="none"
      null-label="—"
      :rounded="false"
      :virtual="false"
      :context-menu="false"
    >
      <template #header-kind>
        <RsTooltip icon content="命令名，例如 KEYS、HGETALL。">类型</RsTooltip>
      </template>
      <template #header-usec>
        <RsTooltip icon content="这条命令执行用了多久。">耗时</RsTooltip>
      </template>
      <template #usec="{ row }">
        <span :data-level="(row as SlowRow).usec >= 100000 ? 'alert' : 'watch'">{{ ms((row as SlowRow).usec) }}</span>
      </template>
      <template #command="{ row }">
        <span class="fg-command">{{ (row as SlowRow).command || '—' }}</span>
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
  padding: 12px 14px 14px;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
.fg-command {
  white-space: normal;
  word-break: break-all;
}
span[data-level='alert'] {
  color: #e23b55;
}
span[data-level='watch'] {
  color: #c49212;
}
</style>
