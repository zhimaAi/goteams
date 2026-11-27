<template>
  <section class="issue-history-list">
    <a-empty
      v-if="!items.length"
      :description="$t('issue_invite_no_history')"
    />
    <div v-else class="issue-history-list__cards">
      <div
        v-for="item in items"
        :key="item.requirement_id"
        class="issue-history-list__card"
        role="button"
        tabindex="0"
        @click="select(item)"
        @keydown.enter="select(item)"
        @keydown.space.prevent="select(item)"
      >
        <div class="issue-history-list__card-top">
          <h2 class="issue-history-list__title">{{ item.title }}</h2>
          <span
            class="issue-history-list__status-tag"
            :style="statusTagStyle(item)"
            >{{ item.status_name }}</span
          >
        </div>
        <p v-if="item.description" class="issue-history-list__desc">
          {{ plainDescription(item.description) }}
        </p>
        <p class="issue-history-list__time">
          {{ formatLocalDate(item.created_at, true) }}
        </p>
        <button
          v-if="
            ['available', 'submitted'].includes(
              satisfactionStates[item.requirement_id],
            )
          "
          type="button"
          class="issue-history-list__satisfaction"
          @click.stop="emit('evaluate', item.requirement_id)"
          @keydown.enter.stop
          @keydown.space.stop
        >
          <StarOutlined aria-hidden="true" />
          <span>{{
            $t(
              satisfactionStates[item.requirement_id] === "submitted"
                ? "satisfaction_view"
                : "satisfaction_invite_entry",
            )
          }}</span>
          <RightOutlined
            class="issue-history-list__satisfaction-arrow"
            aria-hidden="true"
          />
        </button>
      </div>
    </div>
    <div v-if="total > pageSize" class="issue-history-list__pagination">
      <a-pagination
        :current="page"
        :page-size="pageSize"
        :total="total"
        :show-size-changer="false"
        @change="changePage"
      />
    </div>
  </section>
</template>

<script setup>
import { StarOutlined, RightOutlined } from "@ant-design/icons-vue";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  items: { type: Array, default: () => [] },
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  total: { type: Number, default: 0 },
  satisfactionStates: { type: Object, default: () => ({}) },
});
const emit = defineEmits(["select", "page-change", "evaluate"]);

function select(item) {
  emit("select", item?.requirement_id);
}

// Match the admin status-column pill: text and border carry the workspace-configured
// status color returned by the backend; fall back to a neutral gray when absent.
function statusTagStyle(item) {
  const color = item?.status_color || "#8c8c9c";
  return { color, borderColor: color };
}

// The description may contain the server-appended attachment block HTML (inline <img>/<a>);
// strip tags so the card summary stays plain text.
function plainDescription(html) {
  return String(html || "")
    .replace(/<[^>]*>/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

function changePage(page) {
  if (page > 0) emit("page-change", page);
}
</script>

<style scoped lang="less">
.issue-history-list {
  padding: 12px 16px;

  &__cards {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  &__card {
    background: #fff;
    border-radius: 8px;
    padding: 16px;
    cursor: pointer;
    transition: box-shadow 0.2s ease;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);

    &:hover {
      box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
    }

    &:focus-visible {
      outline: 2px solid #0d68ff;
      outline-offset: 2px;
    }
  }

  &__card-top {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }

  &__status-tag {
    display: inline-flex;
    align-items: center;
    padding: 2px 8px;
    font-size: 12px;
    line-height: 20px;
    border: 1px solid transparent;
    border-radius: 12px;
    flex-shrink: 0;
    white-space: nowrap;
  }

  &__title {
    margin: 0;
    font-size: 15px;
    font-weight: 600;
    line-height: 22px;
    color: #1d2129;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__desc {
    margin: 0 0 6px;
    font-size: 13px;
    line-height: 20px;
    color: #4e5969;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    word-break: break-word;
  }

  &__time {
    margin: 0;
    font-size: 12px;
    line-height: 18px;
    color: #c9cdd4;
  }

  &__satisfaction {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    margin-top: 16px;
    padding: 14px 0 0;
    border: 0;
    border-top: 1px solid #edf0f5;
    background: transparent;
    color: #4679ff;
    text-align: left;
    font-size: 15px;
    font-weight: 600;
    cursor: pointer;

    > :deep(.anticon) {
      font-size: 21px;
    }
    &:focus-visible {
      outline: 2px solid #4679ff;
      outline-offset: 4px;
    }
  }
  &__satisfaction-arrow {
    margin-left: auto;
    color: #c5cad3;
  }

  &__pagination {
    display: flex;
    justify-content: center;
    margin-top: 16px;
  }
}
</style>
