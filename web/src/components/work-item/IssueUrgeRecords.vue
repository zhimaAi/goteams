<template>
  <section class="issue-urge-records" :aria-busy="loading">
    <h3>{{ t("issue_urge_customer_records") }}</h3>
    <a-spin v-if="loading" />
    <a-alert
      v-else-if="failed"
      type="error"
      :message="t('common_requestFailed')"
      show-icon
    >
      <template #action
        ><a-button @click="load()">{{ t("issue_retry") }}</a-button></template
      >
    </a-alert>
    <template v-else>
      <a-empty
        v-if="!records.length"
        :image="Empty.PRESENTED_IMAGE_SIMPLE"
        :description="t('issue_urge_empty')"
      />
      <article
        v-for="(record, index) in records"
        :key="record.id"
        class="urge-record"
      >
        <header class="urge-record-header">
          <a-avatar :size="20" class="urge-avatar">{{
            t("issue_invite_submitter_avatar")
          }}</a-avatar>
          <span>{{ t("issue_invite_filler") }}</span>
          <time>{{ formatLocalDate(record.created_at, true) }}</time>
          <a-tag color="orange" class="urge-record-number">{{
            incompleteHistory
              ? t("view_field_urge_status")
              : t("issue_urge_nth", { n: total - (page - 1) * size - index })
          }}</a-tag>
        </header>
        <p>{{ t("issue_urge_content") }}</p>
      </article>
      <a-pagination
        v-if="total > size"
        :current="page"
        :page-size="size"
        :total="total"
        :show-size-changer="false"
        :disabled="loading"
        size="small"
        @change="changePage"
      />
    </template>
  </section>
</template>

<script setup>
import { ref, watch, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import { Empty } from "ant-design-vue";
import { listIssueUrgeRecords } from "@/api/issueUrge";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  wsId: { type: [String, Number], required: true },
  requirementId: { type: [String, Number], required: true },
  reloadKey: { type: [String, Number], default: 0 },
});
const { t } = useI18n();
const records = ref([]);
const page = ref(1);
const size = 20;
const total = ref(0);
const incompleteHistory = ref(false);
const loading = ref(false);
const failed = ref(false);
let sequence = 0;
let controller;

async function load() {
  const request = ++sequence;
  controller?.abort();
  controller = new AbortController();
  records.value = [];
  total.value = 0;
  incompleteHistory.value = false;
  failed.value = false;
  if (!props.wsId || !props.requirementId) {
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const result = await listIssueUrgeRecords(
      props.wsId,
      props.requirementId,
      { page: page.value, size },
      controller.signal,
    );
    if (request !== sequence) return;
    records.value = result.list;
    total.value = result.total;
    incompleteHistory.value = result.incomplete_history;
  } catch {
    if (request === sequence) failed.value = true;
  } finally {
    if (request === sequence) loading.value = false;
  }
}

function changePage(value) {
  page.value = value;
  void load();
}

watch(
  () => [props.wsId, props.requirementId, props.reloadKey],
  () => {
    page.value = 1;
    void load();
  },
  { immediate: true },
);
onBeforeUnmount(() => {
  sequence++;
  controller?.abort();
});
</script>

<style scoped lang="less">
.issue-urge-records {
  padding: 12px 0;
  color: var(--app-text);
  h3 {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
  }
  > .ant-alert {
    margin-bottom: 12px;
  }
}
.urge-record {
  padding: 12px;
  margin-bottom: 10px;
  border: 1px solid var(--app-border, #e8edf4);
  border-radius: 8px;
  background: var(--app-surface);
  p {
    margin: 8px 0 0;
  }
}
.urge-record-header {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  time {
    color: var(--app-text-muted, #8590a0);
  }
}
.urge-avatar {
  background: var(--ant-color-success, #52c41a);
}
.urge-record-number {
  margin-inline: auto 0;
  border: 0;
  border-radius: 999px;
}
</style>
