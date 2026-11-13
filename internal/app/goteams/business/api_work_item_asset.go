// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/lib/pq"
	"github.com/zhimaAi/go_tools/msql"
)

// 开放 API 可上传的富文本素材类型，与界面编辑器的三类上传一一对应。
const (
	ApiWorkItemAssetImage = `image`
	ApiWorkItemAssetFile  = `file`
	ApiWorkItemAssetVideo = `video`
)

var (
	// ErrApiAssetMissing 表示请求里没带文件或文件为空。
	ErrApiAssetMissing = errors.New(`api asset file missing`)
	// ErrApiAssetTooLarge 表示文件超过该类型的上限。
	ErrApiAssetTooLarge = errors.New(`api asset too large`)
	// ErrApiAssetType 表示扩展名不在该类型的白名单里。
	ErrApiAssetType = errors.New(`api asset type invalid`)
	// ErrApiAssetKind 表示素材类型非法。
	ErrApiAssetKind = errors.New(`api asset kind invalid`)
	// ErrApiWorkItemFilesInvalid 表示 image_file_ids 不是本用户在本项目里上传的待回绑素材。
	ErrApiWorkItemFilesInvalid = errors.New(`api work item files invalid`)
)

// apiAssetRule 是一类素材的扩展名白名单与大小上限，规则直接引用界面同一条链路的常量与判断。
type apiAssetRule struct {
	MaxSize   int64
	IsAllowed func(ext string) bool
}

// apiAssetRuleOf 返回该类型的规则；类型非法时 ok 为 false。
func apiAssetRuleOf(kind string) (apiAssetRule, bool) {
	switch kind {
	case ApiWorkItemAssetImage:
		return apiAssetRule{MaxSize: MaxImageSize, IsAllowed: IsAllowedImageExt}, true
	case ApiWorkItemAssetFile:
		return apiAssetRule{MaxSize: MaxEditorFileSize, IsAllowed: IsAllowedEditorFileExt}, true
	case ApiWorkItemAssetVideo:
		return apiAssetRule{MaxSize: MaxEditorVideoSize, IsAllowed: IsAllowedEditorVideoExt}, true
	}
	return apiAssetRule{}, false
}

// ValidateApiWorkItemAsset 校验素材能否上传：类型合法、非空、未超上限、扩展名在白名单内。
// 与落库分开，让 controller 能把参数错误（400）放在占用额度之前。
func ValidateApiWorkItemAsset(kind string, file *multipart.FileHeader) error {
	rule, ok := apiAssetRuleOf(kind)
	if !ok {
		return ErrApiAssetKind
	}
	if file == nil || file.Size <= 0 {
		return ErrApiAssetMissing
	}
	if file.Size > rule.MaxSize {
		return ErrApiAssetTooLarge
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(file.Filename)), `.`)
	if !rule.IsAllowed(ext) {
		return ErrApiAssetType
	}
	return nil
}

// UploadApiWorkItemAsset 把素材写入存储并登记成「待回绑」台账（source_id = 0），与界面编辑器上传同一条链路。
// 调用方拿到地址与素材 ID 后：地址写进正文，素材 ID 放进新建/修改工作项的 image_file_ids，文件才会绑到那条工作项上。
func UploadApiWorkItemAsset(adminId, workspaceId, uploaderId int, file *multipart.FileHeader) (string, int, error) {
	return uploadEditorAsset(adminId, workspaceId, uploaderId, file)
}

// ValidateApiWorkItemPendingFiles 校验待回绑的台账 id 全部是「本租户 + 本项目 + 本 token 归属用户上传、尚未绑定」的文件。
// 必须逐个命中，少一个就整体拒绝：否则可以拿别人的上传 id 把文件绑到自己的工作项上（台账 id 是自增数字）。
func ValidateApiWorkItemPendingFiles(adminId, workspaceId, userId int, fileIds []int) error {
	if len(fileIds) == 0 {
		return nil
	}
	unique := make([]int, 0, len(fileIds))
	seen := make(map[int]bool, len(fileIds))
	for _, id := range fileIds {
		if id <= 0 || seen[id] {
			return ErrApiWorkItemFilesInvalid
		}
		seen[id] = true
		unique = append(unique, id)
	}
	tableName := define.GetEnterpriseFileTable(adminId)
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM `+tableName+`
		WHERE admin_id = $1 AND workspace_id = $2 AND source_id = 0 AND comment_id = 0 AND uploader_id = $3
			AND id = ANY($4::INTEGER[])`, nil,
		adminId, workspaceId, userId, pq.Array(unique))
	if err != nil {
		return err
	}
	if len(rows) != len(unique) {
		return ErrApiWorkItemFilesInvalid
	}
	return nil
}
