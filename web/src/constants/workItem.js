export const REQUIREMENT_TYPE = {
  requirement: {
    color: 'blue',
    labelKey: 'requirement_feature',
  },
  defect: {
    color: 'red',
    labelKey: 'requirement_bug',
  },
}

export const REQUIREMENT_STATUS = {
  pending_plan: {
    color: 'default',
    labelKey: 'requirement_backlog',
  },
  pending_assign: {
    color: 'blue',
    labelKey: 'requirement_assigned',
  },
  in_progress: {
    color: 'processing',
    labelKey: 'requirement_inProgress',
  },
  done: {
    color: 'success',
    labelKey: 'requirement_done',
  },
  closed: {
    color: 'default',
    labelKey: 'requirement_closed',
  },
  rejected: {
    color: 'error',
    labelKey: 'requirement_rejected',
  },
}

export const REQUIREMENT_STATUS_KEYS = Object.keys(REQUIREMENT_STATUS)

export const REQUIREMENT_PRIORITY = {
  urgent: {
    color: 'red',
    labelKey: 'requirement_urgent',
  },
  high: {
    color: 'orange',
    labelKey: 'requirement_high',
  },
  medium: {
    color: 'blue',
    labelKey: 'requirement_medium',
  },
  low: {
    color: 'default',
    labelKey: 'requirement_low',
  },
}

export const REQUIREMENT_PRIORITY_KEYS = Object.keys(REQUIREMENT_PRIORITY)
