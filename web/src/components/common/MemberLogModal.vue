<template>
  <a-modal
    :open="open"
    :title="$t('member_logTitle')"
    :width="720"
    :footer="null"
    @cancel="handleClose"
  >
    <div class="member-log-search">
      <a-input-search
        v-model:value="keyword"
        :placeholder="$t('member_logSearchPlaceholder')"
        allow-clear
        @search="handleSearch"
        @press-enter="handleSearch"
      />
    </div>

    <a-table
      :columns="columns"
      :data-source="items"
      :loading="loading"
      :pagination="pagination"
      row-key="id"
      size="middle"
      @change="handleTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'action'">
          <a-tag :color="actionColor(record.action)">
            {{ actionLabel(record.action) }}
          </a-tag>
        </template>
        <template v-else-if="column.key === 'target'">
          <span class="member-log-name">{{ record.target_name || "-" }}</span>
        </template>
        <template v-else-if="column.key === 'operator'">
          <span>{{ record.operator_name || "-" }}</span>
        </template>
        <template v-else-if="column.key === 'created_at'">
          <span class="member-log-time">{{
            formatLocalDate(record.created_at, true)
          }}</span>
        </template>
      </template>
    </a-table>

    <div class="member-log-footer">
      <a-button @click="handleClose">{{ $t("common_close") }}</a-button>
    </div>
  </a-modal>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { listMemberLogs } from "@/api/workspace";
import { formatLocalDate } from "@/utils/date";

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
});
const emit = defineEmits(["update:open"]);

const items = ref([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref("");
const loading = ref(false);

const columns = computed(() => [
  { title: t("member_logColumnAction"), key: "action", width: 120 },
  { title: t("member_logColumnTarget"), key: "target" },
  { title: t("member_logColumnOperator"), key: "operator", width: 160 },
  { title: t("member_logColumnTime"), key: "created_at", width: 190 },
]);

const pagination = computed(() => ({
  current: page.value,
  pageSize: pageSize.value,
  total: total.value,
  showSizeChanger: false,
  hideOnSinglePage: true,
}));

const ACTION_COLORS = {
  create: "blue",
  remove: "red",
  enable: "green",
  disable: "orange",
};

function actionColor(action) {
  return ACTION_COLORS[action] || "default";
}

function actionLabel(action) {
  const key = {
    create: "member_logActionCreate",
    remove: "member_logActionRemove",
    enable: "member_logActionEnable",
    disable: "member_logActionDisable",
  }[action];
  return key ? t(key) : action;
}

async function fetchLogs() {
  loading.value = true;
  try {
    const res = await listMemberLogs({
      page: page.value,
      page_size: pageSize.value,
      keyword: keyword.value || undefined,
    });
    items.value = res?.items || [];
    total.value = Number(res?.total || 0);
  } catch {
    items.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function handleSearch() {
  page.value = 1;
  fetchLogs();
}

function handleTableChange(pag) {
  page.value = pag.current;
  fetchLogs();
}

function handleClose() {
  emit("update:open", false);
}

// Reset to the first page on each open so a stale keyword/page never carries over.
watch(
  () => props.open,
  (visible) => {
    if (!visible) return;
    keyword.value = "";
    page.value = 1;
    fetchLogs();
  },
);
</script>

<style scoped lang="less">
.member-log-search {
  margin-bottom: 16px;
  max-width: 320px;
}

.member-log-name {
  font-weight: 500;
  color: #182b50;
}

.member-log-time {
  color: #6b7a99;
}

.member-log-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
