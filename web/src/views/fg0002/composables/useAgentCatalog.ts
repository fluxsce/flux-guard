import { computed, onMounted, onScopeDispose, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { getDefaultPageSize, pageSizePolicy } from '@/api/pagination'
import type { FgFormField, FgFormMode, FgSearchField } from '@/components/form/types'
import type { FgGridMenuItem } from '@/components/grid/types'
import { useSessionStore } from '@/stores/session'
import { rsConfirm, useRsToast } from '@/ui'
import {
  deleteAgents,
  dispatchCommand,
  queryAgents,
  queryAgentUpgradeStatuses,
  queryCommand,
  registerAgent,
  renameAgent,
  type Agent,
  type AgentUpgradeStatus,
  type CommandView,
} from '../api'

export type FlagOption = { label: string; value: string }

export const onlineOptions: FlagOption[] = [
  { label: '在线', value: 'Y' },
  { label: '离线', value: 'N' },
]

export const agentCodeOptions: FlagOption[] = [
  { label: '主机', value: 'host' },
  { label: 'Go', value: 'go' },
  { label: 'Go', value: 'app' },
  { label: 'Java', value: 'java' },
  { label: '应用', value: 'appside' },
]

export const agentColumns = [
  { key: 'agentName', title: '名称', minWidth: 140 },
  { key: 'agentCode', title: '种类', width: 88 },
  { key: 'agentVersion', title: '版本', width: 100 },
  { key: 'upgradeStatus', title: '升级', width: 100 },
  { key: 'goos', title: '系统', width: 80 },
  { key: 'goarch', title: '架构', width: 80 },
  { key: 'machineId', title: '机器编号', minWidth: 160 },
  { key: 'hostName', title: '主机', minWidth: 140 },
  { key: 'ipAddress', title: '地址', minWidth: 140 },
  { key: 'commandAddress', title: '命令端口', minWidth: 160 },
  { key: 'onlineFlag', title: '在线', width: 88 },
  { key: 'lastSeenTime', title: '最近心跳', width: 180 },
]

/** 升级命令/队列状态文案。 */
export function upgradeStatusText(status: string, matched?: string) {
  if (status === 'pending') return '待继续'
  if (status === 'done') {
    if (matched === 'Y') return '已对齐'
    if (matched === 'N') return '未对齐'
    return '待上报'
  }
  if (status === 'failed') return '失败'
  if (status === 'waiting') return '待取'
  if (status === 'running') return '执行中'
  if (status === 'dialing') return '拨号'
  return status || '—'
}

/** 升级状态标签色。 */
export function upgradeStatusVariant(status: string, matched?: string): 'success' | 'danger' | 'warning' | 'default' {
  if (status === 'failed' || (status === 'done' && matched === 'N')) return 'danger'
  if (status === 'done' && matched === 'Y') return 'success'
  if (status === 'pending' || status === 'waiting' || status === 'dialing' || status === 'running' || status === 'done') {
    return 'warning'
  }
  return 'default'
}

/** 代理种类展示名。同机可同时跑主机 / Go / 应用代理，列表用种类区分。 */
export function agentCodeLabel(value: string) {
  return flagLabel(agentCodeOptions, value)
}

/** 是否支持中央自更新的代理种类。 */
export function isUpgradableAgent(row: { agentCode?: string } | null | undefined) {
  if (!row?.agentCode) return false
  return row.agentCode === 'host' || row.agentCode === 'go' || row.agentCode === 'appside'
}

export function isHostAgent(row: { agentCode?: string } | null | undefined) {
  if (!row) return false
  return !row.agentCode || row.agentCode === 'host'
}

export const agentMenuItems: FgGridMenuItem[] = [
  { key: 'view', label: '查看', icon: 'eye' },
  { key: 'rename', label: '改名', icon: 'pencil', code: 'fg0002:edit' },
  {
    key: 'collect',
    label: '立即采集',
    icon: 'play',
    code: 'fg0002:command',
    disabled: (row) => !isHostAgent(row),
  },
  {
    key: 'interval',
    label: '修改间隔',
    icon: 'timer',
    code: 'fg0002:command',
    disabled: (row) => !isHostAgent(row),
  },
  {
    key: 'shell',
    label: '打开终端',
    icon: 'square-terminal',
    code: 'fg0002:shell',
    disabled: (row) => !isHostAgent(row),
  },
  { key: 'history', label: '操作记录', icon: 'history', code: 'fg0002:view' },
  { key: 'delete', label: '删除', icon: 'trash', code: 'fg0002:delete', danger: true },
]

export const agentSearchFields: FgSearchField[] = [
  { field: 'agentName', label: '名称', placeholder: '请输入名称' },
  { field: 'hostName', label: '主机', placeholder: '请输入主机' },
  {
    field: 'onlineFlag',
    label: '在线',
    type: 'select',
    placeholder: '全部',
    options: [{ label: '全部', value: '' }, ...onlineOptions],
  },
]

export const agentCreateFields: FgFormField[] = [
  { field: 'agentName', label: '名称', required: true, placeholder: '在列表里认出这台代理' },
  { field: 'hostName', label: '主机' },
]

export const agentRenameFields: FgFormField[] = [
  { field: 'agentName', label: '显示名', required: true, placeholder: '只改列表显示，不影响身份分行' },
]

export const agentFormFields: FgFormField[] = [
  { field: 'agentName', label: '名称' },
  { field: 'agentCode', label: '种类' },
  { field: 'agentVersion', label: '版本' },
  { field: 'goos', label: '系统' },
  { field: 'goarch', label: '架构' },
  { field: 'machineId', label: '机器编号' },
  { field: 'hostName', label: '主机' },
  { field: 'ipAddress', label: '地址' },
  { field: 'commandAddress', label: '命令端口' },
  { field: 'onlineFlag', label: '在线', type: 'select', options: onlineOptions },
  { field: 'lastSeenTime', label: '最近心跳' },
]

export const intervalFields: FgFormField[] = [
  { field: 'intervalSec', label: '间隔（秒）', type: 'number', required: true },
]

export function flagLabel(options: FlagOption[], value: string) {
  return options.find((item) => item.value === value)?.label || value || '—'
}

const statusLabel: Record<string, string> = {
  dialing: '拨号中',
  waiting: '待取',
  running: '执行中',
  done: '完成',
  failed: '失败',
}

function deliverModeLabel(mode?: string) {
  if (mode === 'push') return '拨入'
  if (mode === 'pull') return '拉取'
  return ''
}

function formatCommand(view: CommandView) {
  const status = statusLabel[view.status] ?? view.status
  return [status, deliverModeLabel(view.deliverMode), view.message].filter(Boolean).join(' · ')
}

function commandPending(status: string) {
  return status === 'dialing' || status === 'waiting' || status === 'running'
}

function str(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return value.toString()
  return ''
}

function text(row: Agent, key: keyof Agent) {
  const value = row[key]
  if (typeof value === 'string') return value
  return ''
}

function includes(source: string | undefined, keyword: string) {
  if (!keyword) return true
  return (source ?? '').toLowerCase().includes(keyword.toLowerCase())
}

export function useAgentCatalog() {
  const toast = useRsToast()
  const session = useSessionStore()
  const allow = (code: string) => session.allowButton(code)
  const rows = ref<Agent[]>([])
  const page = ref(1)
  const pageSize = ref(getDefaultPageSize())
  const query = ref({ agentName: '', hostName: '', onlineFlag: '' })
  const error = ref('')
  const formOpen = ref(false)
  const formMode = ref<FgFormMode>('view')
  const current = ref<Record<string, unknown>>({})
  const active = ref<Agent | null>(null)
  const selectedKeys = ref<string[]>([])
  const intervalOpen = ref(false)
  const intervalCurrent = ref<Record<string, unknown>>({ intervalSec: 15 })
  const renameOpen = ref(false)
  const renameCurrent = ref<Record<string, unknown>>({})
  const commandBusy = ref(false)
  const upgradeByAgent = ref<Record<string, AgentUpgradeStatus>>({})
  let commandToken = 0
  let upgradePollTimer = 0

  const filtered = computed(() =>
    rows.value.filter((row) => {
      if (!includes(row.agentName, query.value.agentName.trim())) return false
      if (!includes(row.hostName, query.value.hostName.trim())) return false
      if (query.value.onlineFlag && row.onlineFlag !== query.value.onlineFlag) return false
      return true
    }),
  )
  const total = computed(() => filtered.value.length)
  const pageRows = computed(() => {
    const start = (page.value - 1) * pageSize.value
    return filtered.value.slice(start, start + pageSize.value)
  })

  function fill(row: Agent) {
    return {
      agentId: text(row, 'agentId'),
      agentName: text(row, 'agentName'),
      agentCode: text(row, 'agentCode'),
      agentVersion: text(row, 'agentVersion'),
      goos: text(row, 'goos'),
      goarch: text(row, 'goarch'),
      machineId: text(row, 'machineId'),
      hostName: text(row, 'hostName'),
      ipAddress: text(row, 'ipAddress'),
      commandAddress: text(row, 'commandAddress'),
      onlineFlag: text(row, 'onlineFlag'),
      lastSeenTime: text(row, 'lastSeenTime'),
    }
  }

  function syncActive() {
    if (active.value && !filtered.value.some((item) => item.agentId === active.value?.agentId)) {
      active.value = null
    }
  }

  function syncSelected() {
    const alive = new Set(filtered.value.map((item) => item.agentId))
    selectedKeys.value = selectedKeys.value.filter((id) => alive.has(id))
  }

  function scheduleUpgradePoll() {
    if (upgradePollTimer) {
      window.clearTimeout(upgradePollTimer)
      upgradePollTimer = 0
    }
    const active = Object.values(upgradeByAgent.value).some(
      (item) =>
        item.status === 'pending' ||
        item.status === 'waiting' ||
        item.status === 'dialing' ||
        item.status === 'running' ||
        (item.status === 'done' && item.versionMatched !== 'Y'),
    )
    if (!active) return
    upgradePollTimer = window.setTimeout(() => {
      void reloadUpgradeStatuses()
    }, 4000)
  }

  async function reloadUpgradeStatuses() {
    if (!allow('fg0002:search')) return
    const result = await queryAgentUpgradeStatuses()
    if (!isApiSuccess(result)) return
    const list = parseApiData<AgentUpgradeStatus[]>(result) ?? []
    const next: Record<string, AgentUpgradeStatus> = {}
    for (const item of list) {
      next[item.agentId] = item
    }
    upgradeByAgent.value = next
    scheduleUpgradePoll()
  }

  async function reload() {
    if (!allow('fg0002:search')) return
    const result = await queryAgents()
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    rows.value = parseApiData<Agent[]>(result) ?? []
    syncActive()
    syncSelected()
    void reloadUpgradeStatuses()
  }

  function openView(row: Agent) {
    if (!allow('fg0002:view')) return
    active.value = row
    formMode.value = 'view'
    current.value = fill(row)
    formOpen.value = true
  }

  function openCreate() {
    if (!allow('fg0002:edit')) return
    formMode.value = 'create'
    current.value = { agentName: '', hostName: '' }
    formOpen.value = true
  }

  function openInterval(row?: Agent) {
    const target = row ?? active.value
    if (!target || !allow('fg0002:command') || !isHostAgent(target)) return
    active.value = target
    intervalCurrent.value = { intervalSec: 15 }
    intervalOpen.value = true
  }

  function openRename(row?: Agent) {
    const target = row ?? active.value
    if (!target || !allow('fg0002:edit')) return
    active.value = target
    renameCurrent.value = { agentName: target.agentName || '' }
    renameOpen.value = true
  }

  async function submitRename(values: Record<string, unknown>) {
    const target = active.value
    if (!target || !allow('fg0002:edit')) return
    const agentName = str(values.agentName).trim()
    if (!agentName) {
      error.value = '请填写显示名'
      toast.error(error.value)
      return
    }
    error.value = ''
    try {
      const result = await renameAgent(target.agentId, agentName)
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '改名失败')
        toast.error(error.value)
        return
      }
      renameOpen.value = false
      toast.success('已更新显示名')
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '改名失败')
      toast.error(error.value)
    }
  }

  async function submitAgent(values: Record<string, unknown>) {
    if (formMode.value !== 'create' || !allow('fg0002:edit')) return
    const agentName = str(values.agentName).trim()
    if (!agentName) {
      error.value = '请填写名称'
      toast.error(error.value)
      return
    }
    error.value = ''
    try {
      const result = await registerAgent(agentName, str(values.hostName).trim())
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '登记失败')
        toast.error(error.value)
        return
      }
      formOpen.value = false
      page.value = 1
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '登记失败')
      toast.error(error.value)
    }
  }

  function cancelled(token: number) {
    return token !== commandToken
  }

  async function waitCommand(view: CommandView, token: number) {
    let current = view
    while (commandPending(current.status)) {
      await new Promise((resolve) => setTimeout(resolve, 1000))
      if (cancelled(token)) return null
      const next = await queryCommand(current.commandId)
      if (cancelled(token)) return null
      if (!isApiSuccess(next)) {
        error.value = getApiMessage(next, '查询命令失败')
        toast.error(error.value)
        return null
      }
      const parsed = parseApiData<CommandView>(next)
      if (!parsed) return null
      current = parsed
    }
    return current
  }

  function reportCommand(view: CommandView) {
    const text = formatCommand(view)
    if (view.status === 'failed') {
      error.value = text
      toast.error(text)
      return
    }
    toast.success(text)
  }

  async function sendCommand(row: Agent, action: string, intervalSec?: number) {
    if (!allow('fg0002:command') || commandBusy.value) return
    if (!isHostAgent(row)) {
      error.value = '只接受主机代理'
      toast.error(error.value)
      return
    }
    const token = ++commandToken
    error.value = ''
    commandBusy.value = true
    active.value = row
    try {
      const result = await dispatchCommand(row.agentId, action, intervalSec)
      if (cancelled(token) || !isApiSuccess(result)) {
        if (!cancelled(token) && !isApiSuccess(result)) {
          error.value = getApiMessage(result, '下发失败')
          toast.error(error.value)
        }
        return
      }
      const view = parseApiData<CommandView>(result)
      if (!view) return
      const done = await waitCommand(view, token)
      if (!done || cancelled(token)) return
      reportCommand(done)
      await reload()
    } catch (err) {
      if (cancelled(token)) return
      error.value = getHttpErrorMessage(err, '下发失败')
      toast.error(error.value)
    } finally {
      if (!cancelled(token)) commandBusy.value = false
    }
  }

  function collect(row?: Agent) {
    const target = row ?? active.value
    if (!target) return
    void sendCommand(target, 'host.collect.now')
  }

  async function submitInterval(values: Record<string, unknown>) {
    const target = active.value
    if (!target) return
    const intervalSec = Number(str(values.intervalSec))
    if (!Number.isFinite(intervalSec) || intervalSec <= 0) {
      error.value = '请填写间隔秒数'
      toast.error(error.value)
      return
    }
    intervalOpen.value = false
    await sendCommand(target, 'host.interval.set', intervalSec)
  }

  function onRow(row: Agent) {
    active.value = row
  }

  /** 解析待删代理：右键传入单行；工具栏优先勾选，否则用当前高亮行。 */
  function targetsForRemove(row?: Agent): Agent[] {
    if (row) return [row]
    if (selectedKeys.value.length > 0) {
      const picked = new Set(selectedKeys.value)
      return filtered.value.filter((item) => picked.has(item.agentId))
    }
    return active.value ? [active.value] : []
  }

  async function removeAgents(row?: Agent) {
    if (!allow('fg0002:delete')) return
    const targets = targetsForRemove(row)
    if (targets.length === 0) {
      error.value = '请先勾选或选中要删除的代理'
      toast.error(error.value)
      return
    }
    const names = targets.map((item) => item.agentName || item.agentId).join('、')
    const ok = await rsConfirm.confirm({
      title: '删除代理',
      description:
        targets.length === 1
          ? `确定删除代理 ${names}？将从库中移除该代理及身份、终端、命令等关联数据；同机再次上报会新建一行。`
          : `确定删除选中的 ${targets.length} 台代理（${names}）？将从库中移除这些代理及身份、终端、命令等关联数据。`,
      confirmText: '删除',
      confirmVariant: 'danger',
      overlayBlur: 0,
      teleportTo: '#fg0002',
    })
    if (!ok) return
    error.value = ''
    try {
      const result = await deleteAgents(targets.map((item) => item.agentId))
      if (!isApiSuccess(result)) {
        error.value = getApiMessage(result, '删除失败')
        toast.error(error.value)
        return
      }
      const deleted = parseApiData<{ deleted?: number }>(result)?.deleted ?? targets.length
      if (active.value && targets.some((item) => item.agentId === active.value?.agentId)) {
        active.value = null
      }
      selectedKeys.value = []
      toast.success(`已删除 ${deleted} 台代理`)
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '删除失败')
      toast.error(error.value)
    }
  }

  async function onSearch(values: Record<string, unknown>) {
    page.value = 1
    selectedKeys.value = []
    query.value = {
      agentName: str(values.agentName),
      hostName: str(values.hostName),
      onlineFlag: str(values.onlineFlag),
    }
    await reload()
  }

  async function onReset() {
    page.value = 1
    selectedKeys.value = []
    query.value = { agentName: '', hostName: '', onlineFlag: '' }
    await reload()
  }

  watch([total, pageSize], () => {
    const maxPage = Math.max(1, Math.ceil(total.value / pageSize.value) || 1)
    if (page.value > maxPage) page.value = maxPage
  })

  watch(pageSizePolicy, (policy) => {
    if (pageSize.value > policy.maxPageSize || pageSize.value < 1) {
      pageSize.value = policy.defaultPageSize
    }
  })

  onMounted(async () => {
    try {
      await reload()
    } catch (err) {
      error.value = getHttpErrorMessage(err, '加载失败')
      toast.error(error.value)
    }
  })

  onScopeDispose(() => {
    commandToken += 1
    if (upgradePollTimer) {
      window.clearTimeout(upgradePollTimer)
      upgradePollTimer = 0
    }
  })

  return {
    page,
    pageSize,
    total,
    error,
    formOpen,
    formMode,
    current,
    active,
    selectedKeys,
    rows,
    pageRows,
    intervalOpen,
    intervalCurrent,
    renameOpen,
    renameCurrent,
    commandBusy,
    upgradeByAgent,
    openView,
    openCreate,
    openInterval,
    openRename,
    submitAgent,
    submitInterval,
    submitRename,
    collect,
    removeAgents,
    onRow,
    onSearch,
    onReset,
    reload,
    reloadUpgradeStatuses,
  }
}
