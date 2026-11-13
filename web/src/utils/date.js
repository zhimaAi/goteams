export function formatTimestamp(value, locale = "zh-CN") {
  const seconds = Number(value);
  if (!Number.isFinite(seconds) || seconds <= 0) return "-";

  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(seconds * 1000));
}

/** Format a Unix timestamp into a local date, optionally precise to the second. */
function localDayTimestamp(value) {
  const seconds = Number(value);
  if (!Number.isFinite(seconds) || seconds <= 0) return null;
  const date = new Date(seconds * 1000);
  return (
    new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime() /
    1000
  );
}

/** Determine whether the planned date range covers the target calendar day in the browser's local timezone. */
export function plannedDateCoversDay(start, end, target) {
  const startDay = localDayTimestamp(start);
  const endDay = localDayTimestamp(end);
  const targetDay = localDayTimestamp(target);
  if (targetDay === null || (startDay === null && endDay === null))
    return false;
  return (
    (startDay === null || startDay <= targetDay) &&
    (endDay === null || targetDay <= endDay)
  );
}

export function formatLocalDate(timestamp, withTime = false) {
  const seconds = Number(timestamp);
  if (!Number.isFinite(seconds) || seconds <= 0) return "-";
  const date = new Date(seconds * 1000);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const dateText = `${year}-${month}-${day}`;
  if (!withTime) return dateText;

  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  const second = String(date.getSeconds()).padStart(2, "0");
  return `${dateText} ${hour}:${minute}:${second}`;
}

/** Format a Unix timestamp as today/yesterday/MM-DD in the browser timezone. */
export function formatFriendlyLocalDate(
  timestamp,
  todayLabel = "今天",
  yesterdayLabel = "昨天",
) {
  const seconds = Number(timestamp);
  if (!Number.isFinite(seconds) || seconds <= 0) return "-";
  const date = new Date(seconds * 1000);
  const now = new Date();
  const targetDay = new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
  ).getTime();
  const today = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate(),
  ).getTime();
  const dayDiff = Math.round((today - targetDay) / 86400000);
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  if (dayDiff === 0) return `${todayLabel} ${hour}:${minute}`;
  if (dayDiff === 1) return `${yesterdayLabel} ${hour}:${minute}`;
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${month}-${day} ${hour}:${minute}`;
}
