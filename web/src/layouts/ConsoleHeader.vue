<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getHttpErrorMessage } from '@/api/request'
import { setLocale, type AppLocale } from '@/locales'
import { useModuleI18n } from '@/locales/useModuleI18n'
import { moduleIcon } from '@/router'
import { ACCOUNT_PATH, modulePath, useSessionStore } from '@/stores/session'
import { setTheme, theme, type AppTheme } from '@/layouts/theme'
import ServerUpgradeDialog from '@/layouts/ServerUpgradeDialog.vue'
import { useProductVersion } from '@/productVersion'
import { RsAvatar, RsBreadcrumb, RsIcon, RsPopover } from '@/ui'
import type { Menu } from '@/views/login/api'

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { t, locale } = useModuleI18n('shell')
const { version } = useProductVersion()
const now = ref(new Date())
const accountOpen = ref(false)
const upgradeOpen = ref(false)
const query = ref('')
const searchOpen = ref(false)
const activeIndex = ref(0)
let clockTimer = 0

const currentMenu = computed(() => session.menus.find((item) => modulePath(item.resourceCode) === route.path))
const pageTitle = computed(() => {
  if (route.path === ACCOUNT_PATH) return t('profile')
  return currentMenu.value?.resourceName || t('overview')
})
const clockTime = computed(() => now.value.toLocaleTimeString(locale.value, { hour12: false }))
const crumbs = computed(() => {
  const home = session.homePath()
  const page = pageTitle.value
  const group = currentMenu.value?.groupName || ''
  const items: { key: string; label: string; to?: string }[] = [
    { key: 'home', label: t('product'), to: route.path === home ? undefined : home },
  ]
  if (group && group !== page) items.push({ key: 'group', label: group })
  if (page !== t('product')) items.push({ key: 'page', label: page })
  return items
})
const userLabel = computed(() => session.user?.realName || session.user?.userName || t('signedOut'))
const userAccount = computed(() => session.user?.userName || '')
const userMark = computed(() => userLabel.value.trim().slice(0, 1))
const canUpgrade = computed(() => session.allowButton('fg0001:upgrade'))
const hits = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  const menus = session.menus.filter((menu) => menu.resourceCode)
  const matched = keyword
    ? menus.filter((menu) => `${menu.resourceName} ${menu.groupName} ${menu.resourceCode}`.toLowerCase().includes(keyword))
    : menus
  const list = matched.slice(0, 8)
  if (!keyword) return list
  const account: Menu = {
    resourceCode: 'account',
    resourceName: t('profile'),
    resourcePath: ACCOUNT_PATH,
    parentResourceId: '',
    groupName: '',
    sortOrder: 0,
  }
  const text = `${account.resourceName} account`.toLowerCase()
  if (!text.includes(keyword)) return list
  return [account, ...list.filter((item) => item.resourceCode !== 'account')].slice(0, 8)
})
const cursor = computed(() => Math.min(activeIndex.value, Math.max(hits.value.length - 1, 0)))
const searchListOpen = computed(() => searchOpen.value && (query.value.trim().length > 0 || hits.value.length > 0))

function onCrumb(item: { to?: string }, event: MouseEvent) {
  if (!item.to) return
  event.preventDefault()
  void router.push(item.to)
}

function pickTheme(next: AppTheme) {
  setTheme(next)
}

function pickLocale(next: AppLocale) {
  void setLocale(next)
}

function openSearch() {
  searchOpen.value = true
}

function goPage(menu: Menu) {
  query.value = ''
  searchOpen.value = false
  activeIndex.value = 0
  const path = modulePath(menu.resourceCode)
  if (path && path !== route.path) void router.push(path)
}

function onSearchKey(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    searchOpen.value = true
    if (!hits.value.length) return
    const step = event.key === 'ArrowDown' ? 1 : -1
    activeIndex.value = (cursor.value + step + hits.value.length) % hits.value.length
    return
  }
  if (event.key === 'Enter') {
    const hit = hits.value[cursor.value]
    if (!hit) return
    event.preventDefault()
    goPage(hit)
    return
  }
  if (event.key === 'Escape') searchOpen.value = false
}

function onSearchBlur(event: FocusEvent) {
  const next = event.relatedTarget
  const root = event.currentTarget
  if (next instanceof Node && root instanceof Node && root.contains(next)) return
  searchOpen.value = false
}

function openProfile() {
  accountOpen.value = false
  if (route.path !== ACCOUNT_PATH) void router.push(ACCOUNT_PATH)
}

function openUpgrade() {
  accountOpen.value = false
  upgradeOpen.value = true
}

function hitIcon(code: string) {
  if (code === 'account') return 'user-pen'
  return moduleIcon(code)
}

async function onLogout() {
  accountOpen.value = false
  try {
    await session.signOut()
  } catch (err) {
    const message = getHttpErrorMessage(err, '')
    if (!message) return
  }
  await router.push('/login')
}

onMounted(() => {
  clockTimer = window.setInterval(() => {
    now.value = new Date()
  }, 1000)
})

onUnmounted(() => {
  window.clearInterval(clockTimer)
})

watch(query, () => {
  activeIndex.value = 0
})

watch(
  () => route.path,
  () => {
    query.value = ''
    searchOpen.value = false
  },
)
</script>

<template>
  <header class="fg-bar">
    <RsBreadcrumb class="fg-crumb" :items="crumbs" @click="onCrumb" />
    <div class="fg-search" @focusout="onSearchBlur">
      <label class="fg-search-field">
        <RsIcon name="search" :size="14" />
        <input
          v-model="query"
          type="search"
          :placeholder="t('searchPages')"
          :aria-label="t('searchPages')"
          role="combobox"
          aria-autocomplete="list"
          aria-controls="fg-page-search-list"
          :aria-expanded="searchListOpen"
          @focus="openSearch"
          @input="openSearch"
          @keydown="onSearchKey"
        />
      </label>
      <ul id="fg-page-search-list" v-show="searchListOpen" class="fg-search-list" role="listbox">
        <li v-if="query.trim() && !hits.length" class="fg-search-empty">{{ t('searchEmpty') }}</li>
        <li v-for="(hit, index) in hits" :key="hit.resourceCode">
          <button
            type="button"
            role="option"
            :class="{ 'is-active': index === cursor }"
            :aria-selected="index === cursor"
            @mousedown.prevent
            @mouseenter="activeIndex = index"
            @click="goPage(hit)"
          >
            <RsIcon :name="hitIcon(hit.resourceCode)" :size="14" />
            <span class="fg-search-name">{{ hit.resourceName }}</span>
            <span v-if="hit.groupName" class="fg-search-group">{{ hit.groupName }}</span>
          </button>
        </li>
      </ul>
    </div>
    <div class="fg-bar-end">
      <time class="fg-clock" :datetime="now.toISOString()">{{ clockTime }}</time>
      <RsPopover
        v-model:open="accountOpen"
        side="bottom"
        align="end"
        :side-offset="8"
        width="auto"
        size="sm"
        popup-class-name="fg-account-pop"
        :aria-label="t('accountMenu')"
      >
        <button type="button" class="fg-user" :class="{ 'is-open': accountOpen }" :aria-expanded="accountOpen">
          <RsAvatar :name="userLabel" :fallback="userMark" size="sm" tone="primary" />
          <span class="fg-user-name">{{ userLabel }}</span>
          <RsIcon name="chevron-down" :size="12" class="fg-user-caret" />
        </button>
        <template #content>
          <div class="fg-account">
            <div class="fg-account-id">
              <RsAvatar :name="userLabel" size="md" tone="primary" />
              <div class="fg-account-copy">
                <strong>{{ userLabel }}</strong>
                <span>{{ userAccount || t('consoleAccount') }}</span>
                <span v-if="version">v{{ version }}</span>
              </div>
            </div>
            <div class="fg-account-prefs">
              <div class="fg-account-row">
                <span class="fg-account-kicker">
                  <RsIcon name="sun-moon" :size="13" />
                  {{ t('appearance') }}
                </span>
                <div class="fg-seg" role="group" :aria-label="t('appearance')">
                  <button type="button" :aria-pressed="theme === 'light'" @click="pickTheme('light')">{{ t('light') }}</button>
                  <button type="button" :aria-pressed="theme === 'dark'" @click="pickTheme('dark')">{{ t('dark') }}</button>
                </div>
              </div>
              <div class="fg-account-row">
                <span class="fg-account-kicker">
                  <RsIcon name="languages" :size="13" />
                  {{ t('language') }}
                </span>
                <div class="fg-seg" role="group" :aria-label="t('language')">
                  <button type="button" :aria-pressed="locale === 'zh-CN'" @click="pickLocale('zh-CN')">{{ t('zh') }}</button>
                  <button type="button" :aria-pressed="locale === 'en-US'" @click="pickLocale('en-US')">{{ t('en') }}</button>
                </div>
              </div>
            </div>
            <button v-if="canUpgrade" type="button" class="fg-account-set" @click="openUpgrade">
              <RsIcon name="package" :size="14" />
              <span>{{ t('upgradeServer') }}</span>
            </button>
            <button type="button" class="fg-account-set" @click="openProfile">
              <RsIcon name="user-pen" :size="14" />
              <span>{{ t('profile') }}</span>
            </button>
            <button type="button" class="fg-account-out" @click="onLogout">
              <RsIcon name="log-out" :size="14" />
              <span>{{ t('signOut') }}</span>
            </button>
          </div>
        </template>
      </RsPopover>
      <ServerUpgradeDialog v-model:open="upgradeOpen" />
    </div>
  </header>
</template>

<style scoped>
.fg-bar {
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: var(--fg-header-h);
  min-height: var(--fg-header-h);
  max-height: var(--fg-header-h);
  flex: none;
  padding: 0 16px;
  border: 0;
  box-shadow: inset 0 calc(var(--fg-line) * -1) 0 var(--rs-border);
  background: var(--rs-surface);
}

.fg-crumb {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
}

.fg-search {
  position: relative;
  flex: 0 1 280px;
  min-width: 160px;
}

.fg-search-field {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  border: 1px solid color-mix(in srgb, var(--rs-border) 80%, transparent);
  border-radius: 999px;
  background: color-mix(in srgb, var(--rs-bg) 72%, var(--rs-surface));
  color: var(--rs-text-tertiary);
}

.fg-search-field:focus-within {
  border-color: color-mix(in srgb, var(--rs-primary) 45%, var(--rs-border));
  color: var(--rs-text-secondary);
}

.fg-search-field input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--rs-text-primary);
  font: inherit;
  font-size: 13px;
  line-height: 1;
  outline: none;
}

.fg-search-field input::placeholder {
  color: var(--rs-text-tertiary);
}

.fg-search-field input::-webkit-search-cancel-button {
  appearance: none;
}

.fg-search-list {
  position: absolute;
  z-index: 30;
  top: calc(100% + 6px);
  right: 0;
  left: 0;
  margin: 0;
  padding: 4px;
  list-style: none;
  border: 1px solid var(--rs-border);
  border-radius: 10px;
  background: var(--rs-surface);
  box-shadow: var(--rs-shadow-md, 0 8px 24px rgb(0 0 0 / 12%));
}

.fg-search-list button,
.fg-search-empty {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--rs-text-primary);
  font: inherit;
  font-size: 13px;
  text-align: left;
}

.fg-search-list button {
  cursor: pointer;
}

.fg-search-list button.is-active,
.fg-search-list button:hover {
  background: color-mix(in srgb, var(--rs-primary) 12%, transparent);
}

.fg-search-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-search-group,
.fg-search-empty {
  color: var(--rs-text-tertiary);
  font-size: 12px;
}

.fg-crumb :deep(.rs-breadcrumb__list) {
  flex-wrap: nowrap;
}

.fg-bar-end {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
  height: 100%;
}

.fg-clock {
  font-family: var(--rs-font-mono, ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
  font-size: 13px;
  line-height: 1;
  letter-spacing: 0.02em;
  color: var(--rs-text-secondary);
}

.fg-user {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  margin: 0;
  padding: 0 4px 0 2px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--rs-text-secondary);
  font: inherit;
  font-size: 13px;
  font-weight: 400;
  cursor: pointer;
}

.fg-user:hover,
.fg-user.is-open {
  background: color-mix(in srgb, var(--rs-text-primary) 6%, transparent);
  color: var(--rs-text-primary);
}

.fg-user:focus-visible {
  outline: 2px solid var(--rs-primary);
  outline-offset: 2px;
}

.fg-user :deep(.rs-avatar) {
  --rs-avatar-size: 20px;
  border-color: transparent;
  box-shadow: none;
}

.fg-user :deep(.rs-avatar__fallback) {
  font-size: 11px;
  font-weight: 600;
}

.fg-user-name {
  max-width: 96px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.fg-user-caret {
  color: var(--rs-text-tertiary);
  transition: transform 160ms ease, color 160ms ease;
}

.fg-user.is-open .fg-user-caret {
  color: var(--rs-primary);
  transform: rotate(180deg);
}

.fg-account {
  display: flex;
  flex-direction: column;
  width: 280px;
}

.fg-account-id {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 14px 14px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--rs-primary) 18%, transparent), transparent 78%),
    linear-gradient(100deg, color-mix(in srgb, var(--fg-aurora-c) 16%, transparent), transparent 58%);
  box-shadow: inset 0 calc(var(--fg-line) * -1) 0 var(--rs-border-subtle);
}

.fg-account-id::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  background: linear-gradient(90deg, var(--fg-aurora-a), var(--fg-aurora-b) 48%, var(--fg-aurora-c));
}

.fg-account-id :deep(.rs-avatar) {
  border-color: color-mix(in srgb, var(--rs-primary) 40%, var(--rs-border));
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--rs-primary) 16%, var(--rs-surface-elevated)),
    0 8px 16px color-mix(in srgb, var(--rs-primary) 18%, transparent);
}

.fg-account-copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
  min-width: 0;
}

.fg-account-copy strong,
.fg-account-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fg-account-copy strong {
  font-size: 14px;
  font-weight: 600;
  line-height: 1.2;
  letter-spacing: 0.01em;
}

.fg-account-copy span {
  font-size: 12px;
  line-height: 1.2;
  color: var(--rs-text-secondary);
}

.fg-account-prefs {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 12px 10px;
}

.fg-account-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fg-account-kicker {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--rs-text-tertiary);
  font-size: 11px;
  line-height: 1;
}

.fg-seg {
  display: grid;
  grid-template-columns: 1fr 1fr;
  height: 30px;
  padding: 3px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--rs-text-primary) 6%, transparent);
}

.fg-seg button {
  height: 24px;
  margin: 0;
  padding: 0 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--rs-text-secondary);
  font: inherit;
  font-size: 12px;
  line-height: 24px;
  cursor: pointer;
  transition: background 140ms ease, color 140ms ease, box-shadow 140ms ease;
}

.fg-seg button:hover {
  color: var(--rs-text-primary);
}

.fg-seg button:focus-visible {
  outline: 2px solid var(--rs-primary);
  outline-offset: 1px;
}

.fg-seg button[aria-pressed='true'] {
  background: color-mix(in srgb, var(--rs-primary) 16%, var(--rs-surface-elevated));
  color: var(--rs-primary);
  font-weight: 600;
  box-shadow:
    0 1px 2px color-mix(in srgb, var(--rs-text-primary) 10%, transparent),
    inset 0 0 0 1px color-mix(in srgb, var(--rs-primary) 38%, transparent);
}

.fg-account-set,
.fg-account-out {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  margin: 0;
  padding: 0 14px;
  border: 0;
  border-radius: 0;
  background: transparent;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  box-shadow: inset 0 var(--fg-line) 0 var(--rs-border-subtle);
}

.fg-account-set {
  color: var(--rs-text-primary);
}

.fg-account-set:hover {
  background: color-mix(in srgb, var(--rs-primary) 10%, transparent);
}

.fg-account-set:focus-visible {
  outline: 2px solid var(--rs-primary);
  outline-offset: -2px;
}

.fg-account-out {
  color: var(--rs-danger);
}

.fg-account-out:hover {
  background: color-mix(in srgb, var(--rs-danger) 12%, transparent);
}

.fg-account-out:focus-visible {
  outline: 2px solid var(--rs-danger);
  outline-offset: 1px;
}
</style>
