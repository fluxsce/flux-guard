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
import { execZk, type ZkNodeStat } from '../api'

const props = defineProps<{
  componentId: string
}>()

interface AclRow {
  scheme: string
  id: string
  perms: string
}

const toast = useRsToast()
const path = ref('/')
const selected = ref('/')
const expanded = ref<string[]>([])
const nodes = ref<RsTreeNode[]>([{ key: '/', label: '/', isLeaf: false, children: [] }])
const node = ref<ZkNodeStat | null>(null)
const data = ref('')
const binary = ref(false)
const error = ref('')
const busy = ref(false)
const creating = ref(false)
const moving = ref(false)
const childName = ref('')
const childMode = ref('persistent')
const childData = ref('')
const target = ref('')
const tab = ref<'data' | 'acl'>('data')
const aclRows = ref<AclRow[]>([])
const aversion = ref(-1)
const aclLoaded = ref(false)
const menuKey = ref('/')
const nodeMenu = ref<{ open: (point?: { x: number; y: number }) => void } | null>(null)
const query = ref('')

const menuItems = computed<RsContextMenuItem[]>(() => {
  const key = menuKey.value
  const root = key === '/'
  const treeNode = findNode(nodes.value, key)
  const opened = expanded.value.includes(key)
  const title = key === '/' ? '/' : key.slice(key.lastIndexOf('/') + 1)
  const knownBinary = key === path.value && (!node.value || binary.value)
  return [
    { key: 'head', label: title, separator: true },
    { key: 'refresh', label: '刷新', icon: 'refresh-cw' },
    {
      key: 'toggle',
      label: opened ? '收起' : '展开',
      icon: opened ? 'chevron-down' : 'chevron-right',
      disabled: treeNode?.isLeaf === true,
    },
    { key: 'sep-edit', label: '', separator: true },
    {
      key: 'create',
      label: '新建子节点',
      icon: 'plus',
      children: modeOptions.map((item) => ({ key: `create:${String(item.value)}`, label: String(item.label) })),
    },
    { key: 'copy-path', label: '复制路径', icon: 'copy' },
    { key: 'copy-data', label: '复制数据', icon: 'clipboard', disabled: knownBinary },
    { key: 'sep-move', label: '', separator: true },
    { key: 'move', label: '移动', icon: 'folder-input', disabled: root },
    { key: 'acl', label: '权限', icon: 'shield' },
    { key: 'sep-danger', label: '', separator: true },
    { key: 'delete', label: '删除', icon: 'trash-2', danger: true, disabled: root },
  ]
})

const modeOptions: RsSelectOption[] = [
  { label: '持久', value: 'persistent' },
  { label: '临时', value: 'ephemeral' },
  { label: '持久顺序', value: 'persistent_sequential' },
  { label: '临时顺序', value: 'ephemeral_sequential' },
]
const schemeOptions: RsSelectOption[] = [
  { label: 'world', value: 'world' },
  { label: 'auth', value: 'auth' },
  { label: 'digest', value: 'digest' },
  { label: 'ip', value: 'ip' },
  { label: 'sasl', value: 'sasl' },
]
const permItems = [
  { letter: 'c', label: '创建' },
  { letter: 'r', label: '读取' },
  { letter: 'w', label: '写入' },
  { letter: 'd', label: '删除' },
  { letter: 'a', label: '管理' },
]

function join(parent: string, name: string) {
  if (parent === '/') return `/${name}`
  return `${parent}/${name}`
}

function parentOf(value: string) {
  if (value === '/') return '/'
  const cut = value.lastIndexOf('/')
  return cut <= 0 ? '/' : value.slice(0, cut)
}

function matchNode(node: RsTreeNode, keyword: string) {
  const q = keyword.trim().toLowerCase()
  if (!q) return true
  const label = String(node.label ?? '').toLowerCase()
  if (label.includes(q)) return true
  const key = String(node.key ?? '').toLowerCase()
  if (key === '/') return false
  return key.includes(q)
}

function findNode(list: RsTreeNode[], key: string): RsTreeNode | null {
  for (const item of list) {
    if (item.key === key) return item
    const children = item.children ?? []
    const found = findNode(children, key)
    if (found) return found
  }
  return null
}

function fail(err: unknown, fallback: string) {
  error.value = err instanceof Error ? err.message : getHttpErrorMessage(err, fallback)
  toast.error(error.value)
}

async function call<T>(command: string, args: Record<string, string>) {
  const result = await execZk(props.componentId, command, args)
  if (!isApiSuccess(result)) {
    throw new Error(getApiMessage(result, '目录操作失败'))
  }
  return parseApiData<T>(result)
}

async function fill(treeNode: RsTreeNode, key: string) {
  const listed = await call<{ children?: string[] }>('children', { path: key })
  const names = [...(listed?.children ?? [])].sort((a, b) => a.localeCompare(b))
  treeNode.children = names.map((name) => ({
    key: join(key, name),
    label: name,
    isLeaf: false,
    children: [],
  }))
  treeNode.isLeaf = names.length === 0
}

async function loadData(treeNode: RsTreeNode, key: string) {
  try {
    await fill(treeNode, key)
  } catch (err) {
    fail(err, '目录操作失败')
    throw err
  }
}

async function refreshChildren(parent: string) {
  const treeNode = findNode(nodes.value, parent)
  if (!treeNode) return
  await fill(treeNode, parent)
}

function rememberScheme(scheme: string) {
  if (scheme && !schemeOptions.some((item) => item.value === scheme)) {
    schemeOptions.push({ label: scheme, value: scheme })
  }
}

async function loadAcl() {
  const got = await call<{ aversion?: number; acl?: AclRow[] }>('acl', { path: path.value })
  aversion.value = got?.aversion ?? -1
  aclRows.value = (got?.acl ?? []).map((row) => {
    rememberScheme(row.scheme)
    return { scheme: row.scheme, id: row.id ?? '', perms: row.perms ?? '' }
  })
  aclLoaded.value = true
}

async function open(next: string) {
  error.value = ''
  busy.value = true
  creating.value = false
  moving.value = false
  try {
    const got = await call<ZkNodeStat>('get', { path: next })
    path.value = next
    selected.value = next
    node.value = got ?? null
    binary.value = !!got?.dataBase64 && got.data == null
    data.value = got?.data ?? ''
    aclRows.value = []
    aclLoaded.value = false
    if (tab.value === 'acl') await loadAcl()
  } catch (err) {
    fail(err, '目录操作失败')
    selected.value = path.value
  } finally {
    busy.value = false
  }
}

async function refresh() {
  busy.value = true
  error.value = ''
  try {
    await refreshChildren(path.value)
    await open(path.value)
  } catch (err) {
    fail(err, '目录操作失败')
    busy.value = false
  }
}

async function save() {
  if (!node.value || binary.value) return
  busy.value = true
  error.value = ''
  try {
    const version = node.value.version == null ? '' : String(node.value.version)
    await call('set', { path: path.value, data: data.value, version })
    toast.success('已保存')
    await open(path.value)
  } catch (err) {
    fail(err, '保存失败')
    busy.value = false
  }
}

async function remove() {
  if (path.value === '/') return
  const recursive = (node.value?.numChildren ?? 0) > 0
  const ok = await rsConfirm.confirm({
    title: recursive ? '删除子树' : '删除节点',
    description: recursive ? `删除 ${path.value} 及其全部子节点。` : `删除 ${path.value}。`,
    confirmText: '删除',
    confirmVariant: 'danger',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    const args: Record<string, string> = { path: path.value }
    if (recursive) args.recursive = 'Y'
    else if (node.value?.version != null) args.version = String(node.value.version)
    await call('delete', args)
    toast.success(recursive ? '已删除子树' : '已删除')
    const parent = parentOf(path.value)
    await refreshChildren(parent)
    await open(parent)
  } catch (err) {
    fail(err, '删除失败')
    busy.value = false
  }
}

function targetPath(raw: string) {
  const text = raw.trim()
  if (!text || text.includes('\0') || text === '.' || text === '..') {
    throw new Error('目标路径无效')
  }
  if (text.startsWith('/')) {
    if (text.length > 1 && text.endsWith('/')) throw new Error('目标路径无效')
    if (text.split('/').slice(1).some((part) => !part || part === '.' || part === '..')) {
      throw new Error('目标路径无效')
    }
    return text
  }
  if (text.includes('/')) throw new Error('目标路径无效')
  return join(parentOf(path.value), text)
}

async function move() {
  if (path.value === '/') return
  let dest = ''
  try {
    dest = targetPath(target.value)
  } catch (err) {
    fail(err, '目标路径无效')
    return
  }
  const ephemeral = !!node.value?.ephemeralOwner
  const ok = await rsConfirm.confirm({
    title: '移动节点',
    description: ephemeral
      ? `把 ${path.value} 移动到 ${dest}。临时节点会改挂到当前连接，连接断开后消失。`
      : `把 ${path.value} 移动到 ${dest}。`,
    confirmText: '移动',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    await call('move', { path: path.value, dest })
    const from = parentOf(path.value)
    await refreshChildren(from)
    await refreshChildren(parentOf(dest))
    target.value = ''
    moving.value = false
    toast.success('已移动')
    await open(dest)
  } catch (err) {
    fail(err, '移动失败')
    busy.value = false
  }
}

async function createChild() {
  const name = childName.value.trim()
  if (!name || name.includes('/') || name === '.' || name === '..') {
    toast.error('节点名无效')
    return
  }
  busy.value = true
  error.value = ''
  try {
    const created = await call<{ path: string }>('create', {
      path: join(path.value, name),
      data: childData.value,
      mode: childMode.value,
    })
    const next = created?.path || join(path.value, name)
    childName.value = ''
    childData.value = ''
    creating.value = false
    await refreshChildren(path.value)
    if (!expanded.value.includes(path.value)) expanded.value = [...expanded.value, path.value]
    toast.success('已新建')
    await open(next)
  } catch (err) {
    fail(err, '新建失败')
    busy.value = false
  }
}

function togglePerm(row: AclRow, letter: string) {
  const has = new Set(row.perms)
  if (has.has(letter)) has.delete(letter)
  else has.add(letter)
  row.perms = permItems.map((item) => item.letter).filter((item) => has.has(item)).join('')
}

function addAcl() {
  aclRows.value = [...aclRows.value, { scheme: 'world', id: 'anyone', perms: 'r' }]
}

function dropAcl(index: number) {
  aclRows.value = aclRows.value.filter((_, at) => at !== index)
}

async function showAcl() {
  tab.value = 'acl'
  if (aclLoaded.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    await loadAcl()
  } catch (err) {
    fail(err, '读取权限失败')
  } finally {
    busy.value = false
  }
}

async function saveAcl() {
  if (!aclRows.value.length || aclRows.value.some((row) => !row.perms)) {
    toast.error('每条权限至少要有一项')
    return
  }
  const ok = await rsConfirm.confirm({
    title: '修改权限',
    description: `更新 ${path.value} 的 ACL。`,
    confirmText: '保存',
    overlayBlur: 0,
  })
  if (!ok) return
  busy.value = true
  error.value = ''
  try {
    const version = aversion.value < 0 ? '' : String(aversion.value)
    await call('setacl', {
      path: path.value,
      acl: JSON.stringify(aclRows.value.map((row) => ({ scheme: row.scheme, id: row.id, perms: row.perms }))),
      version,
    })
    toast.success('已更新权限')
    await loadAcl()
  } catch (err) {
    fail(err, '更新权限失败')
  } finally {
    busy.value = false
  }
}

function when(ms?: number) {
  if (!ms) return '—'
  const date = new Date(ms)
  if (Number.isNaN(date.getTime())) return '—'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function zxidText(value?: number) {
  if (value == null) return '—'
  return `0x${value.toString(16)}`
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success('已复制')
  } catch {
    toast.error('复制失败')
  }
}

async function useNode(key: string) {
  selected.value = key
  if (path.value === key && node.value) return true
  await open(key)
  return path.value === key && !!node.value
}

async function toggleBranch(key: string) {
  const treeNode = findNode(nodes.value, key)
  if (!treeNode || treeNode.isLeaf) return
  if (expanded.value.includes(key)) {
    expanded.value = expanded.value.filter((item) => item !== key)
    return
  }
  if (!(treeNode.children?.length)) {
    busy.value = true
    try {
      await fill(treeNode, key)
    } catch (err) {
      fail(err, '目录操作失败')
      busy.value = false
      return
    }
    busy.value = false
  }
  if (treeNode.isLeaf || expanded.value.includes(key)) return
  expanded.value = [...expanded.value, key]
}

async function refreshNode(key: string) {
  const treeNode = findNode(nodes.value, key)
  if (!treeNode) return
  busy.value = true
  error.value = ''
  try {
    await fill(treeNode, key)
  } catch (err) {
    fail(err, '目录操作失败')
    busy.value = false
    return
  }
  const alive = path.value === key || !!findNode(nodes.value, path.value)
  if (path.value === key || !alive) {
    await open(alive ? path.value : key)
    return
  }
  busy.value = false
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
  if (action === 'copy-path') {
    await copyText(key)
    return
  }
  if (action === 'toggle') {
    await toggleBranch(key)
    return
  }
  if (action === 'refresh') {
    await refreshNode(key)
    return
  }
  if (!(await useNode(key))) return
  if (action.startsWith('create:')) {
    const mode = action.slice('create:'.length)
    if (modeOptions.some((item) => item.value === mode)) childMode.value = mode
    creating.value = true
    moving.value = false
    return
  }
  if (action === 'move') {
    moving.value = true
    creating.value = false
    target.value = ''
    return
  }
  if (action === 'acl') {
    await showAcl()
    return
  }
  if (action === 'copy-data') {
    if (binary.value) {
      toast.error('节点数据不是文本')
      return
    }
    await copyText(data.value)
    return
  }
  if (action === 'delete') await remove()
}

onMounted(async () => {
  const root = nodes.value[0]
  if (!root) return
  try {
    await fill(root, '/')
    expanded.value = ['/']
  } catch (err) {
    fail(err, '目录操作失败')
  }
  await open('/')
})
</script>

<template>
  <section class="fg-zk">
    <header class="fg-zk-bar">
      <strong class="fg-zk-path">{{ path }}</strong>
      <div class="fg-zk-actions">
        <RsButton size="sm" variant="secondary" :disabled="busy" @click="refresh">刷新</RsButton>
        <RsButton size="sm" :disabled="busy" @click="creating = !creating">新建</RsButton>
        <RsButton size="sm" variant="secondary" :disabled="busy || path === '/'" @click="moving = !moving">移动</RsButton>
        <RsButton size="sm" :disabled="busy || tab !== 'data' || binary || !node" @click="save">保存</RsButton>
        <RsButton size="sm" variant="ghost" :disabled="busy || path === '/'" @click="remove">删除</RsButton>
      </div>
    </header>
    <p v-if="error" class="fg-zk-error">{{ error }}</p>
    <form v-if="creating" class="fg-zk-create" @submit.prevent="createChild">
      <RsInput v-model="childName" placeholder="子节点名" :disabled="busy" />
      <RsSelect v-model="childMode" :options="modeOptions" :disabled="busy" />
      <RsTextarea v-model="childData" placeholder="数据，可空" :disabled="busy" :rows="3" />
      <RsButton size="sm" type="submit" :disabled="busy">创建</RsButton>
    </form>
    <form v-if="moving" class="fg-zk-create" @submit.prevent="move">
      <RsInput v-model="target" placeholder="新名字，或完整目标路径" :disabled="busy" />
      <RsButton size="sm" type="submit" :disabled="busy || path === '/'">移动</RsButton>
    </form>
    <div class="fg-zk-body">
      <div class="fg-zk-tree">
        <RsInput
          v-model="query"
          class="fg-zk-filter"
          size="sm"
          clearable
          placeholder="过滤已展开的节点"
          aria-label="过滤已展开的节点"
        />
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
          aria-label="ZooKeeper 节点"
          @node-click="(_node, key) => open(key)"
          @node-contextmenu="onNodeMenu"
        />
        <RsContextMenu ref="nodeMenu" :items="menuItems" aria-label="ZooKeeper 节点操作" @select="onMenuSelect" />
      </div>
      <div class="fg-zk-detail">
        <div class="fg-zk-tabs">
          <button type="button" :data-on="tab === 'data'" @click="tab = 'data'">数据</button>
          <button type="button" :data-on="tab === 'acl'" @click="showAcl">权限</button>
        </div>
        <dl v-if="node && tab === 'data'" class="fg-zk-stat">
          <div><dt>数据版本</dt><dd>{{ node.version ?? '—' }}</dd></div>
          <div><dt>子节点版本</dt><dd>{{ node.cversion ?? '—' }}</dd></div>
          <div><dt>权限版本</dt><dd>{{ node.aversion ?? '—' }}</dd></div>
          <div><dt>子节点</dt><dd>{{ node.numChildren ?? '—' }}</dd></div>
          <div><dt>数据长度</dt><dd>{{ node.dataLength ?? '—' }}</dd></div>
          <div><dt>临时</dt><dd>{{ node.ephemeralOwner ? '是' : '否' }}</dd></div>
          <div><dt>创建时间</dt><dd>{{ when(node.ctime) }}</dd></div>
          <div><dt>修改时间</dt><dd>{{ when(node.mtime) }}</dd></div>
          <div><dt>创建事务</dt><dd class="fg-mono">{{ zxidText(node.czxid) }}</dd></div>
          <div><dt>修改事务</dt><dd class="fg-mono">{{ zxidText(node.mzxid) }}</dd></div>
          <div><dt>子节点事务</dt><dd class="fg-mono">{{ zxidText(node.pzxid) }}</dd></div>
        </dl>
        <template v-if="tab === 'data'">
          <p v-if="binary" class="fg-zk-note">节点数据不是文本，这里不展示，也不能在页面上改。移动和删除仍可用。</p>
          <RsTextarea v-else v-model="data" class="fg-zk-data" :disabled="busy || !node" :rows="12" />
        </template>
        <div v-else class="fg-zk-acl">
          <p class="fg-zk-note">权限版本 {{ aversion < 0 ? '—' : aversion }}。digest 填用户名:密码；已经是摘要的会原样保存。auth 的标识可空。</p>
          <div v-for="(row, index) in aclRows" :key="index" class="fg-zk-acl-row">
            <RsSelect v-model="row.scheme" :options="schemeOptions" :disabled="busy" />
            <RsInput v-model="row.id" placeholder="标识" :disabled="busy" />
            <div class="fg-zk-perms">
              <label v-for="item in permItems" :key="item.letter">
                <input
                  type="checkbox"
                  :disabled="busy"
                  :checked="row.perms.includes(item.letter)"
                  @change="togglePerm(row, item.letter)"
                />
                {{ item.label }}
              </label>
            </div>
            <RsButton size="sm" variant="ghost" :disabled="busy" @click="dropAcl(index)">移除</RsButton>
          </div>
          <div class="fg-zk-actions">
            <RsButton size="sm" variant="secondary" :disabled="busy" @click="addAcl">添加</RsButton>
            <RsButton size="sm" :disabled="busy" @click="saveAcl">保存权限</RsButton>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.fg-zk {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  background: var(--rs-surface);
}
.fg-zk-bar,
.fg-zk-actions,
.fg-zk-create,
.fg-zk-tabs,
.fg-zk-acl-row,
.fg-zk-perms {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fg-zk-bar {
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-zk-path {
  min-width: 0;
  overflow: hidden;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fg-zk-error {
  margin: 0;
  padding: 8px 12px;
  color: #e23b55;
}
.fg-zk-create {
  align-items: stretch;
  padding: 8px 12px;
  border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-zk-body {
  display: grid;
  grid-template-columns: minmax(220px, 320px) minmax(0, 1fr);
  flex: 1;
  min-height: 0;
}
.fg-zk-tree {
  min-height: 0;
  overflow: auto;
  padding: 8px;
  border-right: 1px solid var(--fg-panel-border, var(--rs-border));
}
.fg-zk-filter {
  position: sticky;
  top: 0;
  z-index: 1;
  margin-bottom: 8px;
  background: var(--rs-surface);
}
.fg-zk-detail {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 12px;
  gap: 8px;
  overflow: auto;
}
.fg-zk-tabs button {
  border: 0;
  background: transparent;
  color: var(--rs-text-secondary);
  cursor: pointer;
  font: inherit;
}
.fg-zk-tabs button[data-on='true'] {
  color: var(--rs-text-primary);
  font-weight: 650;
}
.fg-zk-stat {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 20px;
  margin: 0;
}
.fg-zk-stat div {
  display: flex;
  gap: 6px;
}
.fg-zk-stat dt,
.fg-zk-note {
  color: var(--rs-text-secondary);
}
.fg-zk-note {
  margin: 0;
  font-size: 13px;
}
.fg-zk-stat dd {
  margin: 0;
}
.fg-mono {
  font-family: var(--rs-font-mono, ui-monospace, monospace);
}
.fg-zk-data {
  flex: 1;
  min-height: 160px;
}
.fg-zk-acl {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.fg-zk-acl-row {
  align-items: center;
  flex-wrap: wrap;
}
.fg-zk-perms {
  flex-wrap: wrap;
  font-size: 13px;
}
.fg-zk-perms label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>
