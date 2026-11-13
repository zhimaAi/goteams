package business

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// NotificationModel is the API-facing notification projection.
type NotificationModel struct {
	PriorityName             string
	PriorityColor            string
	CreatorName              string
	AssigneeNames            string
	ID                       int
	EventType                string
	Content                  string
	ActorUserID              int
	ActorName                string
	ActorAvatar              string
	WorkspaceID              int
	WorkspaceName            string
	WorkspaceIcon            string
	RequirementID            int
	RequirementType          string
	RequirementTypeIconText  string
	RequirementTypeIconColor string
	RequirementTitle         string
	FieldName                string
	Read                     bool
	CreatedAt                int64
}

// NotificationListModel contains a paginated notification result.
type NotificationListModel struct {
	Items       []NotificationModel
	Total       int
	UnreadTotal int
	Page        int
	Size        int
}

type notificationEvent struct {
	AdminID       int
	RecipientID   int
	WorkspaceID   int
	ItemID        int
	EventType     string
	ActorID       int
	FieldName     string
	Content       string
	StatusName    string
	AssigneeNames []string
}

func newAssigneeNotificationRecipients(actorID int, newIDs, oldIDs []int) []int {
	oldSet := make(map[int]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}
	seen := make(map[int]bool, len(newIDs))
	recipients := make([]int, 0, len(newIDs))
	for _, id := range newIDs {
		if id <= 0 || id == actorID || oldSet[id] || seen[id] {
			continue
		}
		seen[id] = true
		recipients = append(recipients, id)
	}
	return recipients
}

// CreateNotificationTx inserts one notification in the caller's transaction.
func CreateNotificationTx(tx *sql.Tx, event notificationEvent) error {
	if event.AdminID <= 0 || event.RecipientID <= 0 || event.WorkspaceID <= 0 || event.ItemID <= 0 || event.EventType == `` {
		return nil
	}
	_, err := msql.RawExec(define.Postgres, `INSERT INTO notifications
		(admin_id, recipient_user_id, workspace_id, requirement_id, event_type, actor_user_id, field_name, content, read_at, created_at)
		SELECT r.admin_id, $1, r.workspace_id, r.id, $2, $3, $4, $5, 0, $6
		FROM requirements r WHERE r.admin_id = $7 AND r.workspace_id = $8 AND r.id = $9`, tx,
		event.RecipientID, event.EventType, event.ActorID, event.FieldName, event.Content, tool.Time2Int(), event.AdminID, event.WorkspaceID, event.ItemID)
	return err
}

// CreateWorkspaceNotificationTx inserts a project-level notification without a work item.
func CreateWorkspaceNotificationTx(tx *sql.Tx, event notificationEvent) error {
	if event.AdminID <= 0 || event.RecipientID <= 0 || event.WorkspaceID <= 0 || event.EventType == `` {
		return errors.New(`invalid workspace notification`)
	}
	const query = `INSERT INTO ` + define.TableNotifications + `
		(admin_id, recipient_user_id, workspace_id, requirement_id, event_type, actor_user_id, field_name, content, read_at, created_at)
		SELECT w.admin_id, $1, w.id, NULL, $2, $3, '', $4, 0, $5
		FROM ` + define.TableWorkspaces + ` w WHERE w.admin_id = $6 AND w.id = $7`
	_, err := msql.RawExec(define.Postgres, query, tx,
		event.RecipientID, event.EventType, event.ActorID, event.Content, tool.Time2Int(), event.AdminID, event.WorkspaceID)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return err
}

// ListNotifications returns notifications for one tenant user across all workspaces.
func ListNotifications(adminID, userID int, unreadOnly bool, page, size int) (NotificationListModel, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size
	where := `n.admin_id = $1 AND n.recipient_user_id = $2`
	args := []any{adminID, userID}
	if unreadOnly {
		where += ` AND n.read_at = 0`
	}
	count, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM notifications n WHERE `+where, nil, args...)
	if err != nil {
		return NotificationListModel{}, err
	}
	unread, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM notifications WHERE admin_id = $1 AND recipient_user_id = $2 AND read_at = 0`, nil, adminID, userID)
	if err != nil {
		return NotificationListModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT n.id, n.event_type, n.content, n.actor_user_id, n.field_name, n.workspace_id,
		COALESCE(w.name, '') AS workspace_name, COALESCE(w.icon, '') AS workspace_icon, n.requirement_id, COALESCE(r.type, '') AS requirement_type,
		COALESCE(a.icon_text, '') AS requirement_type_icon_text, COALESCE(a.icon_color, '') AS requirement_type_icon_color,
		COALESCE(p.name, '') AS priority_name, COALESCE(p.color, '') AS priority_color,
		COALESCE(NULLIF(creator.name, ''), creator.username, removed_creator.name, '') AS creator_name,
		COALESCE(assignees.names, '') AS assignee_names, COALESCE(r.title, '') AS requirement_title, COALESCE(NULLIF(u.name, ''), u.username, '') AS actor_name,
		COALESCE(u.avatar, '') AS actor_avatar, n.read_at, n.created_at
		FROM notifications n LEFT JOIN workspaces w ON w.admin_id = n.admin_id AND w.id = n.workspace_id
		LEFT JOIN requirements r ON r.admin_id = n.admin_id AND r.workspace_id = n.workspace_id AND r.id = n.requirement_id
		LEFT JOIN workspace_priorities p ON p.admin_id = r.admin_id AND p.workspace_id = r.workspace_id AND p.type_key = r.type AND p.id = r.priority
		LEFT JOIN users creator ON creator.admin_id = r.admin_id AND creator.id = r.created_by
		LEFT JOIN removed_users removed_creator ON removed_creator.admin_id = r.admin_id AND removed_creator.user_id = r.created_by
		LEFT JOIN LATERAL (
			SELECT string_agg(COALESCE(NULLIF(assigned_user.name, ''), assigned_user.username, removed_user.name, ''), ', ' ORDER BY assigned.ordinality) AS names
			FROM unnest(r.assignee_ids) WITH ORDINALITY AS assigned(user_id, ordinality)
			LEFT JOIN users assigned_user ON assigned_user.admin_id = r.admin_id AND assigned_user.id = assigned.user_id
			LEFT JOIN removed_users removed_user ON removed_user.admin_id = r.admin_id AND removed_user.user_id = assigned.user_id
		) assignees ON true
		LEFT JOIN workspace_work_item_attributes a ON a.admin_id = n.admin_id AND a.workspace_id = n.workspace_id AND a.type_key = r.type
		LEFT JOIN users u ON u.admin_id = n.admin_id AND u.id = n.actor_user_id
		WHERE `+where+` ORDER BY n.created_at DESC, n.id DESC LIMIT $3 OFFSET $4`, nil, append(args, size, offset)...)
	if err != nil {
		return NotificationListModel{}, err
	}
	items := make([]NotificationModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, NotificationModel{PriorityName: row[`priority_name`], PriorityColor: row[`priority_color`], CreatorName: row[`creator_name`], AssigneeNames: row[`assignee_names`], ID: cast.ToInt(row[`id`]), EventType: row[`event_type`], Content: row[`content`], ActorUserID: cast.ToInt(row[`actor_user_id`]), ActorName: row[`actor_name`], ActorAvatar: row[`actor_avatar`], WorkspaceID: cast.ToInt(row[`workspace_id`]), WorkspaceName: row[`workspace_name`], WorkspaceIcon: row[`workspace_icon`], RequirementID: cast.ToInt(row[`requirement_id`]), RequirementType: row[`requirement_type`], RequirementTypeIconText: row[`requirement_type_icon_text`], RequirementTypeIconColor: row[`requirement_type_icon_color`], RequirementTitle: row[`requirement_title`], FieldName: row[`field_name`], Read: cast.ToInt64(row[`read_at`]) > 0, CreatedAt: cast.ToInt64(row[`created_at`])})
	}
	return NotificationListModel{Items: items, Total: cast.ToInt(count[0][`total`]), UnreadTotal: cast.ToInt(unread[0][`total`]), Page: page, Size: size}, nil
}

// MarkNotificationsRead marks selected notifications for the current user.
func MarkNotificationsRead(adminID, userID int, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := msql.RawExec(define.Postgres, `UPDATE notifications SET read_at = $1 WHERE admin_id = $2 AND recipient_user_id = $3 AND id = ANY($4::INTEGER[]) AND read_at = 0`, nil, tool.Time2Int(), adminID, userID, pq.Array(ids))
	return err
}

// MarkAllNotificationsRead marks all notifications for the current user.
func MarkAllNotificationsRead(adminID, userID int) error {
	_, err := msql.RawExec(define.Postgres, `UPDATE notifications SET read_at = $1 WHERE admin_id = $2 AND recipient_user_id = $3 AND read_at = 0`, nil, tool.Time2Int(), adminID, userID)
	return err
}

// CountUnreadNotifications returns the current user's unread total across workspaces.
func CountUnreadNotifications(adminID, userID int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT COUNT(*) AS total FROM notifications WHERE admin_id = $1 AND recipient_user_id = $2 AND read_at = 0`, nil, adminID, userID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`total`]), nil
}

// DeleteNotificationsByItems removes notifications whose work items were deleted.
func DeleteNotificationsByItems(tx *sql.Tx, adminID, workspaceID int, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	_, err := msql.RawExec(define.Postgres, `DELETE FROM notifications WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = ANY($3::BIGINT[])`, tx, adminID, workspaceID, pq.Array(itemIDs))
	return err
}

// NotificationMentionRecipients resolves @username/@display-name mentions in comment text.
func NotificationMentionRecipients(adminID, workspaceID, actorID int, content string) ([]int, error) {
	plain := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(content, " ")
	plain = html.UnescapeString(plain)
	rows, err := msql.RawValues(define.Postgres, `SELECT u.id, u.username, u.name FROM users u JOIN workspace_members wm ON wm.admin_id = u.admin_id AND wm.workspace_id = $1 AND wm.user_id = u.id WHERE u.admin_id = $2 AND u.status = 1`, nil, workspaceID, adminID)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	ids := make([]int, 0)
	for _, row := range rows {
		id := cast.ToInt(row[`id`])
		if id <= 0 || id == actorID || seen[id] {
			continue
		}
		for _, label := range []string{row[`username`], row[`name`]} {
			label = strings.TrimSpace(label)
			if label != `` && mentionExists(plain, label) {
				seen[id] = true
				ids = append(ids, id)
				break
			}
		}
	}
	return ids, nil
}

func mentionExists(content, label string) bool {
	pattern := regexp.MustCompile(`(^|[^\p{L}\p{N}_])@` + regexp.QuoteMeta(label) + `([^\p{L}\p{N}_]|$)`)
	return pattern.MatchString(content)
}

// notificationItemType 把工作项类型 key 转成通知文案里用的显示名（需求/缺陷）。
func notificationItemType(typeKey string) string {
	if typeKey == define.TypeKeyDefect {
		return define.TypeNameDefect
	}
	return define.TypeNameRequirement
}

func notificationContent(event notificationEvent, actor, itemType string) string {
	fieldLabels := map[string]string{
		`status`: `状态`, `priority`: `优先级`, `assignee`: `处理人`, `title`: `标题`, `description`: `描述`,
		`planned_end_date`: `截止日期`, `planned_start_date`: `开始日期`, `version_id`: `所属迭代`, `parent_id`: `父级工作项`,
		`work_hours`: `工时`,
	}
	field := event.FieldName
	if label, ok := fieldLabels[field]; ok {
		field = label
	}
	switch event.EventType {
	case define.NotificationEventItemUpdated:
		if event.FieldName == `status` && event.StatusName != `` {
			return fmt.Sprintf(`%s 更新了您的%s，状态更新为%s`, actor, itemType, lib_i18n.ShowStored(lib_i18n.LangZhCn, event.StatusName))
		}
		if event.FieldName == `assignee` && len(event.AssigneeNames) > 0 {
			return fmt.Sprintf(`%s 更新了您的%s，处理人更新为%s`, actor, itemType, strings.Join(event.AssigneeNames, `、`))
		}
		return fmt.Sprintf(`%s 更新了您的%s，更新字段为%s`, actor, itemType, field)
	case define.NotificationEventItemAssigned:
		return fmt.Sprintf(`%s 给您分配了一个%s`, actor, itemType)
	case define.NotificationEventMention:
		return fmt.Sprintf(`给%s添加了一条评论并@了你`, itemType)
	case define.NotificationEventAssigneeCommented:
		return fmt.Sprintf(`%s 评论了您处理的%s`, actor, itemType)
	case define.NotificationEventIssueInviteUrged:
		return `用户催单，请加急处理`
	default:
		return fmt.Sprintf(`%s 回复了您的评论`, actor)
	}
}

func notificationAssigneeNames(adminID int, assigneeIDs []int) []string {
	if len(assigneeIDs) == 0 {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(NULLIF(u.name, ''), u.username, '') AS display_name
		FROM unnest($2::INTEGER[]) WITH ORDINALITY AS ids(id, ordinality)
		LEFT JOIN users u ON u.id = ids.id AND u.admin_id = $1
		ORDER BY ids.ordinality`, nil, adminID, pq.Array(assigneeIDs))
	if err != nil {
		logNotificationError(`load notification assignee names`, err)
		return nil
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if name := row[`display_name`]; name != `` {
			names = append(names, name)
		}
	}
	return names
}

func notificationActorName(adminID, userID int) string {
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(NULLIF(name, ''), username, '') AS actor_name FROM users WHERE admin_id = $1 AND id = $2`, nil, adminID, userID)
	if err != nil || len(rows) == 0 {
		logNotificationError(`load notification actor`, err)
		return ``
	}
	return rows[0][`actor_name`]
}

func logNotificationError(context string, err error) {
	if err != nil {
		logs.Error(`%s, err:%s`, context, err.Error())
	}
}
