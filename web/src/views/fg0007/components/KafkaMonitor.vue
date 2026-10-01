<script setup lang="ts">
import MetricSections from './MetricSections.vue'
import type { Board } from '../composables/boards'
import type { Point, PointDetail } from '../api'

defineProps<{
  board: Board
  points: Point[]
  detail: PointDetail
}>()

function lagText(lag: number) {
  return lag < 0 ? '—' : String(lag)
}
</script>

<template>
  <MetricSections :sections="board.sections" :points="points" />
  <p v-if="detail.groupNote" class="fg-note">{{ detail.groupNote }}</p>
  <section class="fg-section">
    <h3>Topic</h3>
    <p v-if="!detail.topics?.length" class="fg-note">还没有 Topic。集群里没有主题时，这里是空的。</p>
    <table v-else class="fg-table">
      <thead>
        <tr><th>名称</th><th>分区</th><th>离线</th><th>未同步</th></tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.topics" :key="item.name">
          <td>{{ item.name }}</td>
          <td>{{ item.partitions }}</td>
          <td :data-level="item.offline > 0 ? 'alert' : undefined">{{ item.offline }}</td>
          <td :data-level="item.under > 0 ? 'watch' : undefined">{{ item.under }}</td>
        </tr>
      </tbody>
    </table>
  </section>
  <section class="fg-section">
    <h3>消费组</h3>
    <p v-if="!detail.groups?.length" class="fg-note">没有消费组，或协调者暂时读不到。</p>
    <table v-else class="fg-table">
      <thead>
        <tr><th>消费组</th><th>延迟</th><th>主要 Topic</th></tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.groups" :key="item.name">
          <td>{{ item.name }}</td>
          <td :data-level="item.lag >= 10000 ? 'alert' : item.lag >= 1000 ? 'watch' : undefined">{{ lagText(item.lag) }}</td>
          <td>{{ item.topic || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </section>
</template>

<style scoped>
.fg-section h3 {
  margin: 12px 0 8px;
  font-size: 13px;
  font-weight: 600;
}
.fg-note {
  margin: 0;
  color: var(--rs-text-secondary, #4d647c);
}
.fg-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.fg-table th,
.fg-table td {
  padding: 6px 8px;
  text-align: left;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-table th {
  color: var(--rs-text-secondary, #4d647c);
  font-weight: 600;
}
.fg-table td[data-level='alert'] {
  color: #e23b55;
}
.fg-table td[data-level='watch'] {
  color: #c49212;
}
</style>
