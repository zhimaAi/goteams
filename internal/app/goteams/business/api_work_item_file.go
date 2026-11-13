// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/zhimaAi/go_tools/msql"
)

// ApiWorkItemFileModel 是下载工作项内素材所需的最小字段。
type ApiWorkItemFileModel struct {
	Name        string
	ContentType string
	StoredPath  string
}

// GetApiWorkItemFile 按素材地址定位工作项名下的一个文件。
//
// 地址就是入库时的 stored_path：富文本里 img/video 的 src 与附件列表返回的 url 都是它，
// 也可能被调用方带上域名或 OSS 前缀，所以先按 stored_path 精确匹配，再按路径末段兜底。
// 两种匹配都限定在「本租户 + 本工作项」的行内，拿别的地址来试探只会落空，读不到别处的文件。
func GetApiWorkItemFile(adminId, workspaceId, itemId int, src string) (ApiWorkItemFileModel, error) {
	model := ApiWorkItemFileModel{}
	name := apiFileStoredName(src)
	if name == `` {
		return model, ErrAttachmentNotFound
	}
	tableName := define.GetEnterpriseFileTable(adminId)
	rows, err := msql.RawValues(define.Postgres, `SELECT file_name, content_type, stored_path FROM `+tableName+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = $3
			AND (stored_path = $4 OR stored_path LIKE $5 ESCAPE '\')
		ORDER BY id DESC LIMIT 1`, nil,
		adminId, workspaceId, itemId, strings.TrimSpace(src), `%/`+escapeLikePattern(name))
	if err != nil {
		return model, err
	}
	if len(rows) == 0 {
		return model, ErrAttachmentNotFound
	}
	model.Name = rows[0][`file_name`]
	model.ContentType = rows[0][`content_type`]
	model.StoredPath = rows[0][`stored_path`]
	// 文件名会进 Content-Disposition，带路径分隔符的名字退回地址末段，避免写出非法文件名。
	if model.Name == `` || strings.ContainsAny(model.Name, `\/`) {
		model.Name = name
	}
	return model, nil
}

// apiFileStoredName 从素材地址里取出末段文件名：先去掉 query 与 fragment，再取最后一个 / 或 \ 之后的片段。
func apiFileStoredName(src string) string {
	src = strings.TrimSpace(src)
	if src == `` {
		return ``
	}
	if index := strings.IndexAny(src, `?#`); index >= 0 {
		src = src[:index]
	}
	src = strings.TrimRight(src, `\/`)
	if index := strings.LastIndexAny(src, `\/`); index >= 0 {
		src = src[index+1:]
	}
	return src
}

// escapeLikePattern 转义 LIKE 的通配符：文件名里的 % 与 _ 必须按字面匹配，不能当模式用。
func escapeLikePattern(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
