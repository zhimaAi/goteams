import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

export const useThemeStore = defineStore(
  'theme',
  () => {
    const mode = ref('dark')
    const isDark = computed(() => mode.value === 'dark')

    function setMode(value) {
      if (value === 'light' || value === 'dark') mode.value = value
    }

    return { mode, isDark, setMode }
  },
  { persist: { key: 'goteams-theme' } },
)
