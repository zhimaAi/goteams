import { computed, ref } from 'vue'

const ISSUE_OPENID_STORAGE_KEY = 'goteams_issue_openid'

const OPENID_PATTERN = /^[A-Za-z0-9._-]{1,128}$/

// useIssueOpenid resolves URL-provided or locally generated public submitter state.
export function useIssueOpenid(route) {
  const localOpenid = ref('')
  const queryOpenid = computed(() => String(route.query.openid || ''))
  const resolvedOpenid = computed(() => queryOpenid.value || localOpenid.value)
  const invalid = ref(false)

  function generateOpenid() {
    // 24 random bytes encode to 32 URL-safe base64url characters (A-Z a-z 0-9 - _).
    const bytes = new Uint8Array(24)
    crypto.getRandomValues(bytes)
    let binary = ''
    for (const byte of bytes) binary += String.fromCharCode(byte)
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  }

  function persistOpenid(value) {
    try {
      localStorage.setItem(ISSUE_OPENID_STORAGE_KEY, value)
    } catch (error) {
      // Storage may be unavailable in private browsing; the in-memory value still works this session.
    }
  }

  function initialize() {
    const query = queryOpenid.value

    // A provided query value must pass validation, otherwise the link is treated as invalid.
    if (query) {
      if (!OPENID_PATTERN.test(query)) {
        invalid.value = true
        return
      }
      invalid.value = false
      localOpenid.value = ''
      persistOpenid(query)
      return
    }

    // Reuse a previously persisted valid value.
    let stored = ''
    try {
      stored = localStorage.getItem(ISSUE_OPENID_STORAGE_KEY) || ''
    } catch (error) {
      stored = ''
    }
    if (stored && OPENID_PATTERN.test(stored)) {
      invalid.value = false
      localOpenid.value = stored
      return
    }

    // Generate a fresh, stable identifier without fingerprinting the device.
    const generated = generateOpenid()
    persistOpenid(generated)
    localOpenid.value = generated
    invalid.value = false
  }

  return { resolvedOpenid, invalid, initialize }
}

export { ISSUE_OPENID_STORAGE_KEY }
