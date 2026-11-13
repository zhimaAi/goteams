// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package route

import (
	"net/http"

	"goteams/internal/app/goteams/controller"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_web"
)

// RegIssueInviteRoutes registers authenticated management/share and unauthenticated public routes.
func RegIssueInviteRoutes(r lib_web.Route) {
	r[http.MethodGet][`/api/workspaces/:wsId/issue-invite/config`] = wrapAction(withPerm(define.PermissionTicketManage), controller.GetIssueInviteConfig)
	r[http.MethodPatch][`/api/workspaces/:wsId/issue-invite/config`] = wrapAction(withPerm(define.PermissionTicketManage), controller.UpdateIssueInviteConfig)
	r[http.MethodPost][`/api/workspaces/:wsId/issue-invite/theme-image`] = wrapAction(withPerm(define.PermissionTicketManage), controller.UploadIssueInviteThemeImage)
	r[http.MethodGet][`/api/workspaces/:wsId/issue-invite/share`] = wrapAction(withPerm(define.PermissionTicketCopyLink), controller.GetIssueInviteShare)
	r[http.MethodGet][`/api/workspaces/:wsId/issue-invite/templates`] = wrapAction(withAnyPerm(`workspace:view`, define.PermissionTicketCopyLink, define.PermissionTicketManage), controller.ListIssueInviteTemplates)
	r[http.MethodPost][`/api/workspaces/:wsId/issue-invite/templates`] = wrapAction(withPerm(define.PermissionTicketManage), controller.CreateIssueInviteTemplate)
	r[http.MethodPut][`/api/workspaces/:wsId/issue-invite/templates/order`] = wrapAction(withPerm(define.PermissionTicketManage), controller.ReorderIssueInviteTemplates)
	r[http.MethodGet][`/api/workspaces/:wsId/issue-invite/templates/:templateId`] = wrapAction(withPerm(define.PermissionTicketManage), controller.GetIssueInviteTemplate)
	r[http.MethodPatch][`/api/workspaces/:wsId/issue-invite/templates/:templateId`] = wrapAction(withPerm(define.PermissionTicketManage), controller.UpdateIssueInviteTemplate)
	r[http.MethodDelete][`/api/workspaces/:wsId/issue-invite/templates/:templateId`] = wrapAction(withPerm(define.PermissionTicketManage), controller.RemoveIssueInviteTemplate)

	// Public endpoints intentionally have no authentication middleware; business resolves opaque keys.
	r[http.MethodGet][`/api/public/issue-invites/:inviteKey`] = controller.GetPublicIssueInvite
	r[http.MethodGet][`/api/public/issue-invites/:inviteKey/templates/:templateKey`] = controller.GetPublicIssueTemplate
	r[http.MethodPost][`/api/public/issue-invites/:inviteKey/templates/:templateKey/requirements`] = controller.SubmitPublicIssue
	r[http.MethodPost][`/api/public/issue-invites/:inviteKey/attachments`] = controller.UploadPublicIssueAttachment
	r[http.MethodDelete][`/api/public/issue-invites/:inviteKey/attachments/:attachmentId`] = controller.DeletePublicIssueAttachment
	r[http.MethodGet][`/api/public/issue-invites/:inviteKey/requirements`] = controller.ListPublicIssueHistory
	r[http.MethodGet][`/api/public/issue-invites/:inviteKey/requirements/:requirementId`] = controller.GetPublicIssueDetail
	r[http.MethodPut][`/api/public/issue-invites/:inviteKey/requirements/:requirementId`] = controller.ResubmitPublicIssue
	r[http.MethodPost][`/api/public/issue-invites/:inviteKey/requirements/:requirementId/recall`] = controller.RecallPublicIssue
	r[http.MethodPost][`/api/public/issue-invites/:inviteKey/requirements/:requirementId/urge`] = controller.UrgePublicIssue
	r[http.MethodGet][`/api/public/issue-invites/:inviteKey/requirements/:requirementId/handle-records`] = controller.GetPublicIssueHandleRecords
	r[http.MethodPost][`/api/public/issue-invites/:inviteKey/requirements/:requirementId/comments`] = controller.CreatePublicIssueComment
}
