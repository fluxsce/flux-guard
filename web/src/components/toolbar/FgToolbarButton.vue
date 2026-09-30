<script setup lang="ts">
import { computed } from 'vue'
import { RsButton, RsDropdown } from '@/ui'
import type { FgToolbarButton, FgToolbarButtonTone, FgToolbarSize } from './types'

const props = defineProps<{
  button: FgToolbarButton
  size: FgToolbarSize
}>()

const emit = defineEmits<{
  click: [key: string]
  dropdown: [buttonKey: string, optionKey: string]
}>()

const tone = computed<FgToolbarButtonTone>(() => {
  if (props.button.tone) return props.button.tone
  const type = props.button.type
  if (type === 'success' || type === 'warning' || type === 'info') return type
  if (type === 'error' || props.button.variant === 'danger') return 'danger'
  if (type === 'primary' || props.button.variant === 'primary') return 'primary'
  return 'neutral'
})

const resolvedSize = computed(() => props.button.size ?? 'sm')

const menuItems = computed(() => {
  const items: Array<{
    label: string
    value: string
    icon?: string
    disabled?: boolean
    type?: 'item' | 'divider'
  }> = []
  for (const option of props.button.dropdownOptions ?? []) {
    if (option.divider) {
      items.push({ label: ' ', value: `${option.key}:divider`, type: 'divider' })
    }
    items.push({
      label: option.label,
      value: option.key,
      icon: option.icon,
      disabled: option.disabled,
    })
  }
  return items
})

async function onClick() {
  emit('click', props.button.key)
  await props.button.onClick?.(props.button.key)
}

async function onSelect(value: string) {
  emit('dropdown', props.button.key, value)
  const option = props.button.dropdownOptions?.find((item) => item.key === value)
  await option?.onClick?.(value)
}
</script>

<template>
  <RsDropdown
    v-if="button.dropdown"
    :items="menuItems"
    :placeholder="button.label || button.tooltip || button.key"
    :disabled="button.disabled"
    :show-selected="false"
    content-width="fit"
    :size="resolvedSize"
    :aria-label="button.tooltip || button.label || button.key"
    @select="onSelect"
  />
  <RsButton
    v-else
    class="fg-toolbar-btn"
    variant="text"
    :tone="tone"
    :size="resolvedSize"
    radius="sm"
    :icon="button.icon"
    :icon-only="!button.label"
    :disabled="button.disabled"
    :loading="button.loading"
    :tooltip="button.tooltip"
    :aria-label="button.label ? undefined : button.tooltip || button.key"
    @click="onClick"
  >
    {{ button.label }}
  </RsButton>
</template>

<style scoped>
.fg-toolbar-btn :deep(svg) {
  width: 14px;
  height: 14px;
}
</style>
