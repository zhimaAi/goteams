<template>
  <div class="agent-setup-guide">
    <div class="setup-card">
      <!-- Left: copy area -->
      <div class="setup-left">
        <p class="setup-eyebrow">{{ $t('pipeline_setup_eyebrow') }}</p>
        <h1 class="setup-title">{{ $t('pipeline_setup_title') }}</h1>
        <p class="setup-desc">{{ activeDesc }}</p>

        <div class="setup-tabs">
          <button
            class="setup-tab"
            :class="{ active: activeTab === 'employee' }"
            @click="activeTab = 'employee'"
          >
            {{ $t('pipeline_setup_tab_employee') }}
          </button>
          <button
            class="setup-tab"
            :class="{ active: activeTab === 'asset' }"
            @click="activeTab = 'asset'"
          >
            {{ $t('pipeline_setup_tab_asset') }}
          </button>
        </div>

        <div class="setup-actions">
          <a-button class="setup-start-btn" @click="goStart">
            {{ $t('pipeline_setup_start') }}
          </a-button>
          <a-button class="setup-download-btn" @click="openDownload">
            <img :src="downloadIcon" class="setup-download-icon" alt="" />
            {{ $t('agent_download_client') }}
          </a-button>
        </div>
      </div>

      <!-- Right: feature preview -->
      <div class="setup-right">
        <div class="setup-preview">
          <img :src="previewImage" class="setup-preview-img" alt="" />
        </div>
      </div>
    </div>

    <!-- 下载客户端弹窗（与 Agent 列表页共用） -->
    <AgentDownloadModal v-model:open="downloadVisible" />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AgentDownloadModal from '@/components/agent/AgentDownloadModal.vue'
import downloadIcon from '@/image/agent/download-icon.svg'
import previewImage from '@/image/agent/setup-guide-employee.png'

const router = useRouter()
const { t: $t } = useI18n()

// A flag marking that the setup guide was viewed, to avoid an empty-list auto-redirect loop.
const SEEN_KEY = 'pipeline_setup_guide_seen'

const activeTab = ref('employee')
const downloadVisible = ref(false)

// Switch the description copy when changing tabs (the preview image reuses the same placeholder).
const activeDesc = computed(() =>
  activeTab.value === 'employee'
    ? $t('pipeline_setup_employee_desc')
    : $t('pipeline_setup_asset_desc'),
)

// Get started: mark as viewed and return to the pipeline list.
function goStart() {
  localStorage.setItem(SEEN_KEY, '1')
  router.push('/pipelines')
}

// 下载客户端：标记已查看并打开下载弹窗
function openDownload() {
  localStorage.setItem(SEEN_KEY, '1')
  downloadVisible.value = true
}
</script>

<style scoped>
.agent-setup-guide {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  background: #fff;
  color: #1f1f1f;
}

.setup-card {
  display: grid;
  grid-template-columns: 320px 480px;
  gap: 56px;
  width: 100%;
  max-width: 960px;
  padding: 48px;
  background: #ffffff;
  border: 1px solid #e8e8e8;
  border-radius: 24px;
  box-shadow: 0 14px 40px rgba(15, 23, 42, 0.08);
}

.setup-left {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.setup-eyebrow {
  margin: 0 0 16px;
  color: #8c8c8c;
  font-family: 'PingFang SC', sans-serif;
  font-size: 14px;
  font-weight: 500;
  letter-spacing: 0.04em;
}

.setup-title {
  margin: 0 0 16px;
  color: #1f1f1f;
  font-family: 'PingFang SC', sans-serif;
  font-size: 36px;
  font-weight: 600;
  line-height: 46px;
  letter-spacing: -0.02em;
}

.setup-desc {
  margin: 0 0 32px;
  color: #595959;
  font-family: 'PingFang SC', sans-serif;
  font-size: 20px;
  font-weight: 400;
  line-height: 32px;
}

.setup-tabs {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  margin-bottom: 32px;
}

.setup-tab {
  height: 56px;
  padding: 0 20px;
  border: none;
  border-radius: 12px;
  background: #f5f7fa;
  color: #595959;
  font-family: 'PingFang SC', sans-serif;
  font-size: 18px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
  transition: all 0.2s ease;
}

.setup-tab.active {
  background: #e5efff;
  color: #3157e2;
  font-weight: 600;
}

.setup-actions {
  display: flex;
  gap: 16px;
}

.setup-start-btn {
  height: 56px;
  padding: 0 32px;
  border-radius: 10px;
  background: #3157e2;
  border-color: #3157e2;
  box-shadow: 0 8px 18px rgba(49, 87, 226, 0.2);
  color: #fff;
  font-size: 16px;
  font-weight: 600;
}

.setup-start-btn:hover {
  background: #2a4bc9;
  border-color: #2a4bc9;
  transform: translateY(-1px);
  box-shadow: 0 12px 22px rgba(49, 87, 226, 0.28);
  color: #fff;
}

.setup-download-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 56px;
  padding: 0 24px;
  border-radius: 10px;
  background: #fff;
  border-color: #d9d9d9;
  color: #1f1f1f;
  font-size: 16px;
  font-weight: 500;
}

.setup-download-btn:hover {
  border-color: #3157e2;
  color: #3157e2;
  transform: translateY(-1px);
}

.setup-download-icon {
  width: 20px;
  height: 20px;
}

.setup-right {
  display: flex;
  align-items: center;
}

.setup-preview {
  width: 100%;
  padding: 14px;
  background: #eef2ff;
  border-radius: 26px;
}

.setup-preview-img {
  display: block;
  width: 100%;
  height: 261.76px;
  object-fit: cover;
  border-radius: 14px;
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.1);
}

@media (max-width: 960px) {
  .setup-card {
    grid-template-columns: 1fr;
    gap: 40px;
    padding: 32px;
  }

  .setup-right {
    order: 2;
  }

  .setup-actions {
    width: 100%;
  }

  .setup-start-btn,
  .setup-download-btn {
    flex: 1;
    justify-content: center;
  }
}
</style>
