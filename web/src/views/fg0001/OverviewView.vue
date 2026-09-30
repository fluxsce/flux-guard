<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsCard, RsEmpty, RsIcon, RsStatCard, RsTable, RsTag, useRsToast } from '@/ui'
import { queryAgents, type Agent } from '@/views/fg0002/api'
import { queryOverview, type Overview } from './api'

const moduleId = 'fg0001'

const data = ref<Overview | null>(null)
const agents = ref<Agent[]>([])
const toast = useRsToast()
const error = ref('')
const refreshing = ref(false)

const offlineCount = computed(() => {
  if (!data.value) return null
  return Math.max(0, data.value.agentCount - data.value.onlineCount)
})

const received = computed(() => data.value?.lastReceivedTime || '—')

const onlineRatio = computed(() => {
  if (!data.value || data.value.agentCount <= 0) return 0
  return Math.round((data.value.onlineCount / data.value.agentCount) * 100)
})

const health = computed(() => {
  if (!data.value || data.value.agentCount <= 0) return 'idle'
  if (data.value.onlineCount <= 0) return 'down'
  if (data.value.onlineCount < data.value.agentCount) return 'warn'
  return 'live'
})

const healthLabel = computed(() => {
  if (health.value === 'live') return '链路正常'
  if (health.value === 'warn') return '部分离线'
  if (health.value === 'down') return '全部离线'
  return '等待接入'
})

const healthDetail = computed(() => {
  if (!data.value || data.value.agentCount <= 0) return '未接入'
  if (health.value === 'live') return '在线'
  if (health.value === 'down') return '离线'
  return `离线 ${offlineCount.value}`
})

const agentColumns = [
  { key: 'agentName', title: '名称', dataIndex: 'agentName', ellipsis: true },
  { key: 'machineId', title: '机器编号', dataIndex: 'machineId', ellipsis: true },
  { key: 'hostName', title: '主机', dataIndex: 'hostName', ellipsis: true },
  { key: 'ipAddress', title: '地址', dataIndex: 'ipAddress' },
  { key: 'onlineFlag', title: '状态', dataIndex: 'onlineFlag', width: 88 },
  { key: 'lastSeenTime', title: '最近心跳', dataIndex: 'lastSeenTime' },
]

async function load() {
  error.value = ''
  refreshing.value = true
  try {
    const [overview, agentResult] = await Promise.all([
      queryOverview(),
      queryAgents(),
    ])
    if (!isApiSuccess(overview)) {
      error.value = getApiMessage(overview, '加载失败')
      toast.error(error.value)
      return
    }
    data.value = parseApiData<Overview>(overview) ?? null
    agents.value = isApiSuccess(agentResult) ? (parseApiData<Agent[]>(agentResult) ?? []) : []
  } catch (err) {
    error.value = getHttpErrorMessage(err, '加载失败')
    toast.error(error.value)
  } finally {
    refreshing.value = false
  }
}

onMounted(load)

useModuleAi(() => ({
  moduleId,
  title: '总览',
  snapshot: () => ({
    health: health.value,
    healthLabel: healthLabel.value,
    onlineRatio: onlineRatio.value,
    agentCount: data.value?.agentCount ?? 0,
    onlineCount: data.value?.onlineCount ?? 0,
    pluginCount: data.value?.pluginCount ?? 0,
    eventCount: data.value?.eventCount ?? 0,
    lastReceivedTime: data.value?.lastReceivedTime ?? '',
    error: error.value,
    agents: agents.value.slice(0, 20).map((agent) => ({
      agentId: agent.agentId,
      agentName: agent.agentName,
      hostName: agent.hostName,
      onlineFlag: agent.onlineFlag,
    })),
  }),
}))
</script>

<template>
  <div class="fg-page fg-dash" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <section class="fg-status" :data-state="health">
      <span class="fg-status-dot" aria-hidden="true" />
      <div class="fg-status-copy">
        <strong>{{ healthLabel }}</strong>
        <span>{{ healthDetail }}</span>
      </div>
      <div class="fg-ratio">
        <div class="fg-ratio-track" aria-hidden="true">
          <span :style="{ width: `${onlineRatio}%` }" />
        </div>
        <span>在线 {{ data ? `${data.onlineCount}/${data.agentCount}` : '—' }} · {{ onlineRatio }}%</span>
      </div>
      <div class="fg-received">
        <span>最近接收</span>
        <strong>{{ received }}</strong>
      </div>
      <RsButton variant="secondary" size="sm" icon="refresh-cw" data-fg-ai-action="refresh" data-fg-ai-risk="read" :disabled="refreshing" @click="load">刷新</RsButton>
    </section>

    <section class="fg-dash-kpis">
      <RsStatCard label="采集代理" :value="data?.agentCount ?? '—'" description="已登记" accent="primary" size="lg">
        <template #icon><RsIcon name="server" :size="18" /></template>
      </RsStatCard>
      <RsStatCard
        label="在线代理"
        :value="data?.onlineCount ?? '—'"
        :description="offlineCount == null ? '心跳状态' : `离线 ${offlineCount}`"
        accent="success"
        size="lg"
      >
        <template #icon><RsIcon name="heart-pulse" :size="18" /></template>
      </RsStatCard>
      <RsStatCard label="插件" :value="data?.pluginCount ?? '—'" description="已加载" accent="info" size="lg">
        <template #icon><RsIcon name="puzzle" :size="18" /></template>
      </RsStatCard>
      <RsStatCard label="事件" :value="data?.eventCount ?? '—'" description="已接收" size="lg">
        <template #icon><RsIcon name="activity" :size="18" /></template>
      </RsStatCard>
    </section>

    <section class="fg-dash-panels">
      <RsCard class="fg-panel" title="采集代理" :description="`最近接收 ${received}`" fill :padding="false">
        <template #actions>
          <RsButton variant="secondary" size="sm" :disabled="refreshing" @click="load">刷新</RsButton>
        </template>
        <RsEmpty v-if="agents.length === 0" title="暂无采集代理" />
        <RsTable
          v-else
          :columns="agentColumns"
          :data="agents"
          row-key="agentId"
          size="sm"
          cell-focus="none"
          null-label="—"
          highlight-row
          :rounded="false"
        >
          <template #onlineFlag="{ row }">
            <RsTag size="sm" :variant="(row as Agent).onlineFlag === 'Y' ? 'success' : 'default'">
              {{ (row as Agent).onlineFlag === 'Y' ? '在线' : '离线' }}
            </RsTag>
          </template>
        </RsTable>
      </RsCard>
    </section>
  </div>
</template>

<style scoped>
.fg-dash {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.fg-status {
  box-sizing: border-box;
  display: flex;
  align-items: center;
  gap: 18px;
  min-height: 64px;
  padding: 12px 16px;
  border: 0;
  border-radius: var(--rs-radius, 8px);
  background: var(--rs-surface);
  box-shadow: inset 0 0 0 var(--fg-line) var(--rs-border);
}

.fg-status[data-state='live'] {
  box-shadow: inset 3px 0 0 var(--rs-success), inset 0 0 0 var(--fg-line) var(--rs-border);
}

.fg-status[data-state='warn'] {
  box-shadow: inset 3px 0 0 var(--rs-warning), inset 0 0 0 var(--fg-line) var(--rs-border);
}

.fg-status[data-state='down'] {
  box-shadow: inset 3px 0 0 var(--rs-danger), inset 0 0 0 var(--fg-line) var(--rs-border);
}

.fg-status-dot {
  width: 8px;
  height: 8px;
  flex: none;
  border-radius: 50%;
  background: var(--rs-text-secondary);
}

.fg-status[data-state='live'] .fg-status-dot {
  background: var(--rs-success);
}

.fg-status[data-state='warn'] .fg-status-dot {
  background: var(--rs-warning);
}

.fg-status[data-state='down'] .fg-status-dot {
  background: var(--rs-danger);
}

.fg-status-copy {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 180px;
}

.fg-status-copy strong {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.02em;
}

.fg-status-copy span,
.fg-ratio span,
.fg-received span {
  font-size: 12px;
  color: var(--rs-text-secondary);
}

.fg-ratio {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 160px;
}

.fg-ratio-track {
  height: 6px;
  border-radius: 99px;
  overflow: hidden;
  background: color-mix(in srgb, var(--rs-text-primary) 10%, transparent);
}

.fg-ratio-track span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--rs-success);
}

.fg-status[data-state='warn'] .fg-ratio-track span {
  background: var(--rs-warning);
}

.fg-status[data-state='down'] .fg-ratio-track span,
.fg-status[data-state='idle'] .fg-ratio-track span {
  background: var(--rs-text-secondary);
}

.fg-received {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-inline-start: auto;
}

.fg-received strong {
  font-family: var(--rs-font-mono, ui-monospace, monospace);
  font-size: 13px;
  font-weight: 500;
}

.fg-dash-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.fg-dash-panels {
  flex: 1;
  min-height: 280px;
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
}

.fg-panel {
  min-width: 0;
  min-height: 0;
}

.fg-panel :deep(.rs-card__body) {
  overflow: auto;
}

@media (max-width: 1100px) {
  .fg-dash-kpis,
  .fg-dash-panels,
  .fg-status {
    grid-template-columns: 1fr;
  }

  .fg-status {
    display: grid;
    align-items: start;
  }

  .fg-received {
    margin-inline-start: 0;
  }

  .fg-dash {
    height: auto;
  }
}
</style>
