// ganttViewSections.js
// Field descriptors for the gantt view editor's three filter sections.
// The common section only offers fields shared by requirements and defects;
// the per-type sections additionally expose status and priority.
import { computed } from "vue";

function fieldDescriptor(t, key, control, operator) {
  return { field: key, control, operator, label: t(`view_field_${key}`) };
}

/**
 * Build the field list for one section.
 * @param {(key:string)=>string} t i18n translator
 * @param {boolean} withTypeScoped include status/priority (per-type sections only)
 */
function sectionFields(t, withTypeScoped) {
  const fields = [
    fieldDescriptor(t, "title", "text", "contains"),
    fieldDescriptor(t, "id", "text", "eq"),
    fieldDescriptor(t, "assignee", "person", "in"),
    fieldDescriptor(t, "creator", "person", "in"),
    fieldDescriptor(t, "version", "multi", "in"),
    fieldDescriptor(t, "planned_start_date", "date", "between"),
    fieldDescriptor(t, "planned_end_date", "date", "between"),
    fieldDescriptor(t, "created_at", "date", "between"),
  ];
  if (withTypeScoped) {
    fields.splice(2, 0, fieldDescriptor(t, "status", "multi", "in"));
    fields.splice(3, 0, fieldDescriptor(t, "priority", "multi", "in"));
  }
  return fields;
}

/**
 * Reactive section option sources for ViewEditModal's gantt mode.
 * @param {import('vue').Ref<object>} metadata workspace metadata ref
 * @param {(key:string)=>string} t i18n translator
 */
export function useGanttViewSections(metadata, t) {
  return computed(() => ({
    common: {
      statuses: [],
      priorities: [],
      categories: [],
      fields: sectionFields(t, false),
    },
    requirement: {
      statuses: metadata.value?.requirement?.statuses || [],
      priorities: metadata.value?.requirement?.priorities || [],
      categories: [],
      fields: sectionFields(t, true),
    },
    defect: {
      statuses: metadata.value?.defect?.statuses || [],
      priorities: metadata.value?.defect?.priorities || [],
      categories: [],
      fields: sectionFields(t, true),
    },
  }));
}
