<script setup lang="ts">
import { RsEmpty } from '@/ui'
import { terminalNote } from '../composables/panels'

defineProps<{
  kindCode: string
  kindName: string
}>()
</script>

<template>
  <RsEmpty :title="`${kindName || kindCode} 终端`" :description="terminalNote(kindCode)" />
</template>
