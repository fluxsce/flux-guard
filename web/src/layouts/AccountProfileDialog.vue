<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { useSessionStore } from '@/stores/session'
import { RsButton, RsDialog, RsForm, RsFormItem, RsInput, RsSelect, useRsToast } from '@/ui'
import { profile, saveProfile, type SelfProfile } from '@/views/login/api'

const open = defineModel<boolean>('open', { required: true })

const session = useSessionStore()
const { t } = useModuleI18n('shell')
const loading = ref(false)
const saving = ref(false)
const toast = useRsToast()
const error = ref('')
const form = ref({
  userName: '',
  realName: '',
  email: '',
  mobile: '',
  gender: 0,
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const genderOptions = computed(() => [
  { label: t('genderUnknown'), value: 0 },
  { label: t('genderMale'), value: 1 },
  { label: t('genderFemale'), value: 2 },
])

watch(open, (shown) => {
  if (!shown) return
  error.value = ''
  void load()
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await profile()
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, t('profileLoadFailed'))
      toast.error(error.value)
      return
    }
    const data = parseApiData<SelfProfile>(result)
    form.value = {
      userName: data?.userName || session.user?.userName || '',
      realName: data?.realName || '',
      email: data?.email || '',
      mobile: data?.mobile || '',
      gender: data?.gender ?? 0,
      currentPassword: '',
      newPassword: '',
      confirmPassword: '',
    }
  } catch (err) {
    error.value = getHttpErrorMessage(err, t('profileLoadFailed'))
    toast.error(error.value)
  } finally {
    loading.value = false
  }
}

async function save() {
  error.value = ''
  const next = form.value.newPassword
  const current = form.value.currentPassword
  if ((next || current || form.value.confirmPassword) && next !== form.value.confirmPassword) {
    error.value = t('passwordMismatch')
    toast.error(error.value)
    return
  }
  saving.value = true
  try {
    const result = await saveProfile({
      realName: form.value.realName.trim(),
      email: form.value.email.trim(),
      mobile: form.value.mobile.trim(),
      gender: Number(form.value.gender) || 0,
      currentPassword: current,
      newPassword: next,
    })
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, t('profileFailed'))
      toast.error(error.value)
      return
    }
    const data = parseApiData<SelfProfile>(result)
    if (data?.realName) session.applyRealName(data.realName)
    open.value = false
  } catch (err) {
    error.value = getHttpErrorMessage(err, t('profileFailed'))
    toast.error(error.value)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <RsDialog v-model:open="open" :title="t('profileTitle')" width="420px">
    <RsForm :model="form" label-position="left" label-align="end" label-width="5.5rem" size="md">
      <RsFormItem name="userName" :label="t('userName')">
        <RsInput :model-value="form.userName" disabled size="md" />
      </RsFormItem>
      <RsFormItem name="realName" :label="t('realName')">
        <RsInput v-model="form.realName" maxlength="50" size="md" :disabled="loading" />
      </RsFormItem>
      <RsFormItem name="email" :label="t('email')">
        <RsInput v-model="form.email" maxlength="255" size="md" :disabled="loading" />
      </RsFormItem>
      <RsFormItem name="mobile" :label="t('mobile')">
        <RsInput v-model="form.mobile" maxlength="20" size="md" :disabled="loading" />
      </RsFormItem>
      <RsFormItem name="gender" :label="t('gender')">
        <RsSelect v-model="form.gender" :options="genderOptions" block match-trigger-width size="md" :disabled="loading" />
      </RsFormItem>
      <RsFormItem name="currentPassword" :label="t('currentPassword')">
        <RsInput v-model="form.currentPassword" type="password" autocomplete="current-password" :placeholder="t('passwordHint')" size="md" :disabled="loading" />
      </RsFormItem>
      <RsFormItem name="newPassword" :label="t('newPassword')">
        <RsInput v-model="form.newPassword" type="password" autocomplete="new-password" size="md" :disabled="loading" />
      </RsFormItem>
      <RsFormItem name="confirmPassword" :label="t('confirmPassword')">
        <RsInput v-model="form.confirmPassword" type="password" autocomplete="new-password" size="md" :disabled="loading" />
      </RsFormItem>
    </RsForm>
    <template #footer>
      <RsButton size="md" :disabled="saving" @click="open = false">{{ t('cancel') }}</RsButton>
      <RsButton variant="primary" size="md" :disabled="loading || saving" @click="save">{{ t('save') }}</RsButton>
    </template>
  </RsDialog>
</template>

