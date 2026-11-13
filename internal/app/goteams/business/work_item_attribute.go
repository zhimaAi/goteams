// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ======================== Work Item Attributes ========================

// workItemAttributeQueryFields are the work item attribute table's query fields.
const workItemAttributeQueryFields = `id,type_key,name,icon_color,icon_text,description`

// WorkItemAttributeModel is one configured work item attribute row (a workspace+type config).
type WorkItemAttributeModel struct {
	Id          int
	TypeKey     string
	Name        string
	IconColor   string
	IconText    string
	Description string
}

// ListWorkItemAttributesModel describes the read use case.
type ListWorkItemAttributesModel struct {
	AdminId int
	WsId    string
	TypeKey string // Empty means both requirement and defect.
	Items   []WorkItemAttributeModel
}

// SaveWorkItemAttributeModel describes the upsert use case (insert or update by workspace+type).
type SaveWorkItemAttributeModel struct {
	AdminId     int
	WsId        string
	TypeKey     string
	Name        string
	IconColor   string
	IconText    string
	Description string
}

// ListWorkItemAttributes returns configured attribute rows of the workspace (optionally by type).
func ListWorkItemAttributes(model ListWorkItemAttributesModel) (ListWorkItemAttributesModel, error) {
	m := msql.Model(define.TableWorkItemAttributes, define.Postgres)
	wheres := [][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
	}
	if len(model.TypeKey) > 0 {
		wheres = append(wheres, []string{`type_key`, model.TypeKey})
	}
	list, err := m.Field(workItemAttributeQueryFields).Where2(wheres).Order(`type_key asc, id asc`).Select()
	if err != nil {
		logs.Error("sql:%s, err:%s", m.GetLastSql(), err.Error())
		return ListWorkItemAttributesModel{}, err
	}
	items := make([]WorkItemAttributeModel, 0, len(list))
	for _, row := range list {
		items = append(items, WorkItemAttributeModel{
			Id:          cast.ToInt(row[`id`]),
			TypeKey:     row[`type_key`],
			Name:        row[`name`],
			IconColor:   row[`icon_color`],
			IconText:    row[`icon_text`],
			Description: row[`description`],
		})
	}
	model.Items = items
	return model, nil
}

// SaveWorkItemAttribute inserts or updates the attribute row of a workspace+type (multi-tenant scoped).
func SaveWorkItemAttribute(model SaveWorkItemAttributeModel) error {
	now := tool.Time2Int()
	data := msql.Datas{
		`name`:        model.Name,
		`icon_color`:  model.IconColor,
		`icon_text`:   model.IconText,
		`description`: model.Description,
		`updated_at`:  now,
	}

	queryM := msql.Model(define.TableWorkItemAttributes, define.Postgres)
	exist, err := queryM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Value(`id`)
	if err != nil {
		logs.Error("sql:%s, err:%s", queryM.GetLastSql(), err.Error())
		return err
	}
	if len(exist) == 0 {
		data[`admin_id`] = model.AdminId
		data[`workspace_id`] = model.WsId
		data[`type_key`] = model.TypeKey
		data[`created_at`] = now
		insertM := msql.Model(define.TableWorkItemAttributes, define.Postgres)
		_, err = insertM.Insert(data, `id`)
		if err != nil {
			logs.Error("sql:%s, err:%s", insertM.GetLastSql(), err.Error())
		}
		return err
	}
	updateM := msql.Model(define.TableWorkItemAttributes, define.Postgres)
	_, err = updateM.Where2([][]string{
		{`admin_id`, cast.ToString(model.AdminId)},
		{`workspace_id`, model.WsId},
		{`type_key`, model.TypeKey},
	}).Update(data)
	if err != nil {
		logs.Error("sql:%s, err:%s", updateM.GetLastSql(), err.Error())
		return fmt.Errorf("update work item attributes: %w", err)
	}
	return nil
}
