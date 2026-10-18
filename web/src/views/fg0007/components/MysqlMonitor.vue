<script setup lang="ts">
import { RsCard, RsTooltip } from '@/ui'
import type { PointDetail } from '../api'

defineProps<{
  panelCode: string
  detail: PointDetail
}>()

function bytes(value: number | undefined) {
  if (value == null || value <= 0) return '—'
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${Math.round(value)} B`
}

function secs(value: number) {
  if (value >= 3600) return `${(value / 3600).toFixed(1)} 小时`
  if (value >= 60) return `${(value / 60).toFixed(1)} 分钟`
  return `${value} 秒`
}

function thread(value: string | undefined) {
  if (value === 'Yes') return '运行'
  if (value === 'No') return '停止'
  if (value === 'Connecting') return '连接中'
  return value || '—'
}
</script>

<template>
  <RsCard v-if="panelCode === 'replica'" class="fg-monitor" title="复制线程" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自 SHOW REPLICA STATUS。有复制位点才算从库；Seconds_Behind 为空时延迟不显示数字。" aria-label="复制说明" />
    </template>
    <p v-if="!detail.mysqlReplica" class="fg-note">当前不是从库，或读不到复制状态。</p>
    <table v-else class="fg-table">
      <tbody>
        <tr>
          <th>主库</th>
          <td>{{ detail.mysqlReplica.source || '—' }}</td>
        </tr>
        <tr>
          <th>IO 线程</th>
          <td>{{ thread(detail.mysqlReplica.io) }}</td>
        </tr>
        <tr>
          <th>SQL 线程</th>
          <td>{{ thread(detail.mysqlReplica.sql) }}</td>
        </tr>
        <tr v-if="detail.mysqlReplica.ioError">
          <th>IO 错误</th>
          <td>{{ detail.mysqlReplica.ioError }}</td>
        </tr>
        <tr v-if="detail.mysqlReplica.sqlError">
          <th>SQL 错误</th>
          <td>{{ detail.mysqlReplica.sqlError }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
  <RsCard v-else-if="panelCode === 'slow'" class="fg-monitor" title="正在执行" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自 information_schema.PROCESSLIST。只保留已执行超过 1 秒、且不是 Sleep 或复制转储的语句。这不是慢查询日志。" aria-label="正在执行说明" />
    </template>
    <p v-if="detail.mysqlSessionNote" class="fg-note">{{ detail.mysqlSessionNote }}</p>
    <p v-if="!detail.mysqlSessions?.length && !detail.mysqlSessionNote" class="fg-note">没有执行超过 1 秒的语句。</p>
    <table v-if="detail.mysqlSessions?.length" class="fg-table">
      <thead>
        <tr>
          <th class="fg-num">已运行</th>
          <th>账号</th>
          <th>库</th>
          <th>命令</th>
          <th>状态</th>
          <th>语句</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.mysqlSessions" :key="item.id">
          <td class="fg-num">{{ secs(item.time) }}</td>
          <td>{{ item.user || '—' }}<span v-if="item.host" class="fg-host">@{{ item.host }}</span></td>
          <td>{{ item.db || '—' }}</td>
          <td>{{ item.command || '—' }}</td>
          <td>{{ item.state || '—' }}</td>
          <td class="fg-sql">{{ item.info || '—' }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
  <RsCard v-else-if="panelCode === 'innodb'" class="fg-monitor" title="缓冲池容量" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="innodb_buffer_pool_size。命中率是两次采集之间的增量，不是从启动累计。" aria-label="缓冲池说明" />
    </template>
    <p class="fg-note">缓冲池 {{ bytes(detail.mysqlPoolBytes) }}</p>
  </RsCard>
</template>

<style scoped>
.fg-monitor {
  flex: none;
  align-self: stretch;
}
.fg-note {
  margin: 0;
  padding: 12px 14px;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
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
  white-space: nowrap;
}
.fg-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.fg-host {
  color: var(--rs-text-secondary, #4d647c);
}
.fg-sql {
  max-width: 420px;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
