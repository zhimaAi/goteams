// SLA durations use effective seconds supplied by the server, not wall-clock subtraction.
export function slaDuration(seconds, t) {
  const value = Math.ceil(Math.max(0, Number(seconds) || 0));
  const days = Math.floor(value / 86400);
  const hours = Math.floor((value % 86400) / 3600);
  const minutes = Math.floor((value % 3600) / 60);
  const rest = value % 60;
  return [
    days && t("sla_durationDays", { value: days }),
    hours && t("sla_durationHours", { value: hours }),
    minutes && t("sla_durationMinutes", { value: minutes }),
    (rest || !value) && t("sla_durationSeconds", { value: rest }),
  ]
    .filter(Boolean)
    .join(" ");
}
export function slaStageText(stage, t) {
  if (stage.calculation_state === "calendar_unavailable")
    return t("sla_calendarPending");
  if (stage.state === "not_started") return t("sla_notStarted");
  if (stage.state === "interrupted") return t(`sla_reason_${stage.reason}`);
  if (stage.state === "completed")
    return t(
      stage.overdue_seconds > 0 ? "sla_completedLate" : "sla_completedOnTime",
    );
  return t(stage.overdue_seconds > 0 ? "sla_overdueBy" : "sla_dueWithin", {
    time: slaDuration(
      stage.overdue_seconds > 0
        ? stage.overdue_seconds
        : stage.remaining_seconds,
      t,
    ),
  });
}
