// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Enterprise Department Management ========================

var (
	ErrDepartmentNotFound      = errors.New("department not found")
	ErrDepartmentNameExists    = errors.New("department name exists")
	ErrDepartmentDepthExceeded = errors.New("department depth exceeded")
	ErrDepartmentInvalid       = errors.New("department invalid")
)

const (
	// DepartmentMaxDepth caps the tree depth (defensive limit; real orgs rarely exceed 5 levels).
	DepartmentMaxDepth = 10
	// DepartmentMaxNameLen is the maximum department name length in runes.
	DepartmentMaxNameLen = 50
)

// DepartmentModel describes an enterprise department node.
type DepartmentModel struct {
	Id        int
	ParentId  int
	Name      string
	CreatedAt int64
}

// GetEnterpriseName exposes the tenant's enterprise name for the organization tree root
// (viewable with member:view). Yun stores the name on the plan/enterprise module, so
// this reuses getEnterpriseName instead of querying users again.
func GetEnterpriseName(adminId int) (string, error) {
	return getEnterpriseName(adminId)
}

// ListDepartments returns all departments of the tenant in creation order; the frontend assembles the tree.
func ListDepartments(adminId int) ([]DepartmentModel, error) {
	m := msql.Model(define.TableDepartments, define.Postgres)
	rows, err := m.Where(`admin_id`, cast.ToString(adminId)).
		Field(`id, parent_id, name, created_at`).
		Order(`id ASC`).
		Select()
	if err != nil {
		logs.Error("list departments error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	items := make([]DepartmentModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, DepartmentModel{
			Id:        cast.ToInt(row[`id`]),
			ParentId:  cast.ToInt(row[`parent_id`]),
			Name:      row[`name`],
			CreatedAt: cast.ToInt64(row[`created_at`]),
		})
	}
	return items, nil
}

// NormalizeDepartmentName trims and validates a department name.
func NormalizeDepartmentName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == `` || utf8.RuneCountInString(name) > DepartmentMaxNameLen {
		return ``, ErrDepartmentInvalid
	}
	return name, nil
}

// CreateDepartment creates a department under parentId (0 = top level).
func CreateDepartment(adminId, parentId int, name string) (int, error) {
	name, err := NormalizeDepartmentName(name)
	if err != nil {
		return 0, err
	}
	if parentId != 0 {
		parent, err := getDepartment(adminId, parentId)
		if err != nil {
			return 0, err
		}
		if len(parent) == 0 {
			return 0, ErrDepartmentNotFound
		}
	}
	depth, err := departmentDepth(adminId, parentId)
	if err != nil {
		return 0, err
	}
	if depth+1 > DepartmentMaxDepth {
		return 0, ErrDepartmentDepthExceeded
	}
	exists, err := departmentNameExists(adminId, parentId, name, 0)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, ErrDepartmentNameExists
	}
	now := tool.Time2Int()
	m := msql.Model(define.TableDepartments, define.Postgres)
	id, err := m.Insert(msql.Datas{
		`admin_id`:   adminId,
		`parent_id`:  parentId,
		`name`:       name,
		`created_at`: now,
		`updated_at`: now,
	}, `id`)
	if err != nil {
		logs.Error("create department error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		if isDepartmentNameConflict(err) {
			return 0, ErrDepartmentNameExists
		}
		return 0, err
	}
	return cast.ToInt(id), nil
}

// RenameDepartment renames a department; the name must stay unique among its siblings.
func RenameDepartment(adminId, id int, name string) error {
	name, err := NormalizeDepartmentName(name)
	if err != nil {
		return err
	}
	dept, err := getDepartment(adminId, id)
	if err != nil {
		return err
	}
	if len(dept) == 0 {
		return ErrDepartmentNotFound
	}
	exists, err := departmentNameExists(adminId, cast.ToInt(dept[`parent_id`]), name, id)
	if err != nil {
		return err
	}
	if exists {
		return ErrDepartmentNameExists
	}
	m := msql.Model(define.TableDepartments, define.Postgres)
	_, err = m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(id)},
	}).Update(msql.Datas{`name`: name, `updated_at`: tool.Time2Int()})
	if err != nil {
		logs.Error("rename department error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		if isDepartmentNameConflict(err) {
			return ErrDepartmentNameExists
		}
	}
	return err
}

// DeleteDepartment cascade-deletes the department subtree and detaches all member associations.
// Members themselves are untouched; they only lose the deleted department labels.
func DeleteDepartment(adminId, id int) error {
	dept, err := getDepartment(adminId, id)
	if err != nil {
		return err
	}
	if len(dept) == 0 {
		return ErrDepartmentNotFound
	}
	ids, err := listDepartmentSubtreeIds(adminId, id)
	if err != nil {
		return err
	}
	idList := joinInts(ids)
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`DELETE FROM user_departments
		WHERE admin_id = $1 AND department_id IN (%s)`, idList), tx, adminId); err != nil {
		logs.Error("delete department member links error: %s", err.Error())
		return err
	}
	if _, err = msql.RawExec(define.Postgres, fmt.Sprintf(`DELETE FROM departments
		WHERE admin_id = $1 AND id IN (%s)`, idList), tx, adminId); err != nil {
		logs.Error("delete department subtree error: %s", err.Error())
		return err
	}
	return tx.Commit()
}

// listDepartmentSubtreeIds returns the department id plus all descendant ids.
func listDepartmentSubtreeIds(adminId, id int) ([]int, error) {
	rows, err := msql.RawValues(define.Postgres, `WITH RECURSIVE sub AS (
			SELECT id FROM departments WHERE admin_id = $1 AND id = $2
			UNION ALL
			SELECT d.id FROM departments d JOIN sub s ON d.parent_id = s.id WHERE d.admin_id = $1
		) SELECT id FROM sub`, nil, adminId, id)
	if err != nil {
		logs.Error("list department subtree error: %s", err.Error())
		return nil, err
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, cast.ToInt(row[`id`]))
	}
	return ids, nil
}

// expandDepartmentSubtreeIds returns the given departments plus all of their descendant
// department ids, deduped. It loads the tenant's department list once instead of running
// one recursive query per department.
func expandDepartmentSubtreeIds(adminId int, ids []int) ([]int, error) {
	departments, err := ListDepartments(adminId)
	if err != nil {
		return nil, err
	}
	children := make(map[int][]int, len(departments))
	for _, dept := range departments {
		children[dept.ParentId] = append(children[dept.ParentId], dept.Id)
	}

	seen := make(map[int]bool, len(ids))
	result := make([]int, 0, len(ids))
	queue := append([]int{}, ids...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
		queue = append(queue, children[id]...)
	}
	return result, nil
}

// departmentDepth returns the depth of the node identified by parentId (0 = tree top, depth 0).
// A department whose parent_id is 0 sits at depth 1.
func departmentDepth(adminId, parentId int) (int, error) {
	depth := 0
	current := parentId
	for current != 0 {
		row, err := getDepartment(adminId, current)
		if err != nil {
			return 0, err
		}
		if len(row) == 0 {
			return 0, ErrDepartmentNotFound
		}
		depth++
		if depth > DepartmentMaxDepth {
			return 0, ErrDepartmentDepthExceeded
		}
		current = cast.ToInt(row[`parent_id`])
	}
	return depth, nil
}

func getDepartment(adminId, id int) (msql.Params, error) {
	m := msql.Model(define.TableDepartments, define.Postgres)
	row, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`id`, cast.ToString(id)},
	}).Find()
	if err != nil {
		logs.Error("get department error: sql=%s, err=%s", m.GetLastSql(), err.Error())
	}
	return row, err
}

func departmentNameExists(adminId, parentId int, name string, excludedId int) (bool, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM departments
		WHERE admin_id = $1 AND parent_id = $2 AND name = $3 AND ($4 = 0 OR id <> $4) LIMIT 1`,
		nil, adminId, parentId, name, excludedId)
	if err != nil {
		logs.Error("check department name error: %s", err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}

func isDepartmentNameConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), `idx_departments_admin_parent_name`)
}

// ======================== Member-Department Associations ========================

// ValidateTenantDepartmentIds dedupes the ids and requires every one to belong to the tenant.
func ValidateTenantDepartmentIds(adminId int, ids []int) ([]int, error) {
	result := make([]int, 0, len(ids))
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		dept, err := getDepartment(adminId, id)
		if err != nil {
			return nil, err
		}
		if len(dept) == 0 {
			return nil, ErrDepartmentInvalid
		}
		seen[id] = true
		result = append(result, id)
	}
	return result, nil
}

// FilterExistingDepartmentIds dedupes the ids and silently drops ones that no longer exist
// (e.g. a department deleted while an invitation was still active).
func FilterExistingDepartmentIds(adminId int, ids []int) []int {
	result := make([]int, 0, len(ids))
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		dept, err := getDepartment(adminId, id)
		if err != nil || len(dept) == 0 {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// SetUserDepartments replaces a user's department associations.
func SetUserDepartments(adminId, userId int, departmentIds []int) error {
	m := msql.Model(define.TableUserDepartments, define.Postgres)
	if _, err := m.Where2([][]string{
		{`admin_id`, cast.ToString(adminId)},
		{`user_id`, cast.ToString(userId)},
	}).Delete(); err != nil {
		logs.Error("clear user departments error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return err
	}
	return insertUserDepartments(nil, adminId, userId, departmentIds)
}

// setUserDepartmentsTx inserts a user's department associations inside a transaction (new user, no old rows to clear).
func setUserDepartmentsTx(tx *sql.Tx, adminId, userId int, departmentIds []int) error {
	return insertUserDepartments(tx, adminId, userId, departmentIds)
}

// replaceUserDepartmentsTx 在一个事务内先删除再重新插入成员的部门关联。
// SetUserDepartments 非事务（中途报错会让成员落到无部门状态），setUserDepartmentsTx
// 只插入（换部门时残留旧行）；同步需要两者原子完成，所以单独给一个 helper。
func replaceUserDepartmentsTx(tx *sql.Tx, adminId, userId int, departmentIds []int) error {
	if _, err := msql.RawExec(define.Postgres, `DELETE FROM user_departments
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId); err != nil {
		logs.Error("clear user departments error: %s", err.Error())
		return err
	}
	return insertUserDepartments(tx, adminId, userId, departmentIds)
}

func insertUserDepartments(tx *sql.Tx, adminId, userId int, departmentIds []int) error {
	now := tool.Time2Int()
	for _, departmentId := range departmentIds {
		if _, err := msql.RawExec(define.Postgres, `INSERT INTO user_departments
			(admin_id, user_id, department_id, created_at) VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, department_id) DO NOTHING`, tx, adminId, userId, departmentId, now); err != nil {
			logs.Error("insert user department error: %s", err.Error())
			return err
		}
	}
	return nil
}

// RemoveUserDepartments removes every department binding of a member (member removal). tx may be nil.
func RemoveUserDepartments(tx *sql.Tx, adminId, userId int) error {
	_, err := msql.RawExec(define.Postgres, `DELETE FROM `+define.TableUserDepartments+`
		WHERE admin_id = $1 AND user_id = $2`, tx, adminId, userId)
	if err != nil {
		logs.Error("remove user departments error: %s", err.Error())
	}
	return err
}

// ListUserDepartmentIds returns every member's department ids for the tenant.
func ListUserDepartmentIds(adminId int) (map[int][]int, error) {
	m := msql.Model(define.TableUserDepartments, define.Postgres)
	rows, err := m.Where(`admin_id`, cast.ToString(adminId)).
		Field(`user_id, department_id`).
		Order(`department_id ASC`).
		Select()
	if err != nil {
		logs.Error("list user department ids error: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return nil, err
	}
	result := make(map[int][]int, len(rows))
	for _, row := range rows {
		userId := cast.ToInt(row[`user_id`])
		result[userId] = append(result[userId], cast.ToInt(row[`department_id`]))
	}
	return result, nil
}

// ======================== Department Paths (Excel import/export) ========================

// DepartmentPathNames returns each department's full path, e.g. "产品部/设计组".
func DepartmentPathNames(adminId int) (map[int]string, error) {
	items, err := ListDepartments(adminId)
	if err != nil {
		return nil, err
	}
	byId := make(map[int]DepartmentModel, len(items))
	for _, item := range items {
		byId[item.Id] = item
	}
	paths := make(map[int]string, len(items))
	for _, item := range items {
		segments := make([]string, 0, DepartmentMaxDepth)
		current, ok := item, true
		for steps := 0; ok && steps < DepartmentMaxDepth; steps++ {
			segments = append([]string{current.Name}, segments...)
			if current.ParentId == 0 {
				break
			}
			current, ok = byId[current.ParentId]
		}
		paths[item.Id] = strings.Join(segments, `/`)
	}
	return paths, nil
}

// EnsureDepartmentPath resolves a path of names from the top level, creating missing nodes, and returns the leaf id.
func EnsureDepartmentPath(adminId int, names []string) (int, error) {
	if len(names) == 0 {
		return 0, ErrDepartmentInvalid
	}
	if len(names) > DepartmentMaxDepth {
		return 0, ErrDepartmentDepthExceeded
	}
	parentId := 0
	for _, rawName := range names {
		name, err := NormalizeDepartmentName(rawName)
		if err != nil {
			return 0, err
		}
		rows, err := msql.RawValues(define.Postgres, `SELECT id FROM departments
			WHERE admin_id = $1 AND parent_id = $2 AND name = $3 LIMIT 1`, nil, adminId, parentId, name)
		if err != nil {
			logs.Error("find department by path error: %s", err.Error())
			return 0, err
		}
		if len(rows) > 0 {
			parentId = cast.ToInt(rows[0][`id`])
			continue
		}
		id, err := CreateDepartment(adminId, parentId, name)
		if err != nil {
			// Unique-index fallback for concurrent creation: re-query once on conflict.
			if errors.Is(err, ErrDepartmentNameExists) {
				retry, retryErr := msql.RawValues(define.Postgres, `SELECT id FROM departments
					WHERE admin_id = $1 AND parent_id = $2 AND name = $3 LIMIT 1`, nil, adminId, parentId, name)
				if retryErr == nil && len(retry) > 0 {
					parentId = cast.ToInt(retry[0][`id`])
					continue
				}
			}
			return 0, err
		}
		parentId = id
	}
	return parentId, nil
}

func joinInts(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	if len(parts) == 0 {
		return `0`
	}
	return strings.Join(parts, `,`)
}
