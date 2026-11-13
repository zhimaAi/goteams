import { computed, ref, watch } from 'vue'
import { useLocaleStore } from '@/stores/locale'

// Supported public-link languages; anything else falls back to zh-CN.
const SUPPORTED_LANGS = ['zh-CN', 'en-US']
const DEFAULT_LANG = 'zh-CN'

// The link carries the language as `?Lang=`; `lang` is accepted as a tolerant alias.
const LANG_QUERY_KEYS = ['Lang', 'lang']

// localStorage key used by the locale store's persist plugin; read directly because
// pinia restores persisted state asynchronously, which can lag behind the first render.
const LOCALE_STORAGE_KEY = 'goteams-locale'

// Normalize an incoming language value; empty or unknown values fall back to zh-CN.
export function normalizeIssueInviteLang(value) {
  const raw = String(value ?? '').trim()
  if (!raw) return DEFAULT_LANG
  // Accept case-insensitive matches (e.g. `en-us`) but keep the canonical spelling.
  const matched = SUPPORTED_LANGS.find((item) => item.toLowerCase() === raw.toLowerCase())
  return matched || DEFAULT_LANG
}

// readStoredLang reads the persisted language straight from localStorage.
// The persist plugin stores a JSON payload; a bare string is tolerated so a value
// written by another part of the app is still understood. Corrupt or missing
// values yield an empty string, letting the caller fall through to the default.
function readStoredLang() {
  let raw = ''
  try {
    raw = localStorage.getItem(LOCALE_STORAGE_KEY) || ''
  } catch (error) {
    // Storage may be unavailable in private browsing; fall through to the default.
    return ''
  }
  if (!raw) return ''
  try {
    const parsed = JSON.parse(raw)
    // The persist plugin serializes the store state, so the language sits under `currentLang`.
    if (parsed && typeof parsed === 'object') {
      return String(parsed.currentLang ?? '').trim()
    }
  } catch (error) {
    // Not JSON: treat the raw value as the language itself.
  }
  return raw.trim()
}

// useIssueInviteLang resolves the display language of a public invitation page.
// Priority: `?Lang=` in the URL > the language persisted in localStorage > zh-CN.
// The URL always wins so a shared link renders in the language its sender picked.
export function useIssueInviteLang(route) {
  const localeStore = useLocaleStore()
  const explicitLang = ref('')
  // Captured once: the persisted value is the language the visitor last used before
  // opening this link, which is the fallback the link itself does not override.
  const storedLang = ref(normalizeIssueInviteLang(readStoredLang()))

  const queryLang = computed(() => {
    for (const key of LANG_QUERY_KEYS) {
      const value = route.query[key]
      if (value !== undefined && value !== null && String(value).trim() !== '') {
        return String(value)
      }
    }
    return ''
  })

  const currentLang = computed(
    () => explicitLang.value || storedLang.value || localeStore.currentLang || DEFAULT_LANG,
  )

  // Re-read on entry and whenever the link's language changes, e.g. history
  // navigation between two links carrying different `Lang` values.
  watch(
    queryLang,
    (raw) => {
      if (!raw) {
        explicitLang.value = ''
        return
      }
      const lang = normalizeIssueInviteLang(raw)
      explicitLang.value = lang
      // Keep the app language in sync so `$t()` and any later request agree.
      if (localeStore.currentLang !== lang) {
        localeStore.setLang(lang)
      }
    },
    { immediate: true },
  )

  return { currentLang }
}

export { SUPPORTED_LANGS, DEFAULT_LANG, LOCALE_STORAGE_KEY }
