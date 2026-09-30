<script setup lang="ts">
import { RsMenu } from '@/ui'
import { useModuleAi } from '@/ai'
import EnvVarsPanel from './components/EnvVarsPanel.vue'
import IngestTokenPanel from './components/IngestTokenPanel.vue'
import WebTimeoutForm from './components/WebTimeoutForm.vue'
import { settingsGroups, useSettings } from './composables/useSettings'

const moduleId = 'fg0024'
const { tab, activeLabel, form, envVars, ingestToken, error, saving, save, onEnvSaved, changeIngest } = useSettings()

useModuleAi(
  () => ({
    moduleId,
    title: '环境设置',
    snapshot: () => ({
      tab: tab.value,
      label: activeLabel.value,
      editable: form.value.editable,
      error: error.value,
      saving: saving.value,
      webTimeout: {
        requestTimeoutSeconds: form.value.requestTimeoutSeconds,
        sessionExpireHours: form.value.sessionExpireHours,
        defaultPageSize: form.value.defaultPageSize,
        maxPageSize: form.value.maxPageSize,
      },
      envVars: envVars.value.items.map((item) => ({
        name: item.name,
        kind: item.kind,
        hasValue: item.hasValue,
        note: item.note,
      })),
      ingest: {
        configured: ingestToken.value.configured,
        statusFlag: ingestToken.value.statusFlag,
        hint: ingestToken.value.hint,
      },
    }),
  }),
  () => [
    {
      name: 'fg0024.open',
      execute: async (args) => {
        const key = typeof args.tab === 'string' ? args.tab : ''
        if (settingsGroups.some((item) => item.key === key)) tab.value = key
        return { tab: tab.value }
      },
    },
  ],
)
</script>

<template>
  <div class="fg-page fg-env" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <aside class="fg-env-nav">
      <div class="fg-env-nav-title">环境设置</div>
      <RsMenu v-model="tab" :items="settingsGroups" class="fg-env-menu" aria-label="环境设置分组" />
    </aside>
    <section class="fg-env-main">
      <header class="fg-env-header">
        <h2>{{ activeLabel }}</h2>
      </header>
      <div class="fg-env-body" :class="{ 'fg-env-body--fill': tab === 'envVars' }">
        <WebTimeoutForm v-if="tab === 'webTimeout'" :form="form" :saving="saving" @save="save" />
        <IngestTokenPanel
          v-else-if="tab === 'ingestToken'"
          :token="ingestToken"
          :editable="form.editable"
          :saving="saving"
          @rotate="changeIngest('rotate')"
          @toggle="changeIngest(ingestToken.statusFlag === 'N' ? 'enable' : 'disable')"
        />
        <EnvVarsPanel
          v-else
          class="fg-env-vars"
          :module-id="moduleId"
          :vars="envVars"
          :editable="form.editable"
          @saved="onEnvSaved"
          @error="error = $event"
        />
      </div>
    </section>
  </div>
</template>

<style scoped>
.fg-env {
  --fg-env-nav: 13.5rem;
  flex: 1;
  width: 100%;
  height: 100%;
  min-height: 0;
  box-sizing: border-box;
  display: flex;
  flex-direction: row;
  overflow: hidden;
  background: var(--rs-bg);
}
.fg-env-nav {
  flex: 0 0 var(--fg-env-nav);
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px 8px;
  background: var(--rs-surface);
  border-right: 1px solid var(--rs-border);
}
.fg-env-nav-title {
  flex: none;
  padding: 0 8px 12px;
  color: var(--rs-muted, var(--rs-text-secondary));
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
}
.fg-env-menu {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.fg-env-main {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.fg-env-header {
  flex: none;
  padding: 16px 20px 0;
}
.fg-env-header h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
}
.fg-env-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 16px 20px 20px;
}
.fg-env-body--fill {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.fg-env-vars {
  flex: 1 1 auto;
  min-height: 0;
}
</style>
