import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { isModuleMessagesLoaded, loadModuleMessages, type AppLocale } from '@/locales'

/** 组件里按模块名取文案。键会自动加上模块前缀，例如 t('title') → fg0001.title。 */
export function useModuleI18n(moduleName: string) {
  const i18n = useI18n()
  const loaded = ref(isModuleMessagesLoaded(moduleName, i18n.locale.value as AppLocale))

  async function load(lang = i18n.locale.value) {
    const target = lang === 'en-US' ? 'en-US' : 'zh-CN'
    if (isModuleMessagesLoaded(moduleName, target)) {
      loaded.value = true
      return
    }
    await loadModuleMessages(moduleName, target)
    loaded.value = true
  }

  function t(key: string) {
    const fullKey = key.startsWith(`${moduleName}.`) ? key : `${moduleName}.${key}`
    return i18n.t(fullKey)
  }

  watch(
    () => i18n.locale.value,
    (next) => {
      void load(next)
    },
  )

  if (!loaded.value) {
    void load(i18n.locale.value)
  }

  return {
    t,
    locale: computed(() => i18n.locale.value),
    loaded: computed(() => loaded.value),
  }
}
