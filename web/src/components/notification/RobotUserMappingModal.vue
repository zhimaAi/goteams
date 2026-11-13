<template>
  <a-modal
    :open="open"
    :mask-closable="!mappingSaving"
    :closable="!mappingSaving"
    :keyboard="!mappingSaving"
    :cancel-button-props="{ disabled: mappingSaving }"
    @cancel="emit('update:open', false)"
    :title="$t('automation_paramMapping')"
    :confirm-loading="mappingSaving"
    :ok-button-props="{ disabled: mappingLoading || !mappingLoaded }"
    width="920px"
    @ok="saveMapping"
  >
    <div class="mapping-hint">{{ $t("automation_paramMappingHint") }}</div>
    <a-table
      :loading="mappingLoading"
      :data-source="mappingRows"
      :columns="mappingColumns"
      :pagination="false"
      row-key="user_id"
      size="small"
      bordered
      :scroll="{ x: 880, y: 360 }"
    >
      <template #bodyCell="{ column, record }">
        <a-input
          v-if="mappingPlaceholders[column.dataIndex]"
          v-model:value="record[column.dataIndex]"
          size="small"
          :placeholder="mappingPlaceholders[column.dataIndex]"
        />
      </template>
    </a-table>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { message } from "ant-design-vue";
import { useI18n } from "vue-i18n";
import {
  listNotificationUserMappings,
  saveNotificationUserMappings,
} from "@/api/workspace";
import {
  listAutomationUserMappings,
  saveAutomationUserMappings,
} from "@/api/automation";
const props = defineProps({
  open: Boolean,
  wsId: { type: [String, Number], default: undefined },
  enterprise: Boolean,
  members: { type: Array, default: () => [] },
});
const emit = defineEmits(["update:open"]);
const { t } = useI18n();
const members = computed(() => props.members);
const mappingSaving = ref(false);
const mappingLoading = ref(false);
const mappingRows = ref([]);
const mappingLoaded = ref(false);
let originalRows = new Map();
let loadVersion = 0;
// All robot channels share these API parameter mappings.
const mappingPlaceholders = {
  dingtalk_at_mobiles: "13800000000",
  dingtalk_at_user_ids: "userid",
  feishu_user_id: "ou_xxx / user_id",
  wecom_mentioned_list: "userid",
  wecom_mentioned_mobile_list: "13800000000",
};
const mappingFields = Object.keys(mappingPlaceholders);
const mappingColumns = computed(() => [
  {
    title: t("automation_paramMappingUser"),
    dataIndex: "user_name",
    width: 110,
    fixed: "left",
  },
  {
    title: t("automation_paramMappingPhone"),
    dataIndex: "user_phone",
    width: 130,
  },
  {
    title: t("automation_pushDingtalk"),
    children: [
      { title: "atMobiles", dataIndex: "dingtalk_at_mobiles", width: 140 },
      { title: "atUserIds", dataIndex: "dingtalk_at_user_ids", width: 140 },
    ],
  },
  {
    title: t("automation_pushFeishu"),
    children: [{ title: "user_id", dataIndex: "feishu_user_id", width: 150 }],
  },
  {
    title: t("automation_pushWecom"),
    children: [
      {
        title: "mentioned_list",
        dataIndex: "wecom_mentioned_list",
        width: 150,
      },
      {
        title: "mentioned_mobile_list",
        dataIndex: "wecom_mentioned_mobile_list",
        width: 170,
      },
    ],
  },
]);

async function resetRows() {
  const version = ++loadVersion;
  mappingRows.value = [];
  mappingLoaded.value = false;
  mappingLoading.value = true;
  try {
    const mappings = props.enterprise
      ? await listNotificationUserMappings()
      : await listAutomationUserMappings(props.wsId);
    if (version !== loadVersion || !props.open) return;
    const byId = new Map(
      mappings.map((item) => [Number(item.user_id), item.push]),
    );
    const rows = new Map();
    for (const item of members.value) {
      const id = Number(props.enterprise ? item.id : item.user_id);
      if (rows.has(id) || !byId.has(id)) continue;
      rows.set(id, {
        user_id: id,
        user_name: props.enterprise
          ? item.name || item.username
          : item.user_name,
        user_phone: props.enterprise ? item.phone : item.user_phone,
        ...Object.fromEntries(
          mappingFields.map((field) => [field, byId.get(id)[field] || ""]),
        ),
      });
    }
    mappingRows.value = [...rows.values()];
    originalRows = new Map(
      mappingRows.value.map((row) => [row.user_id, { ...row }]),
    );
    mappingLoaded.value = true;
  } catch {
    if (version === loadVersion) emit("update:open", false);
  } finally {
    if (version === loadVersion) mappingLoading.value = false;
  }
}
async function saveMapping() {
  const workspaceId = props.wsId;
  if (
    (!props.enterprise && !workspaceId) ||
    mappingSaving.value ||
    mappingLoading.value ||
    !mappingLoaded.value
  )
    return;
  mappingSaving.value = true;
  try {
    const items = mappingRows.value
      .map((item) => ({
        user_id: item.user_id,
        push: Object.fromEntries(
          mappingFields
            .filter(
              (field) => item[field] !== originalRows.get(item.user_id)[field],
            )
            .map((field) => [field, item[field]]),
        ),
      }))
      .filter((item) => Object.keys(item.push).length);
    if (items.length) {
      if (props.enterprise) await saveNotificationUserMappings(items);
      else await saveAutomationUserMappings(workspaceId, items);
    }
    message.success(t("common_saveSuccess"));
    emit("update:open", false);
  } finally {
    mappingSaving.value = false;
  }
}
watch(
  () => [props.open, props.wsId, props.enterprise],
  ([open]) => {
    ++loadVersion;
    mappingLoaded.value = false;
    if (open) resetRows();
  },
  { immediate: true },
);
</script>
<style scoped lang="less">
.mapping-hint {
  margin-bottom: 16px;
  color: var(--ant-color-text-secondary, #536074);
}
</style>
