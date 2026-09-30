<script setup lang="ts">
import { computed } from 'vue'
import { RsButton } from '@/ui'
import { theme, toggleTheme } from '@/layouts/theme'

const nextLabel = computed(() => (theme.value === 'dark' ? '浅色' : '暗色'))
const icon = computed(() => (theme.value === 'dark' ? 'sun' : 'moon'))
</script>

<template>
  <RsButton
    variant="ghost"
    size="sm"
    icon-only
    :icon="icon"
    :tooltip="`切换为${nextLabel}`"
    :aria-label="`切换为${nextLabel}主题`"
    @click="toggleTheme"
  />
</template>
