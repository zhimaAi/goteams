<template>
  <a-modal
    :open="open && chooserVisible"
    :title="$t('workspace_createProject')"
    :footer="null"
    width="1168px"
    :destroy-on-close="true"
    wrap-class-name="workspace-template-modal"
    @cancel="closeFlow"
  >
    <div class="template-card-grid">
      <div
        v-for="tpl in templates"
        :key="tpl.id"
        class="template-card"
        @click="createFromTemplate(tpl)"
      >
        <div class="template-card-media">
          <img
            v-if="tpl.icon"
            :src="tpl.icon"
            alt=""
            class="template-card-avatar"
          />
          <span
            v-else
            class="template-card-avatar template-card-avatar--fallback"
            :style="{ backgroundColor: tpl.color || '#0d68ff' }"
            >{{ (tpl.name || "").charAt(0) }}</span
          >
        </div>
        <div class="template-card-body">
          <div class="template-card-title">{{ tpl.name }}</div>
          <div v-if="tpl.description" class="template-card-desc">
            {{ tpl.description }}
          </div>
          <a-button
            type="primary"
            block
            class="template-card-create"
            @click.stop="createFromTemplate(tpl)"
          >
            {{ $t("workspace_createNow") }}
          </a-button>
        </div>
      </div>
    </div>
  </a-modal>

  <a-modal
    :open="open && formVisible"
    :title="$t('workspace_createProject')"
    :footer="null"
    width="800px"
    :destroy-on-close="true"
    wrap-class-name="create-workspace-modal"
    @cancel="closeFlow"
  >
    <a-form
      class="create-workspace-form"
      layout="vertical"
      :model="form"
      @finish="handleSubmit"
    >
      <a-form-item
        name="name"
        :rules="[
          {
            required: true,
            whitespace: true,
            message: $t('workspace_projectNameRequired'),
          },
        ]"
      >
        <div class="logo-name-grid">
          <div class="logo-col">
            <button
              type="button"
              class="logo-box"
              :style="{
                backgroundColor: customIcon ? 'transparent' : form.color,
              }"
              :aria-label="$t('workspace_uploadIcon')"
              @click="triggerUpload"
            >
              <img
                v-if="customIcon"
                :src="customIcon"
                alt=""
                class="logo-img"
              />
              <span
                v-else
                :style="{
                  color: isLightColor(form.color) ? '#333' : '#fff',
                }"
                >{{ logoChar }}</span
              >
              <span class="camera-badge"><CameraOutlined /></span>
            </button>
            <span class="upload-caption" @click="triggerUpload">{{
              $t("workspace_uploadIcon")
            }}</span>
            <input
              ref="fileInput"
              type="file"
              accept="image/jpeg,image/png"
              hidden
              @change="handleFileChange"
            />
          </div>
          <div class="field-col">
            <div class="field-label">
              <span class="required-mark">*</span>
              <span>{{ $t("workspace_projectName") }}</span>
            </div>
            <a-input
              v-model:value="form.name"
              :placeholder="$t('workspace_projectNamePlaceholder')"
              :maxlength="50"
            />
          </div>
        </div>
      </a-form-item>

      <a-form-item
        v-if="canManageTypes"
        name="workspaceType"
        :label="$t('workspace_projectType')"
        :rules="[
          { required: true, message: $t('workspace_projectTypeRequired') },
        ]"
      >
        <a-select
          v-model:value="form.workspaceType"
          :placeholder="$t('workspace_projectTypeRequired')"
          :options="
            workspaceTypes.map((type) => ({
              value: type.key,
              label: type.name,
            }))
          "
        >
          <template v-if="canManageTypes" #dropdownRender="{ menuNode }">
            <component :is="menuNode" />
            <a-divider style="margin: 4px 0" />
            <a-button type="text" block @click="openTypeModal">{{
              $t("workspace_addType")
            }}</a-button>
          </template>
        </a-select>
      </a-form-item>

      <a-form-item name="color" :label="$t('workspace_projectColor')">
        <div class="color-picker">
          <button
            v-for="color in presetColors"
            :key="color"
            type="button"
            class="color-item"
            :class="{ selected: !customIcon && form.color === color }"
            :style="{ backgroundColor: color }"
            :aria-label="color"
            :aria-pressed="!customIcon && form.color === color"
            @click="selectColor(color)"
          >
            <CheckOutlined
              v-if="!customIcon && form.color === color"
              :style="{
                color: isLightColor(color) ? '#333' : '#fff',
                fontSize: '14px',
              }"
            />
          </button>
        </div>
      </a-form-item>

      <a-form-item
        name="description"
        :label="$t('workspace_projectDescription')"
      >
        <a-textarea
          v-model:value="form.description"
          :rows="4"
          :placeholder="$t('workspace_descriptionPlaceholder')"
          :maxlength="500"
        />
      </a-form-item>

      <div class="form-actions">
        <a-button @click="closeFlow">{{ $t("common_cancel") }}</a-button>
        <a-button type="primary" html-type="submit" :loading="creating">{{
          $t("common_add")
        }}</a-button>
      </div>
    </a-form>
  </a-modal>

  <a-modal
    :open="open && typeModalVisible"
    :title="$t('workspace_addType')"
    :confirm-loading="creatingType"
    :ok-button-props="{ disabled: !newTypeName.trim() }"
    :destroy-on-close="true"
    @ok="handleCreateType"
    @cancel="typeModalVisible = false"
  >
    <a-form layout="vertical">
      <a-form-item :label="$t('workspace_typeName')" required>
        <a-input
          v-model:value="newTypeName"
          :maxlength="100"
          :placeholder="$t('workspace_typeNameRequired')"
          @press-enter="handleCreateType"
        />
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { ref, computed, reactive, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { CheckOutlined, CameraOutlined } from "@ant-design/icons-vue";
import { useWorkspaceStore } from "@/stores/workspace";
import {
  uploadWorkspaceIcon,
  listWorkspaceTemplates,
  listWorkspaceTypes,
  createWorkspaceType,
} from "@/api/workspace";

const { t: $t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  // Whether to jump to the new project settings page after creation; list pages keep the user in place.
  redirect: { type: Boolean, default: true },
});
const emit = defineEmits(["update:open", "created"]);

const router = useRouter();
const workspaceStore = useWorkspaceStore();
const fileInput = ref(null);
const customIcon = ref(null);
const selectedIconFile = ref(null);
const creating = ref(false);
// Two-step flow: the template list is fetched before any dialog appears — when template
// workspaces exist the chooser modal opens first and the create form (template pre-selected)
// follows; with an empty list the blank create form opens directly, so no dialog ever flashes.
const chooserVisible = ref(false);
const formVisible = ref(false);
const templates = ref([]);
const workspaceTypes = ref([]);
const canManageTypes = ref(false);
const typeModalVisible = ref(false);
const newTypeName = ref("");
const creatingType = ref(false);
let flowSeq = 0;

const form = reactive({
  name: "",
  color: "#0d68ff",
  description: "",
  templateWorkspaceId: 0,
  workspaceType: "software",
});

const presetColors = [
  "#0d68ff",
  "#52c41a",
  "#fa8c16",
  "#f5222d",
  "#722ed1",
  "#13c2c2",
  "#eb2f96",
  "#faad14",
  "#2f54eb",
  "#a0d911",
  "#fa541c",
];

const logoChar = computed(() =>
  (form.name.trim() || $t("workspace_projectName")).charAt(0),
);

watch(
  () => props.open,
  (open) => {
    if (open) {
      startFlow();
    } else {
      resetFlow();
    }
  },
);

async function startFlow() {
  flowSeq++;
  const seq = flowSeq;
  resetFormState();
  chooserVisible.value = false;
  formVisible.value = false;
  templates.value = [];
  workspaceTypes.value = [];
  canManageTypes.value = false;
  try {
    const types = await listWorkspaceTypes();
    if (seq !== flowSeq) return;
    workspaceTypes.value = types.list;
    canManageTypes.value = types.can_manage;
  } catch {
    if (seq === flowSeq) closeFlow();
    return;
  }
  // Keep every dialog closed while the template list loads; only the dialog that matches
  // the result (template chooser or blank create form) opens afterwards.
  try {
    const list = await listWorkspaceTemplates();
    if (seq !== flowSeq) return;
    // Backend list ids arrive as strings; normalize to numbers so the create payload
    // matches the int field of the create API.
    templates.value = Array.isArray(list)
      ? list.map((tpl) => ({ ...tpl, id: Number(tpl.id) }))
      : [];
    if (templates.value.length === 0) {
      // No templates available: jump straight to the blank create form.
      createFromTemplate(null);
    } else {
      chooserVisible.value = true;
    }
  } catch {
    if (seq !== flowSeq) return;
    templates.value = [];
    // Template lookup failed: keep creation usable with the blank create form.
    createFromTemplate(null);
  }
}

function createFromTemplate(tpl) {
  if (tpl) {
    const workspaceType = tpl.workspace_type || "software";
    if (!workspaceTypes.value.some((type) => type.key === workspaceType)) {
      message.error($t("workspace_templateTypeUnavailable"));
      return;
    }
    form.workspaceType = workspaceType;
    form.templateWorkspaceId = tpl.id;
    form.name = tpl.name || "";
  } else {
    form.workspaceType = "software";
    form.templateWorkspaceId = 0;
    form.name = "";
  }
  formVisible.value = true;
  chooserVisible.value = false;
}

function openTypeModal() {
  newTypeName.value = "";
  typeModalVisible.value = true;
}

async function handleCreateType() {
  const name = newTypeName.value.trim();
  if (!name || creatingType.value) return;
  const seq = flowSeq;
  creatingType.value = true;
  try {
    const type = await createWorkspaceType(name);
    if (seq !== flowSeq) return;
    workspaceTypes.value.push(type);
    form.workspaceType = type.key;
    typeModalVisible.value = false;
  } catch (error) {
    // Conflict responses are left to the caller by the shared request interceptor.
    if (error.response?.status === 409) {
      message.error(error.response.data?.message || $t("common_requestFailed"));
    }
  } finally {
    creatingType.value = false;
  }
}

function closeFlow() {
  emit("update:open", false);
}

function isLightColor(hex) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.6;
}

function triggerUpload() {
  fileInput.value?.click();
}

function resetFlow() {
  flowSeq++;
  chooserVisible.value = false;
  formVisible.value = false;
  templates.value = [];
  resetFormState();
}

function resetFormState() {
  typeModalVisible.value = false;
  form.name = "";
  form.description = "";
  form.color = "#0d68ff";
  form.workspaceType = "software";
  form.templateWorkspaceId = 0;
  customIcon.value = null;
  selectedIconFile.value = null;
  if (fileInput.value) fileInput.value.value = "";
}

function selectColor(color) {
  form.color = color;
  customIcon.value = null;
  selectedIconFile.value = null;
  if (fileInput.value) fileInput.value.value = "";
}

function handleFileChange(event) {
  const file = event.target.files[0];
  if (!file) return;

  if (!["image/jpeg", "image/png"].includes(file.type)) {
    message.error($t("workspace_projectIconInvalid"));
    event.target.value = "";
    return;
  }
  if (file.size > 100 * 1024) {
    message.error($t("workspace_projectIconTooLarge"));
    event.target.value = "";
    return;
  }

  selectedIconFile.value = file;
  const reader = new FileReader();
  reader.onload = (loadEvent) => {
    customIcon.value = loadEvent.target?.result;
  };
  reader.readAsDataURL(file);
}

async function handleSubmit() {
  const name = form.name.trim();
  if (!name || !form.workspaceType) return;
  const iconFile = selectedIconFile.value;

  creating.value = true;
  try {
    const workspace = await workspaceStore.createWorkspace({
      name,
      description: form.description.trim(),
      color: form.color,
      template_workspace_id: form.templateWorkspaceId || undefined,
      workspace_type: form.workspaceType,
    });

    let iconUploadFailed = false;
    if (iconFile) {
      try {
        const result = await uploadWorkspaceIcon(workspace.id, iconFile);
        workspace.icon = result?.icon || "";
        workspaceStore.setWorkspaceIconUrl(workspace.id, workspace.icon);
      } catch {
        iconUploadFailed = true;
      }
    }

    if (iconUploadFailed) {
      message.warning($t("workspace_createIconUploadFailed"));
    } else {
      message.success($t("workspace_createSuccess"));
    }
    emit("update:open", false);
    emit("created", workspace);
    if (props.redirect) router.push(`/workspace/${workspace.id}/settings`);
  } finally {
    creating.value = false;
  }
}
</script>

<style scoped lang="less">
.create-workspace-form {
  padding-top: 16px;
}

.logo-name-grid {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.logo-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-shrink: 0;
}

.logo-box {
  width: 48px;
  height: 48px;
  padding: 0;
  border: 0;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  line-height: 1;
  font-weight: 700;
  cursor: pointer;
  position: relative;
  user-select: none;
}

.logo-img {
  width: 100%;
  height: 100%;
  border-radius: inherit;
  object-fit: cover;
}

.camera-badge {
  position: absolute;
  right: -4px;
  bottom: -4px;
  width: 18px;
  height: 18px;
  border: 1.5px solid #fff;
  border-radius: 50%;
  background: #0d68ff;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 9px;
}

.upload-caption {
  margin-top: 8px;
  font-size: 12px;
  line-height: 16px;
  color: rgba(0, 0, 0, 0.45);
  cursor: pointer;
  white-space: nowrap;

  &:hover {
    color: #0d68ff;
  }
}

.field-col {
  flex: 1;
  min-width: 0;
}

.field-label {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  font-size: 14px;
  line-height: 22px;
  color: #262626;
}

.required-mark {
  margin-right: 4px;
  color: #ff4d4f;
}

.color-picker {
  display: flex;
  gap: 8px;
  flex-wrap: nowrap;
}

.color-item {
  width: 28px;
  height: 28px;
  padding: 0;
  border: 2px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;

  &.selected {
    border-color: #333;
  }
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 4px;
}

:deep(.ant-form-item) {
  margin-bottom: 20px;
}

:deep(.ant-form-item-label) {
  padding-bottom: 8px;
}

:deep(.ant-form-item-label > label) {
  height: 22px;
  color: #262626;
}

:deep(textarea.ant-input) {
  min-height: 98px;
  resize: none;
}

:global(.create-workspace-modal .ant-modal-content) {
  padding: 20px 24px;
  border-radius: 8px;
}

:global(.create-workspace-modal .ant-modal-header) {
  margin-bottom: 0;
}

:global(.create-workspace-modal .ant-modal-title) {
  font-size: 16px;
  line-height: 24px;
}

:global(.create-workspace-modal .ant-modal-close) {
  top: 12px;
  right: 12px;
  width: 32px;
  height: 32px;
}

:global(.workspace-template-modal .ant-modal-content) {
  padding: 20px 24px;
  border-radius: 8px;
}

:global(.workspace-template-modal .ant-modal-header) {
  margin-bottom: 12px;
}

:global(.workspace-template-modal .ant-modal-title) {
  font-size: 16px;
  line-height: 24px;
}

:global(.workspace-template-modal .ant-modal-close) {
  top: 12px;
  right: 12px;
  width: 32px;
  height: 32px;
}

.template-card-grid {
  display: grid;
  grid-template-columns: repeat(3, 359.56px);
  justify-content: center;
  gap: 16px;
  max-height: 52vh;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 2px;
}

.template-card {
  display: flex;
  flex-direction: column;
  border: 1px solid #e5e6eb;
  border-radius: 10px;
  overflow: hidden;
  background: #fff;
  cursor: pointer;
  transition:
    border-color 0.2s,
    box-shadow 0.2s;

  &:hover {
    border-color: #0d68ff;
    box-shadow: 0 4px 12px rgba(13, 104, 255, 0.12);
  }
}

.template-card-media {
  width: 359.56px;
  height: 151.5px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #eaf2ff;
}

.template-card-avatar {
  max-width: 100%;
  max-height: 100%;
  display: block;
}

.template-card-avatar--fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 40px;
  line-height: 1;
  font-weight: 700;
}

.template-card-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  padding: 12px 14px 14px;
}

.template-card-title {
  margin-bottom: 8px;
  font-size: 15px;
  line-height: 22px;
  font-weight: 600;
  color: #262626;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.template-card-desc {
  margin-bottom: 12px;
  font-size: 12px;
  line-height: 18px;
  color: rgba(0, 0, 0, 0.45);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.template-card-create {
  margin-top: auto;
  border-radius: 6px;
}
</style>
