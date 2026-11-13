// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"sort"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/permission"
	"goteams/internal/app/goteams/validate"
	"goteams/internal/pkg/lib_storage"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// CopyDashboardProjectTemplateModel is the dashboard portion of a project-template copy.
// RoleIdMap must map every source project role referenced by a role-visible dashboard to a
// project role in the target workspace. Cross-tenant copies are intentionally rejected.
type CopyDashboardProjectTemplateModel struct {
	SourceAdminId     int
	SourceWorkspaceId int
	TargetAdminId     int
	TargetWorkspaceId int
	OperatorId        int
	RoleIdMap         map[int]int
}

// DashboardProjectTemplateCopyResult exposes target dashboard IDs for other steps in the same
// project-template transaction. CreatedAttachmentPaths must be cleaned with
// CleanupDashboardProjectTemplateFiles if a later outer-transaction step fails.
type DashboardProjectTemplateCopyResult struct {
	DashboardIdMap         map[int]int
	CreatedAttachmentPaths []string
}

// CopyDashboardProjectTemplateTx copies project-shared dashboard configuration into an empty
// target workspace in the caller's transaction. It copies dashboards, mapped role visibility,
// cards, and the workspace announcement; member visibility is normalized to public because target
// members are not copied, while extra editors and personal state are deliberately excluded. Metric
// cards retain their metric type and data scope but clear card filters because project-scoped
// field/status/version IDs cannot safely be reused without a future template-pipeline mapping step.
func CopyDashboardProjectTemplateTx(tx *sql.Tx, model CopyDashboardProjectTemplateModel) (DashboardProjectTemplateCopyResult, error) {
	result := DashboardProjectTemplateCopyResult{
		DashboardIdMap:         make(map[int]int),
		CreatedAttachmentPaths: make([]string, 0),
	}
	if tx == nil || model.SourceAdminId <= 0 || model.TargetAdminId <= 0 ||
		model.SourceWorkspaceId <= 0 || model.TargetWorkspaceId <= 0 || model.OperatorId <= 0 ||
		model.SourceAdminId != model.TargetAdminId || model.SourceWorkspaceId == model.TargetWorkspaceId {
		return DashboardProjectTemplateCopyResult{}, ErrDashboardInvalid
	}

	completed := false
	defer func() {
		if !completed {
			CleanupDashboardProjectTemplateFiles(result.CreatedAttachmentPaths)
		}
	}()

	if err := lockDashboardProjectTemplateWorkspaces(tx, model); err != nil {
		return DashboardProjectTemplateCopyResult{}, err
	}
	if err := ensureDashboardProjectTemplateTargetEmptyTx(tx, model.TargetAdminId, model.TargetWorkspaceId); err != nil {
		return DashboardProjectTemplateCopyResult{}, err
	}

	sourceDashboards, err := loadWorkspaceDashboardsForUpdateTx(tx, model.SourceAdminId, model.SourceWorkspaceId)
	if err != nil {
		return DashboardProjectTemplateCopyResult{}, err
	}
	now := int64(tool.Time2Int())
	announcementSourceName := dashboardDefaultName
	for _, source := range sourceDashboards {
		if _, exists := result.DashboardIdMap[source.Id]; exists {
			return DashboardProjectTemplateCopyResult{}, ErrDashboardInvalid
		}
		roles, err := dashboardProjectTemplateRoleIDsTx(tx, model, source)
		if err != nil {
			return DashboardProjectTemplateCopyResult{}, err
		}
		targetId, err := insertDashboardProjectTemplateDashboardTx(tx, model, source, now)
		if err != nil {
			return DashboardProjectTemplateCopyResult{}, err
		}
		result.DashboardIdMap[source.Id] = targetId
		if err = replaceDashboardRelationsTx(tx, model.TargetAdminId, model.TargetWorkspaceId, targetId, roles, nil, nil, now); err != nil {
			return DashboardProjectTemplateCopyResult{}, err
		}
		if err = copyDashboardProjectTemplateCardsTx(tx, model, source, targetId, now, &result.CreatedAttachmentPaths); err != nil {
			return DashboardProjectTemplateCopyResult{}, err
		}
		if announcementSourceName == dashboardDefaultName {
			announcementSourceName = source.Name
		}
	}
	if err = copyDashboardProjectTemplateAnnouncementTx(tx, model, announcementSourceName, now, &result.CreatedAttachmentPaths); err != nil {
		return DashboardProjectTemplateCopyResult{}, err
	}
	completed = true
	return result, nil
}

// CleanupDashboardProjectTemplateFiles removes storage copies produced by a successfully returned
// template hook when its caller later rolls the outer database transaction back.
func CleanupDashboardProjectTemplateFiles(paths []string) {
	for _, path := range paths {
		if deleteErr := lib_storage.M.Delete(path); deleteErr != nil {
			logs.Error("cleanup project-template dashboard attachment failed, err:%s", deleteErr.Error())
		}
	}
}

func lockDashboardProjectTemplateWorkspaces(tx *sql.Tx, model CopyDashboardProjectTemplateModel) error {
	if model.SourceWorkspaceId < model.TargetWorkspaceId {
		if err := lockDashboardWorkspace(tx, model.SourceAdminId, model.SourceWorkspaceId); err != nil {
			return err
		}
		return lockDashboardWorkspace(tx, model.TargetAdminId, model.TargetWorkspaceId)
	}
	if err := lockDashboardWorkspace(tx, model.TargetAdminId, model.TargetWorkspaceId); err != nil {
		return err
	}
	return lockDashboardWorkspace(tx, model.SourceAdminId, model.SourceWorkspaceId)
}

func ensureDashboardProjectTemplateTargetEmptyTx(tx *sql.Tx, adminId, workspaceId int) error {
	for _, table := range []string{define.TableWorkspaceDashboards, define.TableDashboardAnnouncements} {
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+table+`
			WHERE admin_id = $1 AND workspace_id = $2 LIMIT 1 FOR UPDATE`, tx, adminId, workspaceId)
		if err != nil {
			logs.Error("check project-template dashboard target failed, table:%s, err:%s", table, err.Error())
			return err
		}
		if len(rows) > 0 {
			return ErrDashboardInvalid
		}
	}
	return nil
}

func loadWorkspaceDashboardsForUpdateTx(tx *sql.Tx, adminId, workspaceId int) ([]DashboardModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, name, visibility_type, sort_order, creator_id,
		revision, created_at, updated_at FROM `+define.TableWorkspaceDashboards+`
		WHERE admin_id = $1 AND workspace_id = $2 AND deleted_at = 0 ORDER BY sort_order ASC, id ASC FOR UPDATE`, tx,
		adminId, workspaceId)
	if err != nil {
		logs.Error("lock project-template source dashboards failed, err:%s", err.Error())
		return nil, err
	}
	items := make([]DashboardModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := dashboardModelFromRow(row)
		if convErr != nil {
			return nil, convErr
		}
		items = append(items, item)
	}
	return items, nil
}

func dashboardProjectTemplateRoleIDsTx(tx *sql.Tx, model CopyDashboardProjectTemplateModel, source DashboardModel) ([]int, error) {
	if source.VisibilityType == define.DashboardVisibilityPublic || source.VisibilityType == define.DashboardVisibilityMembers || source.VisibilityType == define.DashboardVisibilitySelf {
		return []int{}, nil
	}
	if source.VisibilityType != define.DashboardVisibilityRoles {
		return nil, ErrDashboardInvalid
	}
	relations, err := dashboardRelationsForIdTx(tx, model.SourceAdminId, model.SourceWorkspaceId, source.Id)
	if err != nil {
		return nil, err
	}
	if len(relations.RoleIds) == 0 {
		return nil, ErrDashboardInvalid
	}
	targetSet := make(map[int]struct{}, len(relations.RoleIds))
	for _, sourceRoleId := range relations.RoleIds {
		targetRoleId, exists := model.RoleIdMap[sourceRoleId]
		if !exists || targetRoleId <= 0 {
			return nil, ErrDashboardInvalid
		}
		targetSet[targetRoleId] = struct{}{}
	}
	targetRoleIds := make([]int, 0, len(targetSet))
	for roleId := range targetSet {
		targetRoleIds = append(targetRoleIds, roleId)
	}
	sort.Ints(targetRoleIds)
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableRoles+`
		WHERE admin_id = $1 AND workspace_id = $2 AND scope = $3 AND id = ANY($4::INTEGER[]) FOR SHARE`, tx,
		model.TargetAdminId, model.TargetWorkspaceId, permission.ScopeProject, pq.Array(targetRoleIds))
	if err != nil {
		logs.Error("validate project-template dashboard role map failed, err:%s", err.Error())
		return nil, err
	}
	if len(rows) != len(targetRoleIds) {
		return nil, ErrDashboardInvalid
	}
	return targetRoleIds, nil
}

func insertDashboardProjectTemplateDashboardTx(tx *sql.Tx, model CopyDashboardProjectTemplateModel, source DashboardModel, now int64) (int, error) {
	visibilityType := source.VisibilityType
	if visibilityType == define.DashboardVisibilityMembers {
		visibilityType = define.DashboardVisibilityPublic
	}
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableWorkspaceDashboards+`
		(admin_id, workspace_id, name, visibility_type, sort_order, creator_id, revision, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $7) RETURNING id`, tx,
		model.TargetAdminId, model.TargetWorkspaceId, source.Name, visibilityType, source.SortOrder, model.OperatorId, now)
	if err != nil {
		logs.Error("insert project-template dashboard failed, err:%s", err.Error())
		return 0, err
	}
	if len(rows) != 1 {
		return 0, ErrDashboardInvalid
	}
	return cast.ToInt(rows[0][`id`]), nil
}

func copyDashboardProjectTemplateCardsTx(tx *sql.Tx, model CopyDashboardProjectTemplateModel, source DashboardModel,
	targetDashboardId int, now int64, createdPaths *[]string) error {
	cards, err := loadDashboardCardsForUpdateTx(tx, model.SourceAdminId, model.SourceWorkspaceId, source.Id)
	if err != nil {
		return err
	}
	if len(cards) > define.DashboardMaxCardsPerDashboard {
		return ErrDashboardInvalidLayout
	}
	for _, sourceCard := range cards {
		card := sourceCard
		if card.CardType == define.DashboardCardTypeChart {
			config, configErr := parseDashboardChartConfig(model.SourceAdminId, model.SourceWorkspaceId, card.Config)
			if configErr != nil {
				return configErr
			}
			if (len(config.SLAStageIds) > 0 || len(config.SLARiskStageThresholdMinutes) > 0) &&
				(model.SourceAdminId != model.TargetAdminId || model.SourceWorkspaceId != model.TargetWorkspaceId) {
				return ErrDashboardInvalidMetric
			}
			config.StatusIds = nil
			if config.ChartType != "sla_dimension" {
				config.PriorityIds = nil
			}
			config.CustomOrder = nil
			// 模板复制不继承来源空间的成员和角色范围。
			if (config.ChartType == "category_stats" || config.ChartType == "workload_stats") && (config.AssigneeScope == "members" || config.AssigneeScope == "roles") {
				config.AssigneeScope = "self"
				config.AssigneeScopeIds = nil
			}
			config.SortOrder = "default"
			encoded, marshalErr := json.Marshal(config)
			if marshalErr != nil {
				return marshalErr
			}
			card.Config = string(encoded)
		}
		if card.CardType == define.DashboardCardTypeMetric {
			config, _, configErr := parseDashboardMetricConfig(model.SourceAdminId, model.SourceWorkspaceId, card.Config)
			if configErr != nil {
				return configErr
			}
			// 不把源空间阶段ID带入目标空间；没有阶段映射时明确拒绝该模板复制。
			if len(config.SLAStageIds) > 0 && (model.SourceAdminId != model.TargetAdminId || model.SourceWorkspaceId != model.TargetWorkspaceId) {
				return ErrDashboardInvalidMetric
			}
			config.Filters = json.RawMessage(`null`)
			encoded, marshalErr := json.Marshal(config)
			if marshalErr != nil {
				logs.Error("encode project-template dashboard metric config failed, err:%s", marshalErr.Error())
				return marshalErr
			}
			card.Config = string(encoded)
		}
		if err = validateDashboardCardPayload(model.TargetAdminId, model.TargetWorkspaceId, card, nil); err != nil {
			return err
		}
		targetCard, insertErr := insertDashboardCardTx(tx, model.TargetAdminId, model.TargetWorkspaceId, targetDashboardId, model.OperatorId, card, now)
		if insertErr != nil {
			return insertErr
		}
		if sourceCard.CardType != define.DashboardCardTypeText {
			continue
		}
		content, copyErr := copyDashboardRichTextAttachmentsTx(tx, model.SourceAdminId, model.SourceWorkspaceId, model.TargetWorkspaceId,
			define.SourceTypeDashboardCard, sourceCard.Id, define.SourceTypeDashboardCard, targetCard.Id,
			model.OperatorId, targetCard.Title, sourceCard.Content, createdPaths)
		if copyErr != nil {
			return copyErr
		}
		if content == sourceCard.Content {
			continue
		}
		if _, copyErr = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardCards+`
			SET content = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND id = $5 AND dashboard_id = $6`, tx,
			content, now, model.TargetAdminId, model.TargetWorkspaceId, targetCard.Id, targetDashboardId); copyErr != nil {
			logs.Error("write project-template dashboard card content failed, err:%s", copyErr.Error())
			return copyErr
		}
	}
	return nil
}

func copyDashboardProjectTemplateAnnouncementTx(tx *sql.Tx, model CopyDashboardProjectTemplateModel, sourceName string,
	now int64, createdPaths *[]string) error {
	source, exists, err := loadDashboardAnnouncementForUpdateTx(tx, model.SourceAdminId, model.SourceWorkspaceId)
	if err != nil || !exists {
		return err
	}
	contentMode, err := dashboardContentMode(source.ContentMode)
	if err != nil || validate.DashboardContent(source.Content) != nil {
		return ErrDashboardInvalid
	}
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableDashboardAnnouncements+`
		(admin_id, workspace_id, content, content_mode, revision, created_by, updated_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 1, $5, $5, $6, $6) RETURNING id`, tx,
		model.TargetAdminId, model.TargetWorkspaceId, source.Content, contentMode, model.OperatorId, now)
	if err != nil {
		logs.Error("insert project-template dashboard announcement failed, err:%s", err.Error())
		return err
	}
	if len(rows) != 1 {
		return ErrDashboardInvalid
	}
	targetId := cast.ToInt(rows[0][`id`])
	content, err := copyDashboardRichTextAttachmentsTx(tx, model.SourceAdminId, model.SourceWorkspaceId, model.TargetWorkspaceId,
		define.SourceTypeDashboardAnnouncement, source.Id, define.SourceTypeDashboardAnnouncement, targetId,
		model.OperatorId, sourceName, source.Content, createdPaths)
	if err != nil {
		return err
	}
	if content == source.Content {
		return nil
	}
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableDashboardAnnouncements+`
		SET content = $1, updated_at = $2 WHERE admin_id = $3 AND workspace_id = $4 AND id = $5`, tx,
		content, now, model.TargetAdminId, model.TargetWorkspaceId, targetId); err != nil {
		logs.Error("write project-template dashboard announcement content failed, err:%s", err.Error())
	}
	return err
}

func loadDashboardAnnouncementForUpdateTx(tx *sql.Tx, adminId, workspaceId int) (DashboardAnnouncementModel, bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, content, content_mode, revision, created_by, updated_by, created_at, updated_at
		FROM `+define.TableDashboardAnnouncements+` WHERE admin_id = $1 AND workspace_id = $2 LIMIT 1 FOR UPDATE`, tx,
		adminId, workspaceId)
	if err != nil {
		logs.Error("lock project-template dashboard announcement failed, err:%s", err.Error())
		return DashboardAnnouncementModel{}, false, err
	}
	if len(rows) == 0 {
		return DashboardAnnouncementModel{}, false, nil
	}
	row := rows[0]
	return DashboardAnnouncementModel{
		Id: cast.ToInt(row[`id`]), AdminId: adminId, WorkspaceId: workspaceId,
		Content: row[`content`], ContentMode: row[`content_mode`], Revision: cast.ToInt64(row[`revision`]),
		CreatedBy: cast.ToInt(row[`created_by`]), UpdatedBy: cast.ToInt(row[`updated_by`]),
		CreatedAt: cast.ToInt64(row[`created_at`]), UpdatedAt: cast.ToInt64(row[`updated_at`]),
	}, true, nil
}
