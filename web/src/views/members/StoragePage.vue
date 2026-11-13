<template>
  <div class="storage-page">
    <!-- Storage overview card -->
    <div class="storage-overview-card">
      <div class="overview-left">
        <div class="overview-icon">
          <CloudOutlined />
        </div>
        <div class="overview-info">
          <div class="overview-head">
            <div class="overview-title">{{ $t("storage_title") }}</div>
            <div class="overview-stats">
              <span>{{ $t("storage_used") }}</span>
              <span class="used-size">{{ formatFileSize(totalSize) }}</span>
              <span> / {{ $t("storage_total") }} {{ storageLimit }} GB</span>
            </div>
          </div>
          <a-progress
            :percent="usagePercent"
            :stroke-color="usageColor"
            :trail-color="'#f1f5f9'"
            :stroke-width="8"
            :show-info="false"
          />
          <div class="overview-detail">
            <span>已用 {{ usagePercentText }}</span>
            <span>{{ $t("storage_remaining") }} {{ remainingSpace }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- File list table -->
    <div class="storage-table-card">
      <div class="table-toolbar" v-if="selectedRowKeys.length > 0">
        <span class="selected-count">已选 {{ selectedRowKeys.length }} 项</span>
        <a-space>
          <a-button
            v-if="canDownload"
            size="small"
            @click="handleBatchDownload"
          >
            <DownloadOutlined /> {{ $t("storage_batch_download") }}
          </a-button>
          <a-button
            v-if="canDelete"
            size="small"
            danger
            @click="handleBatchDelete"
          >
            <DeleteOutlined /> {{ $t("storage_batch_delete") }}
          </a-button>
        </a-space>
      </div>
      <a-table
        :columns="columns"
        :data-source="files"
        :loading="loading"
        :pagination="pagination"
        :row-selection="
          canDelete ? { selectedRowKeys, onChange: onSelectChange } : undefined
        "
        :scroll="{ x: 1000 }"
        row-key="id"
        @change="handleTableChange"
      >
        <template #bodyCell="{ column, record }">
          <!-- Source column -->
          <template v-if="column.key === 'source'">
            <div class="source-cell">
              <span
                :class="[
                  'source-type-tag',
                  record.source_type === 'requirement'
                    ? 'source-req'
                    : record.source_type === 'knowledge'
                      ? 'source-knowledge'
                      : 'source-defect',
                ]"
              >
                {{ sourceTypeText(record.source_type) }}
              </span>
              <a
                v-if="record.source_type !== 'knowledge'"
                class="source-name"
                @click="openSourceDetail(record)"
                >{{ record.source_name || "-" }}</a
              >
              <a
                v-else
                class="source-name"
                @click="openKnowledgeDocument(record)"
                >{{ record.source_name || "-" }}</a
              >
            </div>
          </template>
          <!-- Project column -->
          <template v-else-if="column.key === 'project'">
            <a-tag>{{ record.project_name || "-" }}</a-tag>
          </template>
          <!-- File name column -->
          <template v-else-if="column.key === 'file_name'">
            <span class="file-name" :title="record.file_name">{{
              record.file_name || "-"
            }}</span>
          </template>
          <!-- File type column -->
          <template v-else-if="column.key === 'file_ext'">
            <a-tag color="blue">{{
              record.file_ext?.toUpperCase() || "-"
            }}</a-tag>
          </template>
          <!-- File size column -->
          <template v-else-if="column.key === 'file_size'">
            {{ formatFileSize(record.file_size) }}
          </template>
          <!-- Upload time column -->
          <template v-else-if="column.key === 'created_at'">
            {{ formatLocalDate(record.created_at) }}
          </template>
          <!-- Actions column -->
          <template v-else-if="column.key === 'action'">
            <a-space>
              <a-button
                v-if="canDownload"
                type="link"
                size="small"
                @click="handleDownload(record)"
              >
                <DownloadOutlined /> {{ $t("storage_download") }}
              </a-button>
              <a-popconfirm
                v-if="canDelete"
                :title="$t('storage_delete_confirm', { count: 1 })"
                :ok-text="$t('common_confirm')"
                :cancel-text="$t('common_cancel')"
                @confirm="handleDelete(record)"
              >
                <a-button type="link" size="small" danger>
                  <DeleteOutlined /> {{ $t("storage_delete") }}
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </div>

    <RequirementDetailDrawer
      :open="detailDrawer.open"
      :ws-id="detailDrawer.wsId"
      :requirement-id="detailDrawer.reqId"
      @close="closeDetail"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import {
  CloudOutlined,
  DownloadOutlined,
  DeleteOutlined,
} from "@ant-design/icons-vue";
import {
  listEnterpriseFiles,
  downloadEnterpriseFile,
  deleteEnterpriseFile,
  batchDeleteEnterpriseFiles,
  getEnterprisePlanUsage,
} from "@/api/workspace";
import { formatLocalDate } from "@/utils/date";
import { useAuthStore } from "@/stores/auth";
import RequirementDetailDrawer from "@/components/common/RequirementDetailDrawer.vue";

const { t: $t } = useI18n();
const authStore = useAuthStore();

const loading = ref(false);
const files = ref([]);
const totalSize = ref(0);
const storageLimit = ref(50);
const selectedRowKeys = ref([]);
const totalCount = ref(0);

const canDownload = computed(() => authStore.hasPermission("storage:download"));
const canDelete = computed(() => authStore.hasPermission("storage:delete"));

const detailDrawer = reactive({ open: false, wsId: "", reqId: "" });

const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: true,
  showTotal: (total) => `共 ${total} 条`,
});

const columns = computed(() => [
  { title: $t("storage_source"), key: "source", width: 240 },
  { title: $t("storage_project"), key: "project", width: 120 },
  {
    title: $t("storage_file_name"),
    key: "file_name",
    width: 220,
    ellipsis: true,
  },
  { title: $t("storage_file_type"), key: "file_ext", width: 100 },
  {
    title: $t("storage_file_size"),
    key: "file_size",
    width: 120,
    sorter: true,
  },
  {
    title: $t("storage_upload_time"),
    key: "created_at",
    width: 180,
    sorter: true,
    defaultSortOrder: "descend",
  },
  { title: $t("storage_actions"), key: "action", width: 180 },
]);

const usagePercent = computed(() => {
  if (!storageLimit.value) return 0;
  const usedGB = totalSize.value / (1024 * 1024 * 1024);
  const percent = (usedGB / storageLimit.value) * 100;
  return Math.min(Math.floor(percent * 100) / 100, 100);
});

// Usage percentage display text: based on the raw ratio, shows <0.01% when below 0.01%, otherwise two decimal places (floor).
const usagePercentText = computed(() => {
  if (!storageLimit.value || totalSize.value <= 0) return "0.00%";
  const usedGB = totalSize.value / (1024 * 1024 * 1024);
  const rawPercent = (usedGB / storageLimit.value) * 100;
  if (rawPercent > 0 && rawPercent < 0.01) return "<0.01%";
  return `${usagePercent.value.toFixed(2)}%`;
});

const usageColor = computed(() => {
  if (usagePercent.value >= 90) return "#ef4444";
  if (usagePercent.value >= 70) return "#f59e0b";
  return "#22c55e";
});

const remainingSpace = computed(() => {
  const usedGB = totalSize.value / (1024 * 1024 * 1024);
  const remaining = Math.max(storageLimit.value - usedGB, 0);
  const floor2 = Math.floor(remaining * 100) / 100;
  return remaining >= 1
    ? `${floor2.toFixed(2)} GB`
    : `${(remaining * 1024).toFixed(0)} MB`;
});

onMounted(async () => {
  await Promise.all([fetchFiles(), fetchPlanUsage()]);
});

async function fetchPlanUsage() {
  try {
    const usage = await getEnterprisePlanUsage();
    const storageItem = (usage?.items || []).find((i) => i.key === "storage");
    if (storageItem) {
      storageLimit.value = storageItem.limit || 50;
    }
  } catch {
    /* ignore */
  }
}

async function fetchFiles(page = 1, pageSize = 20, sorter = {}) {
  loading.value = true;
  try {
    const params = { page, page_size: pageSize };
    if (sorter.field === "file_size") {
      params.sort_field = "file_size";
      params.sort_order = sorter.order === "ascend" ? "asc" : "desc";
    } else if (sorter.field === "created_at" || !sorter.field) {
      params.sort_field = "created_at";
      params.sort_order = sorter.order === "ascend" ? "asc" : "desc";
    }
    const result = await listEnterpriseFiles(params);
    files.value = result?.list || [];
    totalCount.value = result?.total || 0;
    totalSize.value = result?.total_size || 0;
    pagination.total = totalCount.value;
    pagination.current = page;
    pagination.pageSize = pageSize;
  } catch {
    /* handled */
  } finally {
    loading.value = false;
  }
}

function handleTableChange(pag, _filters, sorter) {
  fetchFiles(pag.current, pag.pageSize, sorter);
}

function onSelectChange(keys) {
  selectedRowKeys.value = keys;
}

function formatFileSize(bytes) {
  if (!bytes || bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + units[i];
}

function sourceTypeText(type) {
  if (type === "requirement") return $t("storage_requirement");
  if (type === "knowledge") return $t("storage_knowledge");
  return $t("storage_defect");
}

function openSourceDetail(record) {
  if (record.workspace_id && record.source_id) {
    detailDrawer.wsId = record.workspace_id;
    detailDrawer.reqId = record.source_id;
    detailDrawer.open = true;
  }
}

function openKnowledgeDocument(record) {
  if (!record.workspace_id || !record.document_id) return;
  const url = `${window.location.origin}/workspace/${record.workspace_id}/board?tab=knowledge&kbFile=${record.document_id}&kbCollapsed=1`;
  window.open(url, "_blank", "noopener,noreferrer");
}

function closeDetail() {
  detailDrawer.open = false;
}

function handleDownload(record) {
  downloadEnterpriseFile(record.id)
    .then((response) => {
      // The interceptor returns AxiosResponse directly for blob responses; the actual file data is in response.data.
      const url = URL.createObjectURL(response.data);
      const a = document.createElement("a");
      a.href = url;
      a.download = record.file_name || "download";
      a.click();
      URL.revokeObjectURL(url);
    })
    .catch(() => {
      message.error($t("common_error"));
    });
}

async function handleDelete(record) {
  try {
    await deleteEnterpriseFile(record.id);
    message.success($t("storage_delete_success"));
    await fetchFiles(pagination.current, pagination.pageSize);
  } catch {
    /* handled */
  }
}

async function handleBatchDelete() {
  try {
    await batchDeleteEnterpriseFiles({ ids: selectedRowKeys.value });
    message.success(
      $t("storage_batch_delete_success", {
        count: selectedRowKeys.value.length,
      }),
    );
    selectedRowKeys.value = [];
    await fetchFiles(pagination.current, pagination.pageSize);
  } catch {
    /* handled */
  }
}

function handleBatchDownload() {
  selectedRowKeys.value.forEach((id) => {
    const file = files.value.find((f) => f.id === id);
    if (file) handleDownload(file);
  });
  selectedRowKeys.value = [];
}
</script>

<style scoped lang="less">
.storage-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.storage-overview-card {
  display: flex;
  align-items: center;
  padding: 24px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
  height: 116px;
}

.overview-left {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 20px;
}

.overview-info {
  flex: 1;
}

.overview-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 12px;
}

.overview-icon {
  display: flex;
  width: 52px;
  height: 52px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: #fff;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  font-size: 24px;
}

.overview-title {
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
}

.overview-stats {
  color: #64748b;
  font-size: 14px;

  span {
    color: #0f172a;
    font-weight: 600;
  }

  .used-size {
    color: #22c55e;
  }
}

.overview-detail {
  display: flex;
  justify-content: space-between;
  margin-top: 6px;
  color: #94a3b8;
  font-size: 12px;
}

.storage-table-card {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #fff;

  // Keep the table wrapper inside the card's remaining height, leaving the pagination visible at the bottom.
  :deep(.ant-table-wrapper) {
    display: flex;
    flex: 1;
    min-height: 0;
    flex-direction: column;
  }

  :deep(.ant-spin-nested-loading),
  :deep(.ant-spin-container),
  :deep(.ant-table),
  :deep(.ant-table-container) {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
  }

  // A table with horizontal-only scroll renders .ant-table-content instead of .ant-table-body.
  // antd also sets an inline `overflow-y: hidden` on it in that case, so the vertical
  // scrollbar needs !important to win over the inline style.
  :deep(.ant-table-content) {
    flex: 1;
    min-height: 0;
    overflow: auto !important;
  }

  :deep(.ant-table-pagination) {
    flex: none;
  }

  // The header stays visible while the body scrolls inside .ant-table-content.
  :deep(.ant-table-thead > tr > th) {
    position: sticky;
    top: 0;
    z-index: 2;
    padding: 14px 16px;
    color: #64748b;
    background: #f8fafc;
    font-size: 13px;
  }

  :deep(.ant-table-tbody > tr > td) {
    padding: 12px 16px;
    border-bottom-color: #f1f5f9;
  }

  :deep(.ant-table-tbody > tr:hover > td) {
    background: #fafcff;
  }
}

.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid #e2e8f0;
  background: #eff6ff;
}

.selected-count {
  color: #2563eb;
  font-size: 13px;
  font-weight: 600;
}

.source-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.source-type-tag {
  display: inline-block;
  padding: 0 6px;
  border-radius: 4px;
  color: #fff;
  font-size: 11px;
  line-height: 20px;
  white-space: nowrap;
}

.source-req {
  background: #3b82f6;
}

.source-defect {
  background: #ef4444;
}

.source-knowledge {
  background: #22c55e;
}

.source-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.file-name {
  display: inline-block;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}
</style>
