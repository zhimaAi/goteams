package business

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ErrSkillNotFound 表示技能不存在。
var ErrSkillNotFound = errors.New("skill not found")

// ErrSkillChanged 表示审核期间技能来源或审核状态已被其他请求修改。
var ErrSkillChanged = errors.New("skill changed during review")

// ErrSkillSourceURLExists 表示来源地址已被其他技能使用。
var ErrSkillSourceURLExists = errors.New("skill source URL already exists")

// SkillModel 是系统级技能，不包含任何账号或租户身份。
type SkillModel struct {
	ID           int
	Name         string
	Remark       string
	Icon         string
	Description  string
	Sharer       string
	SourceURL    string
	Source       string
	ReviewStatus string
	CreatedAt    int64
	UpdatedAt    int64
}

// CreateSkillModel 是创建技能的输入。
type CreateSkillModel struct {
	Name        string
	Remark      string
	Icon        string
	Description string
	Sharer      string
	SourceURL   string
	Source      string
}

// UpdateSkillModel 只更新提供的字段，允许清空可选文本。
type UpdateSkillModel struct {
	ID           int
	Name         *string
	Remark       *string
	Icon         *string
	Description  *string
	Sharer       *string
	SourceURL    *string
	Source       *string
	ReviewStatus *string
}

// ListSkillsModel 是分页与审核筛选条件。
type ListSkillsModel struct {
	Page         int
	Size         int
	Name         string
	ReviewStatus string
}

// SkillListModel 是技能分页结果。
type SkillListModel struct {
	List  []SkillModel
	Total int
	Page  int
	Size  int
}

const skillFields = "id,name,remark,icon,description,sharer,source_url,source,review_status,created_at,updated_at"

func toSkillModel(row msql.Params) (SkillModel, error) {
	for _, field := range strings.Split(skillFields, ",") {
		if _, ok := row[field]; !ok {
			return SkillModel{}, fmt.Errorf("missing skill column: %s", field)
		}
	}
	id, err := strconv.Atoi(row["id"])
	if err != nil || id <= 0 {
		return SkillModel{}, fmt.Errorf("invalid skill id: %q", row["id"])
	}
	createdAt, err := strconv.ParseInt(row["created_at"], 10, 64)
	if err != nil {
		return SkillModel{}, fmt.Errorf("parse skill created_at: %w", err)
	}
	updatedAt, err := strconv.ParseInt(row["updated_at"], 10, 64)
	if err != nil {
		return SkillModel{}, fmt.Errorf("parse skill updated_at: %w", err)
	}
	return SkillModel{
		ID: id, Name: row["name"], Remark: row["remark"], Icon: row["icon"], Description: row["description"],
		Sharer: row["sharer"], SourceURL: row["source_url"], Source: row["source"],
		ReviewStatus: row["review_status"], CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

// CreateSkill 创建技能，由来源及 GitHub Star 数量决定审核状态。
func CreateSkill(model CreateSkillModel) (int64, error) {
	// 创建和编辑共用字段规则，避免两种写入口的约束漂移。
	fields := UpdateSkillModel{
		Name: &model.Name, Remark: &model.Remark, Icon: &model.Icon, Description: &model.Description, Sharer: &model.Sharer,
		SourceURL: &model.SourceURL, Source: &model.Source,
	}
	if err := validateSkillUpdate(fields); err != nil {
		return 0, err
	}
	if err := checkSkillSourceURL(model.SourceURL, 0); err != nil {
		return 0, err
	}
	reviewStatus, err := reviewSkill(model.Source, model.SourceURL)
	if err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	m := msql.Model(define.TableSkills, define.Postgres)
	id, err := m.Insert(msql.Datas{
		"name": model.Name, "remark": model.Remark, "icon": model.Icon, "description": model.Description, "sharer": model.Sharer,
		"source_url": model.SourceURL, "source_url_hash": skillSourceURLHash(model.SourceURL), "source": model.Source, "review_status": reviewStatus,
		"created_at": now, "updated_at": now,
	}, "id")
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		if isSkillSourceURLConflict(err) {
			return 0, ErrSkillSourceURLExists
		}
	}
	return id, err
}

func skillSourceURLHash(sourceURL string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(sourceURL)))
}

func isSkillSourceURLConflict(err error) bool {
	var pgErr *pq.Error
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.Constraint == "uq_skills_source_url_hash"
}

func checkSkillSourceURL(sourceURL string, excludeID int) error {
	m := msql.Model(define.TableSkills, define.Postgres).Where("source_url_hash", skillSourceURLHash(sourceURL)).Field("id")
	if excludeID > 0 {
		m.WhereRaw("id <> $1", excludeID)
	}
	row, err := m.Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if len(row) > 0 {
		return ErrSkillSourceURLExists
	}
	return nil
}

func validateSkillUpdate(model UpdateSkillModel) error {
	if model.Name != nil {
		if err := validate.ValidateSkillName(*model.Name); err != nil {
			return err
		}
	}
	for _, field := range []struct {
		name  string
		value *string
		limit int
	}{{"remark", model.Remark, 1000}, {"icon", model.Icon, 2048}, {"description", model.Description, 10000}, {"sharer", model.Sharer, 100}} {
		if field.value != nil {
			if err := validate.ValidateSkillText(field.name, *field.value, field.limit); err != nil {
				return err
			}
		}
	}
	if model.SourceURL != nil {
		if err := validate.ValidateSkillSourceURL(*model.SourceURL); err != nil {
			return err
		}
	}
	if model.Source != nil {
		if err := validate.ValidateSkillSource(*model.Source); err != nil {
			return err
		}
	}
	if model.ReviewStatus != nil {
		if err := validate.ValidateSkillReviewStatus(*model.ReviewStatus); err != nil {
			return err
		}
	}
	return nil
}

// ListSkills 按名称或描述包含匹配及审核状态组合筛选并分页。
func ListSkills(model ListSkillsModel) (SkillListModel, error) {
	if model.Page <= 0 || model.Size <= 0 || model.Size > 100 || model.Page-1 > (int(^uint(0)>>1)/model.Size) {
		return SkillListModel{}, &validate.FieldError{Field: "page", Key: define.MsgParamInvalid}
	}
	if model.ReviewStatus != "" {
		if err := validate.ValidateSkillReviewStatus(model.ReviewStatus); err != nil {
			return SkillListModel{}, err
		}
	}
	model.Name = strings.TrimSpace(model.Name)
	if err := validate.ValidateSkillText("name", model.Name, 100); err != nil {
		return SkillListModel{}, err
	}
	m := msql.Model(define.TableSkills, define.Postgres).Field(skillFields)
	if model.Name != "" {
		pattern := "%" + escapeLikePattern(model.Name) + "%"
		m.WhereRaw(`(name ILIKE $1 ESCAPE '\' OR description ILIKE $2 ESCAPE '\')`, pattern, pattern)
	}
	if model.ReviewStatus != "" {
		m.Where("review_status", model.ReviewStatus)
	}
	rows, total, err := m.Order("id DESC").Paginate(model.Page, model.Size)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return SkillListModel{}, err
	}
	result := SkillListModel{List: make([]SkillModel, 0, len(rows)), Total: total, Page: model.Page, Size: model.Size}
	for _, row := range rows {
		item, err := toSkillModel(row)
		if err != nil {
			return SkillListModel{}, err
		}
		result.List = append(result.List, item)
	}
	return result, nil
}

// GetSkill 获取技能详情。
func GetSkill(id int) (SkillModel, error) {
	if id <= 0 {
		return SkillModel{}, &validate.FieldError{Field: "skillId", Key: define.MsgParamInvalid}
	}
	m := msql.Model(define.TableSkills, define.Postgres).Where("id", strconv.Itoa(id)).Field(skillFields)
	row, err := m.Find()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return SkillModel{}, err
	}
	if len(row) == 0 {
		return SkillModel{}, ErrSkillNotFound
	}
	return toSkillModel(row)
}

// UpdateSkill 更新技能，来源变更时重新审核，其他编辑保留未提供的状态。
func UpdateSkill(model UpdateSkillModel) error {
	if model.ID <= 0 {
		return &validate.FieldError{Field: "skillId", Key: define.MsgParamInvalid}
	}
	if err := validateSkillUpdate(model); err != nil {
		return err
	}
	data := msql.Datas{}
	for _, field := range []struct {
		name  string
		value *string
	}{
		{"name", model.Name}, {"remark", model.Remark}, {"icon", model.Icon}, {"description", model.Description}, {"sharer", model.Sharer},
		{"source_url", model.SourceURL}, {"source", model.Source}, {"review_status", model.ReviewStatus},
	} {
		if field.value != nil {
			data[field.name] = *field.value
		}
	}
	if len(data) == 0 {
		return &validate.FieldError{Field: "body", Key: define.MsgParamInvalid}
	}
	if model.SourceURL != nil {
		data["source_url_hash"] = skillSourceURLHash(*model.SourceURL)
	}
	m := msql.Model(define.TableSkills, define.Postgres).Where("id", strconv.Itoa(model.ID))
	if model.Source != nil || model.SourceURL != nil {
		current, err := GetSkill(model.ID)
		if err != nil {
			return err
		}
		if model.SourceURL != nil {
			if err := checkSkillSourceURL(*model.SourceURL, model.ID); err != nil {
				return err
			}
		}
		status, err := reviewSkillUpdate(current, model)
		if err != nil {
			return err
		}
		if status != nil {
			data["review_status"] = *status
		}
		// 远程查询期间不持有数据库锁，写入时校验所依据的来源和状态。
		m.Where("source", current.Source).Where("source_url", current.SourceURL).Where("review_status", current.ReviewStatus)
	}
	data["updated_at"] = strconv.FormatInt(time.Now().Unix(), 10)
	affected, err := m.Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		if isSkillSourceURLConflict(err) {
			return ErrSkillSourceURLExists
		}
		return err
	}
	if affected == 0 {
		if model.Source != nil || model.SourceURL != nil {
			if _, err := GetSkill(model.ID); err != nil {
				return err
			}
			return ErrSkillChanged
		}
		return ErrSkillNotFound
	}
	return nil
}

// DeleteSkill 按主键删除系统级技能。
func DeleteSkill(id int) error {
	if id <= 0 {
		return &validate.FieldError{Field: "skillId", Key: define.MsgParamInvalid}
	}
	m := msql.Model(define.TableSkills, define.Postgres).Where("id", strconv.Itoa(id))
	affected, err := m.Delete()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return err
	}
	if affected == 0 {
		return ErrSkillNotFound
	}
	return nil
}
