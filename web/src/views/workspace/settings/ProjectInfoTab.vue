<template>
  <div class="project-info-tab">
    <a-spin :spinning="loading">
      <a-form
        ref="formRef"
        class="project-info-form"
        layout="vertical"
        :model="form"
        @finish="handleSave"
      >
        <section class="info-card">
          <header class="card-header">{{ $t('workspace_basicInfo') }}</header>

          <div class="card-body">
            <div class="project-logo-section">
              <button
                type="button"
                class="logo-box"
                :class="{ disabled: !canEdit }"
                :style="{
                  backgroundColor: displayIcon ? 'transparent' : form.color,
                }"
                :aria-label="$t('workspace_uploadIcon')"
                :disabled="!canEdit"
                @click="triggerUpload"
              >
                <img
                  v-if="displayIcon"
                  :src="displayIcon"
                  alt=""
                  class="logo-img"
                />
                <span
                  v-else
                  :style="{ color: isLightColor(form.color) ? '#fff' : '#fff' }"
                >
                  {{ logoChar }}
                </span>
                <span class="camera-badge"><CameraOutlined /></span>
              </button>
              <p class="logo-hint">{{ $t('workspace_projectIconHint') }}</p>
              <input
                ref="fileInput"
                type="file"
                accept="image/jpeg,image/png"
                hidden
                @change="handleFileChange"
              />
            </div>

            <a-form-item name="color" :label="$t('workspace_projectColor')">
              <div class="color-picker">
                <button
                  v-for="color in presetColors"
                  :key="color"
                  type="button"
                  class="color-item"
                  :class="{ selected: !displayIcon && form.color === color }"
                  :style="{ backgroundColor: color }"
                  :aria-label="color"
                  :aria-pressed="!displayIcon && form.color === color"
                  :disabled="!canEdit"
                  @click="selectColor(color)"
                >
                  <CheckOutlined
                    v-if="!displayIcon && form.color === color"
                    :style="{
                      color: isLightColor(color) ? '#333' : '#fff',
                      fontSize: '14px',
                    }"
                  />
                </button>
              </div>
            </a-form-item>

            <a-form-item
              name="name"
              :label="$t('workspace_projectName')"
              :rules="[
                {
                  required: true,
                  whitespace: true,
                  message: $t('workspace_projectNameRequired'),
                },
              ]"
            >
              <a-input
                v-model:value="form.name"
                :placeholder="$t('workspace_projectNamePlaceholder')"
                :maxlength="50"
                :disabled="!canEdit"
              />
            </a-form-item>

            <a-form-item
              name="description"
              :label="$t('workspace_projectDescription')"
              class="description-item"
            >
              <a-textarea
                v-model:value="form.description"
                :rows="4"
                :placeholder="$t('workspace_descriptionPlaceholder')"
                :maxlength="500"
                :disabled="!canEdit"
              />
            </a-form-item>
          </div>
        </section>

        <a-button
          v-if="canEdit"
          class="save-button"
          type="primary"
          html-type="submit"
          block
          :loading="saving"
        >
          {{ $t('profile_saveProfile') }}
        </a-button>
      </a-form>
    </a-spin>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { CameraOutlined, CheckOutlined } from '@ant-design/icons-vue'
import {
  deleteWorkspaceIcon,
  getWorkspace,
  uploadWorkspaceIcon,
} from '@/api/workspace'
import { useAuthStore } from '@/stores/auth'
import { useWorkspaceStore } from '@/stores/workspace'

const { t: $t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const workspaceStore = useWorkspaceStore()

const formRef = ref(null)
const fileInput = ref(null)
const pendingIconFile = ref(null)
const iconPreviewUrl = ref('')
const removeSavedIcon = ref(false)
const loading = ref(false)
const saving = ref(false)
const requestSequence = ref(0)
const form = reactive({ name: '', color: '#0d68ff', description: '' })

const presetColors = [
  '#0d68ff',
  '#52c41a',
  '#fa8c16',
  '#f5222d',
  '#722ed1',
  '#13c2c2',
  '#eb2f96',
  '#faad14',
  '#2f54eb',
  '#a0d911',
  '#fa541c',
]

const workspaceId = computed(() => String(route.params.wsId || ''))
const canEdit = computed(() => authStore.hasPermission('project_info:edit'))
const savedIconUrl = computed(
  () => workspaceStore.iconUrls[workspaceId.value] || '',
)
const displayIcon = computed(
  () =>
    iconPreviewUrl.value || (!removeSavedIcon.value ? savedIconUrl.value : ''),
)
const logoChar = computed(() =>
  (form.name.trim() || $t('workspace_projectName')).charAt(0),
)

watch(workspaceId, loadWorkspace, { immediate: true })
onBeforeUnmount(clearIconPreview)

async function loadWorkspace(id) {
  if (!id) return
  const sequence = ++requestSequence.value
  loading.value = true
  try {
    const workspace = await getWorkspace(id)
    if (sequence !== requestSequence.value) return
    form.name = workspace?.name || ''
    form.description = workspace?.description || ''
    form.color = workspace?.color || '#0d68ff'
    clearIconPreview()
    pendingIconFile.value = null
    removeSavedIcon.value = false
    workspaceStore.syncWorkspace(workspace)
    if (workspace?.icon) await workspaceStore.loadWorkspaceIcon(id)
    if (sequence !== requestSequence.value) return
    formRef.value?.clearValidate()
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    if (sequence === requestSequence.value) loading.value = false
  }
}

function isLightColor(hex) {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.6
}

function triggerUpload() {
  if (canEdit.value) fileInput.value?.click()
}

function selectColor(color) {
  if (!canEdit.value) return
  form.color = color
  removeSavedIcon.value = Boolean(displayIcon.value)
  pendingIconFile.value = null
  clearIconPreview()
  if (fileInput.value) fileInput.value.value = ''
}

function handleFileChange(event) {
  const file = event.target.files[0]
  if (!file) return

  if (!['image/jpeg', 'image/png'].includes(file.type)) {
    message.error($t('workspace_projectIconInvalid'))
    event.target.value = ''
    return
  }
  if (file.size > 100 * 1024) {
    message.error($t('workspace_projectIconTooLarge'))
    event.target.value = ''
    return
  }

  clearIconPreview()
  pendingIconFile.value = file
  removeSavedIcon.value = false
  iconPreviewUrl.value = URL.createObjectURL(file)
}

function clearIconPreview() {
  if (iconPreviewUrl.value) URL.revokeObjectURL(iconPreviewUrl.value)
  iconPreviewUrl.value = ''
}

async function handleSave() {
  if (!canEdit.value || saving.value) return
  saving.value = true
  try {
    const iconFile = pendingIconFile.value
    const shouldRemoveIcon = removeSavedIcon.value
    const hadSavedIcon = Boolean(
      workspaceStore.workspaces.find((item) => item.id === workspaceId.value)
        ?.icon,
    )
    await workspaceStore.updateWorkspace(workspaceId.value, {
      name: form.name.trim(),
      description: form.description.trim(),
      color: form.color,
    })
    if (iconFile) {
      const result = await uploadWorkspaceIcon(workspaceId.value, iconFile)
      workspaceStore.setWorkspaceIconUrl(workspaceId.value, result?.icon || '')
    } else if (shouldRemoveIcon && hadSavedIcon) {
      await deleteWorkspaceIcon(workspaceId.value)
      workspaceStore.removeWorkspaceIcon(workspaceId.value)
    }
    clearIconPreview()
    pendingIconFile.value = null
    removeSavedIcon.value = false
    form.name = form.name.trim()
    form.description = form.description.trim()
    message.success($t('workspace_saveSuccess'))
  } catch {
    // The request layer displays errors uniformly.
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="less">
.project-info-tab {
  min-height: 100%;
  padding: 8px 24px 32px;
}

.project-info-form {
  width: min(560px, 100%);
  margin: 0 auto;
}

.info-card {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  background: var(--app-surface);
  box-shadow: 0 2px 10px rgba(24, 43, 80, 0.04);
}

.card-header {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  background: var(--app-surface-muted);
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
}

.card-body {
  padding: 24px 16px 16px;
}

.project-logo-section {
  height: 104px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.logo-box {
  width: 56px;
  height: 56px;
  padding: 0;
  border: 0;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  color: #fff;
  font-size: 24px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  user-select: none;

  &.disabled {
    cursor: not-allowed;
    opacity: 0.7;
  }
}

.logo-img {
  width: 100%;
  height: 100%;
  border-radius: inherit;
  object-fit: cover;
}

.camera-badge {
  position: absolute;
  right: -8px;
  bottom: -7px;
  width: 28px;
  height: 28px;
  border: 2px solid var(--app-surface);
  border-radius: 50%;
  background: #1677ff;
  box-shadow: 0 3px 6px rgba(22, 119, 255, 0.28);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
}

.logo-hint {
  margin: 16px 0 0;
  color: var(--app-text-secondary);
  font-size: 14px;
  line-height: 22px;
}

.color-picker {
  display: flex;
  gap: 8px;
  flex-wrap: nowrap;
}

.color-item {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;

  &.selected {
    border-color: #333;
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }
}

.save-button {
  height: 32px;
  margin-top: 24px;
  background: #365ae6;
  border-color: #365ae6;
}

:deep(.ant-form-item) {
  margin-bottom: 16px;
}

:deep(.ant-form-item-label) {
  padding-bottom: 8px;
}

:deep(.ant-form-item-label > label) {
  height: 22px;
  color: var(--app-text);
}

:deep(.description-item) {
  margin-bottom: 0;
}

:deep(textarea.ant-input) {
  min-height: 98px;
  resize: none;
}

@media (max-width: 720px) {
  .project-info-tab {
    padding: 8px 0 24px;
  }

  .color-picker {
    flex-wrap: wrap;
  }
}
</style>
