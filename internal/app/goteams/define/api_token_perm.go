// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package define

import "strings"

// API token 权限点。这是 token 自带的 scope，与 RBAC（permission.go）无关，
// 由开放 API 中间件校验，不进 AllPermissions。
const (
	ApiPermMyWorkList      = `my_work:list`
	ApiPermWorkspaceList   = `workspace:list`
	ApiPermWorkItemList    = `work_item:list`
	ApiPermWorkItemView    = `work_item:view`
	ApiPermWorkItemCreate  = `work_item:create`
	ApiPermWorkItemComment = `work_item:comment`
	ApiPermWorkItemUpdate  = `work_item:update`
)

// AllApiTokenPermissions 是全部权限点，顺序即前端复选框的展示顺序。
var AllApiTokenPermissions = []string{
	ApiPermMyWorkList,
	ApiPermWorkspaceList,
	ApiPermWorkItemList,
	ApiPermWorkItemView,
	ApiPermWorkItemCreate,
	ApiPermWorkItemComment,
	ApiPermWorkItemUpdate,
}

// IsApiTokenPermission 报告该字符串是否是已定义的权限点。
func IsApiTokenPermission(perm string) bool {
	for _, item := range AllApiTokenPermissions {
		if item == perm {
			return true
		}
	}
	return false
}

// NormalizeApiTokenPermissions 去重并丢弃未定义的权限点，输出按 AllApiTokenPermissions 的顺序稳定排列。
func NormalizeApiTokenPermissions(perms []string) []string {
	seen := make(map[string]bool, len(perms))
	for _, perm := range perms {
		seen[perm] = true
	}
	out := make([]string, 0, len(seen))
	for _, perm := range AllApiTokenPermissions {
		if seen[perm] {
			out = append(out, perm)
		}
	}
	return out
}

// ApiTokenPermissionNameKey 返回权限点展示名对应的 i18n key。
func ApiTokenPermissionNameKey(perm string) string {
	return `api_perm_` + strings.ReplaceAll(perm, `:`, `_`)
}
