export function processingActionKey(item) {
  if (item.action === "created") return "issue_processing_created";
  if (item.action === "status_change") return "issue_status_updated";
  if (
    item.action === "issue_recall" &&
    ["status", "status_id"].includes(item.field_name)
  )
    return "requirement_history_issue_recall";
  if (
    ["assignee_change", "field_update"].includes(item.action) &&
    item.field_name === "assignee"
  )
    return "issue_processing_assigned";
  return "";
}

export function buildIssueProcessingRecords(histories, detail) {
  const records = histories.filter((item) => processingActionKey(item));
  if (
    detail &&
    Number(detail.created_at) > 0 &&
    !histories.some((item) => item.action === "created")
  ) {
    const created = {
      id: `created:${detail.id}`,
      action: "created",
      created_at: detail.created_at,
      user_name: detail.creator_name || "",
    };
    const index = records.findIndex(
      (item) => Number(item.created_at) < Number(created.created_at),
    );
    records.splice(index < 0 ? records.length : index, 0, created);
  }
  return records;
}
