// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

// Knowledge base / workspace application table name constants.
const (
	TableWorkspaceApplications            = `workspace_applications`
	TableKnowledgeItems                   = `knowledge_items`
	TableKnowledgeDocumentContents        = `knowledge_document_contents`
	TableKnowledgeLinkContents            = `knowledge_link_contents`
	TableKnowledgeDocumentVersions        = `knowledge_document_versions`
	TableKnowledgeDocumentVersionContents = `knowledge_document_version_contents`
	TableKnowledgeFavorites               = `knowledge_favorites`
	TableKnowledgeActivities              = `knowledge_activities`
)

// Knowledge item type constants.
const (
	KnowledgeItemTypeFolder   = `folder`
	KnowledgeItemTypeDocument = `document`
	KnowledgeItemTypeLink     = `link`
)

// Knowledge content mode constants.
const (
	KnowledgeContentModeRichText = `rich-text`
	KnowledgeContentModeMarkdown = `markdown`
	KnowledgeContentModeFile     = `file`
	KnowledgeContentModeLink     = `link`
)

// Knowledge document source type constants.
const (
	KnowledgeSourceTypeCreated = `created`
	KnowledgeSourceTypeUpload  = `upload`
)

// Knowledge version type constants.
const (
	KnowledgeVersionTypeCreate  = `create`
	KnowledgeVersionTypeManual  = `manual`
	KnowledgeVersionTypeRestore = `restore`
	KnowledgeVersionTypeConvert = `convert`
)

// Workspace application key constants. Planning/iteration/defect are enabled by default;
// dashboard is a coming-soon placeholder whose switch is disabled in the UI.
const (
	AppKeyKnowledgeBase = `knowledge_base`
	AppKeyGantt         = `gantt`
	AppKeyPlanning      = `planning`
	AppKeyIteration     = `iteration`
	AppKeyDashboard     = `dashboard`
	AppKeyDefect        = `defect`
)

// Knowledge activity action constants.
const (
	KnowledgeActionCreate  = `create`
	KnowledgeActionUpdate  = `update`
	KnowledgeActionRestore = `restore`
	KnowledgeActionDelete  = `delete`
)

// Source type of a row on the enterprise file ledger tables (enterprise_files_xx):
// requirement/defect come from requirements.type, knowledge marks knowledge base documents.
const (
	SourceTypeRequirement = `requirement`
	SourceTypeKnowledge   = `knowledge`
)

// Knowledge base message keys.
const (
	MsgKnowledgeItemNotFound          = `knowledge_item_not_found`
	MsgKnowledgeDocumentNotEditable   = `knowledge_document_not_editable`
	MsgKnowledgeRevisionConflict      = `knowledge_document_revision_conflict`
	MsgKnowledgeContentTooLarge       = `knowledge_document_content_too_large`
	MsgKnowledgeVersionNotFound       = `knowledge_version_not_found`
	MsgKnowledgeFolderNotEmpty        = `knowledge_folder_not_empty`
	MsgKnowledgeFolderDepthExceeded   = `knowledge_folder_depth_exceeded`
	MsgKnowledgeAttachmentNotFound    = `knowledge_attachment_not_found`
	MsgKnowledgeAttachmentTooLarge    = `knowledge_attachment_too_large`
	MsgKnowledgeAttachmentTypeInvalid = `knowledge_attachment_type_invalid`
	MsgWorkspaceApplicationNotFound   = `workspace_application_not_found`
	MsgWorkspaceApplicationComingSoon = `workspace_application_coming_soon`
)
