// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"bytes"
	"fmt"
	"mime"
	"sort"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_storage"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// copiedAssetInfo carries a migrated stored object and the file metadata that the destination
// tenant's enterprise file ledger records for it.
type copiedAssetInfo struct {
	newStored   string
	fileName    string
	fileExt     string
	fileSize    int64
	contentType string
	storedName  string
}

// copiedAssetBinding describes where a migrated asset is bound in the destination workspace.
type copiedAssetBinding struct {
	sourceType string // work item type (requirement/defect) or SourceTypeKnowledge
	sourceID   int    // work item id for requirement/defect assets, 0 for knowledge rows
	sourceName string // work item title
	folderID   int    // knowledge document's parent folder id
	documentID int    // knowledge document id
}

// copiedParentRef queues a copied row whose parent must be remapped after the whole insert
// pass, because the parent's new id is not known before every row exists.
type copiedParentRef struct {
	newId     int
	oldParent int
}

// copyWorkItems copies the template workspace's requirement/defect rows: the title, content
// and the status / priority / iteration it sits on are carried over, the last three translated
// to the destination workspace's own rows, while every other field is initialized the way a
// freshly created work item would be (created_at set to the copy time). Stored assets
// referenced by the content are migrated into the destination tenant's storage and recorded in
// its enterprise file ledger bound to the copied row.
func (c *TemplateConfigCopier) copyWorkItems() error {
	rows, err := c.sourceRows(SrcWorkItems)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	defaultStatus, defaultPriority, err := c.workItemCopyDefaults(define.TypeKeyRequirement, define.TypeKeyDefect)
	if err != nil {
		return err
	}
	copiedIds := make(map[int]int, len(rows))
	parentRefs := make([]copiedParentRef, 0)
	assetCache := make(map[string]copiedAssetInfo)
	for _, row := range rows {
		oldId := cast.ToInt(row[`id`])
		itemType := row[`type`]
		title := lib_storage.StripOSSURLSignatures(row[`title`])
		mode := row[`description_mode`]
		if mode != define.DescriptionModeRichText && mode != define.DescriptionModeMarkdown {
			mode = define.DescriptionModeRichText
		}
		content, assets, err := c.migrateContentAssets(row[`description`], assetCache)
		if err != nil {
			return err
		}
		// Status, priority and iteration travel with the item as the destination workspace's
		// own rows; a reference the copied set cannot resolve (a row deleted on the template
		// side) falls back to what a freshly created item would get.
		oldStatusId := cast.ToInt(row[`status_id`])
		statusId := c.mappings.statuses[oldStatusId]
		if statusId == 0 {
			warnUncopiedWorkItemRef(c.templateWsId, oldId, `status`, oldStatusId)
			statusId = defaultStatus[itemType]
		}
		oldPriorityId := cast.ToInt(row[`priority`])
		priorityId := c.mappings.priorities[oldPriorityId]
		if priorityId == 0 {
			warnUncopiedWorkItemRef(c.templateWsId, oldId, `priority`, oldPriorityId)
			priorityId = defaultPriority[itemType]
		}
		oldVersionId := cast.ToInt(row[`version_id`])
		versionId := c.mappings.versions[oldVersionId]
		if versionId == 0 {
			warnUncopiedWorkItemRef(c.templateWsId, oldId, `version`, oldVersionId)
		}
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableRequirements+`
			(admin_id, workspace_id, parent_id, category_id, title, description, description_mode, type,
			 priority, status_id, version_id, planned_start_date, planned_end_date, work_hours,
			 created_by, created_at, updated_at)
			VALUES ($1,$2,0,0,$3,$4,$5,$6,$7,$8,$9,0,0,0,$10,$11,$11) RETURNING id`, c.tx,
			c.adminId, c.newWsId, title, content, mode, itemType,
			priorityId, statusId, versionId, c.userId, c.now)
		if err != nil {
			logs.Error("copy template [%d] insert work item [%d] err:%s", c.templateWsId, oldId, err.Error())
			return err
		}
		newId := cast.ToInt(ids[0][`id`])
		if err := createHistoryTx(c.tx, c.adminId, c.newWsId, newId, c.userId, define.HistoryActionCreated, "", "", title); err != nil {
			return err
		}
		copiedIds[oldId] = newId
		if oldParent := cast.ToInt(row[`parent_id`]); oldParent > 0 {
			parentRefs = append(parentRefs, copiedParentRef{newId: newId, oldParent: oldParent})
		}
		for _, asset := range assets {
			if _, err := c.insertCopiedAssetLedger(copiedAssetBinding{
				sourceType: itemType,
				sourceID:   newId,
				sourceName: title,
			}, asset); err != nil {
				return err
			}
		}
	}
	if err := c.remapCopiedParents(define.TableRequirements, parentRefs, copiedIds); err != nil {
		return err
	}
	for _, id := range copiedIds {
		if err := recordSLACreatedTx(c.tx, c.adminId, c.newWsId, id, "workspace_template"); err != nil {
			return err
		}
	}
	return nil
}

// warnUncopiedWorkItemRef logs a work item status/priority/iteration reference that was set on
// the template side but has no copied counterpart. References the template item never had stay
// silent, since unset (0) is their value in the destination as well.
func warnUncopiedWorkItemRef(templateWsId, workItemId int, field string, oldId int) {
	if oldId == 0 {
		return
	}
	logs.Warning("copy template [%d] work item [%d] %s [%d] outside copied set, fall back to the destination default",
		templateWsId, workItemId, field, oldId)
}

// copyPlannings copies the template workspace's planning rows by name: the title, the group
// tab they sit on and their position within that group come from the template, while status,
// priority and colour are initialized the way a freshly created planning would be. Deleted,
// archived and blank rows are left behind, and relations to requirements are not carried
// over because they would point at the template account's work items.
func (c *TemplateConfigCopier) copyPlannings() error {
	rows, err := c.sourceRows(SrcPlannings)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	defaultStatus, defaultPriority, err := c.workItemCopyDefaults(define.TypeKeyPlanning)
	if err != nil {
		return err
	}
	// Planning lists are read per group ordered by (sort_order, id), so rows are renumbered
	// with the creation-time gap instead of inheriting the template's raw values.
	lastOrder := make(map[int]int64, len(c.mappings.planningGroups))
	for _, row := range rows {
		oldId := cast.ToInt(row[`id`])
		newGroupId := c.mappings.planningGroups[cast.ToInt(row[`group_id`])]
		if newGroupId == 0 {
			logs.Warning("copy template [%d] planning [%d] group [%d] outside copied set, skipped",
				c.templateWsId, oldId, cast.ToInt(row[`group_id`]))
			continue
		}
		lastOrder[newGroupId] += define.PlanningSortGap
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableRequirements+`
			(admin_id, workspace_id, group_id, title, color, type, priority, status_id, sort_order, created_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`, c.tx,
			c.adminId, c.newWsId, newGroupId, lib_storage.StripOSSURLSignatures(row[`title`]), planningDefaultColor, define.TypeKeyPlanning,
			defaultPriority[define.TypeKeyPlanning], defaultStatus[define.TypeKeyPlanning],
			lastOrder[newGroupId], c.userId, c.now); err != nil {
			logs.Error("copy template [%d] insert planning [%d] err:%s", c.templateWsId, oldId, err.Error())
			return err
		}
	}
	return nil
}

// workItemCopyDefaults resolves the destination workspace's initial status and default
// priority for the given types inside the shared transaction, mirroring the defaults
// applied when a work item is created from scratch.
func (c *TemplateConfigCopier) workItemCopyDefaults(typeKeys ...string) (map[string]int, map[string]int, error) {
	settingsRows, err := msql.RawValues(define.Postgres, `SELECT requirement_initial_status_id, defect_initial_status_id,
		planning_initial_status_id
		FROM `+define.TableWorkspaceSettings+`
		WHERE admin_id = $1 AND workspace_id = $2`, c.tx, c.adminId, c.newWsId)
	if err != nil {
		logs.Error("copy template [%d] load workspace settings err:%s", c.templateWsId, err.Error())
		return nil, nil, err
	}
	defaultStatus := make(map[string]int, len(typeKeys))
	defaultPriority := make(map[string]int, len(typeKeys))
	for _, itemType := range typeKeys {
		initialField, _ := settingsStatusFields(itemType)
		if len(settingsRows) > 0 {
			defaultStatus[itemType] = cast.ToInt(settingsRows[0][initialField])
		}
		if defaultStatus[itemType] == 0 {
			statusRows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemStatuses+`
				WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND is_default = 1
				ORDER BY sort_order ASC, id ASC LIMIT 1`, c.tx, c.adminId, c.newWsId, itemType)
			if err != nil {
				logs.Error("copy template [%d] load default status err:%s", c.templateWsId, err.Error())
				return nil, nil, err
			}
			if len(statusRows) == 0 {
				return nil, nil, fmt.Errorf("initial status not found for type %s", itemType)
			}
			defaultStatus[itemType] = cast.ToInt(statusRows[0][`id`])
		}
		priorityRows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableWorkItemPriorities+`
			WHERE admin_id = $1 AND workspace_id = $2 AND type_key = $3 AND is_default = 1
			ORDER BY sort_order ASC, id ASC LIMIT 1`, c.tx, c.adminId, c.newWsId, itemType)
		if err != nil {
			logs.Error("copy template [%d] load default priority err:%s", c.templateWsId, err.Error())
			return nil, nil, err
		}
		if len(priorityRows) > 0 {
			defaultPriority[itemType] = cast.ToInt(priorityRows[0][`id`])
		}
	}
	return defaultStatus, defaultPriority, nil
}

// remapCopiedParents rewrites parent_id of copied rows whose parent belongs to the copied
// set; parents outside the set flatten to root (0) instead of dangling across tenants. Rows
// insert with parent_id = 0 first and this runs after the whole insert pass: move/drag
// operations can point a child at a later-created parent, so per-row order cannot be relied on.
func (c *TemplateConfigCopier) remapCopiedParents(table string, refs []copiedParentRef, copiedIds map[int]int) error {
	for _, ref := range refs {
		newParent := copiedIds[ref.oldParent]
		if newParent == 0 {
			logs.Warning("copy template [%d] %s row [%d] parent [%d] outside copied set, flatten to root",
				c.templateWsId, table, ref.newId, ref.oldParent)
			continue
		}
		if _, err := msql.RawExec(define.Postgres, `UPDATE `+table+` SET parent_id = $3
			WHERE admin_id = $1 AND workspace_id = $2 AND id = $4`, c.tx,
			c.adminId, c.newWsId, newParent, ref.newId); err != nil {
			logs.Error("copy template [%d] remap %s row [%d] parent err:%s", c.templateWsId, table, ref.newId, err.Error())
			return err
		}
	}
	return nil
}

// copyKnowledgeItems copies the template workspace's knowledge base in full: folders,
// documents (rich text/markdown and uploaded file type) and links, each with its current
// content and a fresh v1 baseline version. Assets referenced by content or stored as file
// attachments are migrated into the destination tenant's storage and recorded in its
// enterprise file ledger. Per-user rows (favorites, activities) and historical versions are
// intentionally not copied.
func (c *TemplateConfigCopier) copyKnowledgeItems() error {
	rows, err := c.sourceRows(SrcKnowledgeItems)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	docContents, linkContents, err := c.knowledgeContentIndexes()
	if err != nil {
		return err
	}
	itemIDMap := make(map[int]int, len(rows))
	parentRefs := make([]copiedParentRef, 0)
	assetCache := make(map[string]copiedAssetInfo)
	// First pass inserts every item as a root; parents are remapped afterwards because
	// moved items can point at a later-created parent (id larger than the child's).
	for _, row := range rows {
		oldId := cast.ToInt(row[`id`])
		ids, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableKnowledgeItems+`
			(admin_id, workspace_id, parent_id, item_type, title, revision, version_seq,
			 creator_id, updated_by, created_at, updated_at, sort_order)
			VALUES ($1,$2,0,$3,$4,1,1,$5,$5,$6,$6,$7) RETURNING id`, c.tx,
			c.adminId, c.newWsId, row[`item_type`], lib_storage.StripOSSURLSignatures(row[`title`]), c.userId, c.now, row[`sort_order`])
		if err != nil {
			logs.Error("copy template [%d] insert knowledge item [%d] err:%s", c.templateWsId, oldId, err.Error())
			return err
		}
		newId := cast.ToInt(ids[0][`id`])
		itemIDMap[oldId] = newId
		if oldParent := cast.ToInt(row[`parent_id`]); oldParent > 0 {
			parentRefs = append(parentRefs, copiedParentRef{newId: newId, oldParent: oldParent})
		}
	}
	if err := c.remapCopiedParents(define.TableKnowledgeItems, parentRefs, itemIDMap); err != nil {
		return err
	}
	// Second pass copies contents once every folder id is final, so asset ledger rows
	// bind to the copied document's real parent folder.
	for _, row := range rows {
		oldId := cast.ToInt(row[`id`])
		newId := itemIDMap[oldId]
		title := lib_storage.StripOSSURLSignatures(row[`title`])
		newParentId := 0
		if oldParent := cast.ToInt(row[`parent_id`]); oldParent > 0 {
			newParentId = itemIDMap[oldParent]
		}
		switch row[`item_type`] {
		case define.KnowledgeItemTypeDocument:
			if err := c.copyKnowledgeDocumentContent(oldId, newId, newParentId, title, docContents[oldId], assetCache); err != nil {
				return err
			}
		case define.KnowledgeItemTypeLink:
			if err := c.copyKnowledgeLinkContent(oldId, newId, title, linkContents[oldId]); err != nil {
				return err
			}
		}
	}
	return nil
}

// knowledgeContentIndexes loads the package's knowledge content rows once and keys them by the
// template item id they belong to. Content entries are ordered by (item id, row id), so the
// first row per key is the one the per-item query used to return.
func (c *TemplateConfigCopier) knowledgeContentIndexes() (map[int]msql.Params, map[int]msql.Params, error) {
	docRows, err := c.sourceRows(SrcKnowledgeDocContents)
	if err != nil {
		return nil, nil, err
	}
	linkRows, err := c.sourceRows(SrcKnowledgeLinkContents)
	if err != nil {
		return nil, nil, err
	}
	return indexPackageRowsByID(docRows, `document_id`), indexPackageRowsByID(linkRows, `link_id`), nil
}

func indexPackageRowsByID(rows []msql.Params, key string) map[int]msql.Params {
	index := make(map[int]msql.Params, len(rows))
	for _, row := range rows {
		if id := cast.ToInt(row[key]); id > 0 {
			if _, ok := index[id]; !ok {
				index[id] = row
			}
		}
	}
	return index
}

// copyKnowledgeLinkContent copies one link item's current content row.
func (c *TemplateConfigCopier) copyKnowledgeLinkContent(oldLinkId, newLinkId int, title string, row msql.Params) error {
	if row == nil {
		logs.Warning("copy template [%d] knowledge link [%d] content missing, copy as empty", c.templateWsId, oldLinkId)
		return nil
	}
	sourceUrl := lib_storage.StripOSSURLSignatures(row[`source_url`])
	if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableKnowledgeLinkContents+`
		(admin_id, workspace_id, link_id, source_url, content_hash, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, c.tx,
		c.adminId, c.newWsId, newLinkId, sourceUrl, knowledgeLinkHash(title, sourceUrl), c.now); err != nil {
		logs.Error("copy template [%d] insert knowledge link [%d] content err:%s", c.templateWsId, oldLinkId, err.Error())
		return err
	}
	return nil
}

// copyKnowledgeDocumentContent copies one document's current content, its uploaded source
// attachment (file-type documents and converted uploads), all stored assets referenced by
// rich text/markdown, and the fresh v1 baseline version.
func (c *TemplateConfigCopier) copyKnowledgeDocumentContent(oldDocId, newDocId, newFolderId int, title string, row msql.Params, assetCache map[string]copiedAssetInfo) error {
	if row == nil {
		logs.Warning("copy template [%d] knowledge document [%d] content missing, copied as empty", c.templateWsId, oldDocId)
		return nil
	}
	contentMode := row[`content_mode`]
	if contentMode != define.KnowledgeContentModeRichText && contentMode != define.KnowledgeContentModeMarkdown &&
		contentMode != define.KnowledgeContentModeFile {
		contentMode = define.KnowledgeContentModeRichText
	}
	sourceType := row[`source_type`]
	if sourceType != define.KnowledgeSourceTypeCreated && sourceType != define.KnowledgeSourceTypeUpload {
		sourceType = define.KnowledgeSourceTypeCreated
	}
	content := lib_storage.StripOSSURLSignatures(row[`content`])
	newAttachmentId, err := c.copyKnowledgeSourceAttachment(row, newDocId, newFolderId, assetCache)
	if err != nil {
		return err
	}
	if contentMode != define.KnowledgeContentModeFile {
		migrated, assets, err := c.migrateContentAssets(content, assetCache)
		if err != nil {
			return err
		}
		content = migrated
		for _, asset := range assets {
			if _, err := c.insertCopiedAssetLedger(copiedAssetBinding{
				sourceType: define.SourceTypeKnowledge,
				folderID:   newFolderId,
				documentID: newDocId,
			}, asset); err != nil {
				return err
			}
		}
	}
	contentHash := knowledgeContentHash(title, contentMode, content)
	if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableKnowledgeDocumentContents+`
		(admin_id, workspace_id, document_id, content, content_mode, content_hash, source_type, source_attachment_id, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, c.tx,
		c.adminId, c.newWsId, newDocId, content, contentMode, contentHash, sourceType, newAttachmentId, c.now); err != nil {
		logs.Error("copy template [%d] insert knowledge document [%d] content err:%s", c.templateWsId, oldDocId, err.Error())
		return err
	}
	versionRows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.TableKnowledgeDocumentVersions+`
		(admin_id, workspace_id, document_id, version_no, version_type, description,
		 restore_from_version_id, edit_session_id, created_by, created_at)
		VALUES ($1,$2,$3,1,$4,'',0,'',$5,$6) RETURNING id`, c.tx,
		c.adminId, c.newWsId, newDocId, define.KnowledgeVersionTypeCreate, c.userId, c.now)
	if err != nil {
		logs.Error("copy template [%d] insert knowledge document [%d] version err:%s", c.templateWsId, oldDocId, err.Error())
		return err
	}
	if _, err := msql.RawExec(define.Postgres, `INSERT INTO `+define.TableKnowledgeDocumentVersionContents+`
		(admin_id, workspace_id, version_id, title, content, content_mode, content_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, c.tx,
		c.adminId, c.newWsId, cast.ToInt(versionRows[0][`id`]), title, content, contentMode, contentHash); err != nil {
		logs.Error("copy template [%d] insert knowledge document [%d] version content err:%s", c.templateWsId, oldDocId, err.Error())
		return err
	}
	return nil
}

// copyKnowledgeSourceAttachment migrates the stored object backing a file-type document or a
// converted upload and returns the new ledger id. The export resolved the template ledger row
// behind source_attachment_id into the package asset key (source_asset); 0 is returned when the
// document references no attachment, the package carries no asset for it, or that asset cannot be
// read, degrading to a document without a downloadable source.
func (c *TemplateConfigCopier) copyKnowledgeSourceAttachment(docContent msql.Params, newDocId, newFolderId int, assetCache map[string]copiedAssetInfo) (int, error) {
	sourceAttachmentId := cast.ToInt(docContent[`source_attachment_id`])
	if sourceAttachmentId <= 0 {
		return 0, nil
	}
	stored := docContent[TemplateDerivedKnowledgeSourceAsset]
	if stored == `` {
		logs.Warning("copy template [%d] knowledge source attachment [%d] not in package, skipped", c.templateWsId, sourceAttachmentId)
		return 0, nil
	}
	asset, ok, err := c.migrateStoredObject(stored, assetCache)
	if err != nil {
		return 0, err
	}
	if !ok {
		logs.Warning("copy template [%d] knowledge source attachment [%d] object missing, skipped", c.templateWsId, sourceAttachmentId)
		return 0, nil
	}
	return c.insertCopiedAssetLedger(copiedAssetBinding{
		sourceType: define.SourceTypeKnowledge,
		folderID:   newFolderId,
		documentID: newDocId,
	}, asset)
}

// migrateContentAssets rewrites every package asset reference found in a rich text/markdown
// content to its migrated copy and returns the replacement content plus one entry per referenced
// object (references the package does not carry are left alone).
func (c *TemplateConfigCopier) migrateContentAssets(content string, cache map[string]copiedAssetInfo) (string, []copiedAssetInfo, error) {
	content = lib_storage.StripOSSURLSignatures(content)
	refs := c.packageAssetRefs(content)
	if len(refs) == 0 {
		return content, nil, nil
	}
	// Replace longer references first: one reference path can be a prefix of another, and
	// replacing the shorter one first would rewrite the longer reference into a mixed path
	// backed by no migrated object.
	sort.Slice(refs, func(i, j int) bool { return len(refs[i]) > len(refs[j]) })
	assets := make([]copiedAssetInfo, 0, len(refs))
	for _, ref := range refs {
		asset, ok, err := c.migrateStoredObject(ref, cache)
		if err != nil {
			return content, nil, err
		}
		if !ok {
			continue
		}
		assets = append(assets, asset)
		content = strings.ReplaceAll(content, ref, asset.newStored)
	}
	return content, assets, nil
}

// packageAssetRefs extracts the unique package asset references from a content string. A package
// never carries the template account's URLs, so the assets index is the only authority: a token
// that is an index key is a reference to migrate, every other URL is content and stays intact.
func (c *TemplateConfigCopier) packageAssetRefs(content string) []string {
	if c.src == nil {
		return nil
	}
	matches := templateAssetRefPattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(matches))
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		if seen[match] || !c.src.HasAsset(match) {
			continue
		}
		seen[match] = true
		refs = append(refs, match)
	}
	return refs
}

// storedURLsInContent extracts the unique stored asset URLs found in live content: local
// "/uploads/" paths owned by the template admin, plus every http(s) value that parses as an
// object on this deployment's storage. It is the export side of the content scan — package
// content carries asset references instead, which the copy reads through packageAssetRefs.
// URLs pointing outside this storage (external sites, other deployments) never match and stay
// intact. A URL of this storage is collected whether or not the template account owns it: the
// export then empties it rather than ship a reference the destination cannot resolve.
func (c *TemplateConfigCopier) storedURLsInContent(content string) []string {
	urls := c.localStoredURLsInContent(content)
	urls = append(urls, c.ossStoredURLsInContent(content)...)
	return urls
}

// localStoredURLsInContent scans literal "/uploads/" prefixes until a URL delimiter and returns
// the unique local stored paths that belong to the template admin. One pass covers both key
// layouts: the admin ID is the first segment of the older "/uploads/{adminID}/{business}/" and the
// second of the current "/uploads/{business}/{adminID}/". The business is deliberately not checked
// here, so no business is ever missed just because the whitelist does not carry it: an
// uncollected URL is left as-is in the package and keeps pointing at the template account, while a
// collected one is emptied by rewriteAssetReferences when the export cannot carry it.
func (c *TemplateConfigCopier) localStoredURLsInContent(content string) []string {
	adminID := cast.ToString(c.templateAdminId)
	seen := make(map[string]bool)
	urls := make([]string, 0)
	for _, url := range storedURLsWithPrefix(content, `/uploads/`) {
		if !seen[url] && isLocalStoredURLOwnedBy(url, adminID) {
			seen[url] = true
			urls = append(urls, url)
		}
	}
	return urls
}

// isLocalStoredURLOwnedBy reports whether a "/uploads/{key}" value has the admin ID as its first
// or second key segment. Segment equality keeps "7" from matching "77".
func isLocalStoredURLOwnedBy(url, adminID string) bool {
	parts := strings.Split(strings.TrimPrefix(url, `/uploads/`), `/`)
	return len(parts) >= 2 && (parts[0] == adminID || parts[1] == adminID)
}

// storedURLsWithPrefix returns every URL in the content that starts with the prefix and runs up to
// the next URL delimiter. Duplicates are kept, the caller decides what to do with them.
func storedURLsWithPrefix(content, prefix string) []string {
	urls := make([]string, 0)
	rest := content
	for {
		index := strings.Index(rest, prefix)
		if index < 0 {
			return urls
		}
		end := index + len(prefix)
		for end < len(rest) && !isStoredURLDelimiter(rest[end]) {
			end++
		}
		urls = append(urls, strings.TrimRight(rest[index:end], `.,;:!?`))
		rest = rest[end:]
	}
}

// ossStoredURLsInContent scans http(s) URLs and keeps the ones that parse as an object on this
// deployment's storage, so content assets written when OSS storage was enabled are collected for
// the package too.
func (c *TemplateConfigCopier) ossStoredURLsInContent(content string) []string {
	seen := make(map[string]bool)
	urls := make([]string, 0)
	for _, scheme := range []string{`https://`, `http://`} {
		rest := content
		for {
			index := strings.Index(rest, scheme)
			if index < 0 {
				break
			}
			end := index + len(scheme)
			for end < len(rest) && !isStoredURLDelimiter(rest[end]) {
				end++
			}
			url := strings.TrimRight(rest[index:end], `.,;:!?`)
			if !seen[url] && c.isSystemOSSURL(url) {
				seen[url] = true
				urls = append(urls, url)
			}
			rest = rest[end:]
		}
	}
	return urls
}

// isSystemOSSURL reports whether url points at an object on this deployment's storage. The tenant
// is deliberately not checked here: a URL of this storage is collected whoever owns it, and the
// export decides per object whether it can be carried. Both key layouts pass on the shape alone.
//
// The shape check is not redundant with the parse. A cname deployment matches by host only, so any
// path on that domain parses successfully, including the site's own routes; clearing those from the
// content would only destroy it. Local values need no such guard: /uploads/ belongs to storage.
func (c *TemplateConfigCopier) isSystemOSSURL(url string) bool {
	if !strings.HasPrefix(url, `https://`) && !strings.HasPrefix(url, `http://`) {
		return false
	}
	_, key, err := lib_storage.ParseStored(url)
	return err == nil && lib_storage.IsObjectKeyShape(key)
}

// isStoredURLDelimiter reports whether the byte terminates a stored asset URL in html or
// markdown content.
func isStoredURLDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '"', '\'', '`', '<', '>', ')', ']':
		return true
	}
	return false
}

// migrateStoredObject copies one package asset into the destination tenant's storage (same
// business namespace, same extension) and returns the metadata booked on the copied ledger row.
// It returns ok=false for stored values the package does not carry so callers keep the original
// reference.
func (c *TemplateConfigCopier) migrateStoredObject(oldStored string, cache map[string]copiedAssetInfo) (copiedAssetInfo, bool, error) {
	if cached, ok := cache[oldStored]; ok {
		return cached, true, nil
	}
	data, meta, ok := c.src.ReadAsset(oldStored)
	if !ok {
		logs.Warning("copy template [%d] skip stored object [%s] not carried by the package", c.templateWsId, oldStored)
		return copiedAssetInfo{}, false, nil
	}
	// The loader rejects an asset whose business is outside the storage namespaces or whose
	// extension is not a safe slug, so the pair is usable as an object key as it stands.
	newStored, err := lib_storage.M.Upload(c.adminId, meta.Business, meta.Ext, bytes.NewReader(data))
	if err != nil {
		logs.Error("copy template [%d] upload stored object [%s] err:%s", c.templateWsId, oldStored, err.Error())
		return copiedAssetInfo{}, false, err
	}
	contentType := meta.ContentType
	if contentType == `` {
		if contentType = mime.TypeByExtension(`.` + meta.Ext); contentType == `` {
			contentType = `application/octet-stream`
		}
	}
	asset := copiedAssetInfo{
		newStored:   newStored,
		fileName:    meta.FileName,
		fileExt:     meta.Ext,
		contentType: contentType,
		storedName:  meta.StoredName,
		// Booked from the bytes we actually copied, not from the index claim.
		fileSize: int64(len(data)),
	}
	cache[oldStored] = asset
	return asset, true, nil
}

// copyCopiedThemeImage returns the destination-side stored value of a template invitation theme
// image. Leaving the template account's URL in place would point the new tenant at an object it
// cannot read, so the object is migrated; because the upload endpoint that produced the original
// recorded an unbound ledger row, the copy books one too, or the copied bytes would escape the
// tenant storage quota. An image the package does not carry keeps its original value.
func (c *TemplateConfigCopier) copyCopiedThemeImage(stored string) (string, error) {
	if stored == `` {
		return ``, nil
	}
	// copiedAssets holds theme images only, so an entry already present means this image was
	// migrated and booked earlier in the same copy.
	_, booked := c.copiedAssets[stored]
	asset, ok, err := c.migrateStoredObject(stored, c.copiedAssets)
	if err != nil {
		return ``, err
	}
	if !ok {
		return stored, nil
	}
	if booked {
		return asset.newStored, nil
	}
	if _, err := c.insertCopiedAssetLedger(copiedAssetBinding{}, asset); err != nil {
		return ``, err
	}
	return asset.newStored, nil
}

// insertCopiedAssetLedger records a migrated asset in the destination tenant's enterprise
// file shard bound according to the given binding and returns the new ledger row id.
func (c *TemplateConfigCopier) insertCopiedAssetLedger(binding copiedAssetBinding, asset copiedAssetInfo) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO `+define.GetEnterpriseFileTable(c.adminId)+`
		(admin_id, workspace_id, source_type, source_id, source_name, comment_id,
		 file_name, file_ext, file_size, stored_path, content_type,
		 uploader_id, uploader_name, created_at, updated_at, stored_name, folder_id, document_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$14,$15,$16,$17) RETURNING id`, c.tx,
		c.adminId, c.newWsId, binding.sourceType, binding.sourceID, binding.sourceName, 0,
		asset.fileName, asset.fileExt, asset.fileSize, asset.newStored, asset.contentType,
		c.userId, c.operatorName(), c.now, asset.storedName, binding.folderID, binding.documentID)
	if err != nil {
		logs.Error("copy template [%d] insert copied asset ledger err:%s", c.templateWsId, err.Error())
		return 0, err
	}
	// Track the copied size so the tenant storage usage cache can be adjusted once after the
	// copy transaction commits, matching every other ledger write path.
	c.copiedStorageDelta += asset.fileSize
	return cast.ToInt(rows[0][`id`]), nil
}

// operatorName returns the destination operator's display name once, recorded as the uploader
// of copied file ledger rows; an empty value is tolerated when the lookup fails.
func (c *TemplateConfigCopier) operatorName() string {
	if c.uploaderName == `` && c.userId > 0 {
		rows, err := msql.RawValues(define.Postgres, `SELECT name FROM `+define.TableUsers+`
			WHERE admin_id = $1 AND id = $2`, c.tx, c.adminId, c.userId)
		if err != nil {
			logs.Error("copy template [%d] load operator name err:%s", c.templateWsId, err.Error())
			return ``
		}
		if len(rows) > 0 {
			c.uploaderName = rows[0][`name`]
		}
	}
	return c.uploaderName
}

// sanitizeCopiedStoredExt keeps only safe lowercase storage extensions (alphanumeric, up to
// 20 characters); anything else maps to "bin" so object keys never carry path fragments.
func sanitizeCopiedStoredExt(ext string) string {
	ext = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), `.`)
	if ext == `` || len(ext) > 20 {
		return `bin`
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return `bin`
		}
	}
	return ext
}
