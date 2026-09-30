<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useModuleAi } from '@/ai'
import { RsCard } from '@/ui'

const route = useRoute()
const fromRoute = route.meta.module
const fromParam = route.params.module
const moduleId =
  (typeof fromRoute === 'string' && fromRoute) || (typeof fromParam === 'string' ? fromParam : '')
const pagePath = route.path

useModuleAi(() => ({
  moduleId: moduleId || 'plugin',
  title: '插件页面',
  snapshot: () => ({
    path: pagePath,
    moduleId,
  }),
}))
</script>

<template>
  <div :id="moduleId || undefined" class="fg-page" data-fg-ai="surface" :data-fg-module="moduleId || undefined">
    <RsCard title="插件页面" :description="route.path">
      <p>地址是模块号。示例告警 fg0100 只登记权限和入口，业务页后续再接到这里。</p>
    </RsCard>
  </div>
</template>
