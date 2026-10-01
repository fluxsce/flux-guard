<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAiStore, useModuleAi } from '@/ai'
import { messageOfCode } from '@/api/errorText'
import { getApiMessage, getHttpErrorMessage, isApiSuccess, parseApiData } from '@/api/request'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { useProductVersion } from '@/productVersion'
import { RsButton, RsCard, RsForm, RsFormItem, RsInput, useRsToast } from '@/ui'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { useSessionStore } from '@/stores/session'
import { captcha as captchaApi, type CaptchaTicket } from '@/views/login/api'

const router = useRouter()
const session = useSessionStore()
const ai = useAiStore()
const { t, locale } = useModuleI18n('login')
const { version } = useProductVersion()
const form = ref({
  userId: 'admin',
  password: '',
  captchaCode: '',
})
const captchaId = ref('')
const captchaImage = ref('')
const captchaPending = ref(false)
const toast = useRsToast()
const error = ref('')
const pending = ref(false)
let captchaSeq = 0
let captchaTimer = 0

function clearCaptchaTimer() {
  if (captchaTimer) {
    window.clearTimeout(captchaTimer)
    captchaTimer = 0
  }
}

function scheduleCaptchaRefresh(expireAt: number) {
  clearCaptchaTimer()
  const wait = expireAt * 1000 - Date.now() - 2000
  if (wait <= 0) return
  captchaTimer = window.setTimeout(() => {
    void refreshCaptcha()
  }, wait)
}

async function refreshCaptcha() {
  const seq = ++captchaSeq
  captchaPending.value = true
  try {
    const result = await captchaApi()
    if (seq !== captchaSeq) return
    const data = parseApiData<CaptchaTicket>(result)
    if (!data?.captchaId || !data.image) {
      captchaId.value = ''
      captchaImage.value = ''
      return
    }
    captchaId.value = data.captchaId
    captchaImage.value = data.image
    form.value.captchaCode = ''
    scheduleCaptchaRefresh(data.expireAt)
  } catch {
    if (seq !== captchaSeq) return
    captchaId.value = ''
    captchaImage.value = ''
  } finally {
    if (seq === captchaSeq) captchaPending.value = false
  }
}

const rules = computed(() => {
  void locale.value
  return {
    userId: [{ required: true, message: messageOfCode('auth.user.required', '用户不能为空') }],
    password: [{ required: true, message: messageOfCode('auth.password.required', '密码不能为空') }],
    captchaCode: [
      { required: true, message: messageOfCode('auth.captcha.required', '验证码不能为空') },
      { len: 6, message: messageOfCode('auth.captcha.invalid', '验证码错误') },
    ],
  }
})

const signals = computed(() => [
  { name: t('agents'), detail: t('agentsDetail'), tone: 'live' },
  { name: t('samples'), detail: t('samplesDetail'), tone: 'info' },
  { name: t('access'), detail: t('accessDetail'), tone: 'warn' },
])

async function submit() {
  error.value = ''
  if (!captchaId.value) {
    error.value = t('captchaLoadFailed')
    toast.error(error.value)
    void refreshCaptcha()
    return
  }
  pending.value = true
  try {
    const result = await session.signIn(form.value.userId, form.value.password, {
      captchaId: captchaId.value,
      captchaCode: form.value.captchaCode,
    })
    if (!isApiSuccess(result) || !session.loaded) {
      error.value = getApiMessage(result, t('failed'))
      toast.error(error.value)
      await refreshCaptcha()
      return
    }
    const home = session.homePath()
    if (home === '/login') {
      error.value = t('noModule')
      toast.error(error.value)
      await session.signOut()
      await refreshCaptcha()
      return
    }
    await router.push(home)
  } catch (err) {
    error.value = getHttpErrorMessage(err, t('failed'))
    toast.error(error.value)
    await refreshCaptcha()
  } finally {
    pending.value = false
  }
}

onMounted(() => {
  void refreshCaptcha()
})

onUnmounted(() => {
  captchaSeq += 1
  clearCaptchaTimer()
})

ai.setActive('login')
useModuleAi(() => ({
  moduleId: 'login',
  title: '登录',
  snapshot: () => ({
    userId: form.value.userId,
    pending: pending.value,
    error: error.value,
  }),
}))
</script>

<template>
  <div class="fg-login" data-fg-ai="surface" data-fg-module="login">
    <div class="fg-login-sky" aria-hidden="true">
      <span class="fg-login-orb fg-login-orb-a" />
      <span class="fg-login-orb fg-login-orb-b" />
      <span class="fg-login-scan" />
    </div>

    <div class="fg-login-tools">
      <ThemeToggle />
    </div>

    <div class="fg-login-stage">
      <section class="fg-login-hero">
        <p class="fg-login-kicker">
          <span>FLUX GUARD</span>
          <span class="fg-login-ver">v{{ version }}</span>
        </p>
        <h1>{{ t('title') }}</h1>
        <p class="fg-login-lead">{{ t('lead') }}</p>
        <ul class="fg-login-signals">
          <li v-for="item in signals" :key="item.name">
            <span class="fg-login-dot" :data-tone="item.tone" />
            <span>
              <strong>{{ item.name }}</strong>
              <small>{{ item.detail }}</small>
            </span>
          </li>
        </ul>
        <svg class="fg-login-wave" viewBox="0 0 360 72" fill="none">
          <path class="fg-login-wave-dim" d="M0 46 H40 L58 46 L74 18 L90 58 L108 30 L126 46 H180 L198 46 L214 22 L232 54 L250 36 L268 46 H360" />
          <path class="fg-login-wave-hot" d="M0 46 H40 L58 46 L74 18 L90 58 L108 30 L126 46 H180 L198 46 L214 22 L232 54 L250 36 L268 46 H360" />
        </svg>
      </section>

      <RsCard class="fg-login-card" variant="glass" elevated radius="lg" :title="t('card')">
        <RsForm class="fg-login-form" :model="form" :rules="rules" label-position="top" @submit="submit">
          <RsFormItem name="userId">
            <template #label>
              <label for="login-user">{{ t('user') }}</label>
            </template>
            <input
              id="login-user"
              v-model="form.userId"
              class="fg-login-user"
              type="text"
              name="username"
              autocomplete="username"
            />
          </RsFormItem>
          <RsFormItem name="password" :label="t('password')">
            <RsInput v-model="form.password" type="password" name="password" autocomplete="current-password" />
          </RsFormItem>
          <RsFormItem name="captchaCode" :label="t('captcha')">
            <div class="fg-login-captcha">
              <RsInput
                v-model="form.captchaCode"
                name="captcha"
                inputmode="numeric"
                :maxlength="6"
                autocomplete="off"
                :placeholder="t('captchaPlaceholder')"
              />
              <button
                class="fg-login-captcha-image"
                type="button"
                :disabled="captchaPending"
                :aria-label="t('captchaHint')"
                @click="refreshCaptcha"
              >
                <img v-if="captchaImage" :src="captchaImage" alt="" />
                <span v-else>{{ t('captcha') }}</span>
              </button>
            </div>
          </RsFormItem>
          <p v-if="error" class="fg-login-error" role="alert">{{ error }}</p>
          <RsButton class="fg-login-submit" variant="primary" type="submit" :disabled="pending">
            {{ pending ? t('pending') : t('submit') }}
          </RsButton>
          <p class="fg-login-build">v{{ version }}</p>
        </RsForm>
      </RsCard>
    </div>
  </div>
</template>

<style scoped>
.fg-login {
  position: relative;
  min-height: 100vh;
  min-height: 100dvh;
  overflow: hidden;
  color: var(--rs-text-primary);
  background:
    radial-gradient(80% 55% at 78% 8%, color-mix(in srgb, var(--rs-primary) 22%, transparent), transparent 58%),
    radial-gradient(60% 45% at 8% 90%, color-mix(in srgb, var(--rs-info) 18%, transparent), transparent 55%),
    var(--rs-bg);
}

.fg-login-tools {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 2;
}

.fg-login-sky {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.fg-login-sky::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(color-mix(in srgb, var(--rs-text-primary) 8%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--rs-text-primary) 8%, transparent) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse 75% 70% at 40% 45%, #000 15%, transparent 78%);
}

.fg-login-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(8px);
}

.fg-login-orb-a {
  width: 280px;
  height: 280px;
  right: 8%;
  top: 12%;
  background: radial-gradient(circle, color-mix(in srgb, var(--rs-primary) 35%, transparent), transparent 68%);
}

.fg-login-orb-b {
  width: 360px;
  height: 360px;
  left: -80px;
  bottom: -40px;
  background: radial-gradient(circle, color-mix(in srgb, var(--rs-info) 28%, transparent), transparent 70%);
}

.fg-login-scan {
  position: absolute;
  left: 0;
  right: 0;
  height: 120px;
  background: linear-gradient(180deg, transparent, color-mix(in srgb, var(--rs-primary) 12%, transparent), transparent);
  animation: fg-scan 7.5s linear infinite;
}

.fg-login-stage {
  position: relative;
  z-index: 1;
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(320px, 420px);
  gap: 48px;
  align-items: center;
  width: min(1080px, calc(100% - 64px));
  margin: 0 auto;
}

.fg-login-kicker {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin: 0 0 12px;
  font-size: 12px;
  color: var(--rs-primary);
}

.fg-login-kicker > span:first-child {
  letter-spacing: 0.22em;
}

.fg-login-ver,
.fg-login-build {
  letter-spacing: 0;
  font-variant-numeric: tabular-nums;
}

.fg-login-build {
  margin: 4px 0 0;
  text-align: center;
  font-size: 12px;
  color: var(--rs-text-tertiary);
}

.fg-login-hero h1 {
  margin: 0;
  font-size: clamp(32px, 4vw, 48px);
  line-height: 1.15;
  letter-spacing: -0.03em;
  color: var(--rs-text-primary);
}

.fg-login-lead {
  max-width: 460px;
  margin: 14px 0 0;
  color: var(--rs-text-secondary);
  line-height: 1.6;
}

.fg-login-signals {
  list-style: none;
  margin: 28px 0 0;
  padding: 0;
  display: grid;
  gap: 10px;
  max-width: 420px;
}

.fg-login-signals li {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 12px 14px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--rs-surface) 72%, transparent);
  border: 1px solid var(--rs-border);
  backdrop-filter: blur(10px);
}

.fg-login-signals strong {
  display: block;
  font-size: 14px;
}

.fg-login-signals small {
  color: var(--rs-text-secondary);
}

.fg-login-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex: none;
  box-shadow: 0 0 0 6px color-mix(in srgb, var(--rs-success) 18%, transparent);
  animation: fg-pulse 2.2s ease-out infinite;
}

.fg-login-dot[data-tone='live'] {
  background: var(--rs-success);
}

.fg-login-dot[data-tone='info'] {
  background: var(--rs-info);
  box-shadow: 0 0 0 6px color-mix(in srgb, var(--rs-info) 18%, transparent);
}

.fg-login-dot[data-tone='warn'] {
  background: var(--rs-warning);
  box-shadow: 0 0 0 6px color-mix(in srgb, var(--rs-warning) 18%, transparent);
}

.fg-login-wave {
  width: min(420px, 100%);
  margin-top: 22px;
  opacity: 0.9;
}

.fg-login-wave-dim,
.fg-login-wave-hot {
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.fg-login-wave-dim {
  stroke: color-mix(in srgb, var(--rs-info) 45%, transparent);
}

.fg-login-wave-hot {
  stroke: var(--rs-primary);
  stroke-dasharray: 80 280;
  animation: fg-trace 3.6s linear infinite;
}

.fg-login-card {
  width: 100%;
}

.fg-login-user {
  width: 100%;
  height: var(--rs-control-height-md);
  padding: 0 var(--rs-space-md);
  border: 1px solid var(--rs-border);
  border-radius: var(--rs-radius-sm, 6px);
  background: var(--rs-input-bg, transparent);
  color: var(--rs-text);
  font: inherit;
  font-size: var(--rs-font-size-sm);
}

.fg-login-user:focus {
  outline: none;
  border-color: var(--rs-focus-border, var(--rs-primary));
  box-shadow: 0 0 0 var(--rs-focus-ring-width, 2px) var(--rs-focus-ring);
}

.fg-login-captcha {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 140px;
  gap: 8px;
  align-items: center;
}

.fg-login-error {
  margin: 0;
  font-size: 13px;
  color: var(--rs-danger);
}

.fg-login-captcha-image {
  height: 40px;
  padding: 0;
  border: 1px solid var(--rs-border);
  border-radius: var(--rs-radius-sm, 6px);
  background: #f4f4f4;
  cursor: pointer;
  overflow: hidden;
}

.fg-login-captcha-image:disabled {
  cursor: wait;
  opacity: 0.7;
}

.fg-login-captcha-image img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.fg-login-captcha-image span {
  display: grid;
  place-items: center;
  height: 100%;
  font-size: 12px;
  color: var(--rs-text-secondary);
}

.fg-login-submit {
  width: 100%;
}

@keyframes fg-scan {
  from {
    top: -120px;
  }
  to {
    top: 110%;
  }
}

@keyframes fg-pulse {
  0% {
    transform: scale(1);
  }
  70% {
    transform: scale(1);
    box-shadow: 0 0 0 8px transparent;
  }
  100% {
    transform: scale(1);
  }
}

@keyframes fg-trace {
  to {
    stroke-dashoffset: -360;
  }
}

@media (max-width: 860px) {
  .fg-login-stage {
    grid-template-columns: 1fr;
    width: min(440px, calc(100% - 32px));
    gap: 24px;
    padding: 32px 0;
    align-items: start;
  }

  .fg-login-wave {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .fg-login-scan,
  .fg-login-dot,
  .fg-login-wave-hot {
    animation: none;
  }
}
</style>
