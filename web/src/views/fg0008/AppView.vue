<script setup lang="ts">
import { computed } from 'vue'
import { useModuleAi } from '@/ai'
import { RsEmpty } from '@/ui'
import FileBrowser from './components/FileBrowser.vue'
import OpConsole from './components/OpConsole.vue'
import SiteList from './components/SiteList.vue'
import { moduleId, useAppWorkbench } from './composables/useAppWorkbench'

const {
  sites,
  files,
  releases,
  keyword,
  current,
  picked,
  loading,
  filesLoading,
  error,
  version,
  canaryCount,
  releasing,
  continueBusy,
  logLines,
  canRelease,
  releaseKind,
  releaseAppCode,
  loadSites,
  openSite,
  refreshFiles,
  toggle,
  pickAll,
  clearPick,
  downloadSelected,
  downloadOne,
  downloadBusy,
  downloadProgress,
  onPickFile,
  submitRelease,
  continueBatch,
  submitRollback,
  openRelease,
  siteKey,
} = useAppWorkbench()

const instanceLabel = computed(() => (instId: string) => {
  const row = files.value.find((item) => item.instId === instId)
  if (!row) return instId
  return row.appCode || row.artifactPath || instId
})

useModuleAi(() => ({
  moduleId,
  title: '应用管理',
  snapshot: () => ({
    site: current.value?.instanceName ?? '',
    files: files.value.length,
    picked: picked.value.length,
    release: releasing.value?.releaseId ?? '',
  }),
}))
</script>

<template>
  <div class="fg-page fg-app" :id="moduleId" data-fg-ai="surface" :data-fg-module="moduleId">
    <div class="fg-app-split">
      <SiteList
        class="fg-app-left"
        :sites="sites"
        :current="current"
        :loading="loading"
        :keyword="keyword"
        :site-key="siteKey"
        @update:keyword="keyword = $event"
        @search="loadSites(keyword)"
        @select="openSite"
        @refresh="loadSites()"
      />
      <div class="fg-app-right">
        <p v-if="error" class="fg-app-error">{{ error }}</p>
        <RsEmpty
          v-if="!current"
          class="fg-app-placeholder"
          title="选择左侧运行时"
          description="应用代理按「实例」看守一套目录或 Tomcat。Tomcat 右侧可进 webapps 换包，也可进 logs 下载启动日志；下方操作日志含部署步骤与启动探测结果。"
        />
        <template v-else>
          <FileBrowser
            class="fg-app-files"
            :site="current"
            :files="files"
            :picked="picked"
            :loading="filesLoading"
            :download-busy="downloadBusy"
            :download-progress="downloadProgress"
            :can-release="canRelease"
            :version="version"
            :canary-count="canaryCount"
            :release-kind="releaseKind"
            :release-app-code="releaseAppCode"
            @refresh="refreshFiles"
            @pick-all="pickAll($event)"
            @clear-pick="clearPick"
            @download="downloadSelected"
            @download-one="downloadOne"
            @toggle="toggle"
            @update:version="version = $event"
            @update:canary-count="canaryCount = $event"
            @pick-file="onPickFile"
            @release="submitRelease"
          />
          <OpConsole
            class="fg-app-console"
            :log-lines="logLines"
            :releasing="releasing"
            :releases="releases"
            :files="files"
            :can-release="canRelease"
            :continue-busy="continueBusy"
            :instance-label="instanceLabel"
            @continue="continueBatch"
            @rollback="submitRollback"
            @open-release="openRelease"
          />
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fg-app {
  position: relative;
  z-index: 0;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.fg-app-split {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(240px, 300px) minmax(0, 1fr);
}

.fg-app-left {
  min-height: 0;
}

.fg-app-right {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.fg-app-error {
  margin: 0;
  padding: 8px 16px;
  color: var(--rs-danger, #b91c1c);
  font-size: 13px;
}

.fg-app-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 40px;
}

.fg-app-files {
  flex: 1 1 58%;
  min-height: 180px;
}

.fg-app-console {
  flex: 0 0 42%;
  min-height: 160px;
  max-height: 46%;
}

@media (max-width: 900px) {
  .fg-app-split {
    grid-template-columns: 1fr;
    grid-template-rows: minmax(180px, 36%) minmax(0, 1fr);
  }

  .fg-app-left {
    border-right: 0;
    border-bottom: 1px solid var(--fg-panel-border, var(--rs-border));
  }
}
</style>
