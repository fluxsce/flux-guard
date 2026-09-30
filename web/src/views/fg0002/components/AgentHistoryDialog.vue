<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import {
  RsButton,
  RsDialog,
  RsEmpty,
  RsTable,
  RsTabs,
  RsTag,
  useRsToast,
  type RsTabItem,
  type RsTableColumn,
} from '@/ui'
import {
  queryCommands,
  queryShellAudits,
  type Agent,
  type CommandHistory,
  type ShellAudit,
} from '../api'

const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{
  agent: Agent | null
  moduleId: string
}>()

const toast = useRsToast()
const tab = ref('commands')
const loading = ref(false)
const commands = ref<CommandHistory[]>([])
const audits = ref<ShellAudit[]>([])

const title = computed(() => {
  const name = props.agent?.agentName || props.agent?.agentId || ''
  return name ? `操作记录 · ${name}` : '操作记录'
})

const tabItems: RsTabItem[] = [
  { value: 'commands', label: '命令回执' },
  { value: 'shell', label: '终端账本' },
]

const commandColumns: RsTableColumn<CommandHistory>[] = [
  { key: 'addTime', title: '时间', width: 170 },
  { key: 'action', title: '动作', minWidth: 140 },
  { key: 'status', title: '状态', width: 88 },
  { key: 'deliverMode', title: '投递', width: 72 },
  { key: 'message', title: '回执', minWidth: 180 },
  { key: 'addWho', title: '操作人', width: 100 },
]

const auditColumns: RsTableColumn<ShellAudit>[] = [
  { key: 'addTime', title: '时间', width: 170 },
  { key: 'eventType', title: '事件', width: 88 },
  { key: 'userName', title: '操作人', width: 100 },
  { key: 'detailText', title: '说明 / 命令行', minWidth: 220 },
  { key: 'shellId', title: '终端号', width: 120 },
]

const statusLabel: Record<string, string> = {
  dialing: '拨号中',
  waiting: '待取',
  running: '执行中',
  done: '完成',
  failed: '失败',
}

const eventLabel: Record<string, string> = {
  OPEN: '打开',
  CLOSE: '关闭',
  INPUT: '输入',
}

const actionLabel: Record<string, string> = {
  'host.collect.now': '立即采集',
  'host.interval.set': '修改间隔',
}

function formatAction(row: CommandHistory) {
  const base = actionLabel[row.action] || row.action
  if (row.action === 'host.interval.set' && row.intervalSec) {
    return `${base}（${row.intervalSec} 秒）`
  }
  return base
}

async function reload() {
  const agentId = props.agent?.agentId
  if (!agentId || !open.value) return
  loading.value = true
  try {
    const [cmdResult, auditResult] = await Promise.all([
      queryCommands(agentId),
      queryShellAudits(agentId),
    ])
    if (!isApiSuccess(cmdResult)) {
      toast.error(getApiMessage(cmdResult, '加载命令记录失败'))
      commands.value = []
    } else {
      commands.value = parseApiData<CommandHistory[]>(cmdResult) ?? []
    }
    if (!isApiSuccess(auditResult)) {
      toast.error(getApiMessage(auditResult, '加载终端账本失败'))
      audits.value = []
    } else {
      audits.value = parseApiData<ShellAudit[]>(auditResult) ?? []
    }
  } catch (err) {
    toast.error(getHttpErrorMessage(err, '加载操作记录失败'))
  } finally {
    loading.value = false
  }
}

watch(
  () => [open.value, props.agent?.agentId] as const,
  ([visible]) => {
    if (visible) {
      tab.value = 'commands'
      void reload()
    }
  },
)
</script>

<template>
  <RsDialog
    v-model:open="open"
    :title="title"
    width="860px"
    :modal="false"
    :teleport-to="`#${moduleId}-layer`"
  >
    <RsTabs v-model="tab" class="history-tabs" :items="tabItems" size="sm" panelless />
    <div v-if="tab === 'commands'" class="history-pane">
      <RsEmpty v-if="!loading && commands.length === 0" description="暂无命令记录" />
      <RsTable
        v-else
        :columns="commandColumns"
        :data="commands"
        row-key="commandId"
        size="sm"
        cell-focus="none"
        null-label="—"
        :rounded="false"
      >
        <template #action="{ row }">
          {{ formatAction(row as CommandHistory) }}
        </template>
        <template #status="{ row }">
          <RsTag
            size="sm"
            :variant="(row as CommandHistory).status === 'done' ? 'success' : (row as CommandHistory).status === 'failed' ? 'info' : 'default'"
          >
            {{ statusLabel[(row as CommandHistory).status] || (row as CommandHistory).status }}
          </RsTag>
        </template>
        <template #deliverMode="{ row }">
          {{ (row as CommandHistory).deliverMode === 'push' ? '拨入' : (row as CommandHistory).deliverMode === 'pull' ? '拉取' : '—' }}
        </template>
      </RsTable>
    </div>
    <div v-else class="history-pane">
      <RsEmpty v-if="!loading && audits.length === 0" description="暂无终端账本" />
      <RsTable
        v-else
        :columns="auditColumns"
        :data="audits"
        row-key="auditId"
        size="sm"
        cell-focus="none"
        null-label="—"
        :rounded="false"
      >
        <template #eventType="{ row }">
          <RsTag size="sm" :variant="(row as ShellAudit).eventType === 'INPUT' ? 'info' : 'default'">
            {{ eventLabel[(row as ShellAudit).eventType] || (row as ShellAudit).eventType }}
          </RsTag>
        </template>
        <template #userName="{ row }">
          {{ (row as ShellAudit).userName || (row as ShellAudit).userId }}
        </template>
        <template #detailText="{ row }">
          {{ (row as ShellAudit).detailText || '—' }}
        </template>
        <template #shellId="{ row }">
          {{ (row as ShellAudit).shellId.slice(0, 8) }}
        </template>
      </RsTable>
    </div>
    <template #footer>
      <RsButton size="sm" :loading="loading" @click="reload">刷新</RsButton>
      <RsButton size="sm" @click="open = false">关闭</RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.history-tabs {
  margin-bottom: 12px;
}
.history-pane {
  min-height: 280px;
  max-height: 480px;
  overflow: auto;
}
</style>
