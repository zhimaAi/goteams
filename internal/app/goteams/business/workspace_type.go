package business

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

var (
	ErrWorkspaceTypeForbidden = errors.New("workspace type management forbidden")
	ErrWorkspaceTypeExists    = errors.New("workspace type name exists")
	ErrWorkspaceTypeInvalid   = errors.New("workspace type not configured")
)

// WorkspaceTypeModel 表示数据库或离线模板包中声明的项目类型。
type WorkspaceTypeModel struct {
	Key  string
	Name string
}

// ListWorkspaceTypesModel 提供可选项目类型及当前用户的维护能力。
type ListWorkspaceTypesModel struct {
	AdminId   int
	UserId    int
	Items     []WorkspaceTypeModel
	CanManage bool
}

// CreateWorkspaceTypeModel 包含类型创建的可信身份与用户输入。
type CreateWorkspaceTypeModel struct {
	AdminId int
	UserId  int
	Name    string
}

// CanManageWorkspaceTypes 仅允许配置指定的模板租户 owner 维护部署级类型。
func CanManageWorkspaceTypes(adminId, userId int) bool {
	return adminId > 0 && adminId == GetTemplateAdminId() && IsTenantOwner(adminId, userId)
}

// ListWorkspaceTypes 合并本部署字典及离线模板包携带的自定义类型。
func ListWorkspaceTypes(model ListWorkspaceTypesModel) (ListWorkspaceTypesModel, error) {
	const query = `SELECT type_key, name FROM ` + define.TableWorkspaceTypes + `
		ORDER BY CASE type_key WHEN 'software' THEN 0 WHEN 'agile' THEN 1 WHEN 'issue' THEN 2 ELSE 3 END, created_at, type_key`
	rows, err := msql.RawValues(define.Postgres, query, nil)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return ListWorkspaceTypesModel{}, err
	}
	model.Items = make([]WorkspaceTypeModel, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		if err := validate.ValidateWorkspaceType(row[`type_key`]); err != nil {
			return ListWorkspaceTypesModel{}, err
		}
		model.Items = append(model.Items, WorkspaceTypeModel{Key: row[`type_key`], Name: row[`name`]})
		seen[row[`type_key`]] = true
	}
	if pkg, err := LoadTemplatePackage(); err == nil {
		for _, row := range pkg.Workspaces() {
			key := row[`workspace_type`]
			if seen[key] || key == "" || row[`workspace_type_name`] == "" {
				continue
			}
			if err := validate.ValidateWorkspaceType(key); err != nil {
				return ListWorkspaceTypesModel{}, err
			}
			model.Items = append(model.Items, WorkspaceTypeModel{Key: key, Name: row[`workspace_type_name`]})
			seen[key] = true
		}
	} else {
		logTemplatePackageUnavailable(`list workspace types`, err)
	}
	model.CanManage = CanManageWorkspaceTypes(model.AdminId, model.UserId)
	return model, nil
}

// CreateWorkspaceType 创建可复用类型；名称冲突由数据库唯一索引裁决。
func CreateWorkspaceType(model CreateWorkspaceTypeModel) (WorkspaceTypeModel, error) {
	if !CanManageWorkspaceTypes(model.AdminId, model.UserId) {
		return WorkspaceTypeModel{}, ErrWorkspaceTypeForbidden
	}
	name := strings.TrimSpace(model.Name)
	if err := validate.ValidateWorkspaceTypeName(name); err != nil {
		return WorkspaceTypeModel{}, err
	}
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return WorkspaceTypeModel{}, err
	}
	key := "custom_" + hex.EncodeToString(random)
	const query = `INSERT INTO ` + define.TableWorkspaceTypes + ` (type_key, name, created_admin_id, created_by, created_at) VALUES ($1,$2,$3,$4,$5)`
	if _, err := msql.RawExec(define.Postgres, query, nil, key, name, model.AdminId, model.UserId, tool.Time2Int()); err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.Constraint == "workspace_types_name_key" {
			return WorkspaceTypeModel{}, ErrWorkspaceTypeExists
		}
		logs.Error("sql:%s, err:%s", query, err.Error())
		return WorkspaceTypeModel{}, err
	}
	return WorkspaceTypeModel{Key: key, Name: name}, nil
}

func workspaceTypeConfigured(key string) (bool, error) {
	const query = `SELECT type_key FROM ` + define.TableWorkspaceTypes + ` WHERE type_key=$1`
	rows, err := msql.RawValues(define.Postgres, query, nil, key)
	if err != nil {
		logs.Error("sql:%s, err:%s", query, err.Error())
		return false, err
	}
	return len(rows) > 0, nil
}
