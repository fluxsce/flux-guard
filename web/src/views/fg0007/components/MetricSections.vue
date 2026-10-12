<script setup lang="ts">
import { RsStatCard, RsTooltip } from '@/ui'
import MetricChart from './MetricChart.vue'
import { seriesOption } from '../composables/charts'
import type { BoardCard, BoardSection } from '../composables/boards'
import type { Point } from '../api'

const props = withDefaults(
  defineProps<{
    sections: BoardSection[]
    points: Point[]
    charts?: boolean
    showCards?: boolean
    chartHeight?: string
  }>(),
  { charts: true, showCards: true, chartHeight: '200px' },
)

function accent(level: BoardCard['level']) {
  if (level === 'alert') return 'danger'
  if (level === 'watch') return 'warning'
  return 'primary'
}

function span(section: BoardSection) {
  if (section.metrics.length === 0) return 'strip'
  if (section.metrics.length >= 3) return 'wide'
  return 'half'
}
</script>

<template>
  <div class="fg-sections">
    <section v-for="section in sections" :key="section.title" class="fg-panel" :data-span="span(section)">
      <h3>{{ section.title }}</h3>
      <div v-if="props.showCards" class="fg-cards">
        <RsStatCard
          v-for="item in section.cards"
          :key="item.label"
          size="sm"
          :label="item.label"
          :value="item.value"
          :accent="accent(item.level)"
          :aria-label="item.help ? `${item.label}。${item.help}` : undefined"
        >
          <template v-if="item.help" #icon>
            <RsTooltip class="fg-help" icon :content="item.help" aria-label="指标说明" />
          </template>
        </RsStatCard>
      </div>
      <MetricChart v-if="charts && section.metrics.length && points.length" :option="seriesOption(section.metrics, points)" :height="chartHeight" />
    </section>
  </div>
</template>

<style scoped>
.fg-sections {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  align-items: start;
}
.fg-panel {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px 10px;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  background: var(--rs-surface-elevated, var(--rs-surface));
  box-shadow: var(--fg-panel-shadow);
}
.fg-panel[data-span='strip'],
.fg-panel[data-span='wide'] {
  grid-column: 1 / -1;
}
.fg-panel[data-span='strip'] {
  padding: 0;
  border: 0;
  background: transparent;
  box-shadow: none;
}
.fg-panel h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}
.fg-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(148px, 1fr));
  gap: 8px;
}
.fg-help {
  color: var(--rs-text-secondary, #4d647c);
}
@media (max-width: 960px) {
  .fg-sections {
    grid-template-columns: 1fr;
  }
  .fg-panel[data-span='half'] {
    grid-column: 1 / -1;
  }
}
</style>
