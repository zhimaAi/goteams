<!--
 * ViewEditModal.vue
 * Create / edit / save-as modal for requirement/defect list views:
 * name, view type (system option gated by permission), and the data-scope
 * condition tree editor.
 -->
<template>
  <!-- z-index 1001 keeps this leaf modal above ViewConfigModal: both default
       to 1000 and stacking would otherwise depend on modal mount order. -->
  <a-modal
    :open="open"
    :title="modalTitle"
    :confirm-loading="saving"
    :ok-text="okText"
    :cancel-text="$t('common_cancel')"
    width="840px"
    :z-index="1001"
    @ok="handleSave"
    @cancel="close"
  >
    <div class="view-edit-form">
      <div class="view-edit-form__label">{{ $t("view_name_label") }}</div>
      <a-input
        v-model:value="form.name"
        :placeholder="$t('view_name_placeholder')"
        :maxlength="50"
      />

      <div class="view-edit-form__label">{{ $t("view_type_label") }}</div>
      <a-radio-group v-model:value="form.viewType" :disabled="mode === 'edit'">
        <a-radio value="personal">{{ $t("view_type_personal") }}</a-radio>
        <a-radio
          v-if="canManageSystemView || form.viewType === 'system'"
          value="system"
        >
          {{ $t("view_type_system") }}
        </a-radio>
      </a-radio-group>

      <div class="view-edit-form__label">{{ $t("view_scope_label") }}</div>
      <template v-if="typeKey === 'gantt'">
        <div class="view-edit-form__hint">
          {{ $t("gantt_view_filters_hint") }}
        </div>
        <GanttFilterSection
          v-for="section in ganttSectionList"
          :key="section.key"
          :title="section.title"
          :group="ganttTrees[section.key]"
          :type-key="section.typeKey"
          :statuses="section.options.statuses"
          :priorities="section.options.priorities"
          :categories="section.options.categories"
          :fields="section.options.fields"
          :members="members"
          :versions="versions"
        />
      </template>
      <div v-else class="view-scope-card">
        <div class="view-scope-card__head">
          <strong>{{ $t("view_filter_title") }}</strong>
          <a-button
            type="link"
            size="small"
            @click="filtersCollapsed = !filtersCollapsed"
          >
            <DownOutlined v-if="filtersCollapsed" />
            <UpOutlined v-else />
            {{
              filtersCollapsed
                ? $t("view_filter_expand")
                : $t("view_filter_collapse")
            }}
          </a-button>
        </div>
        <ViewFilterGroup
          v-show="!filtersCollapsed"
          root
          :group="filterTree"
          :type-key="typeKey"
        />
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import { ref, reactive, computed, watch, provide } from "vue";
import { useI18n } from "vue-i18n";
import { message } from "ant-design-vue";
import { UpOutlined, DownOutlined } from "@ant-design/icons-vue";
import ViewFilterGroup from "@/components/work-item/ViewFilterGroup.vue";
import GanttFilterSection from "@/components/work-item/GanttFilterSection.vue";
import { createWorkItemView, updateWorkItemView } from "@/api/workItemView";
import {
  newViewFilterCondition,
  normalizeViewFilterTree,
} from "@/constants/workItemView";
import {
  normalizeCascaderFilterTree,
  normalizeCascaderViewFilterTree,
} from "@/utils/customField";

const props = defineProps({
  open: { type: Boolean, default: false },
  wsId: { type: [String, Number], required: true },
  typeKey: { type: String, default: "requirement" },
  // create | edit | saveAs (saveAs prefills from `view` but always creates a new one).
  mode: { type: String, default: "create" },
  // Source view for edit/saveAs; null when creating.
  view: { type: Object, default: null },
  canManageSystemView: { type: Boolean, default: false },
  statusOptions: { type: Array, default: () => [] },
  priorityOptions: { type: Array, default: () => [] },
  members: { type: Array, default: () => [] },
  customFieldMembers: { type: Array, default: () => [] },
  versions: { type: Array, default: () => [] },
  categories: { type: Array, default: () => [] },
  fieldDescriptors: { type: Array, default: () => [] },
  // Gantt mode: per-section option sources { common, requirement, defect },
  // each { statuses, priorities, categories, fields }.
  ganttSections: { type: Object, default: null },
  // Gantt mode: the defect block follows the defect app switch. Hiding it only drops the
  // UI — stored defect conditions stay in ganttTrees and are saved back unchanged, so
  // re-enabling the app restores them.
  showDefectSection: { type: Boolean, default: true },
});

const emit = defineEmits({
  "update:open": (open) => true,
  // A view was created/updated. Create/save-as include the new id so the board can switch to it.
  saved: (payload) => true,
});

const { t } = useI18n();
const saving = ref(false);
const filtersCollapsed = ref(false);
const form = reactive({ name: "", viewType: "personal" });
const filterTree = ref({
  logic: "and",
  conditions: [newViewFilterCondition()],
});
// Gantt mode keeps three independent trees (common / requirement / defect).
const ganttTrees = reactive({
  common: newGanttTree(),
  requirement: newGanttTree(),
  defect: newGanttTree(),
});

function newGanttTree() {
  return { logic: "and", conditions: [] };
}

const ganttSectionList = computed(() => {
  if (props.typeKey !== "gantt" || !props.ganttSections) return [];
  const sections = [
    {
      key: "common",
      title: t("gantt_view_common_filters"),
      typeKey: "requirement",
      options: props.ganttSections.common || {},
    },
    {
      key: "requirement",
      title: t("gantt_view_requirement_filters"),
      typeKey: "requirement",
      options: props.ganttSections.requirement || {},
    },
  ];
  if (props.showDefectSection) {
    sections.push({
      key: "defect",
      title: t("gantt_view_defect_filters"),
      typeKey: "defect",
      options: props.ganttSections.defect || {},
    });
  }
  return sections;
});

const viewFields = computed(() => {
  const fields = props.fieldDescriptors;
  if (
    props.typeKey !== "requirement" ||
    !fields.some(({ field }) => field === "assignee")
  ) {
    return fields;
  }
  return [
    ...fields,
    { field: "my_assignment_age", label: t("view_field_my_assignment_age") },
    { field: "assignment_age", label: t("view_field_assignment_age") },
  ];
});

// Option sources for every nested ViewFilterGroup level.
provide("viewFilterOptions", {
  statuses: computed(() => props.statusOptions),
  priorities: computed(() => props.priorityOptions),
  members: computed(() => props.members),
  customFieldMembers: computed(() => props.customFieldMembers),
  versions: computed(() => props.versions),
  categories: computed(() => props.categories),
  fields: viewFields,
});

const modalTitle = computed(() => {
  if (props.mode === "edit") return t("view_edit_title_edit");
  if (props.mode === "saveAs") return t("view_edit_title_save_as");
  return t("view_edit_title_create");
});
const okText = computed(() =>
  props.mode === "saveAs" ? t("view_save_as") : t("common_save"),
);

// Re-initialize the form each time the modal opens.
watch(
  () => props.open,
  (open) => {
    if (!open) return;
    filtersCollapsed.value = false;
    if (props.mode === "create" || !props.view) {
      form.name = "";
      form.viewType = "personal";
      filterTree.value = {
        logic: "and",
        conditions: [newViewFilterCondition()],
      };
      resetGanttTrees(null);
      return;
    }
    form.name = props.view.name || "";
    // Save-as keeps the source type only when the user is allowed to create it.
    form.viewType =
      props.view.view_type === "system" &&
      (props.mode === "edit" || props.canManageSystemView)
        ? "system"
        : "personal";
    if (props.typeKey === "gantt") {
      resetGanttTrees(props.view.filters);
      return;
    }
    let tree = normalizeCascaderViewFilterTree(
      normalizeCascaderFilterTree(
        pruneUnavailableFields(normalizeViewFilterTree(props.view.filters)),
        viewFields.value,
      ),
      viewFields.value,
    );
    if (props.mode === "saveAs") tree = pruneInactiveConditions(tree);
    if (props.mode === "edit" && !tree.conditions.length) {
      tree.conditions.push(newViewFilterCondition());
    }
    filterTree.value = tree;
  },
);

// resetGanttTrees rebuilds the three edit trees from a stored gantt filters
// payload ({common, requirement, defect}) or clears them when null.
function resetGanttTrees(filters) {
  for (const key of ["common", "requirement", "defect"]) {
    const stored = filters?.[key];
    if (!stored) {
      ganttTrees[key] = newGanttTree();
      continue;
    }
    const allowed = new Set(
      (props.ganttSections?.[key]?.fields || []).map((field) => field.field),
    );
    const tree = normalizeViewFilterTree(stored);
    const visit = (group) => {
      group.conditions = (group.conditions || [])
        .map((node) => {
          if (node.field) return allowed.has(node.field) ? node : null;
          const nested = visit(node);
          return nested.conditions.length ? nested : null;
        })
        .filter(Boolean);
      return group;
    };
    ganttTrees[key] = visit(tree);
  }
}

// ganttFilterPayload returns a section tree for saving, or undefined when the
// section holds no conditions (the backend stores nothing for empty sections).
function ganttFilterPayload(key) {
  const tree = ganttTrees[key];
  return tree.conditions.length ? tree : undefined;
}

// Historical views can retain fields that were later disabled or deleted. Keep
// those conditions out of the editor so a subsequent save cleans them up.
function pruneUnavailableFields(tree) {
  const allowed = new Set(viewFields.value.map((field) => field.field));
  const visit = (group) => {
    group.conditions = (group.conditions || [])
      .map((node) => {
        if (node.field) {
          return allowed.has(node.field) ? node : null;
        }
        const nested = visit(node);
        return nested.conditions.length ? nested : null;
      })
      .filter(Boolean);
    return group;
  };
  return visit(tree);
}

// Search settings retain visible empty fields for the toolbar. A saved view
// should only inherit conditions that currently affect the search result.
function pruneInactiveConditions(tree) {
  const visit = (group) => ({
    ...group,
    conditions: (group.conditions || [])
      .map((node) => {
        if (!node.field) {
          const nested = visit(node);
          return nested.conditions.length ? nested : null;
        }
        if (["empty", "not_empty"].includes(node.operator)) return node;
        return node.values?.some((value) => String(value).trim()) ? node : null;
      })
      .filter(Boolean),
  });
  return visit(tree);
}

function close() {
  emit("update:open", false);
}

async function handleSave() {
  const name = form.name.trim();
  if (!name) {
    message.warning(t("view_name_required"));
    return;
  }
  saving.value = true;
  try {
    const isGantt = props.typeKey === "gantt";
    const filtersPayload = isGantt
      ? {
          common_filters: ganttFilterPayload("common"),
          requirement_filters: ganttFilterPayload("requirement"),
          defect_filters: ganttFilterPayload("defect"),
        }
      : {
          filters: normalizeCascaderViewFilterTree(
            normalizeCascaderFilterTree(
              normalizeViewFilterTree(filterTree.value),
              viewFields.value,
            ),
            viewFields.value,
          ),
        };
    if (props.mode === "edit") {
      await updateWorkItemView(props.wsId, props.view.id, {
        name,
        ...filtersPayload,
      });
      message.success(t("common_saveSuccess"));
      emit("saved", { id: props.view.id, mode: props.mode });
    } else {
      const created = await createWorkItemView(props.wsId, {
        type: props.typeKey,
        name,
        view_type: form.viewType,
        ...filtersPayload,
      });
      message.success(t("common_saveSuccess"));
      // Emit the freshly created id so the board can switch to this view.
      emit("saved", { id: created?.id, mode: props.mode });
    }
    close();
  } catch {
    /* handled by interceptor */
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped lang="less">
.view-edit-form__label {
  font-weight: 500;
  margin: 16px 0 8px;
  &:first-child {
    margin-top: 0;
  }
}
.view-edit-form__hint {
  margin: -4px 0 8px;
  color: #8c8c8c;
  font-size: 12px;
  line-height: 18px;
}
.view-scope-card {
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  padding: 12px 16px;
}
.view-scope-card__head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
</style>
