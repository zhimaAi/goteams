<!--
 * ViewConfigModal.vue
 * "Configure views" modal: default view radio, enable switches, view rows
 * (system first, then drag-sortable personal views), per-row edit/save-as/
 * delete actions, and the bottom "remember last viewed" switch. The default/
 * enable/order/remember changes are submitted together on Save; row actions
 * (edit/save-as/delete) take effect immediately via their own APIs.
 -->
<template>
  <a-modal
    :open="open"
    :title="$t('view_config_title')"
    width="860px"
    @cancel="close"
  >
    <div class="view-config">
      <div class="view-config__row view-config__row--head">
        <span class="view-config__drag"
          ><HolderOutlined style="opacity: 0.3"
        /></span>
        <span class="view-config__default">{{ $t("view_col_default") }}</span>
        <span class="view-config__enabled">{{ $t("view_col_enabled") }}</span>
        <span class="view-config__name">{{ $t("view_col_name") }}</span>
        <span class="view-config__type">{{ $t("view_col_type") }}</span>
        <span class="view-config__actions">{{ $t("view_col_actions") }}</span>
      </div>
      <div class="view-config__body">
        <div v-for="row in systemRows" :key="row.id" class="view-config__row">
          <span class="view-config__drag" />
          <span class="view-config__default">
            <a-radio
              :checked="String(defaultViewId) === String(row.id)"
              @click="setDefault(row)"
            />
          </span>
          <span class="view-config__enabled">
            <a-switch
              :checked="row.enabled"
              :disabled="lockAllView && row.builtin_key === 'all'"
              size="small"
              @change="(checked) => toggleEnabled(row, checked)"
            />
          </span>
          <span class="view-config__name">{{ row.name }}</span>
          <span class="view-config__type">
            <a-tag color="blue">{{ $t("view_type_system") }}</a-tag>
          </span>
          <span class="view-config__actions">
            <a-button
              type="link"
              size="small"
              :disabled="!canEditRow(row)"
              @click="$emit('edit', row.raw)"
            >
              {{ $t("common_edit") }}
            </a-button>
            <a-button
              type="link"
              size="small"
              :disabled="lockAllView && row.builtin_key === 'all'"
              @click="$emit('save-as', row.raw)"
            >
              {{ $t("view_save_as") }}
            </a-button>
            <a-popconfirm
              :title="$t('view_delete_confirm')"
              :disabled="!canEditRow(row)"
              @confirm="handleDelete(row)"
            >
              <a-button
                type="link"
                size="small"
                danger
                :disabled="!canEditRow(row)"
              >
                {{ $t("common_delete") }}
              </a-button>
            </a-popconfirm>
          </span>
        </div>
        <div ref="personalListRef">
          <div
            v-for="row in personalRows"
            :key="row.id"
            class="view-config__row view-config__row--personal"
          >
            <span class="view-config__drag view-config__drag-handle"
              ><HolderOutlined
            /></span>
            <span class="view-config__default">
              <a-radio
                :checked="String(defaultViewId) === String(row.id)"
                @click="setDefault(row)"
              />
            </span>
            <span class="view-config__enabled">
              <a-switch
                :checked="row.enabled"
                size="small"
                @change="(checked) => toggleEnabled(row, checked)"
              />
            </span>
            <span class="view-config__name">{{ row.name }}</span>
            <span class="view-config__type">
              <a-tag>{{ $t("view_type_personal") }}</a-tag>
            </span>
            <span class="view-config__actions">
              <a-button
                type="link"
                size="small"
                @click="$emit('edit', row.raw)"
              >
                {{ $t("common_edit") }}
              </a-button>
              <a-button
                type="link"
                size="small"
                @click="$emit('save-as', row.raw)"
              >
                {{ $t("view_save_as") }}
              </a-button>
              <a-popconfirm
                :title="$t('view_delete_confirm')"
                @confirm="handleDelete(row)"
              >
                <a-button type="link" size="small" danger>{{
                  $t("common_delete")
                }}</a-button>
              </a-popconfirm>
            </span>
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <div class="view-config__footer">
        <a-checkbox v-model:checked="rememberLocal">
          {{ $t("view_remember_last") }}
        </a-checkbox>
        <div class="view-config__footer-buttons">
          <a-button @click="close">{{ $t("common_cancel") }}</a-button>
          <a-button type="primary" :loading="saving" @click="handleSave">
            {{ $t("common_save") }}
          </a-button>
        </div>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { ref, computed, watch, nextTick, onBeforeUnmount } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { HolderOutlined } from "@ant-design/icons-vue";
import Sortable from "sortablejs";
import { deleteWorkItemView, saveUserViewSettings } from "@/api/workItemView";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  typeKey: { type: String, default: "requirement" },
  // View list of the current tab (already ordered: system first, then personal).
  views: { type: Array, default: () => [] },
  rememberLast: { type: Boolean, default: true },
  lockAllView: { type: Boolean, default: false },
  canManageSystemView: { type: Boolean, default: false },
});

const emit = defineEmits({
  "update:open": (open) => true,
  // Open the edit modal for a view.
  edit: (view) => true,
  // Open the save-as modal for a view.
  "save-as": (view) => true,
  // A view was deleted (already persisted).
  deleted: (viewId) => true,
  // Default/enabled/order/remember settings were saved.
  saved: () => true,
});

const { t } = useI18n();
const saving = ref(false);
const systemRows = ref([]);
const personalRows = ref([]);
const defaultViewId = ref(null);
const rememberLocal = ref(true);
const personalListRef = ref(null);
let sortableInstance = null;

// Rebuild local rows whenever the modal opens or the source list refreshes
// (row actions persist immediately, so a refresh resets unsaved toggles).
watch(
  [() => props.open, () => props.views],
  ([open]) => {
    if (!open) return;
    const toRow = (view) => ({
      id: view.id,
      name: view.name,
      builtin_key: view.builtin_key,
      view_type: view.view_type,
      enabled: view.enabled,
      raw: view,
    });
    systemRows.value = props.views
      .filter((view) => view.view_type === "system")
      .map(toRow);
    personalRows.value = props.views
      .filter((view) => view.view_type === "personal")
      .map(toRow);
    defaultViewId.value =
      props.views.find((view) => view.is_default)?.id ?? null;
    rememberLocal.value = props.rememberLast;
    nextTick(setupSortable);
  },
  { immediate: true },
);

function setupSortable() {
  destroySortable();
  if (!personalListRef.value) return;
  sortableInstance = Sortable.create(personalListRef.value, {
    handle: ".view-config__drag-handle",
    animation: 150,
    onEnd: ({ oldIndex, newIndex }) => {
      if (oldIndex === newIndex) return;
      const moved = personalRows.value.splice(oldIndex, 1)[0];
      personalRows.value.splice(newIndex, 0, moved);
    },
  });
}

function destroySortable() {
  sortableInstance?.destroy();
  sortableInstance = null;
}

onBeforeUnmount(destroySortable);

const allRow = computed(() =>
  systemRows.value.find((row) => row.builtin_key === "all"),
);

// The builtin "all" view is locked for everyone; other system views need the manage permission.
function canEditRow(row) {
  if (row.builtin_key === "all") return false;
  if (row.view_type === "system") return props.canManageSystemView;
  return true;
}

// Picking a row as default force-enables it.
function setDefault(row) {
  defaultViewId.value = row.id;
  row.enabled = true;
}

// Disabling the current default falls back to the builtin "all" view.
function toggleEnabled(row, checked) {
  row.enabled = checked;
  if (
    !checked &&
    String(defaultViewId.value) === String(row.id) &&
    allRow.value
  ) {
    defaultViewId.value = allRow.value.id;
    allRow.value.enabled = true;
  }
}

async function handleDelete(row) {
  try {
    await deleteWorkItemView(props.wsId, row.id);
    message.success(t("view_delete_success"));
    emit("deleted", row.id);
  } catch {
    /* handled by interceptor */
  }
}

async function handleSave() {
  saving.value = true;
  try {
    const disabledIds = [...systemRows.value, ...personalRows.value]
      .filter((row) => !row.enabled)
      .map((row) => Number(row.id));
    await saveUserViewSettings(props.wsId, {
      type: props.typeKey,
      default_view_id: Number(defaultViewId.value) || 0,
      disabled_view_ids: disabledIds,
      personal_view_order: personalRows.value.map((row) => Number(row.id)),
      remember_last: rememberLocal.value,
    });
    message.success(t("common_saveSuccess"));
    emit("saved");
    close();
  } catch {
    /* handled by interceptor */
  } finally {
    saving.value = false;
  }
}

function close() {
  emit("update:open", false);
}
</script>

<style scoped lang="less">
.view-config__row {
  display: flex;
  align-items: center;
  padding: 10px 8px;
  border-bottom: 1px solid #f0f0f0;
  gap: 8px;
}
.view-config__row--head {
  font-weight: 600;
  background: #fafafa;
  border-radius: 6px 6px 0 0;
}
.view-config__drag {
  width: 24px;
  flex-shrink: 0;
  display: inline-flex;
  justify-content: center;
}
.view-config__drag-handle {
  cursor: grab;
  color: rgba(0, 0, 0, 0.45);
}
.view-config__default,
.view-config__enabled {
  width: 72px;
  flex-shrink: 0;
}
.view-config__name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.view-config__type {
  width: 96px;
  flex-shrink: 0;
}
.view-config__actions {
  width: 168px;
  flex-shrink: 0;
  text-align: right;
}
.view-config__body {
  max-height: 420px;
  overflow-y: auto;
}
.view-config__footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.view-config__footer-buttons {
  display: flex;
  gap: 8px;
}
</style>
