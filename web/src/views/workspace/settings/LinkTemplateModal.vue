<template>
  <a-modal
    :open="open"
    :title="$t('settings_template_link_title')"
    :ok-text="$t('common_done')"
    :confirm-loading="submitting"
    :cancel-button-props="{ style: { display: 'none' } }"
    @ok="submit"
    @cancel="$emit('close')"
  >
    <div v-if="!items.length" class="empty">{{ $t("common_noData") }}</div>
    <div v-for="item in items" :key="item.id" class="pool-row">
      <a-checkbox
        :checked="selectedIds.includes(item.id)"
        @change="toggle(item.id, $event.target.checked)"
      >
        <span class="pool-name">
          <FileTextOutlined class="name-icon" />
          <span>{{ item.name }}</span>
          <span class="bind-count">{{
            $t("settings_template_bound_count", { n: item.bind_count || 0 })
          }}</span>
        </span>
      </a-checkbox>
      <a-popconfirm
        v-if="!item.bind_count && item.can_delete"
        :title="$t('settings_template_delete_pool_confirm')"
        :ok-text="$t('common_confirm')"
        :cancel-text="$t('common_cancel')"
        @confirm="removePool(item)"
      >
        <a-button type="link" size="small" danger>{{
          $t("common_delete")
        }}</a-button>
      </a-popconfirm>
    </div>
  </a-modal>
</template>

<script setup>
import { ref, watch } from "vue";
import { FileTextOutlined } from "@ant-design/icons-vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  deleteWorkItemTemplatePool,
  linkWorkItemTemplates,
  listWorkItemTemplatePool,
} from "@/api/workspace";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], default: "" },
  typeKey: { type: String, default: "requirement" },
});
const emit = defineEmits(["close", "linked"]);
const { t } = useI18n();
const items = ref([]);
const selectedIds = ref([]);
const submitting = ref(false);

async function loadPool() {
  items.value =
    (await listWorkItemTemplatePool(props.wsId, props.typeKey)) || [];
  selectedIds.value = [];
}

function toggle(id, checked) {
  selectedIds.value = checked
    ? [...selectedIds.value, id]
    : selectedIds.value.filter((item) => item !== id);
}

async function submit() {
  if (!selectedIds.value.length) {
    emit("close");
    return;
  }
  submitting.value = true;
  try {
    await linkWorkItemTemplates(props.wsId, props.typeKey, selectedIds.value);
    message.success(t("settings_updated"));
    emit("linked");
    emit("close");
  } finally {
    submitting.value = false;
  }
}

async function removePool(item) {
  await deleteWorkItemTemplatePool(props.wsId, props.typeKey, item.id);
  message.success(t("settings_deleted"));
  await loadPool();
}

watch(
  () => props.open,
  (open) => {
    if (open) loadPool();
  },
);
</script>

<style scoped lang="less">
.empty {
  padding: 24px 0;
  color: #98a2b3;
  text-align: center;
}
.pool-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid #eef1f6;
  border-radius: 8px;
}
.pool-name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.name-icon {
  color: #1677ff;
}
.bind-count {
  color: #98a2b3;
  font-size: 12px;
}
</style>
