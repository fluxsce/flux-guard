<script setup lang="ts">
import { computed } from 'vue'
import { RsButton, RsForm, RsFormItem, RsInputNumber } from '@/ui'
import type { WebTimeout } from '../api'

const props = defineProps<{
  form: WebTimeout
  saving: boolean
}>()

const model = computed(() => props.form as unknown as Record<string, unknown>)

const emit = defineEmits<{ save: [] }>()
</script>

<template>
  <RsForm
    :model="model"
    label-position="left"
    label-align="end"
    label-width="9rem"
    size="md"
    class="fg-env-form"
  >
    <RsFormItem name="requestTimeoutSeconds" label="接口超时">
      <div class="fg-env-field">
        <RsInputNumber
          :model-value="form.requestTimeoutSeconds"
          :min="10"
          :max="600"
          :disabled="!form.editable"
          size="md"
          label-position="top"
          @update:model-value="form.requestTimeoutSeconds = Number($event)"
        />
        <span>秒</span>
      </div>
    </RsFormItem>
    <RsFormItem name="sessionExpireHours" label="会话有效期">
      <div class="fg-env-field">
        <RsInputNumber
          :model-value="form.sessionExpireHours"
          :min="1"
          :max="168"
          :disabled="!form.editable"
          size="md"
          label-position="top"
          @update:model-value="form.sessionExpireHours = Number($event)"
        />
        <span>小时</span>
      </div>
    </RsFormItem>
    <RsFormItem name="defaultPageSize" label="默认每页条数">
      <div class="fg-env-field">
        <RsInputNumber
          :model-value="form.defaultPageSize"
          :min="1"
          :max="form.maxPageSize"
          :disabled="!form.editable"
          size="md"
          label-position="top"
          @update:model-value="form.defaultPageSize = Number($event)"
        />
        <span>条</span>
      </div>
    </RsFormItem>
    <RsFormItem name="maxPageSize" label="每页条数上限">
      <div class="fg-env-field">
        <RsInputNumber
          :model-value="form.maxPageSize"
          :min="1"
          :max="200"
          :disabled="!form.editable"
          size="md"
          label-position="top"
          @update:model-value="form.maxPageSize = Number($event)"
        />
        <span>条</span>
      </div>
    </RsFormItem>
    <div v-if="form.editable" class="fg-env-actions">
      <RsButton variant="primary" size="md" :disabled="saving" @click="emit('save')">保存</RsButton>
    </div>
  </RsForm>
</template>

<style scoped>
.fg-env-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 36rem;
}
.fg-env-field {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 16rem;
}
.fg-env-field :deep(.rs-field) {
  flex: 1;
  min-width: 0;
  width: auto;
}
.fg-env-field > span {
  flex: none;
  color: var(--rs-muted, var(--rs-text-secondary));
  font-size: 13px;
}
.fg-env-actions {
  padding-left: 9rem;
  margin-top: 8px;
}
</style>
