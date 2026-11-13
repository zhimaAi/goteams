import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Supported languages.
const LOCALE_MAP = [
  { lang: 'zh-CN', label: '中文' },
  { lang: 'en-US', label: 'English' },
]

// Store the reference to the i18n instance (set by setupI18n).
let i18nInstance = null
export function setI18nInstance(instance) {
  i18nInstance = instance
}

export const useLocaleStore = defineStore(
  'locale',
  () => {
    // ---- State ----
    const currentLang = ref('zh-CN')
    // ---- Getters ----
    const localeMap = computed(() => LOCALE_MAP)
    const isZhCn = computed(() => currentLang.value === 'zh-CN')
    // 当前语言的显示标签（如 "中文" / "English"）
    const currentLangLabel = computed(() => {
      const item = LOCALE_MAP.find((item) => item.lang === currentLang.value)
      return item ? item.label : currentLang.value
    })

    // ---- Actions ----
    function setLang(lang) {
      if (LOCALE_MAP.some((item) => item.lang === lang)) {
        currentLang.value = lang
        // Sync the vue-i18n instance's locale.
        if (i18nInstance) {
          i18nInstance.global.locale.value = lang
        }
        // Set the html lang attribute.
        document.querySelector('html')?.setAttribute('lang', lang)
      }
    }

    function toggleLang() {
      setLang(currentLang.value === 'zh-CN' ? 'en-US' : 'zh-CN')
    }

    return {
      currentLang,
      localeMap,
      isZhCn,
      currentLangLabel,
      setLang,
      toggleLang,
    }
  },
  {
    persist: {
      key: 'goteams-locale',
    },
  },
)
