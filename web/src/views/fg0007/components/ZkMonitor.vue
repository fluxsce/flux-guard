<script setup lang="ts">
import { computed } from 'vue'
import { RsCard, RsTooltip } from '@/ui'
import type { PointDetail } from '../api'

const props = defineProps<{
  panelCode: string
  detail: PointDetail
}>()

const facts = computed(() => {
  const detail = props.detail
  const rows: { label: string; value: string }[] = []
  if (detail.connections != null) rows.push({ label: '连接', value: String(detail.connections) })
  if (detail.received != null) rows.push({ label: '收包', value: String(detail.received) })
  if (detail.sent != null) rows.push({ label: '发包', value: String(detail.sent) })
  if (detail.minLatency != null || detail.maxLatency != null) {
    rows.push({
      label: '延迟范围',
      value: `${Math.round(detail.minLatency ?? 0)}–${Math.round(detail.maxLatency ?? 0)} ms`,
    })
  }
  if (detail.zxid) rows.push({ label: '事务号', value: detail.zxid })
  return rows
})

const srvrNote = computed(() => props.detail.source === 'srvr')
</script>

<template>
  <RsCard v-if="panelCode === 'overview'" class="fg-monitor" title="会话" size="sm" :padding="false">
    <template #actions>
      <RsTooltip
        icon
        content="优先读 mntr。服务端默认只开放 srvr 时，用 srvr 判断连通、角色和节点数。Watch 与跟随者仍要打开 mntr。"
        aria-label="会话说明"
      />
    </template>
    <p v-if="srvrNote" class="fg-note">当前用 srvr。服务端未把 mntr 放进四字命令白名单，Watch 和跟随者显示为 0。</p>
    <p v-if="!facts.length" class="fg-note">还没有连接和收发数据。下一次采集成功后会出现在这里。</p>
    <table v-else class="fg-table">
      <tbody>
        <tr v-for="item in facts" :key="item.label">
          <th>{{ item.label }}</th>
          <td class="fg-num">{{ item.value }}</td>
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
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-table th {
  width: 7rem;
  color: var(--rs-text-secondary, #4d647c);
  font-weight: 600;
}
.fg-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
</style>
