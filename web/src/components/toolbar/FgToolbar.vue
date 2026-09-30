<script setup lang="ts">
import { computed, useSlots } from 'vue'
import { riskOf } from '@/ai'
import { useButtonAllowed } from '@/components/buttonAccess'
import { RsToolbar } from '@/ui'
import FgToolbarButton from './FgToolbarButton.vue'
import type {
  FgToolbarAlign,
  FgToolbarBorder,
  FgToolbarButton as ToolbarButton,
  FgToolbarGroup,
  FgToolbarSize,
} from './types'

defineOptions({ name: 'FgToolbar' })

const props = withDefaults(
  defineProps<{
    buttons?: ToolbarButton[]
    groups?: FgToolbarGroup[]
    title?: string
    align?: FgToolbarAlign
    border?: FgToolbarBorder
    size?: FgToolbarSize
    /** 嵌在面板里时去掉工具条底色 */
    plain?: boolean
    elevated?: boolean
    compact?: boolean
    wrap?: boolean
    ariaLabel?: string
    /** 模块号。按钮键拼成「模块:键」后按菜单里的按钮授权决定是否渲染。 */
    moduleId?: string
  }>(),
  {
    buttons: () => [],
    groups: () => [],
    align: 'left',
    border: 'bottom',
    size: 'md',
    plain: false,
    elevated: false,
    compact: false,
    wrap: false,
  },
)

const emit = defineEmits<{
  click: [key: string]
  'dropdown-select': [buttonKey: string, optionKey: string]
}>()

const slots = useSlots()
const allowed = useButtonAllowed()

interface Cluster {
  key: string
  title?: string
  divider?: boolean
  buttons: ToolbarButton[]
}

function shown(buttons: ToolbarButton[]) {
  return buttons.filter((button) => button.show !== false && allowed(props.moduleId, button.key, button.code))
}

const usingGroups = computed(() => props.groups.length > 0)

function clusters(atEnd: boolean): Cluster[] {
  if (usingGroups.value) {
    return props.groups
      .map((group) => ({
        key: group.key,
        title: group.title,
        divider: group.divider,
        buttons: shown(group.buttons).filter((button) => (button.atEnd === true) === atEnd),
      }))
      .filter((group) => group.buttons.length > 0)
  }
  const buttons = shown(props.buttons).filter((button) => (button.atEnd === true) === atEnd)
  if (buttons.length === 0) return []
  return [{ key: atEnd ? 'end' : 'start', buttons }]
}

const startClusters = computed(() => clusters(false))
const endClusters = computed(() => clusters(true))

const hasStart = computed(
  () =>
    Boolean(props.title || slots.title || slots.left || slots.default) || startClusters.value.length > 0,
)
const hasCenter = computed(() => Boolean(slots.center))
const hasEnd = computed(() => Boolean(slots.right) || endClusters.value.length > 0)

function onClick(key: string) {
  emit('click', key)
}

function onDropdown(buttonKey: string, optionKey: string) {
  emit('dropdown-select', buttonKey, optionKey)
}
</script>

<template>
  <RsToolbar
    class="fg-toolbar"
    :class="[`fg-toolbar--${align}`, { 'fg-toolbar--plain': plain }]"
    :size="size"
    :border="border"
    :elevated="elevated"
    :compact="compact"
    :wrap="wrap"
    :aria-label="ariaLabel"
  >
    <template v-if="hasStart" #start>
      <slot name="title">
        <span v-if="title" class="fg-toolbar__title">{{ title }}</span>
      </slot>
      <slot name="left" />
      <slot />
      <div
        v-for="(group, groupIndex) in startClusters"
        :key="group.key"
        class="fg-toolbar__group"
        :class="{ 'fg-toolbar__group--divider': group.divider && groupIndex > 0 }"
      >
        <span v-if="group.title" class="fg-toolbar__group-title">{{ group.title }}</span>
        <div
          v-for="(button, index) in group.buttons"
          :key="button.key"
          class="fg-toolbar__button"
          :class="{ 'fg-toolbar__button--divider': index > 0 }"
          :data-fg-ai-action="button.key"
          :data-fg-ai-risk="riskOf(button.key, button.aiRisk)"
        >
          <FgToolbarButton
            :button="button"
            :size="size"
            @click="onClick"
            @dropdown="onDropdown"
          />
        </div>
      </div>
      <slot name="after" />
    </template>
    <template v-if="hasCenter" #center>
      <slot name="center" />
    </template>
    <template v-if="hasEnd" #end>
      <div
        v-for="(group, groupIndex) in endClusters"
        :key="group.key"
        class="fg-toolbar__group"
        :class="{ 'fg-toolbar__group--divider': group.divider && groupIndex > 0 }"
      >
        <span v-if="group.title" class="fg-toolbar__group-title">{{ group.title }}</span>
        <div
          v-for="(button, index) in group.buttons"
          :key="button.key"
          class="fg-toolbar__button"
          :class="{ 'fg-toolbar__button--divider': index > 0 }"
          :data-fg-ai-action="button.key"
          :data-fg-ai-risk="riskOf(button.key, button.aiRisk)"
        >
          <FgToolbarButton
            :button="button"
            :size="size"
            @click="onClick"
            @dropdown="onDropdown"
          />
        </div>
      </div>
      <slot name="right" />
    </template>
  </RsToolbar>
</template>

<style scoped>
.fg-toolbar {
  width: 100%;
}
.fg-toolbar--plain {
  background: transparent;
  --rs-toolbar-bg: transparent;
}
.fg-toolbar--right :deep(.rs-toolbar__start),
.fg-toolbar--center :deep(.rs-toolbar__start) {
  flex: 0 1 auto;
}
.fg-toolbar--right {
  justify-content: flex-end;
}
.fg-toolbar--center {
  justify-content: center;
}
.fg-toolbar--right :deep(.rs-toolbar__end),
.fg-toolbar--center :deep(.rs-toolbar__end) {
  margin-inline-start: 0;
}
.fg-toolbar__title {
  font-weight: 600;
  margin-inline-end: 8px;
  white-space: nowrap;
}
.fg-toolbar__group {
  display: flex;
  align-items: center;
  gap: 0;
  min-width: 0;
}
.fg-toolbar__button {
  display: inline-flex;
  align-items: center;
}
.fg-toolbar__button--divider {
  position: relative;
  margin-inline-start: 2px;
  padding-inline-start: 2px;
}
.fg-toolbar__button--divider::before {
  content: '';
  position: absolute;
  inset-inline-start: 0;
  top: 50%;
  width: 1px;
  height: 14px;
  transform: translateY(-50%);
  background: var(--rs-border, var(--fg-panel-border));
}
.fg-toolbar__group--divider {
  margin-inline-start: 4px;
  padding-inline-start: 12px;
  border-inline-start: 1px solid var(--rs-border, var(--fg-panel-border));
}
.fg-toolbar__group-title {
  font-size: 12px;
  color: var(--rs-text-secondary, var(--rs-text-muted));
  white-space: nowrap;
}
</style>
