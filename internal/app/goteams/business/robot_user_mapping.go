package business

import (
	"fmt"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
	"goteams/internal/app/goteams/define"
	"strconv"
)

// AutomationUserPushModel describes the mention parameters one member owns on every robot.
type AutomationUserPushModel struct {
	DingtalkAtMobiles        string
	DingtalkAtUserIds        string
	FeishuUserId             string
	WecomMentionedList       string
	WecomMentionedMobileList string
}

// AutomationUserMappingModel describes the mention parameters of one member.
type AutomationUserMappingModel struct {
	UserId int
	Push   AutomationUserPushModel
}

// AutomationUserPushUpdateModel 中 nil 保留原值，空字符串明确清空。
type AutomationUserPushUpdateModel struct {
	DingtalkAtMobiles        *string
	DingtalkAtUserIds        *string
	FeishuUserId             *string
	WecomMentionedList       *string
	WecomMentionedMobileList *string
}

// AutomationUserMappingUpdateModel 表示单个企业成员的映射增量。
type AutomationUserMappingUpdateModel struct {
	UserId int
	Push   AutomationUserPushUpdateModel
}

// SaveAutomationUserMappingsModel describes the save-user-mappings use case shared by all robots.
type SaveAutomationUserMappingsModel struct {
	AdminId     int
	WorkspaceId int
	Items       []AutomationUserMappingUpdateModel
}

// automationPushUpdateSQL 只更新明确提交的字段；空字符串用于清空。
const automationPushUpdateSQL = `UPDATE users SET push_dingtalk_at_mobiles=COALESCE($1, push_dingtalk_at_mobiles), push_dingtalk_at_user_ids=COALESCE($2, push_dingtalk_at_user_ids),
	push_feishu_user_id=COALESCE($3, push_feishu_user_id), push_wecom_mentioned_list=COALESCE($4, push_wecom_mentioned_list), push_wecom_mentioned_mobile_list=COALESCE($5, push_wecom_mentioned_mobile_list), updated_at=$6
	WHERE admin_id=$7 AND id=$8`

// SaveAutomationUserMappings stores the mention parameters used by every robot.
func SaveAutomationUserMappings(model SaveAutomationUserMappingsModel) error {
	if len(model.Items) == 0 {
		return nil
	}
	userIDs := make([]int, 0, len(model.Items))
	for _, item := range model.Items {
		if item.UserId <= 0 {
			return ErrAutomationMemberInvalid
		}
		userIDs = append(userIDs, item.UserId)
	}
	if !automationMembersExist(model.AdminId, model.WorkspaceId, userIDs) {
		return ErrAutomationMemberInvalid
	}
	return saveRobotUserMappings(model)
}

func saveRobotUserMappings(model SaveAutomationUserMappingsModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := tool.Time2Int()
	for _, item := range model.Items {
		if _, err = msql.RawExec(define.Postgres, automationPushUpdateSQL, tx,
			item.Push.DingtalkAtMobiles, item.Push.DingtalkAtUserIds, item.Push.FeishuUserId,
			item.Push.WecomMentionedList, item.Push.WecomMentionedMobileList, now, model.AdminId, item.UserId); err != nil {
			logs.Error("save automation user mapping failed: %s", err.Error())
			return err
		}
	}
	return tx.Commit()
}

// ListRobotUserMappingsModel 限定读取映射的租户和可选工作空间。
type ListRobotUserMappingsModel struct {
	AdminId     int
	WorkspaceId int
}

// ListNotificationUserMappings 读取企业共用映射，工作空间入口仅返回其可用成员。
func ListNotificationUserMappings(model ListRobotUserMappingsModel) ([]AutomationUserMappingModel, error) {
	query := `SELECT u.id, u.push_dingtalk_at_mobiles, u.push_dingtalk_at_user_ids,
 u.push_feishu_user_id, u.push_wecom_mentioned_list, u.push_wecom_mentioned_mobile_list FROM ` + define.TableUsers + ` u
 WHERE u.admin_id=$1 AND u.deleted_at=0 AND COALESCE(u.status,1)=1`
	args := []any{model.AdminId}
	if model.WorkspaceId > 0 {
		query += ` AND EXISTS (SELECT 1 FROM ` + define.TableWorkspaceMembers + ` wm WHERE wm.admin_id=u.admin_id AND wm.user_id=u.id AND wm.workspace_id=$2)`
		args = append(args, model.WorkspaceId)
	}
	query += ` ORDER BY u.id`
	rows, err := msql.RawValues(define.Postgres, query, nil, args...)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return nil, err
	}
	items := make([]AutomationUserMappingModel, 0, len(rows))
	for _, row := range rows {
		id, err := strconv.Atoi(row["id"])
		if err != nil {
			return nil, fmt.Errorf("parse robot mapping user id: %w", err)
		}
		items = append(items, AutomationUserMappingModel{UserId: id, Push: AutomationUserPushModel{
			DingtalkAtMobiles: row["push_dingtalk_at_mobiles"], DingtalkAtUserIds: row["push_dingtalk_at_user_ids"],
			FeishuUserId: row["push_feishu_user_id"], WecomMentionedList: row["push_wecom_mentioned_list"], WecomMentionedMobileList: row["push_wecom_mentioned_mobile_list"],
		}})
	}
	return items, nil
}

// SaveNotificationUserMappings 校验企业成员后保存共用的机器人参数映射。
func SaveNotificationUserMappings(model SaveAutomationUserMappingsModel) error {
	if len(model.Items) == 0 {
		return nil
	}
	ids := make([]int, 0, len(model.Items))
	for _, item := range model.Items {
		if item.UserId <= 0 {
			return ErrNotificationStrategyMembers
		}
		ids = append(ids, item.UserId)
	}
	if err := validateNotificationMembers(model.AdminId, ids); err != nil {
		return err
	}
	return saveRobotUserMappings(model)
}
