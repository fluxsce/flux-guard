<script setup lang="ts">
import { onMounted, useTemplateRef } from 'vue'
import { RouterView } from 'vue-router'
import { initRequestTools } from '@/api/requestLoading'
import { locale } from '@/locales'
import { theme } from '@/layouts/theme'
import { RsConfigProvider, RsLoadingBar, RsToaster, type RsLoadingBarApi } from '@/ui'

const loadingBarRef = useTemplateRef<RsLoadingBarApi>('loadingBarRef')

onMounted(() => {
  const loadingBar = loadingBarRef.value
  if (!loadingBar) {
    return
  }
  initRequestTools(loadingBar)
})
</script>

<template>
  <RsConfigProvider :theme="theme" :locale="locale" control-size="md" control-radius="sm" class="fg-app">
    <RsLoadingBar ref="loadingBarRef">
      <div class="fg-app-frame">
        <RouterView />
      </div>
    </RsLoadingBar>
    <RsToaster />
  </RsConfigProvider>
</template>
