<template>
  <a-modal
    :open="open"
    :title="$t('agent_download_client')"
    :footer="null"
    width="760px"
    @update:open="(v) => emit('update:open', v)"
  >
    <p class="download-tip">{{ $t('agent_download_desc') }}</p>
    <a-spin :spinning="releaseLoading">
      <div class="release-grid">
        <div
          v-for="item in releases"
          :key="item.id"
          class="release-card"
          :class="{ 'release-recommended': item.id === currentReleaseKey }"
        >
          <a-tag v-if="item.id === currentReleaseKey" color="green" class="release-badge">
            {{ $t('agent_download_recommended') }}
          </a-tag>
          <div class="release-os">{{ releaseMeta(item).os }}</div>
          <div class="release-arch">{{ releaseMeta(item).arch }}</div>
          <div class="release-desc">{{ releaseMeta(item).desc }}</div>
          <a-button
            type="primary"
            block
            :href="item.download_url"
            target="_blank"
            :disabled="!item.download_url"
          >{{ $t('agent_download') }}</a-button>
        </div>
      </div>
    </a-spin>
    <a-empty
      v-if="!releaseLoading && releases.length === 0"
      :description="$t('agent_download_empty')"
    />
  </a-modal>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listClientReleases } from '@/api/pipeline'

const props = defineProps({
  open: { type: Boolean, default: false },
})
const emit = defineEmits(['update:open'])

const { t: $t } = useI18n()

const releases = ref([])
const releaseLoading = ref(false)

// 根据当前浏览器/系统识别推荐下载版本（platform-arch）
const currentReleaseKey = computed(() => {
  const ua = navigator.userAgent
  let platform = ''
  if (/Win/i.test(ua)) platform = 'windows'
  else if (/Mac/i.test(ua)) platform = 'darwin'
  else if (/Linux|X11/i.test(ua)) platform = 'linux'
  if (!platform) return ''
  let arch = 'amd64'
  if (/\b(ARM64|aarch64|arm64)\b/i.test(ua)) {
    arch = 'arm64'
  }
  // macOS Apple Silicon：Chromium 的 navigator.userAgentData.architecture 更可靠
  const uaData = navigator.userAgentData
  if (platform === 'darwin' && uaData && uaData.architecture === 'arm') {
    arch = 'arm64'
  }
  return `${platform}-${arch}`
})

// 弹窗打开时拉取可下载版本列表（下载地址来自服务端配置）
watch(
  () => props.open,
  (open) => {
    if (open) loadReleases()
  },
)

async function loadReleases() {
  releaseLoading.value = true
  try {
    const res = await listClientReleases()
    releases.value = res.list || []
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    releaseLoading.value = false
  }
}

// 根据平台/架构返回展示用的操作系统名、架构标签与说明文案
function releaseMeta(item) {
  const platformName = {
    darwin: $t('agent_platform_macos'),
    windows: $t('agent_platform_windows'),
    linux: $t('agent_platform_linux'),
  }[item.platform] || item.platform
  const archName = {
    amd64: $t('agent_arch_amd64'),
    arm64: $t('agent_arch_arm64'),
  }[item.arch] || item.arch
  let desc = ''
  if (item.platform === 'darwin') {
    desc = item.arch === 'arm64' ? $t('agent_desc_mac_apple') : $t('agent_desc_mac_intel')
  } else {
    desc = `${platformName} · ${archName}`
  }
  return { os: platformName, arch: archName, desc }
}
</script>

<style scoped>
.download-tip {
  margin-bottom: 16px;
  color: rgba(0, 0, 0, 0.65);
}
.release-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
.release-card {
  position: relative;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: #fff;
}
.release-card.release-recommended {
  border-color: #52c41a;
  box-shadow: 0 0 0 2px rgba(82, 196, 26, 0.15);
}
.release-badge {
  position: absolute;
  top: 8px;
  right: 8px;
}
.release-os {
  font-size: 16px;
  font-weight: 600;
  color: rgba(0, 0, 0, 0.85);
}
.release-arch {
  color: rgba(0, 0, 0, 0.65);
  font-size: 13px;
}
.release-desc {
  color: rgba(0, 0, 0, 0.45);
  font-size: 12px;
  min-height: 32px;
}
.release-card .ant-btn {
  margin-top: auto;
}
</style>
