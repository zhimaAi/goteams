<template>
  <button
    v-if="count"
    type="button"
    class="documents-entry"
    :class="`documents-entry-${variant}`"
    :title="metaText"
    @click="openPage"
  >
    <FileTextOutlined class="documents-entry-icon" />
    <span class="documents-entry-label">{{
      $t("pipeline_documents_entry")
    }}</span>
    <template v-if="variant === 'banner'">
      <span class="documents-entry-meta">{{ metaText }}</span>
      <RightOutlined class="documents-entry-arrow" />
    </template>
  </button>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { FileTextOutlined, RightOutlined } from "@ant-design/icons-vue";
import { listPipelineTaskDocuments } from "@/api/pipelineDocuments";
import { formatLocalDate } from "@/utils/date";

const props = defineProps({
  taskId: { type: [Number, String], default: null },
  // banner：整行信息条（日志详情顶部）；link：与标题同行的紧凑入口（沟通记录标题行）
  variant: {
    type: String,
    default: "banner",
    validator: (value) => ["banner", "link"].includes(value),
  },
});

const router = useRouter();
const { t } = useI18n();

const count = ref(0);
const lastUploadedAt = ref(0);
// 文档页不在 wsId 路由下，跳转时带上空间 id，路由守卫才能加载该空间的项目权限：
// 文档页在新标签打开，新页面 store 里还没有当前空间权限快照。
const workspaceId = ref(0);

// The entry only needs the count and the latest upload time, so it reuses the
// same list endpoint the documents page calls instead of adding a second API.
const metaText = computed(() => {
  if (!count.value) return t("pipeline_documents_empty");
  const counts = t("pipeline_documents_count", { count: count.value });
  if (!lastUploadedAt.value) return counts;
  const time = formatLocalDate(lastUploadedAt.value, true).slice(5, 16);
  return `${counts} · ${t("pipeline_documents_updated_at", { time })}`;
});

async function load() {
  if (!props.taskId) {
    count.value = 0;
    lastUploadedAt.value = 0;
    return;
  }
  try {
    const result = await listPipelineTaskDocuments(props.taskId);
    const list = Array.isArray(result?.documents) ? result.documents : [];
    count.value = list.length;
    workspaceId.value = Number(result?.workspace_id) || 0;
    lastUploadedAt.value = list.reduce(
      (latest, item) => Math.max(latest, Number(item.updated_at) || 0),
      0,
    );
  } catch {
    count.value = 0;
    lastUploadedAt.value = 0;
  }
}

// 一律新标签打开：入口既在抽屉详情里也在整页详情里，同标签跳转会把当前界面顶掉。
function openPage() {
  if (!count.value || !props.taskId) return;
  const location = {
    name: "pipeline-task-documents",
    params: { taskId: props.taskId },
    query: workspaceId.value ? { wsId: String(workspaceId.value) } : undefined,
  };
  window.open(router.resolve(location).href, "_blank", "noopener,noreferrer");
}

watch(() => props.taskId, load, { immediate: true });

defineExpose({ refresh: load });
</script>

<style scoped lang="less">
@import "@/styles/variables.less";

.documents-entry {
  display: flex;
  align-items: center;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

/* 整行信息条：日志详情顶部 */
.documents-entry-banner {
  gap: @spacing-8;
  width: 100%;
  padding: @spacing-10 @spacing-12;
  border: 1px solid @info-border-light;
  border-radius: @border-radius-md;
  background: @info-bg;
  color: @neutral-white-2;
  font-size: @font-size-4;
}

.documents-entry-banner:hover {
  border-color: @primary-hover;
}

/* 紧凑入口：与「沟通记录」标题同行 */
.documents-entry-link {
  gap: 4px;
  padding: 1px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: @primary-color;
  font-size: @font-size-4;
  line-height: 22px;
}

.documents-entry-link:hover {
  background: @info-bg-3;
}

.documents-entry-icon {
  color: @primary-color;
}

.documents-entry-label {
  font-weight: @font-weight-medium;
}

.documents-entry-link .documents-entry-label {
  font-weight: 400;
}

.documents-entry-meta {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: @font-size-5;
  color: @neutral-blue-3;
}

.documents-entry-arrow {
  color: @neutral-blue-3;
}
</style>
