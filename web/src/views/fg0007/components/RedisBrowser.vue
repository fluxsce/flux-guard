<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsButton, RsInput, RsSelect, rsConfirm, useRsToast, type RsSelectOption } from '@/ui'
import {
  execRedis,
  type RedisDatabaseList,
  type RedisKeyView,
  type RedisScanResult,
} from '../api'

const props = defineProps<{
  componentId: string
}>()

const toast = useRsToast()
const databases = ref<RedisDatabaseList['databases']>([])
const database = ref('')
const matchText = ref('')
const keys = ref<string[]>([])
const cursor = ref('0')
const selected = ref('')
const current = ref<RedisKeyView | null>(null)
const seconds = ref('60')
const dbNote = ref('')
const note = ref('')
const error = ref('')
const busy = ref(false)
const opened = ref(false)

const types: Record<string, string> = {
  string: '字符串',
  hash: '哈希',
  list: '列表',
  set: '集合',
  zset: '有序集合',
  stream: '流',
  none: '不存在',
}

const dbOptions = computed<RsSelectOption[]>(() =>
  (databases.value ?? []).map((item) => ({
    label: `${item.name} · ${item.keys} 键`,
    value: item.name.startsWith('db') ? item.name.slice(2) : item.name,
  })),
)

const scored = computed(() => (current.value?.fields ?? []).some((item) => item.score))

function typeText(kind?: string) {
  if (!kind) return '—'
  return types[kind] || kind
}

function ttlText(ttl: number) {
  if (ttl === -2) return '不存在'
  if (ttl < 0) return '不过期'
  if (ttl < 60) return `剩余 ${ttl} 秒`
  if (ttl < 3600) return `剩余 ${Math.floor(ttl / 60)} 分 ${ttl % 60} 秒`
  const hours = Math.floor(ttl / 3600)
  const minutes = Math.floor((ttl % 3600) / 60)
  return `剩余 ${hours} 小时 ${minutes} 分`
}

function fail(err: unknown, fallback: string) {
  error.value = err instanceof Error ? err.message : getHttpErrorMessage(err, fallback)
  toast.error(error.value)
}

async function call<T>(command: string, args: Record<string, string>) {
  const result = await execRedis<T>(props.componentId, command, args)
  if (!isApiSuccess(result)) {
    throw new Error(getApiMessage(result, '操作失败'))
  }
  return parseApiData<T>(result)
}

async function loadDatabases() {
  busy.value = true
  error.value = ''
  try {
    const listed = await call<RedisDatabaseList>('databases', {})
    databases.value = listed?.databases ?? []
    dbNote.value = listed?.note || ''
    note.value = dbNote.value
    const first = databases.value[0]
    database.value = first ? (first.name.startsWith('db') ? first.name.slice(2) : first.name) : ''
    opened.value = true
    await loadKeys(true)
  } catch (err) {
    fail(err, '读库失败')
  } finally {
    busy.value = false
  }
}

async function loadKeys(reset: boolean) {
  if (!database.value) return
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { database: database.value }
    const match = matchText.value.trim()
    if (match) args.match = match
    if (!reset && cursor.value && cursor.value !== '0') args.cursor = cursor.value
    const listed = await call<RedisScanResult>('scan', args)
    const found = listed?.keys ?? []
    keys.value = reset ? found : [...keys.value, ...found.filter((item) => !keys.value.includes(item))]
    cursor.value = listed?.cursor || '0'
    note.value = listed?.note || dbNote.value
    if (selected.value && !keys.value.includes(selected.value)) {
      selected.value = ''
      current.value = null
    }
  } catch (err) {
    fail(err, '读键失败')
  } finally {
    busy.value = false
  }
}

async function openKey(name: string) {
  selected.value = name
  busy.value = true
  error.value = ''
  try {
    current.value = (await call<RedisKeyView>('key', { database: database.value, name })) ?? null
  } catch (err) {
    current.value = null
    fail(err, '读键内容失败')
  } finally {
    busy.value = false
  }
}

async function removeKey() {
  const name = selected.value
  if (!name) return
  const ok = await rsConfirm.confirm({
    title: '删除键',
    description: `删除 ${name}。删除后不能恢复。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('delete', { database: database.value, name })
    toast.success('已删除键')
    keys.value = keys.value.filter((item) => item !== name)
    selected.value = ''
    current.value = null
  } catch (err) {
    fail(err, '删除键失败')
  } finally {
    busy.value = false
  }
}

async function applyExpire(clear: boolean) {
  const name = selected.value
  if (!name) return
  const value = clear ? '0' : seconds.value.trim()
  if (!clear && !value) return
  busy.value = true
  error.value = ''
  try {
    await call('expire', { database: database.value, name, seconds: value })
    toast.success(clear ? '已取消过期' : '已设置过期')
    await openKey(name)
  } catch (err) {
    fail(err, '设置过期失败')
  } finally {
    busy.value = false
  }
}

watch(database, (value, previous) => {
  if (!opened.value || !previous || value === previous) return
  void loadKeys(true)
})

onMounted(() => {
  void loadDatabases()
})
</script>

<template>
  <section class="fg-redis-desk">
    <header class="fg-redis-bar">
      <RsSelect v-model="database" :options="dbOptions" :disabled="busy || !dbOptions.length" />
      <RsInput v-model="matchText" placeholder="匹配，例如 user:*" :disabled="busy" @keydown.enter="loadKeys(true)" />
      <RsButton size="sm" :disabled="busy || !database" @click="loadKeys(true)">查找</RsButton>
      <RsButton v-if="cursor !== '0'" size="sm" variant="secondary" :disabled="busy" @click="loadKeys(false)">继续</RsButton>
    </header>
    <p v-if="error" class="fg-redis-error">{{ error }}</p>
    <p v-else-if="note" class="fg-redis-note">{{ note }}</p>
    <div class="fg-redis-body">
      <div class="fg-redis-list">
        <button v-for="name in keys" :key="name" type="button" :data-on="name === selected" @click="openKey(name)">
          {{ name }}
        </button>
        <p v-if="!busy && !keys.length" class="fg-redis-note">这个库里没有匹配的键。</p>
      </div>
      <div class="fg-redis-detail">
        <p v-if="!current" class="fg-redis-note">选一个键查看内容和过期时间。</p>
        <template v-else>
          <div class="fg-redis-actions">
            <strong>{{ current.name }}</strong>
            <span>{{ typeText(current.type) }}</span>
            <span>{{ ttlText(current.ttl) }}</span>
            <span v-if="current.count != null">{{ current.type === 'string' ? `${current.count} 字节` : `${current.count} 项` }}</span>
            <RsButton size="sm" variant="ghost" :disabled="busy || current.type === 'none'" @click="removeKey">删除</RsButton>
          </div>
          <form v-if="current.type !== 'none'" class="fg-redis-form" @submit.prevent="applyExpire(false)">
            <RsInput v-model="seconds" placeholder="过期秒数" :disabled="busy" />
            <RsButton size="sm" type="submit" :disabled="busy">设置过期</RsButton>
            <RsButton size="sm" variant="secondary" type="button" :disabled="busy" @click="applyExpire(true)">取消过期</RsButton>
          </form>
          <p v-if="current.note" class="fg-redis-note">{{ current.note }}</p>
          <pre v-if="current.type === 'string'" class="fg-redis-text">{{ current.text }}</pre>
          <ul v-else-if="current.items?.length" class="fg-redis-items">
            <li v-for="(item, index) in current.items" :key="index">{{ item }}</li>
          </ul>
          <table v-else-if="current.fields?.length" class="fg-redis-table">
            <thead>
              <tr>
                <th>{{ scored ? '成员' : '字段' }}</th>
                <th v-if="scored">分数</th>
                <th v-else>值</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, index) in current.fields" :key="index">
                <td>{{ item.field }}</td>
                <td>{{ scored ? item.score : item.value }}</td>
              </tr>
            </tbody>
          </table>
        </template>
      </div>
    </div>
  </section>
</template>

<style scoped>
.fg-redis-desk {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 480px;
  background: var(--rs-surface);
}
.fg-redis-bar,
.fg-redis-actions,
.fg-redis-form {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fg-redis-bar,
.fg-redis-form {
  flex-wrap: wrap;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-redis-error,
.fg-redis-note {
  margin: 0;
  padding: 8px 12px 0;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
.fg-redis-error {
  color: #e23b55;
}
.fg-redis-body {
  display: grid;
  grid-template-columns: minmax(220px, 320px) minmax(0, 1fr);
  flex: 1;
  min-height: 0;
}
.fg-redis-list,
.fg-redis-detail {
  min-height: 0;
  overflow: auto;
}
.fg-redis-list {
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-redis-list button {
  display: block;
  width: 100%;
  padding: 8px 12px;
  border: 0;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
  word-break: break-all;
}
.fg-redis-list button[data-on='true'] {
  background: var(--rs-surface-2, rgba(0, 0, 0, 0.04));
}
.fg-redis-actions {
  padding: 12px;
}
.fg-redis-text,
.fg-redis-items {
  margin: 0 12px 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.fg-redis-table {
  width: calc(100% - 24px);
  margin: 0 12px 12px;
  border-collapse: collapse;
}
.fg-redis-table th,
.fg-redis-table td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  text-align: left;
  vertical-align: top;
  word-break: break-all;
}
</style>
