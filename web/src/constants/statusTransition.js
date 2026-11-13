// Shared status-transition form constants.
export const DEFAULT_STATUS_TRANSITION_FORM = {
  enabled: 1,
  fields: [
    { key: "assignee", required: 0 },
    { key: "comment", required: 0 },
  ],
};

export const STATUS_TRANSITION_BUILTIN_KEYS = [
  "version",
  "priority",
  "planned_start_date",
  "planned_end_date",
  "assignee",
];

// Deep-link anchor of the requirement aging (SLA) control card. The requirement aging tab
// links to Settings > Requirement > Status with `?anchor=<value>` and that page scrolls the
// card into view, so both ends must read this same value.
export const AGING_CONTROL_ANCHOR = "aging-control";

// Default requirement aging (SLA) control form.
export const DEFAULT_AGING_CONTROL_FORM = {
  master_enabled: false,
  total_duration: {
    enabled: 0,
    timeout_minutes: 4320,
    notify_enabled: 0,
    warn_minutes: null,
  },
  default_status_timeout_minutes: 1440,
};
