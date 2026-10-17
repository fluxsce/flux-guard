<script setup lang="ts">
import { ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess } from '@/api/request'
import { RsButton, RsCard, RsTooltip, rsConfirm, useRsToast } from '@/ui'
import { execMongo, type MongoRunning, type PointDetail } from '../api'

const props = defineProps<{
  panelCode: string
  detail: PointDetail
  componentId: string
}>()

const toast = useRsToast()
const busy = ref(false)
const hidden = ref<number[]>([])

watch(
  () => props.detail.running,
  (rows) => {
    const live = new Set((rows ?? []).map((item) => item.opid).filter((id): id is number => id != null && id > 0))
    hidden.value = hidden.value.filter((id) => live.has(id))
  },
)

function visibleRunning(rows: MongoRunning[] | undefined) {
  return (rows ?? []).filter((item) => item.opid == null || !hidden.value.includes(item.opid))
}

async function kill(item: MongoRunning) {
  if (!item.opid || busy.value) return
  const ok = await rsConfirm.confirm({
    title: '终止操作',
    description: `终止 ${item.ns || '这条操作'} 上已运行 ${secsText(item.secs)} 的 ${item.op || '操作'}。`,
    confirmText: '终止',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  try {
    const result = await execMongo(props.componentId, 'killOp', { op: String(item.opid) })
    if (!isApiSuccess(result)) {
      throw new Error(getApiMessage(result, '终止失败'))
    }
    hidden.value = [...hidden.value, item.opid]
    toast.success('已发送终止')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : getHttpErrorMessage(err, '终止失败'))
  } finally {
    busy.value = false
  }
}

const states: Record<string, string> = {
  PRIMARY: '主节点',
  SECONDARY: '从节点',
  ARBITER: '仲裁',
  RECOVERING: '恢复',
  STARTUP: '启动',
  STARTUP2: '初始同步',
  ROLLBACK: '回滚',
  DOWN: '下线',
  UNKNOWN: '未知',
  REMOVED: '已移除',
}

function stateText(state: string) {
  return states[state] || state || '—'
}

function stateLevel(state: string) {
  if (state === 'DOWN' || state === 'ROLLBACK') return 'alert'
  if (state === 'RECOVERING' || state === 'STARTUP' || state === 'STARTUP2' || state === 'UNKNOWN') return 'watch'
  return undefined
}

function healthText(health: number) {
  if (health > 0) return '正常'
  if (health === 0) return '异常'
  return '—'
}

function lagText(lag: number) {
  if (lag == null || lag < 0) return '—'
  if (lag >= 3600) return `${(lag / 3600).toFixed(1)} 小时`
  return `${Math.round(lag)} 秒`
}

function lagLevel(lag: number) {
  if (lag == null || lag < 0) return undefined
  if (lag >= 30) return 'alert'
  if (lag >= 10) return 'watch'
  return undefined
}

function bytes(value: number) {
  if (value == null || value < 0) return '—'
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${Math.round(value)} B`
}

function secsText(secs: number) {
  if (secs >= 3600) return `${(secs / 3600).toFixed(1)} 小时`
  if (secs >= 60) return `${(secs / 60).toFixed(1)} 分钟`
  return `${secs.toFixed(secs >= 10 ? 0 : 1)} 秒`
}
</script>

<template>
  <RsCard v-if="panelCode === 'replica'" class="fg-monitor" title="成员" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自 replSetGetStatus。延迟是相对主节点的秒数，主节点自己是 0。" aria-label="成员说明" />
    </template>
    <p v-if="detail.setName" class="fg-note">副本集 {{ detail.setName }}</p>
    <p v-if="detail.memberNote" class="fg-note">{{ detail.memberNote }}</p>
    <p v-if="!detail.members?.length && !detail.memberNote" class="fg-note">不是副本集，或暂时读不到成员。</p>
    <table v-if="detail.members?.length" class="fg-table">
      <thead>
        <tr>
          <th>节点</th>
          <th>状态</th>
          <th>健康</th>
          <th class="fg-num">延迟</th>
          <th>位置</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.members" :key="item.name">
          <td>{{ item.name }}</td>
          <td :data-level="stateLevel(item.state)">{{ stateText(item.state) }}</td>
          <td :data-level="item.health === 0 ? 'alert' : undefined">{{ healthText(item.health) }}</td>
          <td class="fg-num" :data-level="lagLevel(item.lag)">{{ lagText(item.lag) }}</td>
          <td>{{ item.self ? '本机' : '' }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
  <RsCard v-else-if="panelCode === 'storage'" class="fg-monitor" title="库" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自 listDatabases 的 sizeOnDisk。库很多时，优先列出占用最高的。" aria-label="库说明" />
    </template>
    <p v-if="detail.dbNote" class="fg-note">{{ detail.dbNote }}</p>
    <p v-if="!detail.databases?.length && !detail.dbNote" class="fg-note">还没有库。</p>
    <table v-if="detail.databases?.length" class="fg-table">
      <thead>
        <tr>
          <th>名称</th>
          <th class="fg-num">占用</th>
          <th>数据</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in detail.databases" :key="item.name">
          <td>{{ item.name }}</td>
          <td class="fg-num">{{ bytes(item.bytes) }}</td>
          <td>{{ item.empty ? '空' : '有数据' }}</td>
        </tr>
      </tbody>
    </table>
  </RsCard>
  <RsCard v-else-if="panelCode === 'slow'" class="fg-monitor" title="正在执行" size="sm" :padding="false">
    <template #actions>
      <RsTooltip icon content="来自 currentOp，只保留已经执行超过 1 秒的操作。可以按编号终止。这不是 system.profile 里的历史慢日志。" aria-label="慢操作说明" />
    </template>
    <p v-if="detail.runningNote" class="fg-note">{{ detail.runningNote }}</p>
    <p v-if="!visibleRunning(detail.running).length && !detail.runningNote" class="fg-note">没有执行超过 1 秒的操作。</p>
    <table v-if="visibleRunning(detail.running).length" class="fg-table">
      <thead>
        <tr>
          <th>操作</th>
          <th>命名空间</th>
          <th class="fg-num">已运行</th>
          <th>客户端</th>
          <th class="fg-num">编号</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(item, index) in visibleRunning(detail.running)" :key="`${item.opid ?? item.ns}-${index}`">
          <td>{{ item.op || '—' }}</td>
          <td>{{ item.ns || '—' }}</td>
          <td class="fg-num" :data-level="item.secs >= 5 ? 'alert' : 'watch'">{{ secsText(item.secs) }}</td>
          <td>{{ item.client || '—' }}</td>
          <td class="fg-num">{{ item.opid || '—' }}</td>
          <td>
            <RsButton v-if="item.opid" size="sm" variant="ghost" :disabled="busy" @click="kill(item)">终止</RsButton>
          </td>
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
td[data-level='alert'] {
  color: #e23b55;
}
td[data-level='watch'] {
  color: #c49212;
}
</style>
