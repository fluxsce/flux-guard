<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useModuleAi } from '@/ai'
import FgCatalog from '@/components/catalog/FgCatalog.vue'
import FgDataFormModal from '@/components/form/FgDataFormModal.vue'
import FgSearchForm from '@/components/form/FgSearchForm.vue'
import FgGrid from '@/components/grid/FgGrid.vue'
import type { FgToolbarButton } from '@/components/toolbar/types'
import { RsPagination, RsTag } from '@/ui'
import { pageSizeChoices } from '@/api/pagination'
import type { Agent } from './api'
import AgentHistoryDialog from './components/AgentHistoryDialog.vue'
import AgentUpgradeDialog from './components/AgentUpgradeDialog.vue'
import ShellDock from './components/ShellDock.vue'
import {
  agentCodeLabel,
  agentColumns,
  agentCreateFields,
  agentFormFields,
  agentMenuItems,
  agentRenameFields,
  agentSearchFields,
  flagLabel,
  intervalFields,
  isHostAgent,
  onlineOptions,
  upgradeStatusText,
  upgradeStatusVariant,
  useAgentCatalog,
} from './composables/useAgentCatalog'

const moduleId = 'fg0002'
const {
  page,
  pageSize,
  error,
  formOpen,
  formMode,
  current,
  active,
  selectedKeys,
  rows,
  total,
  pageRows,
  intervalOpen,
  intervalCurrent,
  renameOpen,
  renameCurrent,
  commandBusy,
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
  upgradeByAgent,
  reloadUpgradeStatuses,
} = useAgentCatalog()

const formFields = computed(() => (formMode.value === 'view' ? agentFormFields : agentCreateFields))
const formTitle = computed(() => (formMode.value === 'create' ? '登记代理' : '代理'))
const canCommand = computed(() => isHostAgent(active.value) && !commandBusy.value)
const canDelete = computed(() => selectedKeys.value.length > 0 || !!active.value)

const toolbarButtons = computed<FgToolbarButton[]>(() => [
  { key: 'edit', label: '登记', icon: 'plus', onClick: openCreate },
  {
    key: 'view',
    label: '查看',
    icon: 'eye',
    disabled: !active.value,
    onClick: () => {
      if (active.value) openView(active.value)
    },
  },
  {
    key: 'rename',
    label: '改名',
    icon: 'pencil',
    code: 'fg0002:edit',
    disabled: !active.value,
    onClick: () => openRename(),
  },
  {
    key: 'delete',
    label: selectedKeys.value.length > 1 ? `删除(${selectedKeys.value.length})` : '删除',
    icon: 'trash',
    code: 'fg0002:delete',
    disabled: !canDelete.value,
    onClick: () => void removeAgents(),
  },
  {
    key: 'collect',
    label: '立即采集',
    icon: 'play',
    code: 'fg0002:command',
    disabled: !canCommand.value,
    loading: commandBusy.value,
    onClick: () => collect(),
  },
  {
    key: 'interval',
    label: '修改间隔',
    icon: 'timer',
    code: 'fg0002:command',
    disabled: !canCommand.value,
    onClick: () => openInterval(),
  },
  {
    key: 'shell',
    label: '打开终端',
    icon: 'square-terminal',
    code: 'fg0002:shell',
    disabled: !active.value || !isHostAgent(active.value),
    onClick: () => openShell(),
  },
  {
    key: 'upgrade',
    label: '升级代理',
    icon: 'arrow-up-circle',
    code: 'fg0002:upgrade',
    onClick: () => {
      upgradeOpen.value = true
    },
  },
  {
    key: 'history',
    label: '操作记录',
    icon: 'history',
    code: 'fg0002:view',
    disabled: !active.value,
    onClick: () => openHistory(),
  },
])

useModuleAi(
  () => ({
    moduleId,
    title: '代理',
    snapshot: () => ({
      total: total.value,
      page: page.value,
      pageSize: pageSize.value,
      error: error.value,
      formOpen: formOpen.value,
      formMode: formMode.value,
      selectedAgentId: active.value?.agentId ?? '',
      selectedAgentName: active.value?.agentName ?? '',
      selectedCount: selectedKeys.value.length,
      online: pageRows.value.filter((row) => row.onlineFlag === 'Y').length,
    }),
  }),
  () => [
    {
      name: 'fg0002.applySearch',
      execute: async (args) => {
        await onSearch(args)
        return { total: total.value }
      },
    },
  ],
)

const shellOpen = ref(false)
const shellAgent = ref<Agent | null>(null)
const shellDock = ref<{ add: () => Promise<void> } | null>(null)
const historyOpen = ref(false)
const historyAgent = ref<Agent | null>(null)
const upgradeOpen = ref(false)

watch(upgradeOpen, (value, prev) => {
  if (prev && !value) void reload()
})

async function openShell(row?: Agent) {
  const target = row ?? active.value
  if (!target || !isHostAgent(target)) return
  active.value = target
  shellAgent.value = target
  shellOpen.value = true
  await nextTick()
  await shellDock.value?.add()
}

function openHistory(row?: Agent) {
  const target = row ?? active.value
  if (!target) return
  active.value = target
  historyAgent.value = target
  historyOpen.value = true
}

function onMenu(payload: { key: string; row: Agent | null }) {
  if (!payload.row) return
  if (payload.key === 'view') openView(payload.row)
  if (payload.key === 'rename') openRename(payload.row)
  if (payload.key === 'collect') collect(payload.row)
  if (payload.key === 'interval') openInterval(payload.row)
  if (payload.key === 'shell') void openShell(payload.row)
  if (payload.key === 'history') openHistory(payload.row)
  if (payload.key === 'delete') void removeAgents(payload.row)
}
</script>

<template>
  <div class="fg-page fg-agent-host" :class="{ 'shell-on': shellOpen && shellAgent }" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <div class="agent-main">
    <FgCatalog>
      <template #search>
        <FgSearchForm
          plain
          :module-id="moduleId"
          :fields="agentSearchFields"
          :toolbar-buttons="toolbarButtons"
          @search="onSearch"
          @reset="onReset"
        />
      </template>
      <FgGrid
        :data="pageRows"
        :columns="agentColumns"
        row-key="agentId"
        selectable
        v-model:selected-row-keys="selectedKeys"
        :module-id="moduleId"
        :menu-items="agentMenuItems"
        @row-click="onRow"
        @row-dblclick="openView"
        @menu-click="onMenu"
      >
        <template #agentCode="{ row }">
          {{ agentCodeLabel(String((row as Agent).agentCode ?? '')) }}
        </template>
        <template #upgradeStatus="{ row }">
          <template v-if="upgradeByAgent[(row as Agent).agentId]">
            <RsTag
              size="sm"
              :variant="
                upgradeStatusVariant(
                  upgradeByAgent[(row as Agent).agentId].status,
                  upgradeByAgent[(row as Agent).agentId].versionMatched,
                )
              "
            >
              {{
                upgradeStatusText(
                  upgradeByAgent[(row as Agent).agentId].status,
                  upgradeByAgent[(row as Agent).agentId].versionMatched,
                )
              }}
            </RsTag>
          </template>
          <span v-else class="upgrade-empty">—</span>
        </template>
        <template #onlineFlag="{ row }">
          <RsTag size="sm" :variant="(row as Agent).onlineFlag === 'Y' ? 'success' : 'default'">
            {{ flagLabel(onlineOptions, String((row as Agent).onlineFlag ?? '')) }}
          </RsTag>
        </template>
      </FgGrid>
      <template #pager>
        <RsPagination
          v-model:page="page"
          v-model:page-size="pageSize"
          :total="total"
          size="sm"
          align="end"
          show-page-size
          :page-size-options="pageSizeChoices"
        />
      </template>
    </FgCatalog>
    </div>
    <ShellDock v-if="shellOpen && shellAgent" ref="shellDock" :agent="shellAgent" @close="shellOpen = false" />
    <AgentHistoryDialog v-model:open="historyOpen" :agent="historyAgent" :module-id="moduleId" />
    <AgentUpgradeDialog
      v-model:open="upgradeOpen"
      :agents="rows"
      :module-id="moduleId"
      @changed="reloadUpgradeStatuses"
    />
    <FgDataFormModal
      v-model:open="formOpen"
      :mode="formMode"
      :title="formTitle"
      :fields="formFields"
      :initial-data="current"
      :auto-close="false"
      :module-id="moduleId"
      save-action="edit"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submitAgent"
    />
    <FgDataFormModal
      v-model:open="intervalOpen"
      mode="create"
      title="修改间隔"
      :fields="intervalFields"
      :initial-data="intervalCurrent"
      :auto-close="false"
      :module-id="moduleId"
      save-action="command"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submitInterval"
    />
    <FgDataFormModal
      v-model:open="renameOpen"
      mode="create"
      title="修改显示名与描述"
      :fields="agentRenameFields"
      :initial-data="renameCurrent"
      :auto-close="false"
      :module-id="moduleId"
      save-action="edit"
      :teleport-to="`#${moduleId}-layer`"
      @submit="submitRename"
    />
    <div :id="`${moduleId}-layer`" class="fg-layer" />
  </div>
</template>

<style scoped>
.fg-agent-host {
  position: relative;
  z-index: 0;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.agent-main {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.fg-agent-host.shell-on .agent-main {
  flex: 1 1 50%;
}
.fg-layer {
  position: absolute;
  inset: 0;
  z-index: 30;
  pointer-events: none;
}
.fg-layer:deep(> *) {
  pointer-events: auto;
}
.upgrade-empty {
  color: var(--rs-text-muted, #99a);
}
</style>
