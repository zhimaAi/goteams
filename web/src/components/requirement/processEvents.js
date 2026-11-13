const MAX_STEP_DURATION_MS = 60 * 60 * 1000;

function formatDuration(startAt, endAt) {
  const diff = endAt - startAt;
  if (!startAt || !endAt || diff <= 0 || diff > MAX_STEP_DURATION_MS) return "";
  const seconds = diff / 1000;
  if (seconds < 0.1) return "<0.1s";
  return `${seconds.toFixed(1)}s`;
}

function lastMessageSequence(list) {
  let sequence = 0;
  for (const item of list) {
    if (item.type === "message" && String(item.content || "").trim()) {
      sequence = Number(item.sequence) || 0;
    }
  }
  return sequence;
}

function rowOf(key, kind, content) {
  return {
    key,
    kind,
    content,
    name: "",
    args: "",
    result: "",
    durationText: "",
  };
}

// 与客户端执行过程同一套归属：最后一条 message 是最终结论，不在过程里重复；
// 工具结果按 tool_use_id 归到对应调用，没有 ID 时只在唯一未完成调用上归属。
export function buildExecutionProcessRows(events) {
  const list = (Array.isArray(events) ? events : [])
    .filter((item) => item && item.type)
    .slice()
    .sort((left, right) => Number(left.sequence) - Number(right.sequence));
  const finalMessageSequence = lastMessageSequence(list);
  const nextTime = new Map();
  for (let index = 0; index < list.length - 1; index += 1) {
    nextTime.set(Number(list[index].sequence), Number(list[index + 1].timestamp) || 0);
  }
  const rows = [];
  const byId = new Map();
  const startedAt = new Map();

  for (const item of list) {
    const type = item.type;
    const content = String(item.content || "").trim();
    const sequence = Number(item.sequence) || 0;
    const time = Number(item.timestamp) || 0;

    if (type === "thinking") {
      if (!content) continue;
      const row = rowOf(`thinking-${sequence}`, "thinking", content);
      row.durationText = formatDuration(time, nextTime.get(sequence));
      rows.push(row);
      continue;
    }
    if (type === "message") {
      if (!content || sequence === finalMessageSequence) continue;
      rows.push(rowOf(`output-${sequence}`, "output", content));
      continue;
    }
    if (type === "permission_request") {
      if (!content) continue;
      rows.push(rowOf(`permission-${sequence}`, "permission", content));
      continue;
    }
    if (type === "tool_call") {
      const separator = content.indexOf(" ");
      const row = rowOf(`tool-${sequence}`, "tool", "");
      row.name = separator > 0 ? content.slice(0, separator) : content || "tool";
      row.args =
        separator > 0 ? content.slice(separator + 1).replace(/\s+/g, " ").trim() : "";
      rows.push(row);
      startedAt.set(row, time);
      const toolUseId = String(item.tool_use_id || "").trim();
      if (toolUseId) byId.set(toolUseId, row);
      continue;
    }
    if (type !== "tool_result" || !content) continue;

    const toolUseId = String(item.tool_use_id || "").trim();
    let target;
    if (toolUseId) {
      target = byId.get(toolUseId);
    } else {
      const candidates = rows.filter((row) => row.kind === "tool" && !row.result);
      if (candidates.length === 1) target = candidates[0];
    }
    if (!target) continue;
    target.result = content;
    const started = startedAt.get(target);
    if (started) target.durationText = formatDuration(started, time);
  }
  return rows;
}
