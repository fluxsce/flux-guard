<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { RsCard, RsEmpty, RsTable, RsTag, useRsToast } from '@/ui'
import { queryPlugins, type PluginInfo } from './api'

const moduleId = 'fg0003'

const rows = ref<PluginInfo[]>([])
const toast = useRsToast()
const error = ref('')
const columns = [
  { key: 'pluginName', title: '名称', dataIndex: 'pluginName', ellipsis: true },
  { key: 'pluginCode', title: '编码', dataIndex: 'pluginCode' },
  { key: 'pluginKind', title: '种类', dataIndex: 'pluginKind' },
  { key: 'pluginScope', title: '位置', dataIndex: 'pluginScope' },
  { key: 'builtinFlag', title: '内置', dataIndex: 'builtinFlag', width: 88 },
]

async function load() {
  error.value = ''
  try {
    const result = await queryPlugins()
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, '加载失败')
      toast.error(error.value)
      return
    }
    rows.value = parseApiData<PluginInfo[]>(result) ?? []
  } catch (err) {
    error.value = getHttpErrorMessage(err, '加载失败')
    toast.error(error.value)
  }
}

onMounted(load)

useModuleAi(() => ({
  moduleId,
  title: '扩展',
  snapshot: () => ({
    error: error.value,
    total: rows.value.length,
    plugins: rows.value.map((row) => ({
      pluginCode: row.pluginCode,
      pluginName: row.pluginName,
      pluginKind: row.pluginKind,
      pluginScope: row.pluginScope,
      builtinFlag: row.builtinFlag,
    })),
  }),
}))
</script>

<template>
  <div class="fg-page" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
  <RsCard title="插件" :padding="false">
    <RsEmpty v-if="rows.length === 0" title="没有插件" />
    <RsTable
      v-else
      :columns="columns"
      :data="rows"
      row-key="pluginCode"
      size="sm"
      cell-focus="none"
      null-label="—"
      highlight-row
      :rounded="false"
    >
      <template #builtinFlag="{ row }">
        <RsTag size="sm" :variant="(row as PluginInfo).builtinFlag === 'Y' ? 'info' : 'default'">
          {{ (row as PluginInfo).builtinFlag === 'Y' ? '内置' : '扩展' }}
        </RsTag>
      </template>
    </RsTable>
  </RsCard>
  </div>
</template>
