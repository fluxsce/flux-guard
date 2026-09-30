<template>
  <section class="fg-catalog">
    <slot name="alert" />
    <slot name="search" />
    <div class="fg-catalog-panel">
      <slot />
      <div v-if="$slots.pager" class="fg-catalog-pager">
        <slot name="pager" />
      </div>
    </div>
    <slot name="extra" />
  </section>
</template>

<style scoped>
.fg-catalog {
  flex: 1;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--fg-panel-bg, var(--rs-surface));
}
.fg-catalog :deep(.fg-catalog-error) {
  flex: none;
  padding: 8px 12px 0;
}
.fg-catalog-panel {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--fg-panel-border, var(--rs-border));
  overflow: hidden;
}
.fg-catalog-pager {
  flex: none;
  display: flex;
  justify-content: flex-end;
  padding: 8px 12px;
  border-top: 1px solid var(--fg-panel-border, var(--rs-border));
}
</style>
