package business

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// DispatchExternalNotifications sends a matching event to enabled tenant strategies.
// Strategies are configured at enterprise level, so any project under the tenant can trigger them.
// It is called after the source business transaction commits; failures are logged and do not
// change the already-committed work item operation.
func DispatchExternalNotifications(event AutomationEvent) {
	if event.Source == define.SourceAutomation || event.AdminID <= 0 || event.WorkspaceID <= 0 {
		return
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, platform, webhook_url, secret, events::text,
		at_mode, at_user_ids::text, at_roles::text, at_all
		FROM `+define.TableExternalNotificationStrategies+`
		WHERE admin_id=$1 AND enabled=1 ORDER BY id`, nil, event.AdminID)
	if err != nil {
		logs.Error("list external notification strategies for event failed: %s", err.Error())
		return
	}
	if len(rows) == 0 {
		return
	}
	// Every strategy of the tenant pushes about the same work item, so the shared facts are read once.
	messageContext := buildNotificationMessageContext(event)
	// A strategy reports only about its own members, so the whole tenant membership is read once.
	membersByStrategy := notificationStrategyMembersByStrategy(event.AdminID)
	// One operation can satisfy several configured events, and every strategy asks about the same
	// events, so the recipients of each event are resolved once per dispatch.
	recipientsByEvent := make(map[string][]int, len(notificationIntroKeys))
	// todayTriggerDate carries the day the today counters belong to; every push of one dispatch shares it.
	todayTriggerDate := notificationTodayStart()
	for _, row := range rows {
		var events []string
		if err := json.Unmarshal([]byte(row[`events`]), &events); err != nil {
			logs.Error("decode external notification strategy events failed: %s", err.Error())
			continue
		}
		// One operation can satisfy several configured events (a single edit changing both the
		// status and the deadline); each match is pushed once with its own copy.
		matched := externalNotificationMatchedEvents(events, event)
		if len(matched) == 0 {
			continue
		}
		members := membersByStrategy[cast.ToInt(row[`id`])]
		if len(members) == 0 {
			continue
		}
		at := notificationAtConfig{AtMode: defaultNotificationAtMode(row[`platform`])}
		if row[`at_mode`] != "" {
			at.AtMode = row[`at_mode`]
		}
		_ = json.Unmarshal([]byte(row[`at_user_ids`]), &at.AtUserIds)
		_ = json.Unmarshal([]byte(row[`at_roles`]), &at.AtRoles)
		at.AtAll = row[`at_all`] == "true"
		for _, key := range matched {
			recipients, ok := recipientsByEvent[key]
			if !ok {
				recipients = externalNotificationRecipients(key, event)
				recipientsByEvent[key] = recipients
			}
			// An event is only pushed when it is about a member this strategy covers: the
			// strategy's member list is what decides who the robot speaks for.
			if !notificationReachesMembers(recipients, members) {
				continue
			}
			content := buildExternalNotificationContent(messageContext, key, event)
			if err := sendExternalNotificationWebhook(row[`platform`], row[`webhook_url`], row[`secret`], at, content, event); err != nil {
				logs.Error("send external notification strategy %s event %s failed: %s", row[`id`], key, err.Error())
				continue
			}
			now := tool.Time2Int()
			// The today counter has no scheduler to reset it, so its owning day travels with the row:
			// a push on a later day restarts it at 1 instead of piling onto yesterday's count.
			if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableExternalNotificationStrategies+
				` SET today_trigger_count=CASE WHEN today_trigger_date=$1 THEN today_trigger_count+1 ELSE 1 END,
				today_trigger_date=$1,total_trigger_count=total_trigger_count+1,updated_at=$2
				WHERE admin_id=$3 AND id=$4 AND enabled=1`, nil, todayTriggerDate, now, event.AdminID, cast.ToInt(row[`id`])); err != nil {
				logs.Error("update external notification strategy counters failed: %s", err.Error())
			}
		}
	}
}

// externalNotificationMatchedEvents returns the configured events the operation triggers, keeping
// the strategy's own event order so pushes arrive in the order the admin listed them.
func externalNotificationMatchedEvents(events []string, event AutomationEvent) []string {
	matched := make([]string, 0, len(events))
	seen := make(map[string]bool, len(events))
	for _, key := range events {
		if seen[key] {
			continue
		}
		seen[key] = true
		if externalNotificationEventMatches(key, event) {
			matched = append(matched, key)
		}
	}
	return matched
}

func externalNotificationEventMatches(key string, event AutomationEvent) bool {
	switch key {
	case "comment_mention":
		// Who is mentioned is resolved from the comment text, so the event itself only has to be a
		// comment that was written or edited; deleting one retracts it instead of mentioning.
		return event.Operation == "comment" && event.Comment != nil &&
			(event.CommentChange == "add" || event.CommentChange == "edit")
	case "status_change":
		return event.Operation == "update" && event.Before != nil && event.After != nil &&
			cast.ToInt(event.Before.Fields["status"]) != cast.ToInt(event.After.Fields["status"])
	case "due_reminder", "overdue":
		// Both are decided by 时效管控: the SLA worker is the only producer of the verdict, so a
		// user editing a deadline can no longer fake a timeout report.
		if event.SLA == nil {
			return false
		}
		if key == "overdue" {
			return event.SLA.Overdue
		}
		return !event.SLA.Overdue
	case "assign_requirement", "assign_bug":
		itemType := map[string]string{"assign_requirement": define.TypeKeyRequirement, "assign_bug": define.TypeKeyDefect}[key]
		if event.ItemType != itemType || event.After == nil {
			return false
		}
		if event.Operation == "create" && len(event.After.AssigneeIDs) > 0 {
			return true
		}
		return event.Operation == "update" && event.Before != nil && !sameIntSlice(event.Before.AssigneeIDs, event.After.AssigneeIDs)
	}
	return false
}

// notificationTodayStart returns the midnight timestamp a strategy's today counter belongs to.
// Strategies store no timezone and both the counter update and the list read run outside a request,
// so the server's local day is the only day boundary the two sides can agree on.
func notificationTodayStart() int64 {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
}

func sameIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// externalNotificationRecipients resolves the members an event is about, i.e. the ones its copy
// speaks to: the assignee of an assignment, the mentioned member of a comment, the creator of a
// status change and the assignee of a timeout. Nobody is ever their own recipient, so assigning an
// item to yourself, @-mentioning yourself or changing the status of your own item stays silent.
func externalNotificationRecipients(eventKey string, event AutomationEvent) []int {
	actorID := event.TriggeredBy.ID
	switch eventKey {
	case "assign_requirement", "assign_bug":
		return notificationRecipientsExcluding(notificationAddedAssignees(event), actorID)
	case "status_change":
		if event.After == nil {
			return nil
		}
		return notificationRecipientsExcluding([]int{cast.ToInt(event.After.Fields[`created_by`])}, actorID)
	case "comment_mention":
		if event.Comment == nil {
			return nil
		}
		// Mentioning yourself is not a notification, so the comment author is excluded by name
		// rather than by the triggering user: an edit keeps the original author as the actor.
		authorID := event.Comment.UserID
		if authorID <= 0 {
			authorID = actorID
		}
		mentioned, err := NotificationMentionRecipients(event.AdminID, event.WorkspaceID, authorID, event.Comment.Content)
		if err != nil {
			logs.Error("resolve external notification mention recipients failed: %s", err.Error())
			return nil
		}
		return mentioned
	case "due_reminder", "overdue":
		// 时效管控 reports on the people handling the work item; the worker is not a user, so the
		// actor filter is a no-op here and is kept only to share one code path.
		if event.After == nil {
			return nil
		}
		return notificationRecipientsExcluding(event.After.AssigneeIDs, actorID)
	}
	return nil
}

// notificationAddedAssignees returns the assignees the operation added. An update that only drops
// an assignee adds nobody, and a create treats every assignee as added.
func notificationAddedAssignees(event AutomationEvent) []int {
	if event.After == nil {
		return nil
	}
	if event.Operation != "update" || event.Before == nil {
		return event.After.AssigneeIDs
	}
	previous := make(map[int]bool, len(event.Before.AssigneeIDs))
	for _, id := range event.Before.AssigneeIDs {
		previous[id] = true
	}
	added := make([]int, 0, len(event.After.AssigneeIDs))
	for _, id := range event.After.AssigneeIDs {
		if !previous[id] {
			added = append(added, id)
		}
	}
	return added
}

// notificationRecipientsExcluding drops the actor and duplicates from a candidate list.
func notificationRecipientsExcluding(candidates []int, actorID int) []int {
	recipients := make([]int, 0, len(candidates))
	seen := make(map[int]bool, len(candidates))
	for _, id := range candidates {
		if id <= 0 || id == actorID || seen[id] {
			continue
		}
		seen[id] = true
		recipients = append(recipients, id)
	}
	return recipients
}

// notificationReachesMembers reports whether at least one recipient belongs to the strategy.
func notificationReachesMembers(recipients []int, members map[int]bool) bool {
	for _, recipientID := range recipients {
		if members[recipientID] {
			return true
		}
	}
	return false
}

// notificationStrategyMembersByStrategy maps every strategy of the tenant to its member user ids.
// Dispatch runs after the business commit, so a strategy left without usable members simply stops
// pushing instead of failing the read.
func notificationStrategyMembersByStrategy(adminID int) map[int]map[int]bool {
	index := make(map[int]map[int]bool)
	rows, err := msql.RawValues(define.Postgres, `SELECT strategy_id, user_id FROM `+define.TableExternalNotificationStrategyMembers+`
		WHERE admin_id=$1`, nil, adminID)
	if err != nil {
		logs.Error("list external notification strategy members failed: %s", err.Error())
		return index
	}
	for _, row := range rows {
		strategyID, userID := cast.ToInt(row[`strategy_id`]), cast.ToInt(row[`user_id`])
		if strategyID <= 0 || userID <= 0 {
			continue
		}
		if index[strategyID] == nil {
			index[strategyID] = make(map[int]bool)
		}
		index[strategyID][userID] = true
	}
	return index
}

// notificationAtConfig carries a strategy's mention settings, aligned with automation webhook actions.
type notificationAtConfig struct {
	AtMode    string
	AtUserIds []int
	AtRoles   []string
	AtAll     bool
}

// defaultNotificationAtMode returns the first mention parameter each robot API accepts.
func defaultNotificationAtMode(platform string) string {
	if platform == "feishu" {
		return "user_id"
	}
	return "mobile"
}

// notificationAtTargets resolves configured users and trigger roles to the identifiers
// named by the at mode. Members without the identifier are skipped instead of failing the push.
func notificationAtTargets(adminId int, at notificationAtConfig, event AutomationEvent) ([]string, error) {
	return notificationAtTargetsContext(context.Background(), adminId, at, event)
}

func notificationAtTargetsContext(ctx context.Context, adminId int, at notificationAtConfig, event AutomationEvent) ([]string, error) {
	userIDs := notificationAtUserIDs(at, event)
	if len(userIDs) == 0 {
		return nil, nil
	}
	db, err := msql.GetDB(define.Postgres)
	if err != nil {
		return nil, err
	}
	query := `SELECT id,COALESCE(name,''),COALESCE(tel_number,''),
 COALESCE(push_dingtalk_at_mobiles,''),COALESCE(push_dingtalk_at_user_ids,''),COALESCE(push_feishu_user_id,''),
 COALESCE(push_wecom_mentioned_list,''),COALESCE(push_wecom_mentioned_mobile_list,'')
 FROM users WHERE admin_id=$1 AND id=ANY($2)`
	rows, err := db.QueryContext(ctx, query, adminId, pq.Array(userIDs))
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	defer rows.Close()
	indexed := make(map[int]automationAtTarget, len(userIDs))
	for rows.Next() {
		var target automationAtTarget
		if err := rows.Scan(&target.UserID, &target.Name, &target.Mobile, &target.DingtalkAtMobiles, &target.DingtalkAtUserIds, &target.FeishuUserId, &target.WecomMentionedList, &target.WecomMentionedMobileList); err != nil {
			return nil, err
		}
		indexed[target.UserID] = target
	}
	if err := rows.Err(); err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	mentions := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if target, ok := indexed[userID]; ok {
			if value := target.value(at.AtMode); value != "" {
				mentions = append(mentions, value)
			}
		}
	}
	return mentions, nil
}

// notificationAtUserIDs 合并显式成员和动态角色；兼容缺少处理人数组的历史快照。
func notificationAtUserIDs(at notificationAtConfig, event AutomationEvent) []int {
	seen := make(map[int]bool, len(at.AtUserIds)+len(at.AtRoles))
	userIDs := make([]int, 0, len(at.AtUserIds)+len(at.AtRoles))
	add := func(userID int) {
		if userID > 0 && !seen[userID] {
			seen[userID] = true
			userIDs = append(userIDs, userID)
		}
	}
	for _, userID := range at.AtUserIds {
		add(userID)
	}
	if len(at.AtRoles) > 0 {
		snapshot := event.After
		if snapshot == nil {
			snapshot = event.Before
		}
		for _, role := range at.AtRoles {
			if snapshot != nil {
				if role == "assignee" && len(snapshot.AssigneeIDs) > 0 {
					for _, id := range snapshot.AssigneeIDs {
						add(id)
					}
				} else {
					add(cast.ToInt(snapshot.Fields[role]))
				}
			}
		}
	}
	return userIDs
}

// NotificationDeliveryError 表示 Webhook 投递失败，且该失败来自地址、连通性或平台拒绝。
// 文本只保留原因和数字错误码。地址、密钥和上游响应体都可能带出机器人密钥，不能进入错误文本。
type NotificationDeliveryError struct {
	Code   int
	Reason string
}

func (e *NotificationDeliveryError) Error() string {
	if e == nil {
		return "notification delivery failed"
	}
	if e.Code != 0 {
		return fmt.Sprintf("notification delivery %s code %d", e.Reason, e.Code)
	}
	return "notification delivery " + e.Reason
}

func sendExternalNotificationWebhook(platform, rawURL, secret string, at notificationAtConfig, content string, event AutomationEvent) error {
	return sendExternalNotificationWebhookContext(context.Background(), platform, rawURL, secret, at, content, event)
}

func sendExternalNotificationWebhookContext(ctx context.Context, platform, rawURL, secret string, at notificationAtConfig, content string, event AutomationEvent) error {
	mentions, err := notificationAtTargetsContext(ctx, event.AdminID, at, event)
	if err != nil {
		return err
	}
	var body []byte
	// Keep this payload path aligned with automation webhook actions. In particular,
	// Feishu has a different signing algorithm from DingTalk, and robot APIs require
	// their platform-specific envelope even though all three use JSON over HTTP.
	// The notification copy is system generated and ends with the work item facts, so the
	// mention block is put on its own paragraph instead of trailing the assignee line.
	action := AutomationAction{Secret: strings.TrimSpace(secret), AtMode: at.AtMode, AtAll: at.AtAll}
	switch platform {
	case "dingtalk":
		rawURL, body, err = dingtalkRobotRequest(action, rawURL, content, mentions, true)
	case "feishu":
		rawURL, body, err = feishuRobotRequest(action, rawURL, content, mentions, true)
	case "wecom":
		if len(mentions) > 0 || action.AtAll {
			content += "\n"
		}
		body, err = json.Marshal(automationWecomMessage(action, content, mentions))
	default:
		return fmt.Errorf("unsupported external notification platform: %s", platform)
	}
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return &NotificationDeliveryError{Reason: "url"}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := automationHTTPClient().Do(req)
	if err != nil {
		return &NotificationDeliveryError{Reason: "dial"}
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return &NotificationDeliveryError{Reason: "dial"}
	}
	return notificationHTTPResult(platform, resp.StatusCode, responseBody)
}

// notificationHTTPResult 把平台错误码优先于 HTTP 状态。非 2xx 且没有平台错误码时记为 status。
func notificationHTTPResult(platform string, statusCode int, body []byte) error {
	if err := externalNotificationResponseError(platform, body); err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return &NotificationDeliveryError{Reason: "status"}
	}
	return nil
}

// externalNotificationResponseError 识别响应体里的平台错误码。解析失败或错误码为 0 视为没有平台拒绝。
func externalNotificationResponseError(platform string, responseBody []byte) error {
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil
	}
	codeValue, hasCode := payload["errcode"]
	if !hasCode && platform == "feishu" {
		codeValue, hasCode = payload["code"]
	}
	code := cast.ToInt(codeValue)
	if !hasCode || code == 0 {
		return nil
	}
	return &NotificationDeliveryError{Code: code, Reason: "rejected"}
}

func eventTitle(event AutomationEvent) string {
	if event.After != nil && event.After.Title != "" {
		return event.After.Title
	}
	if event.Before != nil {
		return event.Before.Title
	}
	return ""
}

// notificationMessageLang is the language strategy pushes are rendered in. Dispatch runs after the
// business commit with no request attached and a strategy stores no locale, so there is no
// per-recipient language to resolve yet.
const notificationMessageLang = lib_i18n.LangZhCn

// notificationMessageContext carries the work item context every push copy refers to.
type notificationMessageContext struct {
	WorkspaceName string
	ItemTitle     string
	Link          string
	CreatorName   string
	AssigneeNames string
}

// notificationIntroKeys maps every configurable event to the copy that opens its push.
var notificationIntroKeys = map[string]string{
	"assign_requirement": "external_notification_content_assign_requirement",
	"assign_bug":         "external_notification_content_assign_bug",
	"comment_mention":    "external_notification_content_comment_mention",
	"status_change":      "external_notification_content_status_change",
	"due_reminder":       "external_notification_content_due_reminder",
	"overdue":            "external_notification_content_overdue",
	"test":               "external_notification_content_test",
}

// buildNotificationMessageContext resolves the context of one event. Lookups are best effort: a
// missing workspace or user row leaves that part empty instead of dropping the whole push.
func buildNotificationMessageContext(event AutomationEvent) notificationMessageContext {
	snapshot := event.After
	if snapshot == nil {
		snapshot = event.Before
	}
	var assigneeIDs []int
	creatorID := 0
	if snapshot != nil {
		assigneeIDs = snapshot.AssigneeIDs
		creatorID = cast.ToInt(snapshot.Fields[`created_by`])
	}
	names := notificationUserNames(event.AdminID, append(append([]int{}, assigneeIDs...), creatorID))
	creatorName := ``
	if creatorID > 0 {
		creatorName = names[creatorID]
	}
	assigneeNames := make([]string, 0, len(assigneeIDs))
	for _, id := range assigneeIDs {
		if name := names[id]; name != `` {
			assigneeNames = append(assigneeNames, name)
		}
	}
	return notificationMessageContext{
		WorkspaceName: notificationWorkspaceName(event.AdminID, event.WorkspaceID),
		ItemTitle:     eventTitle(event),
		Link:          externalNotificationItemLink(event.WorkspaceID, event.ItemID),
		CreatorName:   creatorName,
		AssigneeNames: strings.Join(assigneeNames, `, `),
	}
}

// buildExternalNotificationContent renders the plain text body of one push: the event copy, its
// optional context line and the work item facts, each on its own line. Lines whose value is
// empty are dropped so an unconfigured site URL or an unassigned item does not leave a bare label.
func buildExternalNotificationContent(context notificationMessageContext, eventKey string, event AutomationEvent) string {
	lines := []string{lib_i18n.Show(notificationMessageLang, notificationIntroKeys[eventKey])}
	if extra := notificationContextLine(eventKey, event); extra != `` {
		lines = append(lines, extra)
	}
	title := context.ItemTitle
	if context.WorkspaceName != `` {
		title = `【` + context.WorkspaceName + `】` + title
	}
	facts := [][2]string{
		{`external_notification_label_title`, title},
		{`external_notification_label_link`, context.Link},
		{`external_notification_label_creator`, context.CreatorName},
		{`external_notification_label_assignee`, context.AssigneeNames},
	}
	for _, fact := range facts {
		if fact[1] == `` {
			continue
		}
		lines = append(lines, lib_i18n.Show(notificationMessageLang, fact[0], fact[1]))
	}
	// Single newline: a blank line between every fact pushed the message to twice its height.
	return strings.Join(lines, "\n")
}

// notificationContextLine renders the extra line only the reporting events carry.
func notificationContextLine(eventKey string, event AutomationEvent) string {
	if event.After == nil {
		return ``
	}
	switch eventKey {
	case "status_change":
		name := notificationStatusName(event.AdminID, event.WorkspaceID, cast.ToInt(event.After.Fields[`status`]), event.ItemType)
		if name == `` {
			return ``
		}
		return lib_i18n.Show(notificationMessageLang, `external_notification_context_status`, name)
	case "due_reminder":
		// The deadline a warning counts down to is the moment the 时效管控 rule breaks, not a
		// separately edited field: both come from the same rule the worker evaluated.
		if event.SLA == nil || event.SLA.BreachAt <= 0 {
			return ``
		}
		return lib_i18n.Show(notificationMessageLang, `external_notification_context_due_in`, notificationRemainingText(event.SLA.BreachAt-event.TriggeredAt))
	}
	return ``
}

// notificationRemainingText renders how much time is left before a deadline in the largest unit
// that still reads naturally on one line; anything under a minute rounds up to one minute.
func notificationRemainingText(remaining int64) string {
	if remaining < 3600 {
		minutes := remaining / 60
		if minutes < 1 {
			minutes = 1
		}
		return lib_i18n.Show(notificationMessageLang, `external_notification_after_minutes`, minutes)
	}
	if remaining < 86400 {
		return lib_i18n.Show(notificationMessageLang, `external_notification_after_hours`, remaining/3600)
	}
	return lib_i18n.Show(notificationMessageLang, `external_notification_after_days`, remaining/86400)
}

// externalNotificationItemLink builds the work item detail URL from the configured site URL.
func externalNotificationItemLink(workspaceID, itemID int) string {
	base := strings.TrimRight(define.Config.WebService[`site_url`], `/`)
	if base == `` || workspaceID <= 0 || itemID <= 0 {
		return ``
	}
	return fmt.Sprintf(`%s/workspace/%d/work-items/%d`, base, workspaceID, itemID)
}

func notificationWorkspaceName(adminID, workspaceID int) string {
	if workspaceID <= 0 {
		return ``
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT name FROM `+define.TableWorkspaces+` WHERE admin_id=$1 AND id=$2`, nil, adminID, workspaceID)
	if err != nil {
		logs.Error("sql: SELECT name FROM %s, err:%s", define.TableWorkspaces, err.Error())
		return ``
	}
	if len(rows) == 0 {
		return ``
	}
	return rows[0][`name`]
}

// notificationUserNames maps user ids to their display names within the tenant.
func notificationUserNames(adminID int, userIDs []int) map[int]string {
	names := make(map[int]string, len(userIDs))
	unique := make([]int, 0, len(userIDs))
	seen := make(map[int]bool, len(userIDs))
	for _, id := range userIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return names
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id,name FROM `+define.TableUsers+` WHERE admin_id=$1 AND id=ANY($2)`, nil, adminID, pq.Array(unique))
	if err != nil {
		logs.Error("sql: SELECT id,name FROM %s, err:%s", define.TableUsers, err.Error())
		return names
	}
	for _, row := range rows {
		names[cast.ToInt(row[`id`])] = row[`name`]
	}
	return names
}

// notificationStatusName resolves a status id to its stored name, translating i18n keys the seed
// writes for preset statuses.
func notificationStatusName(adminID, workspaceID, statusID int, typeKey string) string {
	if statusID <= 0 || workspaceID <= 0 || typeKey == `` {
		return ``
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT name FROM `+define.TableWorkItemStatuses+`
		WHERE admin_id=$1 AND workspace_id=$2 AND type_key=$3 AND id=$4`, nil, adminID, workspaceID, typeKey, statusID)
	if err != nil {
		logs.Error("sql: SELECT name FROM %s, err:%s", define.TableWorkItemStatuses, err.Error())
		return ``
	}
	if len(rows) == 0 {
		return ``
	}
	return lib_i18n.ShowStored(notificationMessageLang, rows[0][`name`])
}

var (
	ErrNotificationStrategyNotFound = errors.New("notification strategy not found")
	ErrNotificationStrategyMembers  = errors.New("notification strategy members invalid")
)

// NotificationStrategyModel is the persisted tenant level notification strategy.
type NotificationStrategyModel struct {
	Id                int                          `json:"id"`
	AdminId           int                          `json:"admin_id"`
	Name              string                       `json:"name"`
	Platform          string                       `json:"platform"`
	WebhookURL        string                       `json:"webhook_url"`
	Secret            string                       `json:"secret,omitempty"`
	Events            []string                     `json:"events"`
	AtMode            string                       `json:"at_mode"`
	AtUserIds         []int                        `json:"at_user_ids"`
	AtRoles           []string                     `json:"at_roles"`
	AtAll             bool                         `json:"at_all"`
	Enabled           int                          `json:"enabled"`
	MemberIds         []int                        `json:"member_ids"`
	Members           []NotificationStrategyMember `json:"members"`
	TodayTriggerCount int                          `json:"today_trigger_count"`
	TotalTriggerCount int                          `json:"total_trigger_count"`
	CreatedBy         int                          `json:"created_by"`
	CreatedAt         int64                        `json:"created_at"`
	UpdatedAt         int64                        `json:"updated_at"`
}

// NotificationStrategyMember is a member displayed in a strategy's avatar group.
type NotificationStrategyMember struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// ListNotificationStrategiesModel describes a tenant strategy list query.
type ListNotificationStrategiesModel struct {
	AdminId int
}

// SaveNotificationStrategyModel describes a create or update request.
type SaveNotificationStrategyModel struct {
	AdminId    int
	StrategyId int
	UserId     int
	Name       string
	Platform   string
	WebhookURL string
	Secret     string
	Events     []string
	MemberIds  []int
	AtMode     string
	AtUserIds  []int
	AtRoles    []string
	AtAll      bool
}

// DeleteNotificationStrategyModel describes a scoped strategy deletion.
type DeleteNotificationStrategyModel struct {
	AdminId    int
	StrategyId int
}

// TestNotificationStrategyModel describes a scoped test push request.
type TestNotificationStrategyModel struct {
	AdminId    int
	StrategyId int
}

// TestNotificationStrategy sends a test message using the persisted strategy configuration.
func TestNotificationStrategy(model TestNotificationStrategyModel) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT platform, webhook_url, secret, at_mode, at_user_ids::text, at_roles::text, at_all
		FROM `+define.TableExternalNotificationStrategies+`
		WHERE admin_id=$1 AND id=$2`, nil, model.AdminId, model.StrategyId)
	if err != nil {
		logs.Error("load external notification strategy for test failed: %s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrNotificationStrategyNotFound
	}
	at := notificationAtConfig{AtMode: defaultNotificationAtMode(rows[0]["platform"])}
	if rows[0]["at_mode"] != "" {
		at.AtMode = rows[0]["at_mode"]
	}
	_ = json.Unmarshal([]byte(rows[0]["at_user_ids"]), &at.AtUserIds)
	_ = json.Unmarshal([]byte(rows[0]["at_roles"]), &at.AtRoles)
	at.AtAll = rows[0]["at_all"] == "true"
	event := AutomationEvent{
		AdminID:     model.AdminId,
		Operation:   "test",
		TriggeredAt: int64(tool.Time2Int()),
		After: &AutomationWorkItemSnapshot{
			Type:  define.TypeKeyRequirement,
			Title: "外部系统通知测试",
		},
	}
	return sendExternalNotificationWebhook(rows[0]["platform"], rows[0]["webhook_url"], rows[0]["secret"], at,
		buildExternalNotificationContent(buildNotificationMessageContext(event), "test", event), event)
}

// ListNotificationStrategies returns every strategy of the tenant.
func ListNotificationStrategies(model ListNotificationStrategiesModel) ([]NotificationStrategyModel, error) {
	// today_trigger_count is only meaningful for the day stored beside it: without a scheduler to
	// reset it, a strategy that stopped pushing still holds yesterday's count.
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name, platform, webhook_url, secret, events::text,
        at_mode, at_user_ids::text, at_roles::text, at_all, enabled,
        CASE WHEN today_trigger_date=$2 THEN today_trigger_count ELSE 0 END AS today_trigger_count,
        total_trigger_count, created_by, created_at, updated_at
        FROM `+define.TableExternalNotificationStrategies+` WHERE admin_id=$1 ORDER BY id DESC`, nil, model.AdminId, notificationTodayStart())
	if err != nil {
		logs.Error("list notification strategies error: %s", err.Error())
		return nil, err
	}
	result := make([]NotificationStrategyModel, 0, len(rows))
	for _, row := range rows {
		item, err := notificationStrategyFromRow(row, model.AdminId)
		if err != nil {
			return nil, err
		}
		members, err := notificationStrategyMembers(model.AdminId, item.Id)
		if err != nil {
			return nil, err
		}
		item.Members = members
		item.MemberIds = make([]int, 0, len(members))
		for _, member := range members {
			item.MemberIds = append(item.MemberIds, member.Id)
		}
		result = append(result, item)
	}
	return result, nil
}

func notificationStrategyFromRow(row msql.Params, adminId int) (NotificationStrategyModel, error) {
	var events []string
	if raw := strings.TrimSpace(row[`events`]); raw != `` {
		if err := json.Unmarshal([]byte(raw), &events); err != nil {
			return NotificationStrategyModel{}, fmt.Errorf("parse notification events: %w", err)
		}
	}
	atMode := defaultNotificationAtMode(row[`platform`])
	if row[`at_mode`] != "" {
		atMode = row[`at_mode`]
	}
	var atUserIds []int
	if raw := strings.TrimSpace(row[`at_user_ids`]); raw != `` {
		if err := json.Unmarshal([]byte(raw), &atUserIds); err != nil {
			return NotificationStrategyModel{}, fmt.Errorf("parse notification at user ids: %w", err)
		}
	}
	var atRoles []string
	if raw := strings.TrimSpace(row[`at_roles`]); raw != `` {
		if err := json.Unmarshal([]byte(raw), &atRoles); err != nil {
			return NotificationStrategyModel{}, fmt.Errorf("parse notification at roles: %w", err)
		}
	}
	return NotificationStrategyModel{Id: cast.ToInt(row[`id`]), AdminId: adminId, Name: row[`name`], Platform: row[`platform`], WebhookURL: row[`webhook_url`], Secret: row[`secret`], Events: events, AtMode: atMode, AtUserIds: atUserIds, AtRoles: atRoles, AtAll: row[`at_all`] == "true", Enabled: cast.ToInt(row[`enabled`]), TodayTriggerCount: cast.ToInt(row[`today_trigger_count`]), TotalTriggerCount: cast.ToInt(row[`total_trigger_count`]), CreatedBy: cast.ToInt(row[`created_by`]), CreatedAt: cast.ToInt64(row[`created_at`]), UpdatedAt: cast.ToInt64(row[`updated_at`])}, nil
}

func notificationStrategyMembers(adminId, strategyId int) ([]NotificationStrategyMember, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT u.id, u.name, u.avatar FROM `+define.TableExternalNotificationStrategyMembers+` sm JOIN users u ON u.id=sm.user_id AND u.admin_id=sm.admin_id WHERE sm.admin_id=$1 AND sm.strategy_id=$2 ORDER BY sm.id`, nil, adminId, strategyId)
	if err != nil {
		logs.Error("list notification strategy members error: %s", err.Error())
		return nil, err
	}
	result := make([]NotificationStrategyMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, NotificationStrategyMember{Id: cast.ToInt(row[`id`]), Name: row[`name`], Avatar: row[`avatar`]})
	}
	return result, nil
}

func validateNotificationMembers(adminId int, memberIds []int) error {
	seen := map[int]bool{}
	unique := make([]int, 0, len(memberIds))
	for _, id := range memberIds {
		if id > 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return ErrNotificationStrategyMembers
	}
	// Strategy members are enterprise users, not project members. Keep tenant
	// isolation and active-account validation while allowing users without a
	// membership row in any project to receive strategy notifications.
	rows, err := msql.RawValues(define.Postgres, `SELECT id AS user_id FROM users
		WHERE admin_id=$1 AND id=ANY($2) AND deleted_at=0 AND COALESCE(status,1)=1`, nil, adminId, pq.Array(unique))
	if err != nil {
		logs.Error("validate notification strategy members error: %s", err.Error())
		return err
	}
	if len(rows) != len(unique) {
		return ErrNotificationStrategyMembers
	}
	return nil
}

// CreateNotificationStrategy creates a strategy and its member links atomically.
func CreateNotificationStrategy(model SaveNotificationStrategyModel) (NotificationStrategyModel, error) {
	atMode := model.AtMode
	if atMode == "" {
		atMode = defaultNotificationAtMode(model.Platform)
	}
	if err := validate.ValidateExternalNotificationStrategy(define.ExternalNotificationStrategyValidation{Name: model.Name, Platform: model.Platform, WebhookURL: model.WebhookURL, MemberIDs: model.MemberIds, Events: model.Events, AtMode: atMode, AtRoles: model.AtRoles}); err != nil {
		return NotificationStrategyModel{}, err
	}
	if err := validateNotificationMembers(model.AdminId, model.MemberIds); err != nil {
		return NotificationStrategyModel{}, err
	}
	if len(model.AtUserIds) > 0 {
		if err := validateNotificationMembers(model.AdminId, model.AtUserIds); err != nil {
			return NotificationStrategyModel{}, err
		}
	}
	events, _ := json.Marshal(model.Events)
	atUserIds, _ := json.Marshal(model.AtUserIds)
	atRoles, _ := json.Marshal(model.AtRoles)
	now := tool.Time2Int()
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return NotificationStrategyModel{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableExternalNotificationStrategies+` (admin_id,name,platform,webhook_url,secret,events,at_mode,at_user_ids,at_roles,at_all,enabled,created_by,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8::jsonb,$9::jsonb,$10,1,$11,$12,$12) RETURNING id`, tx, model.AdminId, strings.TrimSpace(model.Name), model.Platform, strings.TrimSpace(model.WebhookURL), model.Secret, string(events), atMode, string(atUserIds), string(atRoles), model.AtAll, model.UserId, now)
	if err != nil {
		return NotificationStrategyModel{}, err
	}
	strategyId := cast.ToInt(rows[0][`id`])
	for _, memberId := range model.MemberIds {
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableExternalNotificationStrategyMembers+` (admin_id,strategy_id,user_id,created_at) VALUES ($1,$2,$3,$4)`, tx, model.AdminId, strategyId, memberId, now); err != nil {
			return NotificationStrategyModel{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return NotificationStrategyModel{}, err
	}
	model.StrategyId = strategyId
	items, err := ListNotificationStrategies(ListNotificationStrategiesModel{AdminId: model.AdminId})
	if err != nil {
		return NotificationStrategyModel{}, err
	}
	for _, item := range items {
		if item.Id == strategyId {
			return item, nil
		}
	}
	return NotificationStrategyModel{}, ErrNotificationStrategyNotFound
}

// SetNotificationStrategyEnabled toggles whether a strategy receives event pushes.
func SetNotificationStrategyEnabled(adminId, strategyId, enabled int) error {
	if enabled != 0 {
		enabled = 1
	}
	result, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableExternalNotificationStrategies+` SET enabled=$1, updated_at=$2 WHERE admin_id=$3 AND id=$4`, nil, enabled, tool.Time2Int(), adminId, strategyId)
	if err != nil {
		logs.Error("set notification strategy enabled error: %s", err.Error())
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotificationStrategyNotFound
	}
	return nil
}

// UpdateNotificationStrategy updates a strategy and replaces its member links atomically.
func UpdateNotificationStrategy(model SaveNotificationStrategyModel) error {
	atMode := model.AtMode
	if atMode == "" {
		atMode = defaultNotificationAtMode(model.Platform)
	}
	if err := validate.ValidateExternalNotificationStrategy(define.ExternalNotificationStrategyValidation{Name: model.Name, Platform: model.Platform, WebhookURL: model.WebhookURL, MemberIDs: model.MemberIds, Events: model.Events, AtMode: atMode, AtRoles: model.AtRoles}); err != nil {
		return err
	}
	if err := validateNotificationMembers(model.AdminId, model.MemberIds); err != nil {
		return err
	}
	if len(model.AtUserIds) > 0 {
		if err := validateNotificationMembers(model.AdminId, model.AtUserIds); err != nil {
			return err
		}
	}
	events, _ := json.Marshal(model.Events)
	atUserIds, _ := json.Marshal(model.AtUserIds)
	atRoles, _ := json.Marshal(model.AtRoles)
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	now := tool.Time2Int()
	result, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableExternalNotificationStrategies+` SET name=$1,platform=$2,webhook_url=$3,secret=$4,events=$5::jsonb,at_mode=$6,at_user_ids=$7::jsonb,at_roles=$8::jsonb,at_all=$9,updated_at=$10 WHERE admin_id=$11 AND id=$12`, tx, strings.TrimSpace(model.Name), model.Platform, strings.TrimSpace(model.WebhookURL), model.Secret, string(events), atMode, string(atUserIds), string(atRoles), model.AtAll, now, model.AdminId, model.StrategyId)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotificationStrategyNotFound
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableExternalNotificationStrategyMembers+` WHERE admin_id=$1 AND strategy_id=$2`, tx, model.AdminId, model.StrategyId); err != nil {
		return err
	}
	for _, memberId := range model.MemberIds {
		if _, err = msql.RawExec(define.Postgres, `INSERT INTO `+define.TableExternalNotificationStrategyMembers+` (admin_id,strategy_id,user_id,created_at) VALUES ($1,$2,$3,$4)`, tx, model.AdminId, model.StrategyId, memberId, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteNotificationStrategy deletes a strategy and its member links inside the tenant scope.
func DeleteNotificationStrategy(model DeleteNotificationStrategyModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableExternalNotificationStrategyMembers+` WHERE admin_id=$1 AND strategy_id=$2`, tx, model.AdminId, model.StrategyId); err != nil {
		return err
	}
	result, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableExternalNotificationStrategies+` WHERE admin_id=$1 AND id=$2`, tx, model.AdminId, model.StrategyId)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotificationStrategyNotFound
	}
	return tx.Commit()
}
