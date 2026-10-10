<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useModuleAi } from '@/ai'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { RsButton, RsCard, RsEmpty, RsIcon, RsTag, useRsToast } from '@/ui'
import { queryPlugins, type PluginInfo, type PluginMethod } from './api'

const moduleId = 'fg0026'
const { t } = useModuleI18n(moduleId)

const rows = ref<PluginInfo[]>([])
const toast = useRsToast()
const error = ref('')
const refreshing = ref(false)
const loaded = ref(false)

const componentNames: Record<string, string> = {
  redis: 'Redis',
  mysql: 'MySQL',
  mongo: 'MongoDB',
  kafka: 'Kafka',
  zk: 'ZooKeeper',
  nacos: 'Nacos',
}

const langNames: Record<string, string> = {
  java: 'Java',
  go: 'Go',
}

const summary = computed(() => {
  const parts = [`${t('registered')} ${rows.value.length}`]
  const running = rows.value.filter((row) => row.status === 'running').length
  const idle = rows.value.filter((row) => row.status === 'idle').length
  const held = rows.value.filter((row) => row.status === 'held').length
  const exited = rows.value.filter((row) => row.status === 'exited').length
  if (running) parts.push(`${t('statusRunning')} ${running}`)
  if (idle) parts.push(`${t('statusIdle')} ${idle}`)
  if (held) parts.push(`${t('statusHeld')} ${held}`)
  if (exited) parts.push(`${t('statusExited')} ${exited}`)
  return parts.join(' · ')
})

function statusText(status: string) {
  if (status === 'running') return t('statusRunning')
  if (status === 'exited') return t('statusExited')
  if (status === 'held') return t('statusHeld')
  return t('statusIdle')
}

function statusVariant(status: string) {
  if (status === 'running') return 'success' as const
  if (status === 'exited') return 'danger' as const
  if (status === 'held') return 'warning' as const
  return 'default' as const
}

function componentText(component: string) {
  if (!component) return t('componentNone')
  return componentNames[component] || component
}

function langText(lang: string) {
  if (!lang) return '—'
  return langNames[lang] || lang
}

function startupText(startup: string) {
  if (startup === 'lazy') return t('startupLazy')
  return startup || '—'
}

function methodText(name: string) {
  if (name === 'probe') return t('methodProbe')
  if (name === 'maintain') return t('methodMaintain')
  if (name === 'file.put') return t('methodFilePut')
  if (name === 'file.get') return t('methodFileGet')
  return name
}

function opText(name: string) {
  if (name === 'create_topic') return t('opCreateTopic')
  if (name === 'delete_topic') return t('opDeleteTopic')
  return name
}

function opLine(method: PluginMethod) {
  return (method.ops ?? []).map(opText).join(' · ')
}

async function load() {
  error.value = ''
  refreshing.value = true
  try {
    const result = await queryPlugins()
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, t('loadFail'))
      toast.error(error.value)
      return
    }
    rows.value = parseApiData<PluginInfo[]>(result) ?? []
  } catch (err) {
    error.value = getHttpErrorMessage(err, t('loadFail'))
    toast.error(error.value)
  } finally {
    refreshing.value = false
    loaded.value = true
  }
}

onMounted(load)

useModuleAi(() => ({
  moduleId,
  title: t('title'),
  snapshot: () => ({
    error: error.value,
    total: rows.value.length,
    plugins: rows.value.map((row) => ({
      pluginCode: row.pluginCode,
      pluginName: row.pluginName,
      version: row.version,
      component: row.component,
      status: row.status,
      pid: row.pid ?? 0,
    })),
  }),
}))
</script>

<template>
  <div class="fg-page fg-plugins" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <header class="fg-plugins-bar">
      <div class="fg-plugins-copy">
        <h2>{{ t('title') }}</h2>
        <p>{{ summary }}</p>
      </div>
      <RsButton
        variant="secondary"
        size="sm"
        icon="refresh-cw"
        data-fg-ai-action="refresh"
        data-fg-ai-risk="read"
        :disabled="refreshing"
        @click="load"
      >
        {{ t('refresh') }}
      </RsButton>
    </header>

    <p v-if="error" class="fg-error">{{ error }}</p>

    <RsEmpty v-if="loaded && rows.length === 0" :title="t('empty')" :description="t('emptyHint')" />

    <div v-else class="fg-plugin-grid">
      <RsCard
        v-for="row in rows"
        :key="row.pluginCode"
        class="fg-plugin"
        :data-status="row.status"
        variant="outlined"
        size="sm"
      >
        <template #header>
          <div class="fg-plugin-head">
            <span class="fg-plugin-mark" aria-hidden="true">
              <RsIcon name="puzzle" :size="16" />
            </span>
            <div class="fg-plugin-name">
              <strong>
                {{ row.pluginName }}
                <em v-if="row.version">{{ row.version }}</em>
              </strong>
              <span>{{ row.pluginCode }}</span>
            </div>
          </div>
        </template>
        <template #actions>
          <RsTag size="sm" :variant="statusVariant(row.status)">{{ statusText(row.status) }}</RsTag>
        </template>
        <dl class="fg-plugin-facts">
          <div>
            <dt>{{ t('component') }}</dt>
            <dd>{{ componentText(row.component) }}</dd>
          </div>
          <div>
            <dt>{{ t('lang') }}</dt>
            <dd>{{ langText(row.lang) }}</dd>
          </div>
          <div>
            <dt>{{ t('startup') }}</dt>
            <dd>{{ startupText(row.startup) }}</dd>
          </div>
          <div v-if="row.pid">
            <dt>{{ t('process') }}</dt>
            <dd class="fg-plugin-pid">{{ row.pid }}</dd>
          </div>
        </dl>
        <ul v-if="row.methods?.length" class="fg-plugin-methods">
          <li v-for="method in row.methods" :key="method.name">
            <RsTag size="sm">{{ methodText(method.name) }}</RsTag>
            <span v-if="opLine(method)">{{ opLine(method) }}</span>
          </li>
        </ul>
      </RsCard>
    </div>
  </div>
</template>

<style scoped>
.fg-plugins {
  gap: 16px;
}

.fg-plugins-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.fg-plugins-copy h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.02em;
}

.fg-plugins-copy p {
  margin: 4px 0 0;
  color: var(--rs-text-secondary);
  font-size: 12px;
}

.fg-plugin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
  align-content: start;
}

.fg-plugin[data-status='running'] {
  box-shadow: inset 3px 0 0 var(--rs-success);
}

.fg-plugin[data-status='exited'] {
  box-shadow: inset 3px 0 0 var(--rs-danger);
}

.fg-plugin[data-status='held'] {
  box-shadow: inset 3px 0 0 var(--rs-warning);
}

.fg-plugin-head {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.fg-plugin-mark {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex: none;
  border-radius: 8px;
  color: var(--rs-primary, #3b6cff);
  background: color-mix(in srgb, var(--rs-primary, #3b6cff) 12%, transparent);
}

.fg-plugin-name {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.fg-plugin-name strong {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.fg-plugin-name em {
  margin-left: 6px;
  color: var(--rs-text-secondary);
  font-family: var(--rs-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-style: normal;
  font-weight: 500;
}

.fg-plugin-name span,
.fg-plugin-pid {
  overflow: hidden;
  color: var(--rs-text-secondary);
  font-family: var(--rs-font-mono, ui-monospace, monospace);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-plugin-pid {
  color: var(--rs-text-primary);
}

.fg-plugin-facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
}

.fg-plugin-facts div {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.fg-plugin-facts dt {
  color: var(--rs-text-secondary);
  font-size: 12px;
  font-weight: 400;
}

.fg-plugin-facts dd {
  margin: 0;
  font-size: 13px;
}

.fg-plugin-methods {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 12px 0 0;
  padding: 12px 0 0;
  list-style: none;
  border-top: 1px solid var(--rs-border);
}

.fg-plugin-methods li {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.fg-plugin-methods span {
  color: var(--rs-text-secondary);
  font-size: 12px;
}

@media (max-width: 720px) {
  .fg-plugins-bar {
    align-items: flex-start;
  }

  .fg-plugin-facts {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
