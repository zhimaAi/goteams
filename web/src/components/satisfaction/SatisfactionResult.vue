<template>
  <a-card
    size="small"
    class="satisfaction-result"
    :class="{ 'is-detail': detail }"
  >
    <a-space v-if="!detail" class="satisfaction-result-head" wrap
      ><a-avatar
        v-if="data.reviewer_avatar"
        :src="data.reviewer_avatar"
      /><strong>{{ data.reviewer_name || t("satisfaction_customer") }}</strong
      ><span>{{ formatLocalDate(data.submitted_at, true) }}</span
      ><a-tag
        v-if="data.resolved !== null"
        :color="data.resolved ? 'green' : 'orange'"
        >{{
          t(data.resolved ? "satisfaction_resolved" : "satisfaction_unresolved")
        }}</a-tag
      ></a-space
    >
    <div v-else class="satisfaction-detail-head">
      <a-avatar
        class="satisfaction-detail-avatar"
        :size="24"
        :src="data.reviewer_avatar || undefined"
        ><span>{{
          (data.reviewer_name || t("satisfaction_customer")).trim().slice(0, 1)
        }}</span></a-avatar
      >
      <strong>{{ data.reviewer_name || t("satisfaction_customer") }}</strong>
      <span class="satisfaction-detail-time">{{
        formatLocalDate(data.submitted_at, true)
      }}</span>
      <span
        v-if="typeof data.resolved === 'boolean'"
        class="satisfaction-detail-status"
        :class="{ 'is-unresolved': !data.resolved }"
        >{{
          t(data.resolved ? "satisfaction_resolved" : "satisfaction_unresolved")
        }}</span
      >
    </div>
    <p class="satisfaction-result-rating">
      <span
        v-if="detail && data.rule.rating_mode === 2"
        class="satisfaction-detail-binary"
        :class="{ 'is-negative': data.value === 1 }"
        ><LikeFilled
          :class="{ 'is-negative': data.value === 1 }"
          aria-hidden="true"
        /><strong>{{ data.label }}</strong></span
      >
      <a-rate
        v-else-if="detail || data.rule.rating_mode === 5"
        :count="data.rule.rating_mode"
        :value="data.value"
        disabled
      /><strong v-else>{{ data.label }}</strong>
    </p>
    <a-space v-if="!detail || data.rule.rating_mode === 2" wrap
      ><a-tag v-for="tag in data.tags" :key="tag.key">{{
        tag.label
      }}</a-tag></a-space
    >
    <p v-if="data.content" class="satisfaction-result-content">
      {{ data.content }}
    </p>
    <p v-if="!detail" class="satisfaction-result-note">
      {{ t("satisfaction_readonly") }}
    </p>
  </a-card>
</template>
<script setup>
import { LikeFilled } from "@ant-design/icons-vue";
import { useI18n } from "vue-i18n";
import { formatLocalDate } from "@/utils/date";
defineProps({
  data: { type: Object, required: true },
  detail: { type: Boolean, default: false },
});
const { t } = useI18n();
</script>
<style scoped lang="less">
.satisfaction-result.is-detail {
  border-color: #eceef2;
  border-radius: 8px;
  color: #1d2129;
  :deep(.ant-card-body) {
    padding: 12px 14px;
  }
  .satisfaction-result-rating {
    margin: 8px 0 0;
    line-height: 22px;
  }
  :deep(.ant-rate) {
    color: #ff8800;
    font-size: 14px;
    line-height: 22px;
  }
  :deep(.ant-rate-star:not(:last-child)) {
    margin-inline-end: 4px;
  }
  .satisfaction-result-content {
    margin: 8px 0 0;
    font-size: 14px;
    line-height: 22px;
  }
}
.satisfaction-detail-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  line-height: 24px;
}
.satisfaction-detail-head strong {
  overflow-wrap: anywhere;
}
.satisfaction-detail-avatar {
  flex-shrink: 0;
  background: #35bc4a;
}
.satisfaction-detail-time {
  color: #8c95a8;
  font-size: 12px;
}
.satisfaction-detail-status {
  margin-inline-start: auto;
  padding: 0 10px;
  border-radius: 12px;
  background: #e8f7eb;
  color: #35bc4a;
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
}
.satisfaction-detail-status.is-unresolved {
  background: #fff3e8;
  color: #e88b2f;
}
.satisfaction-detail-binary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #ff8800;
}
.satisfaction-detail-binary.is-negative {
  color: #a6adb8;
}
.satisfaction-detail-binary .anticon.is-negative {
  transform: rotate(180deg);
}
.satisfaction-result-head {
  display: flex;
  justify-content: space-between;
}
.satisfaction-result-content {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin-top: 12px;
}
.satisfaction-result-note {
  color: var(--ant-color-text-secondary, #8c8c8c);
  margin: 12px 0 0;
}
</style>
