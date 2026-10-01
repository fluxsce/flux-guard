<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { useSessionStore } from '@/stores/session'
import { RsAvatar, RsButton, RsCard, RsForm, RsFormItem, RsIcon, RsInput, useRsToast } from '@/ui'
import { profile, saveProfile, type SelfProfile } from '@/views/login/api'

const session = useSessionStore()
const { t } = useModuleI18n('shell')
const toast = useRsToast()
const loading = ref(false)
const saving = ref(false)
const loadFailed = ref(false)
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

const userLabel = computed(() => form.value.realName || form.value.userName || session.user?.realName || session.user?.userName || t('signedOut'))
const genderOptions = computed(() => [
  { label: t('genderUnknown'), value: 0 },
  { label: t('genderMale'), value: 1 },
  { label: t('genderFemale'), value: 2 },
])

function fill(data?: SelfProfile | null) {
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
}

async function load() {
  loading.value = true
  error.value = ''
  loadFailed.value = false
  try {
    const result = await profile()
    if (!isApiSuccess(result)) {
      error.value = getApiMessage(result, t('profileLoadFailed'))
      loadFailed.value = true
      toast.error(error.value)
      return
    }
    fill(parseApiData<SelfProfile>(result))
  } catch (err) {
    error.value = getHttpErrorMessage(err, t('profileLoadFailed'))
    loadFailed.value = true
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
    fill(data)
    toast.success(t('profileSaved'))
  } catch (err) {
    error.value = getHttpErrorMessage(err, t('profileFailed'))
    toast.error(error.value)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="account">
    <header class="account-hero">
      <RsAvatar :name="userLabel" size="lg" tone="primary" />
      <div class="account-id">
        <h1>{{ userLabel }}</h1>
        <p>{{ form.userName || t('consoleAccount') }}</p>
        <p class="account-lead">{{ t('profileLead') }}</p>
      </div>
    </header>

    <p v-if="error" class="account-error">
      <RsIcon name="circle-alert" :size="14" />
      <span>{{ error }}</span>
      <button v-if="loadFailed && !saving" type="button" @click="load">{{ t('retry') }}</button>
    </p>

    <div class="account-grid">
      <RsCard :title="t('profileSection')" variant="outlined" elevated>
        <RsForm :model="form" label-position="top" size="md">
          <div class="account-fields">
            <RsFormItem name="userName" :label="t('userName')">
              <RsInput :model-value="form.userName" disabled size="md" />
            </RsFormItem>
            <RsFormItem name="realName" :label="t('realName')">
              <RsInput v-model="form.realName" :maxlength="50" size="md" :disabled="loading || saving" />
            </RsFormItem>
            <RsFormItem name="email" :label="t('email')">
              <RsInput v-model="form.email" :maxlength="255" size="md" :disabled="loading || saving" />
            </RsFormItem>
            <RsFormItem name="mobile" :label="t('mobile')">
              <RsInput v-model="form.mobile" :maxlength="20" size="md" :disabled="loading || saving" />
            </RsFormItem>
            <RsFormItem class="account-span" name="gender" :label="t('gender')">
              <div class="account-seg" role="group" :aria-label="t('gender')">
                <button
                  v-for="option in genderOptions"
                  :key="option.value"
                  type="button"
                  :aria-pressed="form.gender === option.value"
                  :disabled="loading || saving"
                  @click="form.gender = option.value"
                >
                  {{ option.label }}
                </button>
              </div>
            </RsFormItem>
          </div>
        </RsForm>
      </RsCard>

      <RsCard :title="t('passwordSection')" :description="t('passwordHint')" variant="outlined" elevated>
        <RsForm :model="form" label-position="top" size="md">
          <RsFormItem name="currentPassword" :label="t('currentPassword')">
            <RsInput v-model="form.currentPassword" type="password" autocomplete="current-password" size="md" :disabled="loading || saving" />
          </RsFormItem>
          <RsFormItem name="newPassword" :label="t('newPassword')">
            <RsInput v-model="form.newPassword" type="password" autocomplete="new-password" size="md" :disabled="loading || saving" />
          </RsFormItem>
          <RsFormItem name="confirmPassword" :label="t('confirmPassword')">
            <RsInput v-model="form.confirmPassword" type="password" autocomplete="new-password" size="md" :disabled="loading || saving" />
          </RsFormItem>
        </RsForm>
      </RsCard>
    </div>

    <div class="account-bar">
      <RsButton variant="primary" size="md" :disabled="loading || saving" @click="save">{{ t('save') }}</RsButton>
    </div>
  </div>
</template>

<style scoped>
.account {
  width: min(920px, 100%);
  margin: 4px auto 8px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.account-hero {
  position: relative;
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 22px 22px 18px;
  overflow: hidden;
  border: 1px solid var(--rs-border);
  border-radius: 14px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--rs-primary) 16%, var(--rs-surface)), var(--rs-surface) 70%),
    linear-gradient(110deg, color-mix(in srgb, var(--fg-aurora-c) 14%, transparent), transparent 55%),
    var(--rs-surface);
}

.account-hero::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: linear-gradient(90deg, var(--fg-aurora-a), var(--fg-aurora-b) 48%, var(--fg-aurora-c));
}

.account-hero :deep(.rs-avatar) {
  flex: none;
  --rs-avatar-size: 56px;
  border-color: color-mix(in srgb, var(--rs-primary) 40%, var(--rs-border));
  box-shadow:
    0 0 0 4px color-mix(in srgb, var(--rs-primary) 14%, var(--rs-surface)),
    0 10px 18px color-mix(in srgb, var(--rs-primary) 16%, transparent);
}

.account-hero :deep(.rs-avatar__fallback) {
  font-size: 22px;
  font-weight: 600;
}

.account-id {
  min-width: 0;
}

.account-id h1,
.account-id p {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-id h1 {
  font-size: 20px;
  font-weight: 600;
  line-height: 1.25;
  letter-spacing: 0.01em;
}

.account-id p {
  margin-top: 4px;
  color: var(--rs-text-secondary);
  font-size: 13px;
  line-height: 1.3;
}

.account-lead {
  color: var(--rs-text-tertiary) !important;
  font-size: 12px !important;
}

.account-error {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--rs-danger) 10%, var(--rs-surface));
  color: var(--rs-danger);
  font-size: 13px;
}

.account-error span {
  flex: 1;
  min-width: 0;
}

.account-error button {
  margin: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  text-decoration: underline;
}

.account-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(280px, 0.8fr);
  gap: 14px;
  align-items: start;
}

.account-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 12px;
}

.account-span {
  grid-column: 1 / -1;
}

.account-seg {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  height: 32px;
  padding: 3px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--rs-text-primary) 6%, transparent);
}

.account-seg button {
  height: 26px;
  margin: 0;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--rs-text-secondary);
  font: inherit;
  font-size: 13px;
  line-height: 26px;
  cursor: pointer;
}

.account-seg button:hover:not(:disabled) {
  color: var(--rs-text-primary);
}

.account-seg button:disabled {
  cursor: default;
  opacity: 0.6;
}

.account-seg button:focus-visible {
  outline: 2px solid var(--rs-primary);
  outline-offset: 1px;
}

.account-seg button[aria-pressed='true'] {
  background: color-mix(in srgb, var(--rs-primary) 16%, var(--rs-surface-elevated));
  color: var(--rs-primary);
  font-weight: 600;
  box-shadow:
    0 1px 2px color-mix(in srgb, var(--rs-text-primary) 10%, transparent),
    inset 0 0 0 1px color-mix(in srgb, var(--rs-primary) 38%, transparent);
}

.account-bar {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 860px) {
  .account-grid,
  .account-fields {
    grid-template-columns: 1fr;
  }

  .account-span {
    grid-column: auto;
  }
}
</style>
