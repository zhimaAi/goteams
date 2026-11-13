import { createI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/en'
import { useLocaleStore, setI18nInstance } from '@/stores/locale'
import zhCN from './zh-CN'
import enUS from './en-US'

const DAYJS_LOCALE_MAP = {
  'zh-CN': 'zh-cn',
  'en-US': 'en',
}

export let i18n

export const setupI18n = async (app) => {
  const localeStore = useLocaleStore()
  const lang = localeStore.currentLang

  i18n = createI18n({
    legacy: false,
    locale: lang,
    fallbackLocale: 'zh-CN',
    messages: {
      'zh-CN': zhCN,
      'en-US': enUS,
    },
    silentTranslationWarn: true,
    missingWarn: false,
    silentFallbackWarn: true,
  })

  // Inject the i18n instance into the locale store to avoid circular dependency.
  setI18nInstance(i18n)

  app.use(i18n)

  // After the Pinia persist plugin restores asynchronously, re-sync the i18n locale.
  // Use watch to ensure the value restored by persist is synced to vue-i18n.
  const { watch } = await import('vue')
  watch(
    () => localeStore.currentLang,
    (newLang) => {
      if (i18n && i18n.global.locale.value !== newLang) {
        i18n.global.locale.value = newLang
      }
      dayjs.locale(DAYJS_LOCALE_MAP[newLang] || 'zh-cn')
      document.querySelector('html')?.setAttribute('lang', newLang)
    },
    { immediate: true },
  )
}
