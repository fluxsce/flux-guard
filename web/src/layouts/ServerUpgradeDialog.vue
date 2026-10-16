<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess } from '@/api/request'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { RsButton, RsDialog, RsForm, RsFormItem, RsInput, RsUpload, useRsToast } from '@/ui'
import { upgradeServer } from '@/layouts/upgradeServer'

const open = defineModel<boolean>('open', { default: false })

const { t } = useModuleI18n('shell')
const toast = useRsToast()
const files = ref<File[]>([])
const sha256 = ref('')
const busy = ref(false)

const formModel = computed(() => ({
  file: files.value,
  sha256: sha256.value,
}))

const ready = computed(() => files.value.length > 0 && /^[0-9a-fA-F]{64}$/.test(sha256.value.trim()) && !busy.value)

watch(open, (value) => {
  if (value) return
  files.value = []
  sha256.value = ''
  busy.value = false
})

async function onSubmit() {
  const file = files.value[0]
  if (!file) {
    toast.error(t('upgradeServerNeedFile'))
    return
  }
  if (!/^[0-9a-fA-F]{64}$/.test(sha256.value.trim())) {
    toast.error(t('upgradeServerNeedSha'))
    return
  }
  busy.value = true
  try {
    const res = await upgradeServer(file, sha256.value)
    if (!isApiSuccess(res)) {
      toast.error(getApiMessage(res, t('upgradeServerNeedFile')))
      busy.value = false
      return
    }
    toast.success(t('upgradeServerOk'))
    open.value = false
  } catch (err) {
    toast.error(getHttpErrorMessage(err, t('upgradeServerNeedFile')))
    busy.value = false
  }
}
</script>

<template>
  <RsDialog v-model:open="open" :title="t('upgradeServerTitle')" layout="form" width="480px">
    <p class="fg-upgrade-lead">{{ t('upgradeServerLead') }}</p>
    <RsForm :model="formModel" label-position="top" size="sm">
      <RsFormItem name="file" :label="t('upgradeServerFile')" required>
        <RsUpload
          v-model="files"
          name="file"
          variant="dropzone"
          size="sm"
          :max-count="1"
          replace
          accept=".tar.gz,.tgz"
          :label="t('upgradeServerFile')"
          hint="flux-guard-版本-系统-架构.tar.gz"
        />
      </RsFormItem>
      <RsFormItem name="sha256" :label="t('upgradeServerSha')" :tooltip="t('upgradeServerShaHint')" required>
        <RsInput v-model="sha256" :placeholder="t('upgradeServerShaHint')" clearable spellcheck="false" />
      </RsFormItem>
    </RsForm>
    <template #footer>
      <RsButton size="sm" variant="ghost" :disabled="busy" @click="open = false">{{ t('cancel') }}</RsButton>
      <RsButton size="sm" variant="primary" :disabled="!ready" :loading="busy" @click="onSubmit">
        {{ t('upgradeServerSubmit') }}
      </RsButton>
    </template>
  </RsDialog>
</template>

<style scoped>
.fg-upgrade-lead {
  margin: 0 0 12px;
  color: var(--rs-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}
</style>
