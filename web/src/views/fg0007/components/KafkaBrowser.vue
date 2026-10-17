<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsInput, RsSelect, rsConfirm, useRsToast, type RsSelectOption } from '@/ui'
import {
  execKafka,
  type KafkaGroupLive,
  type KafkaTopicConfig,
  type KafkaTopicLive,
} from '../api'

const props = defineProps<{
  componentId: string
}>()

const toast = useRsToast()
const tab = ref<'topic' | 'group'>('topic')
const topics = ref<KafkaTopicLive[]>([])
const groups = ref<KafkaGroupLive[]>([])
const selected = ref('')
const config = ref<KafkaTopicConfig | null>(null)
const note = ref('')
const error = ref('')
const busy = ref(false)
const form = ref<'' | 'create' | 'partitions' | 'config' | 'reset'>('')
const topicName = ref('')
const partitions = ref('1')
const replicas = ref('1')
const totalParts = ref('')
const configKey = ref('retention.ms')
const configValue = ref('')
const resetTopic = ref('')
const groupName = ref('')

const configOptions: RsSelectOption[] = [
  { label: '保留毫秒', value: 'retention.ms' },
  { label: '保留字节', value: 'retention.bytes' },
  { label: '清理策略', value: 'cleanup.policy' },
  { label: '最小同步副本', value: 'min.insync.replicas' },
  { label: '单条上限', value: 'max.message.bytes' },
]

const states: Record<string, string> = {
  Stable: '稳定',
  Empty: '空闲',
  Dead: '失效',
  PreparingRebalance: '再均衡',
  CompletingRebalance: '再均衡',
}

const current = computed(() => topics.value.find((item) => item.name === selected.value) ?? null)

function stateText(state?: string) {
  if (!state) return '—'
  return states[state] || state
}

function configText(value: string) {
  return value ? value : '默认'
}

function fail(err: unknown, fallback: string) {
  error.value = err instanceof Error ? err.message : getHttpErrorMessage(err, fallback)
  toast.error(error.value)
}

async function call<T>(command: string, args: Record<string, string>) {
  const result = await execKafka<T>(props.componentId, command, args)
  if (!isApiSuccess(result)) {
    throw new Error(getApiMessage(result, '操作失败'))
  }
  return parseApiData<T>(result)
}

async function loadTopics() {
  const listed = await call<{ topics?: KafkaTopicLive[]; note?: string }>('topics', {})
  topics.value = listed?.topics ?? []
  note.value = listed?.note || ''
  if (selected.value && !topics.value.some((item) => item.name === selected.value)) {
    selected.value = ''
    config.value = null
  }
}

async function loadGroups() {
  const listed = await call<{ groups?: KafkaGroupLive[]; note?: string }>('groups', {})
  groups.value = listed?.groups ?? []
  if (listed?.note) note.value = listed.note
}

async function openTopic(name: string) {
  selected.value = name
  form.value = ''
  error.value = ''
  busy.value = true
  try {
    config.value = (await call<KafkaTopicConfig>('topic_config', { name })) ?? null
  } catch (err) {
    config.value = null
    fail(err, '读配置失败')
  } finally {
    busy.value = false
  }
}

async function refresh() {
  busy.value = true
  error.value = ''
  try {
    if (tab.value === 'topic') {
      await loadTopics()
      if (selected.value) config.value = (await call<KafkaTopicConfig>('topic_config', { name: selected.value })) ?? null
    } else {
      await loadGroups()
    }
  } catch (err) {
    fail(err, '刷新失败')
  } finally {
    busy.value = false
  }
}

function showForm(next: typeof form.value) {
  form.value = form.value === next ? '' : next
  error.value = ''
  topicName.value = ''
  partitions.value = '1'
  replicas.value = '1'
  totalParts.value = current.value ? String(current.value.partitions) : ''
  configValue.value = ''
  resetTopic.value = selected.value
  groupName.value = ''
}

async function createTopic() {
  const name = topicName.value.trim()
  if (!name) {
    fail(new Error('请填写主题名'), '请填写主题名')
    return
  }
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { name }
    if (partitions.value.trim()) args.partitions = partitions.value.trim()
    if (replicas.value.trim()) args.replication_factor = replicas.value.trim()
    await call('create_topic', args)
    toast.success('已新建主题')
    form.value = ''
    await loadTopics()
    await openTopic(name)
  } catch (err) {
    fail(err, '新建主题失败')
  } finally {
    busy.value = false
  }
}

async function removeTopic() {
  const name = selected.value
  if (!name || current.value?.internal) return
  const ok = await rsConfirm.confirm({
    title: '删除主题',
    description: `删除主题 ${name}。主题上的消息会一起去掉。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('delete_topic', { name })
    toast.success('已删除主题')
    selected.value = ''
    config.value = null
    await loadTopics()
  } catch (err) {
    fail(err, '删除主题失败')
  } finally {
    busy.value = false
  }
}

async function addPartitions() {
  const name = selected.value
  const total = totalParts.value.trim()
  if (!name || !total) return
  const ok = await rsConfirm.confirm({
    title: '增加分区',
    description: `把 ${name} 的分区数调整到 ${total}。分区只能增加，不能减少。`,
    confirmText: '增加',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('add_partitions', { name, partitions: total })
    toast.success('已增加分区')
    form.value = ''
    await loadTopics()
    await openTopic(name)
  } catch (err) {
    fail(err, '增加分区失败')
  } finally {
    busy.value = false
  }
}

async function alterConfig() {
  const name = selected.value
  const value = configValue.value.trim()
  if (!name || !value) return
  const ok = await rsConfirm.confirm({
    title: '修改配置',
    description: `把 ${name} 的 ${configKey.value} 设为 ${value}。`,
    confirmText: '修改',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('alter_config', { name, key: configKey.value, value })
    toast.success('已修改配置')
    form.value = ''
    config.value = (await call<KafkaTopicConfig>('topic_config', { name })) ?? null
  } catch (err) {
    fail(err, '修改配置失败')
  } finally {
    busy.value = false
  }
}

async function removeGroup(name: string) {
  const ok = await rsConfirm.confirm({
    title: '删除消费组',
    description: `删除消费组 ${name}。组里还有成员时，服务端会拒绝。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('delete_group', { name })
    toast.success('已删除消费组')
    await loadGroups()
  } catch (err) {
    fail(err, '删除消费组失败')
  } finally {
    busy.value = false
  }
}

async function resetOffsets(to: 'earliest' | 'latest') {
  const name = groupName.value.trim()
  const topic = resetTopic.value.trim()
  if (!name || !topic) {
    fail(new Error('请填写消费组和主题'), '请填写消费组和主题')
    return
  }
  const ok = await rsConfirm.confirm({
    title: '重置位点',
    description: `把 ${name} 在 ${topic} 上的位点重置到${to === 'earliest' ? '最早' : '最新'}。组正在消费时会失败。`,
    confirmText: '重置',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('reset_offsets', { name, topic, to })
    toast.success('已重置位点')
    form.value = ''
    await loadGroups()
  } catch (err) {
    fail(err, '重置位点失败')
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  busy.value = true
  try {
    await loadTopics()
  } catch (err) {
    fail(err, '读主题失败')
  } finally {
    busy.value = false
  }
})
</script>

<template>
  <section class="fg-kafka-desk">
    <header class="fg-kafka-bar">
      <div class="fg-kafka-tabs">
        <button type="button" :data-on="tab === 'topic'" @click="tab = 'topic'">主题</button>
        <button type="button" :data-on="tab === 'group'" @click="tab = 'group'; refresh()">消费组</button>
      </div>
      <div class="fg-kafka-actions">
        <RsButton size="sm" variant="secondary" :disabled="busy" @click="refresh">刷新</RsButton>
        <RsButton v-if="tab === 'topic'" size="sm" :disabled="busy" @click="showForm('create')">新建主题</RsButton>
        <RsButton v-if="tab === 'group'" size="sm" :disabled="busy" @click="showForm('reset')">重置位点</RsButton>
      </div>
    </header>
    <p v-if="error" class="fg-kafka-error">{{ error }}</p>
    <p v-else-if="note" class="fg-kafka-note">{{ note }}</p>
    <form v-if="form === 'create'" class="fg-kafka-form" @submit.prevent="createTopic">
      <RsInput v-model="topicName" placeholder="主题名" :disabled="busy" />
      <RsInput v-model="partitions" placeholder="分区数" :disabled="busy" />
      <RsInput v-model="replicas" placeholder="副本因子" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy">创建</RsButton>
    </form>
    <form v-else-if="form === 'partitions'" class="fg-kafka-form" @submit.prevent="addPartitions">
      <RsInput v-model="totalParts" placeholder="调整后的分区总数" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy || !selected">增加</RsButton>
    </form>
    <form v-else-if="form === 'config'" class="fg-kafka-form" @submit.prevent="alterConfig">
      <RsSelect v-model="configKey" :options="configOptions" :disabled="busy" />
      <RsInput v-model="configValue" placeholder="新值。清理策略填 delete、compact 或 delete,compact" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy || !selected">修改</RsButton>
    </form>
    <form v-else-if="form === 'reset'" class="fg-kafka-form" @submit.prevent="resetOffsets('latest')">
      <RsInput v-model="groupName" placeholder="消费组" :disabled="busy" />
      <RsInput v-model="resetTopic" placeholder="主题" :disabled="busy" />
      <RsButton size="sm" variant="secondary" type="button" :disabled="busy" @click="resetOffsets('earliest')">重置到最早</RsButton>
      <RsButton size="sm" type="submit" :disabled="busy">重置到最新</RsButton>
    </form>
    <div v-if="tab === 'topic'" class="fg-kafka-body">
      <div class="fg-kafka-list">
        <button
          v-for="item in topics"
          :key="item.name"
          type="button"
          :data-on="item.name === selected"
          @click="openTopic(item.name)"
        >
          <span>{{ item.name }}</span>
          <small>{{ item.partitions }} 分区</small>
        </button>
        <p v-if="!topics.length" class="fg-kafka-note">还没有主题。</p>
      </div>
      <div class="fg-kafka-detail">
        <p v-if="!selected" class="fg-kafka-note">选一个主题查看保留时间、清理策略和最小同步副本。</p>
        <template v-else-if="config">
          <div class="fg-kafka-actions">
            <strong>{{ config.name }}</strong>
            <RsButton size="sm" variant="secondary" :disabled="busy || current?.internal" @click="showForm('partitions')">增加分区</RsButton>
            <RsButton size="sm" variant="secondary" :disabled="busy || current?.internal" @click="showForm('config')">修改配置</RsButton>
            <RsButton size="sm" variant="ghost" :disabled="busy || current?.internal" @click="removeTopic">删除</RsButton>
          </div>
          <p v-if="current?.internal" class="fg-kafka-note">内部主题不能在这里改。</p>
          <dl class="fg-kafka-stat">
            <div><dt>分区</dt><dd>{{ current?.partitions ?? '—' }}</dd></div>
            <div><dt>副本因子</dt><dd>{{ current?.replicas ?? '—' }}</dd></div>
            <div><dt>保留毫秒</dt><dd>{{ configText(config.retentionMs) }}</dd></div>
            <div><dt>保留字节</dt><dd>{{ configText(config.retentionBytes) }}</dd></div>
            <div><dt>清理策略</dt><dd>{{ configText(config.cleanup) }}</dd></div>
            <div><dt>最小同步副本</dt><dd>{{ configText(config.minIsr) }}</dd></div>
            <div><dt>单条上限</dt><dd>{{ configText(config.maxMessageBytes) }}</dd></div>
          </dl>
        </template>
      </div>
    </div>
    <div v-else class="fg-kafka-detail fg-kafka-groups">
      <p v-if="!groups.length" class="fg-kafka-note">没有消费组。</p>
      <table v-else class="fg-kafka-table">
        <thead>
          <tr>
            <th>消费组</th>
            <th>状态</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in groups" :key="item.name">
            <td>{{ item.name }}</td>
            <td>{{ stateText(item.state) }}</td>
            <td>
              <RsButton size="sm" variant="ghost" :disabled="busy" @click="groupName = item.name; resetTopic = selected; form = 'reset'">重置位点</RsButton>
              <RsButton size="sm" variant="ghost" :disabled="busy" @click="removeGroup(item.name)">删除</RsButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.fg-kafka-desk {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  background: var(--rs-surface);
}
.fg-kafka-bar,
.fg-kafka-actions,
.fg-kafka-form,
.fg-kafka-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fg-kafka-bar {
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-kafka-tabs button {
  border: 0;
  background: transparent;
  color: var(--rs-text-secondary);
  cursor: pointer;
  font: inherit;
}
.fg-kafka-tabs button[data-on='true'] {
  color: var(--rs-text-primary);
  font-weight: 650;
}
.fg-kafka-error,
.fg-kafka-note {
  margin: 0;
  padding: 8px 12px 0;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
.fg-kafka-error {
  color: #e23b55;
}
.fg-kafka-form {
  flex-wrap: wrap;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-kafka-body {
  display: grid;
  grid-template-columns: minmax(220px, 320px) minmax(0, 1fr);
  flex: 1;
  min-height: 0;
}
.fg-kafka-list {
  min-height: 0;
  overflow: auto;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-kafka-list button {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  border: 0;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}
.fg-kafka-list button[data-on='true'] {
  background: var(--rs-surface-2, rgba(0, 0, 0, 0.04));
}
.fg-kafka-list small {
  color: var(--rs-text-secondary, #4d647c);
}
.fg-kafka-detail {
  min-width: 0;
  min-height: 0;
  overflow: auto;
  padding: 12px;
}
.fg-kafka-groups {
  flex: 1;
}
.fg-kafka-stat {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  margin: 12px 0 0;
}
.fg-kafka-stat div {
  display: flex;
  gap: 6px;
}
.fg-kafka-stat dt {
  color: var(--rs-text-secondary, #4d647c);
}
.fg-kafka-stat dd {
  margin: 0;
}
.fg-kafka-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.fg-kafka-table th,
.fg-kafka-table td {
  padding: 6px 10px;
  text-align: left;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-kafka-table th {
  color: var(--rs-text-secondary, #4d647c);
  font-weight: 600;
}
</style>
