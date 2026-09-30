import { ref } from 'vue'

/** 与 index.html 里首屏脚本使用同一个键。 */
export const THEME_STORAGE_KEY = 'fg_theme'

export type AppTheme = 'light' | 'dark'

function readTheme(): AppTheme {
  try {
    const raw = localStorage.getItem(THEME_STORAGE_KEY)
    if (raw === 'light' || raw === 'dark') return raw
  } catch {
    // 读不到本地偏好时用默认暗色。
  }
  return 'dark'
}

export const theme = ref<AppTheme>(readTheme())

export function setTheme(next: AppTheme) {
  theme.value = next
  try {
    localStorage.setItem(THEME_STORAGE_KEY, next)
  } catch {
    // 写不进时只保留本次会话里的选择。
  }
}

export function toggleTheme() {
  setTheme(theme.value === 'dark' ? 'light' : 'dark')
}
