<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import {
  RsButton,
  RsContextMenu,
  RsInput,
  RsSelect,
  RsTextarea,
  RsTree,
  rsConfirm,
  useRsToast,
  type RsContextMenuItem,
  type RsSelectOption,
  type RsTreeNode,
} from '@/ui'
import {
  execMongo,
  type MongoCollStats,
  type MongoCollectionList,
  type MongoCountResult,
  type MongoDatabase,
  type MongoDatabaseList,
  type MongoExplainResult,
  type MongoFindResult,
  type MongoIndex,
  type MongoIndexList,
  type MongoProfileResult,
  type MongoProfileRow,
} from '../api'

const props = defineProps<{
  componentId: string
}>()

const toast = useRsToast()
const nodes = ref<RsTreeNode[]>([])
const selected = ref('')
const expanded = ref<string[]>([])
const query = ref('')
const database = ref('')
const collection = ref('')
const dbMeta = ref<Record<string, MongoDatabase>>({})
const collType = ref<Record<string, string>>({})
const stats = ref<MongoCollStats | null>(null)
const indexes = ref<MongoIndex[]>([])
const note = ref('')
const error = ref('')
const busy = ref(false)
const form = ref<'' | 'database' | 'collection' | 'rename' | 'index'>('')
const dbName = ref('')
const collName = ref('')
const newName = ref('')
const indexKeys = ref('')
const indexName = ref('')
const unique = ref('N')
const sparse = ref('N')
const ttl = ref('')
const filterText = ref('')
const limitText = ref('20')
const documents = ref<string[]>([])
const counted = ref<number | null>(null)
const plan = ref<MongoExplainResult | null>(null)
const profileRows = ref<MongoProfileRow[]>([])
const profileNote = ref('')
const menuKey = ref('')
const nodeMenu = ref<{ open: (point?: { x: number; y: number }) => void } | null>(null)

const sep = '\u0000'
const flagOptions: RsSelectOption[] = [
  { label: '否', value: 'N' },
  { label: '是', value: 'Y' },
]

const menuItems = computed<RsContextMenuItem[]>(() => {
  const key = menuKey.value
  const parsed = parseKey(key)
  const title = parsed.collection || parsed.database || '库'
  const opened = expanded.value.includes(key)
  const treeNode = findNode(nodes.value, key)
  const system = parsed.collection.startsWith('system.')
  const protectedDb = isProtected(parsed.database)
  return [
    { key: 'head', label: title, separator: true },
    { key: 'refresh', label: '刷新', icon: 'refresh-cw' },
    {
      key: 'toggle',
      label: opened ? '收起' : '展开',
      icon: opened ? 'chevron-down' : 'chevron-right',
      disabled: !parsed.database || !!parsed.collection || treeNode?.isLeaf === true,
    },
    { key: 'sep-edit', label: '', separator: true },
    { key: 'create-db', label: '新建库', icon: 'plus' },
    { key: 'create-coll', label: '新建集合', icon: 'plus', disabled: !parsed.database },
    { key: 'rename', label: '重命名', icon: 'pencil', disabled: !parsed.collection || system },
    { key: 'copy', label: '复制名称', icon: 'copy' },
    { key: 'index', label: '新建索引', icon: 'key', disabled: !parsed.collection || system || collType.value[key] === 'view' },
    { key: 'sep-danger', label: '', separator: true },
    {
      key: 'delete',
      label: '删除',
      icon: 'trash-2',
      danger: true,
      disabled: !parsed.database || system || (!parsed.collection && protectedDb),
    },
  ]
})

function dbKey(name: string) {
  return `db:${name}`
}

function collKey(db: string, name: string) {
  return `coll:${db}${sep}${name}`
}

function parseKey(key: string) {
  if (key.startsWith('coll:')) {
    const rest = key.slice(5)
    const cut = rest.indexOf(sep)
    if (cut < 0) return { database: '', collection: '' }
    return { database: rest.slice(0, cut), collection: rest.slice(cut + 1) }
  }
  if (key.startsWith('db:')) return { database: key.slice(3), collection: '' }
  return { database: '', collection: '' }
}

function isProtected(name: string) {
  return name === 'admin' || name === 'local' || name === 'config'
}

function findNode(list: RsTreeNode[], key: string): RsTreeNode | null {
  for (const item of list) {
    if (item.key === key) return item
    const found = findNode(item.children ?? [], key)
    if (found) return found
  }
  return null
}

function matchNode(node: RsTreeNode, keyword: string) {
  const q = keyword.trim().toLowerCase()
  if (!q) return true
  return String(node.label ?? '').toLowerCase().includes(q) || String(node.key ?? '').toLowerCase().includes(q)
}

function bytes(value?: number) {
  if (value == null || value < 0) return '—'
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${Math.round(value)} B`
}

function typeText(kind: string) {
  if (kind === 'view') return '视图'
  if (kind === 'timeseries') return '时序'
  if (!kind || kind === 'collection') return '集合'
  return kind
}

function indexFlags(item: MongoIndex) {
  const flags: string[] = []
  if (item.unique) flags.push('唯一')
  if (item.sparse) flags.push('稀疏')
  if (item.hidden) flags.push('隐藏')
  if (item.partial) flags.push('部分')
  if (item.ttl) flags.push(`${item.ttl} 秒过期`)
  return flags.length ? flags.join('、') : '—'
}

function fail(err: unknown, fallback: string) {
  error.value = err instanceof Error ? err.message : getHttpErrorMessage(err, fallback)
  toast.error(error.value)
}

async function call<T>(command: string, args: Record<string, string>) {
  const result = await execMongo<T>(props.componentId, command, args)
  if (!isApiSuccess(result)) {
    throw new Error(getApiMessage(result, '操作失败'))
  }
  return parseApiData<T>(result)
}

async function loadDatabases() {
  const listed = await call<MongoDatabaseList>('databases', {})
  const rows = listed?.databases ?? []
  const meta: Record<string, MongoDatabase> = {}
  nodes.value = rows.map((item) => {
    meta[item.name] = item
    const key = dbKey(item.name)
    const previous = findNode(nodes.value, key)
    return {
      key,
      label: item.name,
      isLeaf: false,
      children: previous?.children ?? [],
    }
  })
  dbMeta.value = meta
  note.value = listed?.note || ''
  expanded.value = expanded.value.filter((key) => parseKey(key).collection || !!meta[parseKey(key).database])
}

async function fill(treeNode: RsTreeNode, key: string) {
  const parsed = parseKey(key)
  if (!parsed.database || parsed.collection) return
  const listed = await call<MongoCollectionList>('collections', { database: parsed.database })
  const rows = listed?.collections ?? []
  const types = { ...collType.value }
  treeNode.children = rows.map((item) => {
    const child = collKey(parsed.database, item.name)
    types[child] = item.type || 'collection'
    return { key: child, label: item.name, isLeaf: true }
  })
  treeNode.isLeaf = rows.length === 0
  collType.value = types
  if (listed?.note) note.value = listed.note
}

async function loadData(treeNode: RsTreeNode, key: string) {
  try {
    await fill(treeNode, key)
  } catch (err) {
    fail(err, '读集合失败')
    throw err
  }
}

async function open(key: string) {
  const parsed = parseKey(key)
  if (!parsed.database) return
  error.value = ''
  selected.value = key
  if (database.value !== parsed.database) {
    profileRows.value = []
    profileNote.value = ''
  }
  database.value = parsed.database
  collection.value = parsed.collection
  form.value = ''
  documents.value = []
  counted.value = null
  plan.value = null
  if (!parsed.collection) {
    stats.value = null
    indexes.value = []
    return
  }
  busy.value = true
  stats.value = null
  indexes.value = []
  const db = parsed.database
  const coll = parsed.collection
  const [statResult, indexResult] = await Promise.all([
    call<MongoCollStats>('stats', { database: db, collection: coll }).then(
      (value) => ({ value, error: '' }),
      (err: unknown) => ({ value: null, error: err instanceof Error ? err.message : getHttpErrorMessage(err, '读集合失败') }),
    ),
    call<MongoIndexList>('indexes', { database: db, collection: coll }).then(
      (value) => ({ value, error: '' }),
      (err: unknown) => ({ value: null, error: err instanceof Error ? err.message : getHttpErrorMessage(err, '读索引失败') }),
    ),
  ])
  if (database.value !== db || collection.value !== coll) {
    busy.value = false
    return
  }
  stats.value = statResult.value ?? null
  indexes.value = indexResult.value?.indexes ?? []
  if (indexResult.value?.note) note.value = indexResult.value.note
  const problem = statResult.error || indexResult.error
  if (problem) fail(new Error(problem), problem)
  busy.value = false
}

async function refreshAll() {
  busy.value = true
  error.value = ''
  try {
    await loadDatabases()
    for (const key of expanded.value) {
      const treeNode = findNode(nodes.value, key)
      if (treeNode && !parseKey(key).collection) await fill(treeNode, key)
    }
    if (selected.value && findNode(nodes.value, selected.value)) await open(selected.value)
    else {
      database.value = ''
      collection.value = ''
      stats.value = null
      indexes.value = []
      selected.value = ''
    }
  } catch (err) {
    fail(err, '刷新失败')
  } finally {
    busy.value = false
  }
}

async function toggleBranch(key: string) {
  const treeNode = findNode(nodes.value, key)
  if (!treeNode || treeNode.isLeaf || parseKey(key).collection) return
  if (expanded.value.includes(key)) {
    expanded.value = expanded.value.filter((item) => item !== key)
    return
  }
  if (!treeNode.children?.length) {
    busy.value = true
    try {
      await fill(treeNode, key)
    } catch (err) {
      fail(err, '读集合失败')
      busy.value = false
      return
    }
    busy.value = false
  }
  if (!treeNode.isLeaf) expanded.value = [...expanded.value, key]
}

function showForm(next: typeof form.value, key = selected.value) {
  const parsed = parseKey(key)
  form.value = next
  error.value = ''
  if (next === 'collection' || next === 'index' || next === 'rename') {
    database.value = parsed.database || database.value
    collection.value = parsed.collection || (next === 'collection' ? '' : collection.value)
  }
  dbName.value = ''
  collName.value = ''
  newName.value = ''
  indexKeys.value = ''
  indexName.value = ''
  unique.value = 'N'
  sparse.value = 'N'
  ttl.value = ''
}

async function createDatabase() {
  const db = dbName.value.trim()
  const coll = collName.value.trim()
  if (!db || !coll) {
    fail(new Error('请填写库名和第一个集合名'), '请填写库名和第一个集合名')
    return
  }
  busy.value = true
  error.value = ''
  try {
    await call('createDatabase', { database: db, collection: coll })
    toast.success('已新建库')
    form.value = ''
    await loadDatabases()
    const key = dbKey(db)
    const treeNode = findNode(nodes.value, key)
    if (treeNode) {
      await fill(treeNode, key)
      if (!expanded.value.includes(key)) expanded.value = [...expanded.value, key]
    }
    await open(collKey(db, coll))
  } catch (err) {
    fail(err, '新建库失败')
  } finally {
    busy.value = false
  }
}

async function createCollection() {
  const db = database.value
  const coll = collName.value.trim()
  if (!db || !coll) {
    fail(new Error('请填写集合名'), '请填写集合名')
    return
  }
  busy.value = true
  error.value = ''
  try {
    await call('createCollection', { database: db, collection: coll })
    toast.success('已新建集合')
    form.value = ''
    const key = dbKey(db)
    const treeNode = findNode(nodes.value, key)
    if (treeNode) await fill(treeNode, key)
    if (!expanded.value.includes(key)) expanded.value = [...expanded.value, key]
    await open(collKey(db, coll))
  } catch (err) {
    fail(err, '新建集合失败')
  } finally {
    busy.value = false
  }
}

async function renameCollection() {
  const db = database.value
  const coll = collection.value
  const name = newName.value.trim()
  if (!db || !coll || !name) return
  const ok = await rsConfirm.confirm({
    title: '重命名集合',
    description: `把 ${db}.${coll} 改名为 ${name}。`,
    confirmText: '重命名',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('renameCollection', { database: db, collection: coll, name })
    toast.success('已重命名')
    form.value = ''
    const parent = dbKey(db)
    const treeNode = findNode(nodes.value, parent)
    if (treeNode) await fill(treeNode, parent)
    await open(collKey(db, name))
  } catch (err) {
    fail(err, '重命名失败')
  } finally {
    busy.value = false
  }
}

async function createIndex() {
  const db = database.value
  const coll = collection.value
  const keys = indexKeys.value.trim()
  if (!db || !coll || !keys) {
    fail(new Error('请填写索引键'), '请填写索引键')
    return
  }
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { database: db, collection: coll, keys, unique: unique.value, sparse: sparse.value }
    if (indexName.value.trim()) args.name = indexName.value.trim()
    if (ttl.value.trim()) args.ttl = ttl.value.trim()
    await call('createIndex', args)
    toast.success('已新建索引')
    form.value = ''
    await open(collKey(db, coll))
  } catch (err) {
    fail(err, '新建索引失败')
  } finally {
    busy.value = false
  }
}

async function remove(key = selected.value) {
  const parsed = parseKey(key)
  if (!parsed.database) return
  const target = parsed.collection ? `${parsed.database}.${parsed.collection}` : parsed.database
  const ok = await rsConfirm.confirm({
    title: parsed.collection ? '删除集合' : '删除库',
    description: parsed.collection ? `删除集合 ${target} 及其索引。` : `删除库 ${target} 及其全部集合。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    if (parsed.collection) {
      await call('dropCollection', { database: parsed.database, collection: parsed.collection })
      toast.success('已删除集合')
      const parent = dbKey(parsed.database)
      const treeNode = findNode(nodes.value, parent)
      if (treeNode) await fill(treeNode, parent)
      await open(parent)
    } else {
      await call('dropDatabase', { database: parsed.database })
      toast.success('已删除库')
      expanded.value = expanded.value.filter((item) => parseKey(item).database !== parsed.database)
      await loadDatabases()
      database.value = ''
      collection.value = ''
      stats.value = null
      indexes.value = []
      selected.value = ''
    }
    form.value = ''
  } catch (err) {
    fail(err, '删除失败')
  } finally {
    busy.value = false
  }
}

async function dropIndex(item: MongoIndex) {
  if (!database.value || !collection.value || item.name === '_id_') return
  const ok = await rsConfirm.confirm({
    title: '删除索引',
    description: `删除 ${database.value}.${collection.value} 上的索引 ${item.name}。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('dropIndex', { database: database.value, collection: collection.value, name: item.name })
    toast.success('已删除索引')
    await open(collKey(database.value, collection.value))
  } catch (err) {
    fail(err, '删除索引失败')
  } finally {
    busy.value = false
  }
}

async function runFind() {
  if (!database.value || !collection.value) return
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { database: database.value, collection: collection.value }
    if (filterText.value.trim()) args.filter = filterText.value.trim()
    if (limitText.value.trim()) args.limit = limitText.value.trim()
    const got = await call<MongoFindResult>('find', args)
    documents.value = got?.documents ?? []
    counted.value = null
    plan.value = null
    if (got?.note) note.value = got.note
    if (!documents.value.length) toast.success('没有匹配的文档')
  } catch (err) {
    fail(err, '查询失败')
  } finally {
    busy.value = false
  }
}

async function runCount() {
  if (!database.value || !collection.value) return
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { database: database.value, collection: collection.value }
    if (filterText.value.trim()) args.filter = filterText.value.trim()
    const got = await call<MongoCountResult>('count', args)
    counted.value = got?.count ?? 0
    plan.value = null
  } catch (err) {
    fail(err, '计数失败')
  } finally {
    busy.value = false
  }
}

async function runExplain() {
  if (!database.value || !collection.value) return
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { database: database.value, collection: collection.value }
    if (filterText.value.trim()) args.filter = filterText.value.trim()
    plan.value = (await call<MongoExplainResult>('explain', args)) ?? null
  } catch (err) {
    fail(err, '执行计划失败')
  } finally {
    busy.value = false
  }
}

async function runProfile() {
  if (!database.value) return
  busy.value = true
  error.value = ''
  profileNote.value = ''
  try {
    const got = await call<MongoProfileResult>('profile', { database: database.value })
    profileRows.value = got?.entries ?? []
    profileNote.value = got?.note || ''
  } catch (err) {
    fail(err, '读慢日志失败')
  } finally {
    busy.value = false
  }
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success('已复制')
  } catch {
    toast.error('复制失败')
  }
}

function onNodeMenu(_treeNode: RsTreeNode, key: string, event: MouseEvent) {
  event.preventDefault()
  event.stopPropagation()
  if (busy.value) return
  menuKey.value = key
  nodeMenu.value?.open({ x: event.clientX, y: event.clientY })
}

async function onMenuSelect(action: string) {
  const key = menuKey.value
  if (!key || busy.value) return
  const parsed = parseKey(key)
  if (action === 'copy') {
    await copyText(parsed.collection || parsed.database)
    return
  }
  if (action === 'toggle') {
    await toggleBranch(key)
    return
  }
  if (action === 'refresh') {
    if (parsed.collection) await open(key)
    else await refreshAll()
    return
  }
  if (action === 'create-db') {
    showForm('database')
    return
  }
  if (action === 'create-coll') {
    selected.value = key
    database.value = parsed.database
    showForm('collection', key)
    return
  }
  if (action === 'rename' || action === 'index' || action === 'delete') {
    await open(key)
  }
  if (action === 'rename') showForm('rename', key)
  if (action === 'index') showForm('index', key)
  if (action === 'delete') await remove(key)
}

const pathText = computed(() => {
  if (collection.value) return `${database.value}.${collection.value}`
  if (database.value) return database.value
  return '选择一个库'
})

const currentDb = computed(() => dbMeta.value[database.value])

onMounted(async () => {
  busy.value = true
  try {
    await loadDatabases()
  } catch (err) {
    fail(err, '读库失败')
  } finally {
    busy.value = false
  }
})
</script>

<template>
  <section class="fg-mongo">
    <header class="fg-mongo-bar">
      <strong class="fg-mongo-path">{{ pathText }}</strong>
      <div class="fg-mongo-actions">
        <RsButton size="sm" variant="secondary" :disabled="busy" @click="refreshAll">刷新</RsButton>
        <RsButton size="sm" :disabled="busy" @click="showForm('database')">新建库</RsButton>
        <RsButton size="sm" variant="secondary" :disabled="busy || !database" @click="showForm('collection')">新建集合</RsButton>
        <RsButton size="sm" variant="secondary" :disabled="busy || !collection || collection.startsWith('system.')" @click="showForm('rename')">重命名</RsButton>
        <RsButton
          size="sm"
          variant="ghost"
          :disabled="busy || !database || (collection.startsWith('system.') || (!collection && isProtected(database)))"
          @click="remove()"
        >删除</RsButton>
      </div>
    </header>
    <p v-if="error" class="fg-mongo-error">{{ error }}</p>
    <p v-else-if="note" class="fg-mongo-note">{{ note }}</p>
    <form v-if="form === 'database'" class="fg-mongo-form" @submit.prevent="createDatabase">
      <RsInput v-model="dbName" placeholder="库名" :disabled="busy" />
      <RsInput v-model="collName" placeholder="第一个集合名" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy">创建</RsButton>
      <span class="fg-mongo-hint">空库留不住，会先建这个集合。</span>
    </form>
    <form v-else-if="form === 'collection'" class="fg-mongo-form" @submit.prevent="createCollection">
      <RsInput v-model="collName" :placeholder="`在 ${database} 里新建集合`" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy">创建</RsButton>
    </form>
    <form v-else-if="form === 'rename'" class="fg-mongo-form" @submit.prevent="renameCollection">
      <RsInput v-model="newName" placeholder="新的集合名" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy">重命名</RsButton>
    </form>
    <form v-else-if="form === 'index'" class="fg-mongo-form" @submit.prevent="createIndex">
      <RsInput v-model="indexKeys" placeholder="字段:1 或 字段:-1，多项用逗号分开" :disabled="busy" />
      <RsInput v-model="indexName" placeholder="索引名，可空" :disabled="busy" />
      <RsInput v-model="ttl" placeholder="过期秒数，可空" :disabled="busy" />
      <label class="fg-mongo-flag">唯一 <RsSelect v-model="unique" :options="flagOptions" :disabled="busy" /></label>
      <label class="fg-mongo-flag">稀疏 <RsSelect v-model="sparse" :options="flagOptions" :disabled="busy" /></label>
      <RsButton size="sm" type="submit" :disabled="busy">创建索引</RsButton>
    </form>
    <div class="fg-mongo-body">
      <div class="fg-mongo-tree">
        <RsInput v-model="query" class="fg-mongo-filter" size="sm" clearable placeholder="过滤库和已展开的集合" aria-label="过滤库和集合" />
        <RsTree
          v-model="selected"
          v-model:expanded-keys="expanded"
          :nodes="nodes"
          :filter="query.trim()"
          :filter-node="matchNode"
          lazy
          show-line
          block-node
          :disabled="busy"
          :load-data="loadData"
          aria-label="MongoDB 库和集合"
          @node-click="(_node, key) => open(key)"
          @node-contextmenu="onNodeMenu"
        />
        <RsContextMenu ref="nodeMenu" :items="menuItems" aria-label="MongoDB 对象操作" @select="onMenuSelect" />
      </div>
      <div class="fg-mongo-detail">
        <p v-if="!database" class="fg-mongo-note">左侧是库。展开后看集合，点进集合看存储和索引。</p>
        <template v-else-if="!collection">
          <dl class="fg-mongo-stat">
            <div><dt>库</dt><dd>{{ database }}</dd></div>
            <div><dt>占用</dt><dd>{{ bytes(currentDb?.bytes) }}</dd></div>
            <div><dt>数据</dt><dd>{{ currentDb?.empty ? '空' : '有数据' }}</dd></div>
          </dl>
          <p v-if="isProtected(database)" class="fg-mongo-note">admin、local、config 不能在这里删除。</p>
          <div class="fg-mongo-index-bar">
            <strong>慢日志</strong>
            <RsButton size="sm" variant="secondary" :disabled="busy" @click="runProfile">读取</RsButton>
          </div>
          <p v-if="profileNote" class="fg-mongo-note">{{ profileNote }}</p>
          <table v-if="profileRows.length" class="fg-mongo-table">
            <thead>
              <tr>
                <th>操作</th>
                <th>命名空间</th>
                <th class="fg-num">毫秒</th>
                <th>计划</th>
                <th>客户端</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, index) in profileRows" :key="`${item.ns}-${index}`">
                <td>{{ item.op || '—' }}</td>
                <td>{{ item.ns || '—' }}</td>
                <td class="fg-num">{{ item.millis }}</td>
                <td>{{ item.plan || '—' }}</td>
                <td>{{ item.client || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </template>
        <template v-else>
          <dl v-if="stats" class="fg-mongo-stat">
            <div><dt>命名空间</dt><dd>{{ stats.ns }}</dd></div>
            <div><dt>类型</dt><dd>{{ typeText(collType[collKey(database, collection)] || '') }}</dd></div>
            <div><dt>文档</dt><dd>{{ stats.count }}</dd></div>
            <div><dt>数据</dt><dd>{{ bytes(stats.size) }}</dd></div>
            <div><dt>平均大小</dt><dd>{{ bytes(stats.avgObjSize) }}</dd></div>
            <div><dt>存储</dt><dd>{{ bytes(stats.storageSize) }}</dd></div>
            <div><dt>索引</dt><dd>{{ stats.nindexes }}</dd></div>
            <div><dt>索引占用</dt><dd>{{ bytes(stats.totalIndexSize) }}</dd></div>
            <div v-if="stats.capped"><dt>固定集合</dt><dd>是</dd></div>
          </dl>
          <form class="fg-mongo-query" @submit.prevent="runFind">
            <RsTextarea v-model="filterText" :rows="3" placeholder="查询条件，JSON 对象，可空。例如 {&quot;status&quot;:&quot;open&quot;}" :disabled="busy" />
            <div class="fg-mongo-actions">
              <RsInput v-model="limitText" class="fg-mongo-limit" placeholder="条数" :disabled="busy" />
              <RsButton size="sm" type="submit" :disabled="busy || collection.startsWith('system.')">查询</RsButton>
              <RsButton size="sm" variant="secondary" :disabled="busy || collection.startsWith('system.')" @click="runCount">计数</RsButton>
              <RsButton size="sm" variant="secondary" :disabled="busy || collection.startsWith('system.')" @click="runExplain">执行计划</RsButton>
              <RsButton size="sm" variant="secondary" :disabled="busy" @click="runProfile">慢日志</RsButton>
            </div>
          </form>
          <p v-if="counted != null" class="fg-mongo-note">匹配 {{ counted }} 条。</p>
          <dl v-if="plan" class="fg-mongo-stat">
            <div><dt>阶段</dt><dd>{{ plan.stage || '—' }}</dd></div>
            <div><dt>索引</dt><dd>{{ plan.index || '—' }}</dd></div>
            <div><dt>返回</dt><dd>{{ plan.nReturned }}</dd></div>
            <div><dt>扫文档</dt><dd>{{ plan.docsExamined }}</dd></div>
            <div><dt>扫索引</dt><dd>{{ plan.keysExamined }}</dd></div>
            <div><dt>毫秒</dt><dd>{{ plan.millis }}</dd></div>
          </dl>
          <pre v-for="(item, index) in documents" :key="index" class="fg-mongo-doc">{{ item }}</pre>
          <p v-if="profileNote" class="fg-mongo-note">{{ profileNote }}</p>
          <table v-if="profileRows.length" class="fg-mongo-table">
            <thead>
              <tr>
                <th>操作</th>
                <th>命名空间</th>
                <th class="fg-num">毫秒</th>
                <th>计划</th>
                <th>客户端</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(item, index) in profileRows" :key="`slow-${item.ns}-${index}`">
                <td>{{ item.op || '—' }}</td>
                <td>{{ item.ns || '—' }}</td>
                <td class="fg-num">{{ item.millis }}</td>
                <td>{{ item.plan || '—' }}</td>
                <td>{{ item.client || '—' }}</td>
              </tr>
            </tbody>
          </table>
          <div class="fg-mongo-index-bar">
            <strong>索引</strong>
            <RsButton size="sm" variant="secondary" :disabled="busy || collection.startsWith('system.') || collType[collKey(database, collection)] === 'view'" @click="showForm('index')">新建索引</RsButton>
          </div>
          <p v-if="!indexes.length" class="fg-mongo-note">没有索引。</p>
          <table v-else class="fg-mongo-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>键</th>
                <th>属性</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in indexes" :key="item.name">
                <td>{{ item.name }}</td>
                <td>{{ item.key || '—' }}</td>
                <td>{{ indexFlags(item) }}</td>
                <td>
                  <RsButton v-if="item.name !== '_id_'" size="sm" variant="ghost" :disabled="busy" @click="dropIndex(item)">删除</RsButton>
                </td>
              </tr>
            </tbody>
          </table>
        </template>
      </div>
    </div>
  </section>
</template>

<style scoped>
.fg-mongo {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  background: var(--rs-surface);
}
.fg-mongo-bar,
.fg-mongo-actions,
.fg-mongo-form,
.fg-mongo-index-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fg-mongo-bar,
.fg-mongo-index-bar {
  justify-content: space-between;
}
.fg-mongo-bar {
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-mongo-path {
  min-width: 0;
  overflow: hidden;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fg-mongo-error,
.fg-mongo-note,
.fg-mongo-hint {
  margin: 0;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
.fg-mongo-error {
  padding: 8px 12px;
  color: #e23b55;
}
.fg-mongo-note {
  padding: 8px 12px 0;
}
.fg-mongo-form {
  flex-wrap: wrap;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-mongo-body {
  display: grid;
  grid-template-columns: minmax(220px, 320px) minmax(0, 1fr);
  flex: 1;
  min-height: 0;
}
.fg-mongo-tree {
  min-height: 0;
  overflow: auto;
  padding: 8px;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-mongo-filter {
  position: sticky;
  top: 0;
  z-index: 1;
  margin-bottom: 8px;
  background: var(--rs-surface);
}
.fg-mongo-detail {
  min-width: 0;
  min-height: 0;
  overflow: auto;
  padding: 12px;
}
.fg-mongo-stat {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  margin: 0 0 12px;
}
.fg-mongo-stat div {
  display: flex;
  gap: 6px;
}
.fg-mongo-stat dt {
  color: var(--rs-text-secondary, #4d647c);
}
.fg-mongo-stat dd {
  margin: 0;
}
.fg-mongo-index-bar {
  margin: 4px 0 8px;
}
.fg-mongo-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.fg-mongo-table th,
.fg-mongo-table td {
  padding: 6px 10px;
  text-align: left;
  vertical-align: top;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-mongo-table th {
  color: var(--rs-text-secondary, #4d647c);
  font-weight: 600;
}
.fg-mongo-flag,
.fg-mongo-query {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.fg-mongo-flag {
  flex-direction: row;
  align-items: center;
  color: var(--rs-text-secondary, #4d647c);
  font-size: 13px;
}
.fg-mongo-query {
  margin: 8px 0 12px;
}
.fg-mongo-limit {
  width: 88px;
}
.fg-mongo-doc {
  margin: 0 0 8px;
  padding: 8px 10px;
  overflow: auto;
  border: 1px solid var(--fg-panel-border, var(--rs-border));
  border-radius: 6px;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}
.fg-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
</style>
