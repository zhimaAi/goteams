<template>
  <TemplateEditor
    v-if="editing"
    :ws-id="wsId"
    :type-key="typeKey"
    :template="editing"
    @cancel="editing = null"
    @saved="onSaved"
  />
  <div v-else class="settings-panel template-panel">
    <a-card size="small" class="settings-card">
      <template #title>
        <span>{{ $t("settings_template") }}</span>
        <span class="template-hint">{{ $t("settings_template_hint") }}</span>
      </template>
      <template #extra>
        <span class="switch-label">{{ $t("settings_template_enable") }}</span>
        <a-switch
          :checked="enabled"
          :loading="switchLoading"
          :checked-children="$t('settings_template_on')"
          :un-checked-children="$t('settings_template_off')"
          @change="onToggle"
        />
      </template>
      <template v-if="enabled">
        <a-tabs v-model:activeKey="activeTab" size="small">
          <a-tab-pane key="create" :tab="$t('settings_template_create_tab')">
        <div class="template-actions">
          <a-button type="primary" @click="openCreate">
            <PlusOutlined />
            {{ $t("settings_template_create") }}
          </a-button>
        </div>
        <a-table
          :data-source="items"
          :columns="columns"
          :pagination="false"
          row-key="id"
          size="middle"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <FileTextOutlined class="name-icon" />
              {{ record.name }}
            </template>
            <template v-else-if="column.key === 'actions'">
              <a-button type="link" size="small" @click="editing = record">
                {{ $t("common_edit") }}
              </a-button>
              <a-popconfirm
                :title="$t('settings_template_delete_confirm')"
                :ok-text="$t('common_confirm')"
                :cancel-text="$t('common_cancel')"
                @confirm="remove(record)"
              >
                <a-button type="link" size="small" danger>
                  {{ $t("common_delete") }}
                </a-button>
              </a-popconfirm>
            </template>
          </template>
        </a-table>
          </a-tab-pane>
          <a-tab-pane key="display" :tab="$t('settings_template_display_tab')">
            <DisplaySettingsTab :ws-id="wsId" :type-key="typeKey" />
          </a-tab-pane>
        </a-tabs>
      </template>
    </a-card>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { FileTextOutlined, PlusOutlined } from "@ant-design/icons-vue";
import {
  deleteWorkItemTemplate,
  listWorkItemTemplates,
  updateWorkItemTemplateSettings,
} from "@/api/workspace";
import TemplateEditor from "./TemplateEditor.vue";
import DisplaySettingsTab from "./DisplaySettingsTab.vue";

const props = defineProps({
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
});

const emit = defineEmits(["editing-change"]);
const { t } = useI18n();
const enabled = ref(false);
const items = ref([]);
const editing = ref(null);
const switchLoading = ref(false);
const activeTab = ref("create");

const columns = computed(() => [
  { title: t("settings_template_name"), key: "name", dataIndex: "name" },
  { title: t("workspace_action"), key: "actions", width: 160 },
]);

async function loadList() {
  const data = await listWorkItemTemplates(props.wsId, props.typeKey);
  enabled.value = !!data?.enabled;
  items.value = data?.items || [];
}

async function onToggle(value) {
  switchLoading.value = true;
  try {
    await updateWorkItemTemplateSettings(props.wsId, props.typeKey, value);
    enabled.value = value;
  } finally {
    switchLoading.value = false;
  }
}

function openCreate() {
  editing.value = { id: 0, name: "", title: "", description: "", fields: [] };
}

async function remove(record) {
  await deleteWorkItemTemplate(props.wsId, props.typeKey, record.id);
  message.success(t("settings_deleted"));
  await loadList();
}

async function onSaved() {
  editing.value = null;
  await loadList();
}

watch(editing, (value) => emit("editing-change", !!value), { immediate: true });

watch(
  () => [props.wsId, props.typeKey],
  () => {
    editing.value = null;
    loadList();
  },
);

onMounted(loadList);
onBeforeUnmount(() => emit("editing-change", false));
</script>

<style scoped lang="less">
.template-panel {
  min-height: 100%;
}
.settings-card {
  overflow: hidden;
  border-color: #eef1f6;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(24, 43, 80, 0.04);
  :deep(.ant-card-head) {
    min-height: 54px;
    padding: 0 16px;
    background: #f5f7fa;
    border-color: #eef1f6;
    font-weight: 600;
  }
  :deep(.ant-card-body) {
    padding: 16px;
  }
}
.template-hint {
  margin-left: 12px;
  color: #98a2b3;
  font-size: 13px;
  font-weight: 400;
}
.switch-label {
  margin-right: 8px;
  color: #667085;
  font-size: 13px;
}
.template-actions {
  display: flex;
  gap: 8px;
  margin: 16px 0;
}
.name-icon {
  margin-right: 8px;
  color: #1677ff;
}
</style>
