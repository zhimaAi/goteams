// Normalizes status-transition drafts and field values.
import dayjs from "dayjs";
import { customFieldValues } from "@/utils/customField";

function toDayjs(value) {
  const timestamp = Number(value);
  return timestamp > 0 ? dayjs.unix(timestamp) : null;
}

export function createStatusTransitionDraft(workItem, customFields) {
  const item = workItem || {};
  const values = customFieldValues(item.custom_fields);
  const custom_fields = {};
  for (const field of customFields) {
    const raw = values[String(field.id)];
    if (field.field_type === "select_multi") {
      if (Array.isArray(raw)) custom_fields[field.id] = raw;
      else {
        try {
          custom_fields[field.id] = raw ? JSON.parse(raw) : [];
        } catch {
          custom_fields[field.id] = [];
        }
      }
    } else if (field.field_type === "date") {
      custom_fields[field.id] = raw ? dayjs(raw) : null;
    } else if (field.field_type === "number" || field.field_type === "user") {
      custom_fields[field.id] =
        raw === undefined || raw === "" ? undefined : Number(raw);
    } else custom_fields[field.id] = raw ?? "";
  }
  return {
    assignee_ids: Array.isArray(item.assignee_ids)
      ? item.assignee_ids.map(Number)
      : [],
    version_id: Number(item.version_id) || undefined,
    priority: Number(item.priority) || undefined,
    planned_start_date: toDayjs(item.planned_start_date),
    planned_end_date: toDayjs(item.planned_end_date),
    comment: "",
    image_file_ids: [],
    custom_fields,
  };
}

export function isStatusTransitionFieldEmpty(field, value, customField) {
  if (field.key === "assignee") return !value?.length;
  if (field.key === "comment") return isRichTextEmpty(value);
  if (field.key === "version" || field.key === "priority") return !value;
  if (field.key === "planned_start_date" || field.key === "planned_end_date")
    return !value;
  if (!customField) return true;
  if (customField.field_type === "select_multi")
    return !Array.isArray(value) || !value.length;
  if (customField.field_type === "date") return !value;
  if (value === undefined || value === null) return true;
  return String(value).trim() === "";
}

function isRichTextEmpty(value) {
  const html = String(value || "").trim();
  if (!html) return true;
  if (typeof document === "undefined")
    return (
      html
        .replace(/<[^>]*>/g, "")
        .replace(/&nbsp;/gi, " ")
        .trim() === ""
    );
  const container = document.createElement("div");
  container.innerHTML = html;
  if (
    container.querySelector(
      "img, video, iframe, table, hr, [data-editor-file], [data-editor-video], [data-editor-code-block]",
    )
  )
    return false;
  return !(container.textContent || "").replace(/\u00a0/g, " ").trim();
}

export function statusTransitionUnixOrZero(value) {
  return value ? dayjs(value).startOf("day").unix() : 0;
}

export function statusTransitionCustomPayloadValue(field, value) {
  if (field.field_type === "date")
    return value ? dayjs(value).format("YYYY-MM-DD") : "";
  return value ?? "";
}
