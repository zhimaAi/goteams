// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
)

// Aging control configuration constants.
const (
	// DefaultTotalDurationMinutes is the default total-duration timeout (minutes) when a new rule is added.
	DefaultTotalDurationMinutes = 4320
	// DefaultStatusTimeoutMinutes is the default per-status timeout (minutes) when a new status rule is added.
	DefaultStatusTimeoutMinutes = 1440
	// AgingTimeoutMinMinutes / AgingTimeoutMaxMinutes bound the editable timeout range (1~43200 minutes).
	AgingTimeoutMinMinutes = 1
	AgingTimeoutMaxMinutes = 43200
	// AgingWarnMinMinutes / AgingWarnMaxMinutes bound the editable warning lead time (1~43200 minutes, i.e. 30 days).
	AgingWarnMinMinutes = 1
	AgingWarnMaxMinutes = 43200

	// Built-in fallback thresholds used when the aging control is disabled (config-fallback judgement).
	AgingFallbackPlannedHours    = 24 // 待规划 / 待分配
	AgingFallbackInProgressHours = 72 // 进行中
)

// aging_rule JSON keys.
const (
	AgingRuleKeyTotal        = `total`
	AgingRuleKeyStatusPrefix = `status:`
)

// ErrAgingFormInvalid indicates the submitted aging control form is invalid.
var ErrAgingFormInvalid = errors.New("aging control form invalid")

// AgingRule is the per-target (total duration or a single status) timing rule.
type AgingRule struct {
	Enabled        int  `json:"enabled"`
	TimeoutMinutes int  `json:"timeout_minutes"`
	NotifyEnabled  int  `json:"notify_enabled"`
	WarnMinutes    *int `json:"warn_minutes,omitempty"`
}

// AgingStatusRule is a per-status rule carrying its status metadata for display.
type AgingStatusRule struct {
	StatusID    int    `json:"status_id"`
	StatusName  string `json:"status_name"`
	StatusColor string `json:"status_color"`
	AgingRule
}

// AgingControlForm is the requirement aging control configuration stored in
// workspaces_settings.requirement_aging_control.
type AgingControlForm struct {
	MasterEnabled bool              `json:"master_enabled"`
	TotalDuration AgingRule         `json:"total_duration"`
	StatusRules   []AgingStatusRule `json:"status_rules"`
}

type AgingRuleJSON struct {
	Enabled        any  `json:"enabled"`
	TimeoutMinutes any  `json:"timeout_minutes"`
	NotifyEnabled  any  `json:"notify_enabled"`
	WarnMinutes    *int `json:"warn_minutes"`
}

type AgingStatusRuleJSON struct {
	StatusID    any    `json:"status_id"`
	StatusName  string `json:"status_name"`
	StatusColor string `json:"status_color"`
	AgingRuleJSON
}

type agingControlFormJSON struct {
	MasterEnabled bool                  `json:"master_enabled"`
	TotalDuration AgingRuleJSON         `json:"total_duration"`
	StatusRules   []AgingStatusRuleJSON `json:"status_rules"`
}

const defaultAgingControlJSON = `{"master_enabled":false,"total_duration":{"enabled":0,"timeout_minutes":4320,"notify_enabled":0},"status_rules":[]}`

// defaultAgingControlForm returns the system preset: master off, total rule 4320m, no status rules.
func defaultAgingControlForm() AgingControlForm {
	return AgingControlForm{
		MasterEnabled: false,
		TotalDuration: AgingRule{Enabled: 0, TimeoutMinutes: DefaultTotalDurationMinutes, NotifyEnabled: 0},
		StatusRules:   make([]AgingStatusRule, 0),
	}
}

func parseAgingControlForm(raw string) AgingControlForm {
	if strings.TrimSpace(raw) == `` {
		return defaultAgingControlForm()
	}
	var payload agingControlFormJSON
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		logs.Error("parse aging control form failed, err:%s", err.Error())
		return defaultAgingControlForm()
	}
	form := AgingControlForm{MasterEnabled: payload.MasterEnabled}
	form.TotalDuration = parseAgingRule(payload.TotalDuration)
	form.StatusRules = make([]AgingStatusRule, 0, len(payload.StatusRules))
	for _, item := range payload.StatusRules {
		rule := AgingStatusRule{
			StatusID:    cast.ToInt(item.StatusID),
			StatusName:  strings.TrimSpace(item.StatusName),
			StatusColor: strings.TrimSpace(item.StatusColor),
		}
		rule.AgingRule = parseAgingRule(item.AgingRuleJSON)
		if rule.StatusID <= 0 {
			continue
		}
		form.StatusRules = append(form.StatusRules, rule)
	}
	return form
}

func parseAgingRule(raw AgingRuleJSON) AgingRule {
	rule := AgingRule{Enabled: 0, TimeoutMinutes: DefaultStatusTimeoutMinutes, NotifyEnabled: 0}
	if cast.ToInt(raw.Enabled) != 0 {
		rule.Enabled = 1
	}
	if cast.ToInt(raw.NotifyEnabled) != 0 {
		rule.NotifyEnabled = 1
	}
	minutes := cast.ToInt(raw.TimeoutMinutes)
	if minutes <= 0 {
		minutes = DefaultStatusTimeoutMinutes
	}
	rule.TimeoutMinutes = minutes
	if raw.WarnMinutes != nil {
		warn := *raw.WarnMinutes
		if warn > 0 {
			rule.WarnMinutes = raw.WarnMinutes
		}
	}
	return rule
}

func encodeAgingControlForm(form AgingControlForm) (string, error) {
	normalized, err := normalizeAgingControlForm(form)
	if err != nil {
		return ``, err
	}
	payload := agingControlFormJSON{
		MasterEnabled: normalized.MasterEnabled,
		TotalDuration: toAgingRuleJSON(normalized.TotalDuration),
		StatusRules:   make([]AgingStatusRuleJSON, 0, len(normalized.StatusRules)),
	}
	for _, item := range normalized.StatusRules {
		payload.StatusRules = append(payload.StatusRules, AgingStatusRuleJSON{
			StatusID:      item.StatusID,
			StatusName:    item.StatusName,
			StatusColor:   item.StatusColor,
			AgingRuleJSON: toAgingRuleJSON(item.AgingRule),
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ``, err
	}
	return string(raw), nil
}

func toAgingRuleJSON(rule AgingRule) AgingRuleJSON {
	return AgingRuleJSON{
		Enabled:        rule.Enabled,
		TimeoutMinutes: rule.TimeoutMinutes,
		NotifyEnabled:  rule.NotifyEnabled,
		WarnMinutes:    rule.WarnMinutes,
	}
}

// normalizeAgingControlForm clamps values, fills missing defaults, and validates ranges.
func normalizeAgingControlForm(form AgingControlForm) (AgingControlForm, error) {
	normalized := AgingControlForm{MasterEnabled: form.MasterEnabled}
	normalized.TotalDuration = normalizeAgingRule(form.TotalDuration, DefaultTotalDurationMinutes)
	normalized.StatusRules = make([]AgingStatusRule, 0, len(form.StatusRules))
	seen := map[int]struct{}{}
	for _, item := range form.StatusRules {
		if item.StatusID <= 0 {
			continue
		}
		if _, exists := seen[item.StatusID]; exists {
			continue
		}
		seen[item.StatusID] = struct{}{}
		normalized.StatusRules = append(normalized.StatusRules, AgingStatusRule{
			StatusID:    item.StatusID,
			StatusName:  item.StatusName,
			StatusColor: item.StatusColor,
			AgingRule:   normalizeAgingRule(item.AgingRule, DefaultStatusTimeoutMinutes),
		})
	}
	return normalized, nil
}

func normalizeAgingRule(rule AgingRule, defaultMinutes int) AgingRule {
	normalized := AgingRule{Enabled: 0, TimeoutMinutes: defaultMinutes, NotifyEnabled: 0}
	if rule.Enabled != 0 {
		normalized.Enabled = 1
	}
	if rule.NotifyEnabled != 0 {
		normalized.NotifyEnabled = 1
	}
	minutes := rule.TimeoutMinutes
	if minutes <= 0 {
		minutes = defaultMinutes
	}
	if minutes < AgingTimeoutMinMinutes {
		minutes = AgingTimeoutMinMinutes
	}
	if minutes > AgingTimeoutMaxMinutes {
		minutes = AgingTimeoutMaxMinutes
	}
	normalized.TimeoutMinutes = minutes
	if rule.WarnMinutes != nil {
		warn := *rule.WarnMinutes
		if warn >= AgingWarnMinMinutes && warn <= AgingWarnMaxMinutes {
			normalized.WarnMinutes = rule.WarnMinutes
		}
	}
	return normalized
}

// AgingRuleByStatus returns the configured rule for a status id, or nil when not present.
func (f AgingControlForm) AgingRuleByStatus(statusID int) *AgingStatusRule {
	for i := range f.StatusRules {
		if f.StatusRules[i].StatusID == statusID {
			return &f.StatusRules[i]
		}
	}
	return nil
}
