// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/lib/pq"
	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	// ErrRequirementCategoryNotFound indicates the category does not exist in the current workspace.
	ErrRequirementCategoryNotFound = errors.New("requirement category not found")
	// ErrRequirementCategoryNameExists indicates the category name already exists in the current workspace.
	ErrRequirementCategoryNameExists = errors.New("requirement category name exists")
	// ErrRequirementCategoryTreeInvalid indicates a category move would create an invalid tree.
	ErrRequirementCategoryTreeInvalid = errors.New("invalid requirement category tree")
)

// ErrRequirementCategoryAssignmentInvalid 表示目标不是可归类的顶层需求。
var ErrRequirementCategoryAssignmentInvalid = errors.New("category assignment requires a top-level requirement")

// ErrRequirementCategoryAssignmentForbidden 表示当前工作空间的分类字段不可编辑。
var ErrRequirementCategoryAssignmentForbidden = errors.New("requirement category field is not editable")

const requirementCategorySortGap int64 = 65536

// RequirementCategoryModel describes one real requirement category.
type RequirementCategoryModel struct {
	Id        int
	ParentId  int
	Name      string
	SortOrder int64
	Count     int
}

// ListRequirementCategoriesModel describes the category-list input and output.
type ListRequirementCategoriesModel struct {
	AdminId            int
	UserId             int
	WorkspaceId        int
	Items              []RequirementCategoryModel
	AllCount           int
	UncategorizedCount int
}

// CreateRequirementCategoryModel describes category creation.
type CreateRequirementCategoryModel struct {
	AdminId     int
	WorkspaceId int
	ParentId    int
	Name        string
	Item        RequirementCategoryModel
}

// UpdateRequirementCategoryModel describes category renaming.
type UpdateRequirementCategoryModel struct {
	AdminId     int
	WorkspaceId int
	CategoryId  int
	Name        string
}

// DeleteRequirementCategoryModel describes category deletion.
type DeleteRequirementCategoryModel struct {
	AdminId     int
	WorkspaceId int
	CategoryId  int
}

// MoveRequirementCategoryModel describes a category drag result.
type MoveRequirementCategoryModel struct {
	AdminId     int
	WorkspaceId int
	CategoryId  int
	ParentId    int
	PrevId      int
	NextId      int
}

func toRequirementCategoryModel(row msql.Params) (RequirementCategoryModel, error) {
	id, err := strconv.Atoi(row[`id`])
	if err != nil {
		return RequirementCategoryModel{}, fmt.Errorf("parse requirement category id: %w", err)
	}
	parentId, err := strconv.Atoi(row[`parent_id`])
	if err != nil {
		return RequirementCategoryModel{}, fmt.Errorf("parse requirement category parent_id: %w", err)
	}
	sortOrder, err := strconv.ParseInt(row[`sort_order`], 10, 64)
	if err != nil {
		return RequirementCategoryModel{}, fmt.Errorf("parse requirement category sort_order: %w", err)
	}
	return RequirementCategoryModel{Id: id, ParentId: parentId, Name: row[`name`], SortOrder: sortOrder}, nil
}

func listRequirementCategoryItems(adminId, workspaceId int) ([]RequirementCategoryModel, error) {
	m := msql.Model(define.TableRequirementCategories, define.Postgres)
	rows, err := m.Field(`id, parent_id, name, sort_order`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(workspaceId)},
	}).Order(`parent_id ASC, sort_order ASC, id ASC`).Select()
	if err != nil {
		logs.Error("list requirement categories failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}

	items := make([]RequirementCategoryModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := toRequirementCategoryModel(row)
		if convErr != nil {
			logs.Error("convert requirement category failed: %s", convErr.Error())
			return nil, convErr
		}
		items = append(items, item)
	}

	return items, nil
}

// ListRequirementCategories 返回分类树与当前用户可见的顶层需求数量。
func ListRequirementCategories(model ListRequirementCategoriesModel) (ListRequirementCategoriesModel, error) {
	items, err := listRequirementCategoryItems(model.AdminId, model.WorkspaceId)
	if err != nil {
		return ListRequirementCategoriesModel{}, err
	}

	directCounts, allCount, err := listRequirementCategoryCounts(model.AdminId, model.WorkspaceId, model.UserId)
	if err != nil {
		return ListRequirementCategoriesModel{}, err
	}
	children := make(map[int][]int, len(items))
	for _, item := range items {
		children[item.ParentId] = append(children[item.ParentId], item.Id)
	}
	memo := make(map[int]int, len(items))
	var subtreeCount func(int, map[int]bool) int
	subtreeCount = func(id int, path map[int]bool) int {
		if count, ok := memo[id]; ok {
			return count
		}
		if path[id] {
			return directCounts[id]
		}
		nextPath := make(map[int]bool, len(path)+1)
		for key := range path {
			nextPath[key] = true
		}
		nextPath[id] = true
		count := directCounts[id]
		for _, childId := range children[id] {
			count += subtreeCount(childId, nextPath)
		}
		memo[id] = count
		return count
	}
	for index := range items {
		items[index].Count = subtreeCount(items[index].Id, map[int]bool{})
	}

	model.Items = items
	model.AllCount = allCount
	model.UncategorizedCount = directCounts[0]
	return model, nil
}

func listRequirementCategoryCounts(adminId, workspaceId, userId int) (map[int]int, int, error) {
	where := []string{`r.admin_id = $1`, `r.workspace_id = $2`, `r.type = $3`, `r.parent_id = 0`}
	args := []any{adminId, workspaceId, define.TypeKeyRequirement}
	scopeCondition, scopeArgs, err := buildDataViewScopeCondition(adminId, cast.ToString(workspaceId), userId, len(args)+1)
	if err != nil {
		return nil, 0, err
	}
	if scopeCondition != `` {
		where = append(where, scopeCondition)
		args = append(args, scopeArgs...)
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT r.category_id, COUNT(*) AS total
		FROM `+define.TableRequirements+` r WHERE `+strings.Join(where, ` AND `)+` GROUP BY r.category_id`, nil, args...)
	if err != nil {
		logs.Error("list requirement category counts failed: %s", err.Error())
		return nil, 0, err
	}
	counts := make(map[int]int, len(rows))
	allCount := 0
	for _, row := range rows {
		categoryId, convErr := strconv.Atoi(row[`category_id`])
		if convErr != nil {
			return nil, 0, fmt.Errorf("parse requirement category count id: %w", convErr)
		}
		count, convErr := strconv.Atoi(row[`total`])
		if convErr != nil {
			return nil, 0, fmt.Errorf("parse requirement category count total: %w", convErr)
		}
		counts[categoryId] = count
		allCount += count
	}
	return counts, allCount, nil
}

// ListRequirementCategorySubtreeIDs returns the selected category and all descendants.
func ListRequirementCategorySubtreeIDs(adminId, workspaceId, categoryId int) ([]int, error) {
	m := msql.Model(define.TableRequirementCategories, define.Postgres)
	rows, err := m.Field(`id, parent_id`).Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`workspace_id`, cast.ToString(workspaceId)},
	}).Select()
	if err != nil {
		logs.Error("list requirement category subtree failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	found := false
	children := make(map[int][]int, len(rows))
	for _, row := range rows {
		id := cast.ToInt(row[`id`])
		parentId := cast.ToInt(row[`parent_id`])
		children[parentId] = append(children[parentId], id)
		if id == categoryId {
			found = true
		}
	}
	if !found {
		return nil, ErrRequirementCategoryNotFound
	}
	result := make([]int, 0)
	seen := make(map[int]bool, len(rows))
	var appendSubtree func(int)
	appendSubtree = func(id int) {
		if seen[id] {
			return
		}
		seen[id] = true
		result = append(result, id)
		for _, childId := range children[id] {
			appendSubtree(childId)
		}
	}
	appendSubtree(categoryId)
	return result, nil
}

// CreateRequirementCategory creates a category at the end of its sibling list.
func CreateRequirementCategory(model CreateRequirementCategoryModel) (CreateRequirementCategoryModel, error) {
	model.Name = strings.TrimSpace(model.Name)
	if model.ParentId < 0 {
		return CreateRequirementCategoryModel{}, ErrRequirementCategoryTreeInvalid
	}
	if err := validate.ValidateRequirementCategoryName(model.Name); err != nil {
		return CreateRequirementCategoryModel{}, err
	}
	if err := validateRequirementCategoryParent(model.AdminId, model.WorkspaceId, model.ParentId); err != nil {
		return CreateRequirementCategoryModel{}, err
	}
	if err := ensureRequirementCategoryNameAvailable(model.AdminId, model.WorkspaceId, 0, model.Name); err != nil {
		return CreateRequirementCategoryModel{}, err
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT COALESCE(MAX(sort_order), 0) AS max_order
		FROM `+define.TableRequirementCategories+` WHERE admin_id = $1 AND workspace_id = $2 AND parent_id = $3`, nil,
		model.AdminId, model.WorkspaceId, model.ParentId)
	if err != nil {
		logs.Error("get requirement category max order failed: %s", err.Error())
		return CreateRequirementCategoryModel{}, err
	}
	maxOrder := int64(0)
	if len(rows) > 0 {
		maxOrder, err = strconv.ParseInt(rows[0][`max_order`], 10, 64)
		if err != nil {
			return CreateRequirementCategoryModel{}, fmt.Errorf("parse requirement category max order: %w", err)
		}
	}
	now := tool.Time2Int()
	m := msql.Model(define.TableRequirementCategories, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`: model.AdminId, `workspace_id`: model.WorkspaceId, `parent_id`: model.ParentId,
		`name`: model.Name, `sort_order`: maxOrder + requirementCategorySortGap,
		`created_at`: now, `updated_at`: now,
	}, `id`)
	if err != nil {
		logs.Error("create requirement category failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		if isRequirementCategoryNameConflict(err) {
			return CreateRequirementCategoryModel{}, ErrRequirementCategoryNameExists
		}
		return CreateRequirementCategoryModel{}, err
	}
	model.Item = RequirementCategoryModel{
		Id: cast.ToInt(id), ParentId: model.ParentId, Name: model.Name,
		SortOrder: maxOrder + requirementCategorySortGap,
	}
	return model, nil
}

// UpdateRequirementCategory renames a category.
func UpdateRequirementCategory(model UpdateRequirementCategoryModel) error {
	model.Name = strings.TrimSpace(model.Name)
	if err := validate.ValidateRequirementCategoryName(model.Name); err != nil {
		return err
	}
	if err := ensureRequirementCategoryExists(model.AdminId, model.WorkspaceId, model.CategoryId, nil); err != nil {
		return err
	}
	if err := ensureRequirementCategoryNameAvailable(model.AdminId, model.WorkspaceId, model.CategoryId, model.Name); err != nil {
		return err
	}
	m := msql.Model(define.TableRequirementCategories, define.Postgres)
	if _, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, cast.ToString(model.WorkspaceId)},
		{`id`, cast.ToString(model.CategoryId)},
	}).Update(msql.Datas{`name`: model.Name, `updated_at`: tool.Time2Int()}); err != nil {
		logs.Error("update requirement category failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		if isRequirementCategoryNameConflict(err) {
			return ErrRequirementCategoryNameExists
		}
		return err
	}
	return nil
}

// DeleteRequirementCategory deletes one category, uncategorizes its requirement trees, and promotes direct children.
func DeleteRequirementCategory(model DeleteRequirementCategoryModel) error {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin requirement category delete failed: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockRequirementHierarchyTx(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}

	items, err := lockRequirementCategories(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	byId, children := indexRequirementCategories(items)
	target, ok := byId[model.CategoryId]
	if !ok {
		return ErrRequirementCategoryNotFound
	}

	// 公开工单提交也可选择分类，但不持有需求树锁。
	if _, err = msql.RawExec(define.Postgres, `LOCK TABLE `+define.TableRequirements+` IN SHARE ROW EXCLUSIVE MODE`, tx); err != nil {
		logs.Error("lock requirements before category delete failed: %s", err.Error())
		return err
	}
	now := tool.Time2Int()
	if _, err = msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirements+` SET category_id = 0, updated_at = $1
		WHERE admin_id = $2 AND workspace_id = $3 AND type = $4 AND category_id = $5`, tx,
		now, model.AdminId, model.WorkspaceId, define.TypeKeyRequirement, model.CategoryId); err != nil {
		logs.Error("uncategorize requirements before category delete failed: %s", err.Error())
		return err
	}

	promoted := append([]int(nil), children[model.CategoryId]...)
	parentSiblings := removeCategoryID(children[target.ParentId], model.CategoryId)
	insertAt := len(parentSiblings)
	for index, siblingId := range children[target.ParentId] {
		if siblingId == model.CategoryId {
			insertAt = index
			break
		}
	}
	if insertAt > len(parentSiblings) {
		insertAt = len(parentSiblings)
	}
	ordered := make([]int, 0, len(parentSiblings)+len(promoted))
	ordered = append(ordered, parentSiblings[:insertAt]...)
	ordered = append(ordered, promoted...)
	ordered = append(ordered, parentSiblings[insertAt:]...)

	if err = updateRequirementCategorySiblingOrder(tx, model.AdminId, model.WorkspaceId, target.ParentId, ordered, now); err != nil {
		return err
	}
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM `+define.TableRequirementCategories+`
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3`, tx,
		model.AdminId, model.WorkspaceId, model.CategoryId); err != nil {
		logs.Error("delete requirement category failed: %s", err.Error())
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit requirement category delete failed: %s", err.Error())
	}
	return err
}

// MoveRequirementCategory moves one category and its subtree to a new sibling position.
func MoveRequirementCategory(model MoveRequirementCategoryModel) error {
	if model.CategoryId <= 0 || model.ParentId < 0 || model.PrevId < 0 || model.NextId < 0 ||
		model.PrevId == model.CategoryId || model.NextId == model.CategoryId {
		return ErrRequirementCategoryTreeInvalid
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		logs.Error("begin requirement category move failed: %s", err.Error())
		return err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := lockRequirementCategories(tx, model.AdminId, model.WorkspaceId)
	if err != nil {
		return err
	}
	byId, children := indexRequirementCategories(items)
	moving, ok := byId[model.CategoryId]
	if !ok {
		return ErrRequirementCategoryNotFound
	}
	if model.ParentId > 0 {
		if _, ok = byId[model.ParentId]; !ok {
			return ErrRequirementCategoryNotFound
		}
	}
	if !validRequirementCategoryMove(byId, children, model.CategoryId, model.ParentId) {
		return ErrRequirementCategoryTreeInvalid
	}

	oldSiblings := removeCategoryID(children[moving.ParentId], model.CategoryId)
	newSiblings := oldSiblings
	if moving.ParentId != model.ParentId {
		newSiblings = removeCategoryID(children[model.ParentId], model.CategoryId)
	}
	insertIndex, ok := requirementCategoryInsertIndex(newSiblings, model.PrevId, model.NextId)
	if !ok {
		return ErrRequirementCategoryTreeInvalid
	}
	newSiblings = insertCategoryID(newSiblings, insertIndex, model.CategoryId)
	now := tool.Time2Int()
	if moving.ParentId != model.ParentId {
		if err = updateRequirementCategorySiblingOrder(tx, model.AdminId, model.WorkspaceId, moving.ParentId, oldSiblings, now); err != nil {
			return err
		}
	}
	if err = updateRequirementCategorySiblingOrder(tx, model.AdminId, model.WorkspaceId, model.ParentId, newSiblings, now); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		logs.Error("commit requirement category move failed: %s", err.Error())
	}
	return err
}

func lockRequirementCategories(tx *sql.Tx, adminId, workspaceId int) ([]RequirementCategoryModel, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id, parent_id, name, sort_order
		FROM `+define.TableRequirementCategories+` WHERE admin_id = $1 AND workspace_id = $2
		ORDER BY parent_id ASC, sort_order ASC, id ASC FOR UPDATE`, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("lock requirement categories failed: %s", err.Error())
		return nil, err
	}
	items := make([]RequirementCategoryModel, 0, len(rows))
	for _, row := range rows {
		item, convErr := toRequirementCategoryModel(row)
		if convErr != nil {
			return nil, convErr
		}
		items = append(items, item)
	}
	return items, nil
}

func indexRequirementCategories(items []RequirementCategoryModel) (map[int]RequirementCategoryModel, map[int][]int) {
	byId := make(map[int]RequirementCategoryModel, len(items))
	children := make(map[int][]int, len(items))
	for _, item := range items {
		byId[item.Id] = item
		children[item.ParentId] = append(children[item.ParentId], item.Id)
	}
	return byId, children
}

func validRequirementCategoryMove(byId map[int]RequirementCategoryModel, children map[int][]int, categoryId, parentId int) bool {
	if categoryId == parentId {
		return false
	}
	seen := map[int]bool{}
	for current := parentId; current > 0; {
		if current == categoryId || seen[current] {
			return false
		}
		seen[current] = true
		item, ok := byId[current]
		if !ok {
			return false
		}
		current = item.ParentId
	}
	parentDepth := 0
	for current := parentId; current > 0; {
		parentDepth++
		item, ok := byId[current]
		if !ok || parentDepth > 3 {
			return false
		}
		current = item.ParentId
	}
	var subtreeHeight func(int, map[int]bool) (int, bool)
	subtreeHeight = func(id int, path map[int]bool) (int, bool) {
		if path[id] {
			return 0, false
		}
		nextPath := make(map[int]bool, len(path)+1)
		for key := range path {
			nextPath[key] = true
		}
		nextPath[id] = true
		height := 1
		for _, childId := range children[id] {
			childHeight, ok := subtreeHeight(childId, nextPath)
			if !ok {
				return 0, false
			}
			if childHeight+1 > height {
				height = childHeight + 1
			}
		}
		return height, true
	}
	height, ok := subtreeHeight(categoryId, map[int]bool{})
	return ok && parentDepth+height <= 3
}

func requirementCategoryInsertIndex(siblings []int, prevId, nextId int) (int, bool) {
	if prevId == 0 && nextId == 0 {
		return len(siblings), true
	}
	if nextId > 0 {
		for index, id := range siblings {
			if id != nextId {
				continue
			}
			if prevId > 0 && (index == 0 || siblings[index-1] != prevId) {
				return 0, false
			}
			return index, true
		}
		return 0, false
	}
	for index, id := range siblings {
		if id == prevId {
			return index + 1, true
		}
	}
	return 0, false
}

func updateRequirementCategorySiblingOrder(tx *sql.Tx, adminId, workspaceId, parentId int, ids []int, now int) error {
	for index, id := range ids {
		if _, err := msql.RawExec(define.Postgres, `UPDATE `+define.TableRequirementCategories+`
			SET parent_id = $1, sort_order = $2, updated_at = $3
			WHERE admin_id = $4 AND workspace_id = $5 AND id = $6`, tx,
			parentId, int64(index+1)*requirementCategorySortGap, now, adminId, workspaceId, id); err != nil {
			logs.Error("update requirement category order failed: %s", err.Error())
			return err
		}
	}
	return nil
}

func removeCategoryID(ids []int, target int) []int {
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != target {
			result = append(result, id)
		}
	}
	return result
}

func insertCategoryID(ids []int, index, id int) []int {
	result := make([]int, 0, len(ids)+1)
	result = append(result, ids[:index]...)
	result = append(result, id)
	return append(result, ids[index:]...)
}

// AssignRequirementCategoryModel 是需求树归类的可信上下文和目标分类。
type AssignRequirementCategoryModel struct {
	AdminId       int
	WorkspaceId   int
	RequirementId int
	UserId        int
	CategoryId    int
}

// AssignRequirementCategory 在一个事务内更新顶层需求及其后代的分类。
func AssignRequirementCategory(model AssignRequirementCategoryModel) error {
	if model.CategoryId < 0 {
		return ErrRequirementCategoryNotFound
	}
	fields, err := ListFieldConfigs(ListFieldConfigsModel{
		AdminId: model.AdminId, WsId: strconv.Itoa(model.WorkspaceId), TypeKey: define.TypeKeyRequirement,
	})
	if err != nil {
		return err
	}
	for _, field := range fields.Items {
		if field.Key == "category" && (field.Enabled != 1 || field.Editable != 1) {
			return ErrRequirementCategoryAssignmentForbidden
		}
	}
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockRequirementHierarchyTx(tx, model.AdminId, model.WorkspaceId); err != nil {
		return err
	}
	query := `SELECT type, COALESCE(parent_id, 0) AS parent_id FROM ` + define.TableRequirements + `
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND deleted_at = 0`
	root, err := msql.RawValues(define.Postgres, query, tx, model.AdminId, model.WorkspaceId, model.RequirementId)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return err
	}
	if len(root) == 0 {
		return ErrRequirementNotFound
	}
	if root[0]["type"] != define.TypeKeyRequirement || root[0]["parent_id"] != "0" {
		return ErrRequirementCategoryAssignmentInvalid
	}
	if err = ensureRequirementCategoryExists(model.AdminId, model.WorkspaceId, model.CategoryId, tx); err != nil {
		return err
	}
	// 按工单 ID 锁定需求，与批量编辑的行锁顺序一致。
	query = `WITH RECURSIVE tree AS (
		SELECT id, category_id FROM ` + define.TableRequirements + ` WHERE admin_id = $1 AND workspace_id = $2 AND id = $3
		UNION
		SELECT r.id, r.category_id FROM ` + define.TableRequirements + ` r JOIN tree t ON r.parent_id = t.id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.type = $4 AND r.deleted_at = 0
	), targets AS (
		SELECT r.id, t.category_id FROM ` + define.TableRequirements + ` r JOIN tree t ON t.id = r.id
		WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.category_id <> $5 ORDER BY r.id FOR UPDATE OF r
	)
	UPDATE ` + define.TableRequirements + ` r SET category_id = $5, updated_at = $6
	FROM targets t WHERE r.admin_id = $1 AND r.workspace_id = $2 AND r.id = t.id
	RETURNING r.id, t.category_id AS old_category_id`
	changed, err := msql.RawValues(define.Postgres, query, tx, model.AdminId, model.WorkspaceId,
		model.RequirementId, define.TypeKeyRequirement, model.CategoryId, tool.Time2Int())
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return err
	}
	changedIds := make([]int, 0, len(changed))
	for _, row := range changed {
		id, parseErr := strconv.Atoi(row["id"])
		if parseErr != nil {
			return fmt.Errorf("parse changed requirement id: %w", parseErr)
		}
		if err = createHistoryTx(tx, model.AdminId, model.WorkspaceId, id, model.UserId,
			define.HistoryActionFieldUpdated, "category_id", row["old_category_id"], strconv.Itoa(model.CategoryId)); err != nil {
			return err
		}
		changedIds = append(changedIds, id)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if len(changedIds) > 0 {
		if err = InvalidateIssueInvitePublicCaches(model.AdminId, changedIds); err != nil {
			logs.Error("invalidate issue invite cache after category assignment failed: %s", err.Error())
		}
	}
	return nil
}

// 归类、改父需求和创建子需求必须在读取需求树之前串行。
func lockRequirementHierarchyTx(tx *sql.Tx, adminId, workspaceId int) error {
	const query = `SELECT pg_advisory_xact_lock($1, $2)`
	_, err := msql.RawValues(define.Postgres, query, tx, adminId, workspaceId)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
	}
	return err
}

func ensureRequirementCategoryExists(adminId, workspaceId, categoryId int, tx *sql.Tx) error {
	if categoryId <= 0 {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableRequirementCategories+`
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 LIMIT 1`, tx,
		adminId, workspaceId, categoryId)
	if err != nil {
		logs.Error("check requirement category exists failed: %s", err.Error())
		return err
	}
	if len(rows) == 0 {
		return ErrRequirementCategoryNotFound
	}
	return nil
}

func validateRequirementCategoryParent(adminId, workspaceId, parentId int) error {
	if parentId == 0 {
		return nil
	}
	rows, err := msql.RawValues(define.Postgres, `SELECT id, parent_id FROM `+define.TableRequirementCategories+`
		WHERE admin_id = $1 AND workspace_id = $2`, nil, adminId, workspaceId)
	if err != nil {
		return err
	}
	parents := make(map[int]int, len(rows))
	for _, row := range rows {
		parents[cast.ToInt(row[`id`])] = cast.ToInt(row[`parent_id`])
	}
	return validateRequirementCategoryParentChain(parents, parentId)
}

func validateRequirementCategoryParentChain(parents map[int]int, parentId int) error {
	if parentId == 0 {
		return nil
	}
	if _, ok := parents[parentId]; !ok {
		return ErrRequirementCategoryNotFound
	}
	depth := 1
	seen := map[int]bool{parentId: true}
	for current := parentId; current > 0; {
		parent, ok := parents[current]
		if !ok || seen[parent] {
			return ErrRequirementCategoryTreeInvalid
		}
		if parent == 0 {
			break
		}
		seen[parent] = true
		depth++
		current = parent
	}
	if depth >= 3 {
		return ErrRequirementCategoryTreeInvalid
	}
	return nil
}

func ensureRequirementCategoryNameAvailable(adminId, workspaceId, excludeId int, name string) error {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+define.TableRequirementCategories+`
		WHERE admin_id = $1 AND workspace_id = $2 AND LOWER(BTRIM(name)) = LOWER(BTRIM($3)) AND id <> $4 LIMIT 1`, nil,
		adminId, workspaceId, name, excludeId)
	if err != nil {
		logs.Error("check requirement category name failed: %s", err.Error())
		return err
	}
	if len(rows) > 0 {
		return ErrRequirementCategoryNameExists
	}
	return nil
}

func isRequirementCategoryNameConflict(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == `23505` && pqErr.Constraint == `uq_req_category_name`
}

func requirementParentCategoryTx(tx *sql.Tx, adminId, workspaceId, parentId int) (int, error) {
	query := `SELECT category_id FROM ` + define.TableRequirements + `
		WHERE admin_id = $1 AND workspace_id = $2 AND id = $3 AND type = $4 AND deleted_at = 0`
	rows, err := msql.RawValues(define.Postgres, query, tx, adminId, workspaceId, parentId, define.TypeKeyRequirement)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, ErrRequirementRelationInvalid
	}
	return strconv.Atoi(rows[0]["category_id"])
}

func resolveCreateRequirementCategoryID(tx *sql.Tx, adminId, workspaceId int, itemType string, parentId, requestedCategoryId int) (int, error) {
	if requestedCategoryId < 0 {
		return 0, ErrRequirementCategoryNotFound
	}
	if itemType != define.TypeKeyRequirement {
		return 0, nil
	}
	if parentId > 0 {
		parentCategoryId, err := requirementParentCategoryTx(tx, adminId, workspaceId, parentId)
		if err != nil {
			return 0, err
		}
		if err = validateRequirementParentCategory(itemType, requestedCategoryId, parentCategoryId); err != nil {
			return 0, err
		}
	}
	if err := ensureRequirementCategoryExists(adminId, workspaceId, requestedCategoryId, tx); err != nil {
		return 0, err
	}
	return requestedCategoryId, nil
}
