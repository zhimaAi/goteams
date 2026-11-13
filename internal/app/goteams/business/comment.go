// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrCommentNotFound means the comment does not exist or does not belong to the current work item.
	ErrCommentNotFound = errors.New(`comment not found`)
	// ErrCommentOwnerRequired means the current user is not the comment author.
	ErrCommentOwnerRequired = errors.New(`comment owner required`)
	// ErrCommentAttachmentsInvalid indicates that staged attachments cannot be bound to the comment.
	ErrCommentAttachmentsInvalid = errors.New(`comment attachments invalid`)
	// ErrCommentParentInvalid means the reply target does not exist in the same work item.
	ErrCommentParentInvalid = errors.New(`comment parent invalid`)
	// ErrCommentPublicUnavailable 表示工作项没有可接收公开回复的邀填人。
	ErrCommentPublicUnavailable = errors.New(`public comment requires an invitation submitter`)
)

// createCommentTx creates a user comment and updates the comment count in a transaction.
func bindInlineCommentFilesTx(tx *sql.Tx, adminID, workspaceID, requirementID, userID, commentID int, fileIDs []int) error {
	if len(fileIDs) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		if fileID <= 0 {
			return ErrCommentAttachmentsInvalid
		}
		if _, exists := seen[fileID]; exists {
			return ErrCommentAttachmentsInvalid
		}
		seen[fileID] = struct{}{}
	}
	files, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.GetEnterpriseFileTable(adminID)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = 0 AND uploader_id = $3
		AND comment_id = 0 AND id = ANY($4::INTEGER[]) FOR UPDATE`, tx,
		adminID, workspaceID, userID, pq.Array(fileIDs))
	if err != nil {
		return err
	}
	if len(files) != len(fileIDs) {
		return ErrCommentAttachmentsInvalid
	}
	row, err := msql.RawValues(define.Postgres, `SELECT type, title FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, tx,
		adminID, workspaceID, requirementID)
	if err != nil {
		return err
	}
	if len(row) == 0 {
		return ErrCommentAttachmentsInvalid
	}
	_, err = msql.RawExec(define.Postgres, `UPDATE `+define.GetEnterpriseFileTable(adminID)+`
		SET source_id = $1, source_type = $2, source_name = $3, comment_id = $4, updated_at = $5
		WHERE admin_id = $6 AND workspace_id = $7 AND source_id = 0 AND uploader_id = $8
		AND comment_id = 0 AND id = ANY($9::INTEGER[])`, tx,
		requirementID, row[0][`type`], row[0][`title`], commentID, tool.Time2Int(),
		adminID, workspaceID, userID, pq.Array(fileIDs))
	return err
}

// createCommentTx 在同一事务中保存明确的公开范围并更新评论总数。
func createCommentTx(tx *sql.Tx, model CreateCommentModel) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRequirementComments+`
		(admin_id, workspace_id, requirement_id, user_id, content, source, parent_id, at_invite, is_public, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'user',$6,$7,$8,$9,$9) RETURNING id`, tx,
		model.AdminId, model.WorkspaceId, model.RequirementId, model.UserId, model.Content,
		model.ParentId, cast.ToInt(model.AtInvite), model.IsPublic, tool.Time2Int())
	if err != nil {
		return 0, err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET comment_count = comment_count + 1,
		updated_at = $1 WHERE admin_id = $2 AND workspace_id = $3 AND id = $4`, tx,
		tool.Time2Int(), model.AdminId, model.WorkspaceId, model.RequirementId); err != nil {
		return 0, err
	}
	return cast.ToInt(rows[0][`id`]), nil
}

// ensureCommentParentValid rejects a reply whose target comment is not in the same work item.
func ensureCommentParentValid(tx *sql.Tx, model CreateCommentModel) error {
	if model.ParentId <= 0 {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, is_public FROM `+define.TableRequirementComments+`
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND id = $4`, tx,
		model.AdminId, model.WorkspaceId, model.RequirementId, model.ParentId)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrCommentParentInvalid
	}
	public, err := strconv.ParseBool(rows[0][`is_public`])
	if err != nil {
		return fmt.Errorf("parse parent comment visibility: %w", err)
	}
	// 公开回复不能引用内部讨论，内部回复也不混入公开会话。
	if public != model.IsPublic {
		return ErrCommentParentInvalid
	}
	return nil
}

// notifyCommentMentionRecipientsTx notifies every given recipient once with the mention event.
// Comment creation passes all @ recipients; editing passes only the newly added ones.
func notifyCommentMentionRecipientsTx(tx *sql.Tx, adminID, workspaceID, requirementID, userID int, actorName, itemType string, recipientIDs []int) error {
	for _, recipientID := range recipientIDs {
		event := notificationEvent{AdminID: adminID, RecipientID: recipientID, WorkspaceID: workspaceID, ItemID: requirementID, EventType: define.NotificationEventMention, ActorID: userID}
		event.Content = notificationContent(event, actorName, itemType)
		if err := CreateNotificationTx(tx, event); err != nil {
			return err
		}
	}
	return nil
}

// createCommentNotificationsTx notifies the mentioned users and the assignees once per recipient.
// Commenting no longer notifies every previous commenter; only an explicit @ mention does.
func createCommentNotificationsTx(tx *sql.Tx, adminID, workspaceID, requirementID, userID int, content string) error {
	itemRows, err := msql.RawValues(define.Postgres, `SELECT type, assignee_ids::text AS assignee_ids FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, tx, adminID, workspaceID, requirementID)
	if err != nil {
		return err
	}
	if len(itemRows) == 0 {
		return ErrCommentNotFound
	}
	actorName := notificationActorName(adminID, userID)
	itemType := notificationItemType(itemRows[0][`type`])
	notified := map[int]bool{userID: true}
	mentionRecipients, err := NotificationMentionRecipients(adminID, workspaceID, userID, content)
	if err != nil {
		return err
	}
	if err = notifyCommentMentionRecipientsTx(tx, adminID, workspaceID, requirementID, userID, actorName, itemType, mentionRecipients); err != nil {
		return err
	}
	for _, recipientID := range mentionRecipients {
		notified[recipientID] = true
	}
	for _, recipientID := range parseStoredAssigneeIDs(itemRows[0][`assignee_ids`]) {
		if notified[recipientID] {
			continue
		}
		event := notificationEvent{AdminID: adminID, RecipientID: recipientID, WorkspaceID: workspaceID, ItemID: requirementID, EventType: define.NotificationEventAssigneeCommented, ActorID: userID}
		event.Content = notificationContent(event, actorName, itemType)
		if err := CreateNotificationTx(tx, event); err != nil {
			return err
		}
		notified[recipientID] = true
	}
	return nil
}

// notifyEditedCommentMentionsTx notifies only the @ recipients this edit added, i.e. the mentions
// of the new content minus the ones of the stored content. Saving the same edit again therefore
// never notifies twice, and dropping a mention does not retract a notification already sent.
func notifyEditedCommentMentionsTx(tx *sql.Tx, adminID, workspaceID, requirementID, userID int, previousContent, content string) error {
	previous, err := NotificationMentionRecipients(adminID, workspaceID, userID, previousContent)
	if err != nil {
		return err
	}
	current, err := NotificationMentionRecipients(adminID, workspaceID, userID, content)
	if err != nil {
		return err
	}
	alreadyNotified := make(map[int]bool, len(previous))
	for _, recipientID := range previous {
		alreadyNotified[recipientID] = true
	}
	added := make([]int, 0, len(current))
	for _, recipientID := range current {
		if !alreadyNotified[recipientID] {
			added = append(added, recipientID)
		}
	}
	if len(added) == 0 {
		return nil
	}
	itemRows, err := msql.RawValues(define.Postgres, `SELECT type FROM requirements
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, tx, adminID, workspaceID, requirementID)
	if err != nil {
		return err
	}
	if len(itemRows) == 0 {
		return ErrCommentNotFound
	}
	return notifyCommentMentionRecipientsTx(tx, adminID, workspaceID, requirementID, userID,
		notificationActorName(adminID, userID), notificationItemType(itemRows[0][`type`]), added)
}

// CommentModel 是后台评论列表的固定业务投影。
type CommentModel struct {
	Id, AdminId, WorkspaceId, RequirementId, UserId, ParentId, AtInvite, AutomationRuleId int
	AutomationExecutionId, CreatedAt, UpdatedAt                                           int64
	Content, Source, AuthorNameSnapshot, AuthorAvatarSnapshot, UserName, UserAvatar       string
	IsPublic                                                                              bool
}

// ListCommentsModel 限定后台评论列表的租户和工作项范围。
type ListCommentsModel struct {
	AdminId, WorkspaceId, RequirementId int
}

// ListCommentsByRequirement 返回工作项的全部评论，供后台按可见性分类。
func ListCommentsByRequirement(model ListCommentsModel) ([]CommentModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT c.id, c.admin_id, c.workspace_id, c.requirement_id,
  c.user_id, c.content, c.source, c.parent_id, c.at_invite, c.is_public,
  c.automation_rule_id, c.automation_execution_id, c.author_name_snapshot, c.author_avatar_snapshot,
  c.created_at, c.updated_at, COALESCE(u.name, c.author_name_snapshot) AS user_name,
  COALESCE(NULLIF(u.avatar,''), c.author_avatar_snapshot) AS user_avatar
  FROM `+define.TableRequirementComments+` c LEFT JOIN users u ON u.id = c.user_id AND u.admin_id=c.admin_id
  WHERE c.admin_id = $1 AND c.workspace_id = $2 AND c.requirement_id = $3
  ORDER BY c.id DESC`, nil, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		logs.Error("list requirement comments failed, err:%s", err.Error())
		return nil, err
	}
	comments := make([]CommentModel, 0, len(rows))
	for _, row := range rows {
		comment, err := commentModelFromRow(row)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, nil
}

func commentModelFromRow(row msql.Params) (CommentModel, error) {
	model := CommentModel{Content: row[`content`], Source: row[`source`],
		AuthorNameSnapshot: row[`author_name_snapshot`], AuthorAvatarSnapshot: row[`author_avatar_snapshot`],
		UserName: row[`user_name`], UserAvatar: row[`user_avatar`]}
	for field, target := range map[string]*int{
		`id`: &model.Id, `admin_id`: &model.AdminId, `workspace_id`: &model.WorkspaceId,
		`requirement_id`: &model.RequirementId, `user_id`: &model.UserId,
		`parent_id`: &model.ParentId, `at_invite`: &model.AtInvite, `automation_rule_id`: &model.AutomationRuleId,
	} {
		value, err := strconv.Atoi(row[field])
		if err != nil {
			return CommentModel{}, fmt.Errorf("parse comment %s: %w", field, err)
		}
		*target = value
	}
	for field, target := range map[string]*int64{
		`automation_execution_id`: &model.AutomationExecutionId, `created_at`: &model.CreatedAt, `updated_at`: &model.UpdatedAt,
	} {
		value, err := strconv.ParseInt(row[field], 10, 64)
		if err != nil {
			return CommentModel{}, fmt.Errorf("parse comment %s: %w", field, err)
		}
		*target = value
	}
	public, err := strconv.ParseBool(row[`is_public`])
	if err != nil {
		return CommentModel{}, fmt.Errorf("parse comment visibility: %w", err)
	}
	model.IsPublic = public
	return model, nil
}

// CreateCommentModel describes comment creation input and output.
type CreateCommentModel struct {
	AdminId       int
	WorkspaceId   int
	RequirementId int
	UserId        int
	Content       string
	// ParentId is the replied-to comment id; 0 means a top-level comment.
	ParentId int
	// AtInvite marks a comment that mentions the work item's invitation submitter.
	AtInvite      bool
	IsPublic      bool
	AttachmentIds []int
	ImageFileIds  []int // Editor-uploaded file ledger ids (image/file/video, source_id = 0) bound to the comment after creation
	Id            int
}

// CreateComment creates a user comment, binds staged attachments and enqueues an automation event
// after a successful commit.
func CreateComment(model CreateCommentModel) (CreateCommentModel, error) {
	model.Content = lib_storage.StripOSSURLSignatures(model.Content)
	before, err := CaptureAutomationSnapshot(model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		return CreateCommentModel{}, err
	}
	if before == nil {
		return CreateCommentModel{}, ErrRequirementNotFound
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return CreateCommentModel{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if model.IsPublic {
		rows, err := msql.RawValues(define.Postgres, `SELECT requirement_id FROM `+define.TableWorkspaceIssueRequirements+`
   WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3`, tx,
			model.AdminId, model.WorkspaceId, model.RequirementId)
		if err != nil {
			return CreateCommentModel{}, err
		}
		if len(rows) == 0 {
			return CreateCommentModel{}, ErrCommentPublicUnavailable
		}
	}
	if len(model.AttachmentIds) > 0 {
		attachments, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM `+define.GetEnterpriseFileTable(model.AdminId)+`
			WHERE admin_id = $1 AND workspace_id = $2 AND source_id = $3 AND uploader_id = $4
			AND comment_id = 0 AND id = ANY($5::INTEGER[]) FOR UPDATE`, tx,
			model.AdminId, model.WorkspaceId, model.RequirementId, model.UserId, pq.Array(model.AttachmentIds))
		if queryErr != nil {
			return CreateCommentModel{}, queryErr
		}
		if len(attachments) != len(model.AttachmentIds) {
			return CreateCommentModel{}, ErrCommentAttachmentsInvalid
		}
	}
	// 富文本编辑器先行上传的图片/文件/视频（source_id = 0）同样要求归属于当前用户且未绑定。
	if len(model.ImageFileIds) > 0 {
		files, queryErr := msql.RawValues(define.Postgres, `SELECT id FROM `+define.GetEnterpriseFileTable(model.AdminId)+`
			WHERE admin_id = $1 AND workspace_id = $2 AND source_id = 0 AND uploader_id = $3
			AND comment_id = 0 AND id = ANY($4::INTEGER[]) FOR UPDATE`, tx,
			model.AdminId, model.WorkspaceId, model.UserId, pq.Array(model.ImageFileIds))
		if queryErr != nil {
			return CreateCommentModel{}, queryErr
		}
		if len(files) != len(model.ImageFileIds) {
			return CreateCommentModel{}, ErrCommentAttachmentsInvalid
		}
	}
	if err = ensureCommentParentValid(tx, model); err != nil {
		return CreateCommentModel{}, err
	}
	model.Id, err = createCommentTx(tx, model)
	if err != nil {
		return CreateCommentModel{}, err
	}
	if len(model.AttachmentIds) > 0 {
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.GetEnterpriseFileTable(model.AdminId)+` SET comment_id = $1, updated_at = $2
			WHERE admin_id = $3 AND workspace_id = $4 AND source_id = $5 AND uploader_id = $6
			AND comment_id = 0 AND id = ANY($7::INTEGER[])`, tx,
			model.Id, tool.Time2Int(), model.AdminId, model.WorkspaceId, model.RequirementId, model.UserId, pq.Array(model.AttachmentIds)); err != nil {
			return CreateCommentModel{}, err
		}
	}
	if len(model.ImageFileIds) > 0 {
		sourceType, sourceName, _ := resolveFileSource(model.AdminId, model.RequirementId, model.UserId)
		if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.GetEnterpriseFileTable(model.AdminId)+`
			SET source_id = $1, source_type = $2, source_name = $3, comment_id = $4, updated_at = $5
			WHERE admin_id = $6 AND workspace_id = $7 AND source_id = 0 AND uploader_id = $8
			AND comment_id = 0 AND id = ANY($9::INTEGER[])`, tx,
			model.RequirementId, sourceType, sourceName, model.Id, tool.Time2Int(),
			model.AdminId, model.WorkspaceId, model.UserId, pq.Array(model.ImageFileIds)); err != nil {
			return CreateCommentModel{}, err
		}
	}
	if err = createCommentNotificationsTx(tx, model.AdminId, model.WorkspaceId, model.RequirementId, model.UserId, model.Content); err != nil {
		return CreateCommentModel{}, err
	}
	if err = tx.Commit(); err != nil {
		return CreateCommentModel{}, err
	}
	enqueueCommentAutomation(model.AdminId, model.WorkspaceId, model.RequirementId, model.UserId, "add",
		AutomationCommentSnapshot{ID: model.Id, Content: model.Content, UserID: model.UserId})
	return model, nil
}

// UpdateCommentModel 保存正文及本次编辑新上传的文件，不允许编辑可见性。
type UpdateCommentModel struct {
	AdminId, WorkspaceId, RequirementId, CommentId, UserId int
	Content                                                string
	ImageFileIds                                           []int
}

// UpdateComment 在同一事务内保存正文、绑定本人上传文件并通知新增提及。
func UpdateComment(model UpdateCommentModel) error {
	adminID, workspaceID, requirementID, commentID, userID, content := model.AdminId, model.WorkspaceId, model.RequirementId, model.CommentId, model.UserId, model.Content
	content = lib_storage.StripOSSURLSignatures(content)
	rows, err := msql.RawValues(define.Postgres, `SELECT id,user_id,content FROM requirement_comments
		WHERE admin_id=$1 AND workspace_id=$2 AND requirement_id=$3 AND id=$4`, nil,
		adminID, workspaceID, requirementID, commentID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return ErrCommentNotFound
	}
	if !canMutateComment(cast.ToInt(rows[0]["user_id"]), userID) {
		return ErrCommentOwnerRequired
	}
	previousContent := rows[0]["content"]
	if previousContent == content && len(model.ImageFileIds) == 0 {
		return nil
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := msql.RawExec(define.Postgres, `UPDATE requirement_comments SET content=$1,updated_at=$2
		WHERE admin_id=$3 AND workspace_id=$4 AND requirement_id=$5 AND id=$6 AND user_id=$7`, tx,
		content, tool.Time2Int(), adminID, workspaceID, requirementID, commentID, userID)
	if err != nil {
		return err
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return affectedErr
	}
	if affected == 0 {
		return ErrCommentNotFound
	}
	if err = bindInlineCommentFilesTx(tx, adminID, workspaceID, requirementID, userID, commentID, model.ImageFileIds); err != nil {
		return err
	}
	if err = notifyEditedCommentMentionsTx(tx, adminID, workspaceID, requirementID, userID, previousContent, content); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	enqueueCommentAutomation(adminID, workspaceID, requirementID, userID, "edit", AutomationCommentSnapshot{ID: commentID, Content: content, UserID: userID})
	return nil
}

// DeleteComment deletes the current user's own comment and its attachments.
func DeleteComment(adminID, workspaceID, requirementID, commentID, userID int) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	comments, err := msql.RawValues(define.Postgres, `SELECT id,user_id,parent_id,content FROM requirement_comments
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND id = $4 FOR UPDATE`, tx,
		adminID, workspaceID, requirementID, commentID)
	if err != nil {
		return err
	}
	if len(comments) == 0 {
		return ErrCommentNotFound
	}
	if !canMutateComment(cast.ToInt(comments[0]["user_id"]), userID) {
		return ErrCommentOwnerRequired
	}
	// 回复按时间线平铺展示，删除时把子回复挂到被删评论的父级上，既不留下悬空引用，
	// 也不级联删除他人内容。
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirement_comments SET parent_id = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND requirement_id = $4 AND parent_id = $5`, tx,
		cast.ToInt(comments[0]["parent_id"]), adminID, workspaceID, requirementID, commentID); err != nil {
		return err
	}
	paths, err := msql.RawValues(define.Postgres, `SELECT stored_path FROM `+define.GetEnterpriseFileTable(adminID)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = $3 AND comment_id = $4`, tx,
		adminID, workspaceID, requirementID, commentID)
	if err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.GetEnterpriseFileTable(adminID)+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = $3 AND comment_id = $4`, tx,
		adminID, workspaceID, requirementID, commentID); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM requirement_comments
		WHERE admin_id = $1 AND workspace_id = $2 AND requirement_id = $3 AND id = $4 AND user_id=$5`, tx,
		adminID, workspaceID, requirementID, commentID, userID); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE requirements SET comment_count = GREATEST(comment_count - 1, 0),
		updated_at = $1 WHERE admin_id = $2 AND workspace_id = $3 AND id = $4`, tx,
		tool.Time2Int(), adminID, workspaceID, requirementID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, path := range paths {
		if stored := path[`stored_path`]; stored != `` {
			if removeErr := lib_storage.M.Delete(stored); removeErr != nil {
				logs.Error("DeleteComment remove attachment failed, err:%s", removeErr.Error())
			}
		}
	}
	enqueueCommentAutomation(adminID, workspaceID, requirementID, userID, "delete", AutomationCommentSnapshot{ID: commentID, Content: comments[0]["content"], UserID: userID})
	return nil
}

// canMutateComment restricts comment mutation to the real author.
func canMutateComment(authorID, userID int) bool {
	return authorID > 0 && authorID == userID
}

// enqueueCommentAutomation enqueues a non-cascading automation event after a comment operation.
func enqueueCommentAutomation(adminID, workspaceID, requirementID, userID int, change string, comment AutomationCommentSnapshot) {
	after, err := CaptureAutomationSnapshot(adminID, workspaceID, requirementID)
	if err != nil || after == nil {
		if err != nil {
			logs.Error("capture comment automation snapshot failed: %s", err.Error())
		}
		return
	}
	user, err := automationUserSnapshot(adminID, userID)
	if err != nil {
		logs.Error("capture comment automation user failed: %s", err.Error())
		return
	}
	event := AutomationEvent{AdminID: adminID, WorkspaceID: workspaceID, ItemID: requirementID, ItemType: after.Type,
		Operation: "comment", CommentChange: change, Comment: &comment, After: after, TriggeredBy: user,
		TriggeredAt: int64(tool.Time2Int()), Source: define.SourceUser}
	if err = EnqueueAutomationEvent(event); err != nil {
		logs.Error("enqueue comment automation failed: %s", err.Error())
	}
}
