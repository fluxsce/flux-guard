<script setup lang="ts">
import { computed } from 'vue'
import { RsCard, RsEmpty, RsTable, type RsTableColumn } from '@/ui'
import MetricChart from './MetricChart.vue'
import MetricSections from './MetricSections.vue'
import type { Board } from '../composables/boards'
import { seriesOption } from '../composables/charts'
import type { Point, PointDetail, RedisDb, RedisSlow } from '../api'

const props = defineProps<{
  board: Board
  points: Point[]
  detail: PointDetail
}>()

function ms(usec: number) {
  if (usec >= 1000) return `${(usec / 1000).toFixed(1)} ms`
  return `${usec} μs`
}

const slowRows = computed(() => (props.detail.slow ?? []).map((item, index) => ({ ...item, id: String(index) })))

const trends = computed(() =>
  props.board.sections
    .filter((section) => section.metrics.length > 0)
    .map((section) => ({ title: section.title, option: seriesOption(section.metrics, props.points) })),
)

const dbColumns: RsTableColumn<RedisDb>[] = [
  { key: 'name', title: '库', dataIndex: 'name', width: 88 },
  { key: 'keys', title: '键', dataIndex: 'keys', align: 'right' },
  { key: 'expires', title: '将过期', dataIndex: 'expires', align: 'right' },
]

const slowColumns: RsTableColumn<RedisSlow & { id: string }>[] = [
  { key: 'usec', title: '耗时', dataIndex: 'usec', width: 96 },
  { key: 'command', title: '命令', dataIndex: 'command', ellipsis: true },
]

function tableHeight(rows: number) {
  return Math.min(280, 36 + Math.max(rows, 1) * 34 + 2)
}
</script>

<template>
  <div class="fg-redis">
    <MetricSections :sections="board.sections" :points="points" :charts="false" />
    <h3 class="fg-section">趋势</h3>
    <RsEmpty v-if="trends.length === 0 || !points.length" title="还没有趋势数据" />
    <section v-else class="fg-charts">
      <article v-for="chart in trends" :key="chart.title" class="fg-chart-panel">
        <h4>{{ chart.title }}</h4>
        <MetricChart :option="chart.option" height="240px" />
      </article>
    </section>
    <div class="fg-tables">
    <RsCard title="各库键数量" size="sm" :padding="false">
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
        :context-menu="false"
        :height="tableHeight(detail.dbs.length)"
      />
    </RsCard>
    <RsCard title="最近慢操作" size="sm" :padding="false">
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
        :context-menu="false"
        :height="tableHeight(slowRows.length)"
      >
        <template #usec="{ row }">
          <span :data-level="(row as RedisSlow).usec >= 100000 ? 'alert' : 'watch'">{{ ms((row as RedisSlow).usec) }}</span>
        </template>
      </RsTable>
    </RsCard>
    </div>
  </div>
</template>

<style scoped>
.fg-redis {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.fg-section {
  margin: 6px 0 0;
  font-size: 13px;
  font-weight: 600;
}
.fg-charts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.fg-chart-panel {
  min-width: 0;
  padding: 8px 10px 4px;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface-elevated, var(--rs-surface));
  box-shadow: var(--fg-panel-shadow);
}
.fg-chart-panel h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--rs-text-secondary);
}
.fg-tables {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  align-items: start;
}
.fg-note {
  margin: 0;
  padding: 12px 14px 14px;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
span[data-level='alert'] {
  color: #e23b55;
}
span[data-level='watch'] {
  color: #c49212;
}
@media (max-width: 960px) {
  .fg-charts,
  .fg-tables {
    grid-template-columns: 1fr;
  }
}
</style>
