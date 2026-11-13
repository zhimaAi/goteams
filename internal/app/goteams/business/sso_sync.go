// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"goteams/internal/app/goteams/define"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
	"github.com/zhimaAi/go_tools/tool"
)

// ErrSsoSyncUnsupported 表示该平台没有成员/部门同步能力。五个平台目前都能同步，
// 保留它以防后续新增只登录不同步的平台；触发时同步整体失败并记 last_error。
var ErrSsoSyncUnsupported = errors.New("sso sync is not supported for this provider")

// ssoScimPageSize 是 SCIM 分页一次取的条数。取大一点减少往返，又不至于撑爆单响应体上限。
const ssoScimPageSize = 100

// ssoSyncMember 是一条成员拉取结果，字段口径与登录侧的 SsoUserInfo 完全一致，
// 保证「同步建的号」和「本人登录建的号」命中同一 external_id。
type ssoSyncMember struct {
	info       SsoUserInfo
	department []string // IdP 侧部门标识，落库前解成本地 department_id
}

// ssoSyncDept 是一条部门拉取结果，parent 为 IdP 侧上级标识（空表示顶层）。
type ssoSyncDept struct {
	externalId string
	name       string
	parent     string
}

// ======================== 字段映射（仅 SCIM：OIDC / OAuth2.0） ========================

// ssoFieldMapping 把「本地字段」映到 IdP 响应里的取值路径。空映射用平台默认值补齐。
// 值支持点号与下标：name.givenName、emails[0].value、memberships[*].value。
type ssoFieldMapping map[string]string

var ssoScimUserDefaults = ssoFieldMapping{
	`external_id`: `id`, `username`: `userName`, `name`: `displayName`,
	`phone`: `phoneNumbers[0].value`, `avatar`: `photos[0].value`, `email`: `emails[0].value`,
	`job_title`: `title`, `employee_no`: `employeeNumber`, `department`: `memberships[*].value`,
}

var ssoScimDeptDefaults = ssoFieldMapping{
	`external_id`: `id`, `name`: `displayName`, `parent_external_id`: ``,
}

// parseSsoFieldMapping 解析配置里的映射 JSON，缺失项回落到默认值。
// 解析失败按「空映射」处理并用默认值：同步宁可拿标准字段跑，也不因一段坏 JSON 整体罢工。
func parseSsoFieldMapping(raw string, defaults ssoFieldMapping) ssoFieldMapping {
	result := make(ssoFieldMapping, len(defaults))
	for key, value := range defaults {
		result[key] = value
	}
	raw = strings.TrimSpace(raw)
	if raw == `` {
		return result
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		logs.Error("parse sso field mapping failed: err=%s", err.Error())
		return result
	}
	for key, value := range parsed {
		if path, ok := value.(string); ok && strings.TrimSpace(path) != `` {
			result[key] = strings.TrimSpace(path)
		}
	}
	return result
}

// ssoPathToken 是一个路径段：字段名 + 可选下标（* 表示取全部）。
type ssoPathToken struct {
	field string
	index int // -1=无下标，-2=[*]
}

var ssoPathSegmentRe = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)(?:\[(\d+|\*)\])?$`)

// ssoSplitPath 把点号路径切成段，非法段忽略。
func ssoSplitPath(path string) []ssoPathToken {
	parts := strings.Split(path, `.`)
	tokens := make([]ssoPathToken, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == `` {
			continue
		}
		match := ssoPathSegmentRe.FindStringSubmatch(part)
		if match == nil {
			continue
		}
		token := ssoPathToken{field: match[1], index: -1}
		switch match[2] {
		case `*`:
			token.index = -2
		case ``:
		default:
			token.index = cast.ToInt(match[2])
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// ssoExtractStrings 沿路径从对象里取值，返回命中的一串字符串（[*] 可能多个）。
func ssoExtractStrings(node any, tokens []ssoPathToken) []string {
	if len(tokens) == 0 {
		return nil
	}
	current := ssoApplyToken(node, tokens[0])
	if len(tokens) == 1 {
		return ssoFlattenStrings(current)
	}
	result := make([]string, 0)
	for _, item := range current {
		result = append(result, ssoExtractStrings(item, tokens[1:])...)
	}
	return result
}

// ssoApplyToken 对单个路径段取值，返回值集合（标量包成一元切片，数组展开）。
func ssoApplyToken(node any, token ssoPathToken) []any {
	object, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	value := object[token.field]
	if token.index == -1 {
		return ssoAsSlice(value)
	}
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	if token.index == -2 {
		return list
	}
	if token.index >= 0 && token.index < len(list) {
		return []any{list[token.index]}
	}
	return nil
}

func ssoAsSlice(value any) []any {
	if value == nil {
		return nil
	}
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{value}
}

// ssoFlattenStrings 把值集合里的标量转成字符串；数组递归展开。
func ssoFlattenStrings(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		switch item := value.(type) {
		case string:
			if text := strings.TrimSpace(item); text != `` {
				result = append(result, text)
			}
		case float64:
			result = append(result, strconv.FormatFloat(item, 'f', -1, 64))
		case bool:
			result = append(result, strconv.FormatBool(item))
		case []any:
			result = append(result, ssoFlattenStrings(item)...)
		}
	}
	return result
}

// ssoFirstString 取路径命中的第一个字符串（单值字段用）。
func ssoFirstString(node any, mapping ssoFieldMapping, key string) string {
	path := mapping[key]
	if path == `` {
		return ``
	}
	values := ssoExtractStrings(node, ssoSplitPath(path))
	if len(values) == 0 {
		return ``
	}
	return values[0]
}

// ssoAllStrings 取路径命中的全部字符串（部门这类多值字段用）。
func ssoAllStrings(node any, mapping ssoFieldMapping, key string) []string {
	path := mapping[key]
	if path == `` {
		return nil
	}
	return ssoExtractStrings(node, ssoSplitPath(path))
}

// ======================== 拉取分发 ========================

// fetchSsoDirectory 按 provider 分派成员与部门拉取：OIDC/OAuth2.0 走 SCIM（受控出站 + 字段映射），
// 钉钉/飞书走各自通讯录 API（写死厂商域名 + 固定结构映射，忽略字段映射）。
// 阶段一纯只读，任一分页失败都返回错误，调用方据此终止整次同步、不写任何业务数据。
// 例外：部门树拉取失败只记日志并跳过部门同步（见 fetchSsoDingtalkDirectory）。
func fetchSsoDirectory(config SsoConfigModel, syncConfig SsoSyncConfigModel) ([]ssoSyncMember, []ssoSyncDept, error) {
	switch config.Provider {
	case define.SsoProviderOidc, define.SsoProviderOauth2:
		return fetchSsoScimDirectory(syncConfig)
	case define.SsoProviderDingtalk:
		return fetchSsoDingtalkDirectory(config)
	// 企业微信本期不开放，分发一并注释；恢复时连同 define.SsoProviders 一起加回。
	// case define.SsoProviderWecom:
	// 	return fetchSsoWecomDirectory(config)
	case define.SsoProviderFeishu:
		return fetchSsoFeishuDirectory(config)
	default:
		return nil, nil, fmt.Errorf("%w: %s", ErrSsoSyncUnsupported, config.Provider)
	}
}

// ======================== SCIM 拉取 ========================

// scimListResponse 是 SCIM 2.0 列资源的通用信封。
type scimListResponse struct {
	TotalResults int              `json:"totalResults"`
	StartIndex   int              `json:"startIndex"`
	ItemsPerPage int              `json:"itemsPerPage"`
	Resources    []map[string]any `json:"Resources"`
}

// ssoScimList 分页拉取一个 SCIM 资源集，读到「无下一页」为止并判完整性。
// 页内累计条数与声明的 totalResults 不符、或中途空页而游标未结束，都按取数失败处理。
func ssoScimList(baseUrl string, resource string, token string) ([]map[string]any, error) {
	base := strings.TrimRight(baseUrl, `/`)
	accessToken := `Bearer ` + token
	accumulated := make([]map[string]any, 0, ssoScimPageSize)
	startIndex := 1
	for {
		target := fmt.Sprintf(`%s/%s?count=%d&startIndex=%d`, base, resource, ssoScimPageSize, startIndex)
		var page scimListResponse
		headers := map[string]string{`Authorization`: accessToken}
		if err := ssoCallJSON(http.MethodGet, target, headers, nil, &page); err != nil {
			return nil, err
		}
		accumulated = append(accumulated, page.Resources...)
		if page.TotalResults == 0 {
			// 空集：首屏就是终页，完整性由「声明总数为 0」保证。
			return accumulated, nil
		}
		if len(page.Resources) == 0 {
			// 声明了非零总数却拿到空页：数据被截断，不能当完整结果继续。
			return nil, fmt.Errorf("%w: scim %s returned an empty page before totalResults", ErrSsoEndpointUnreachable, resource)
		}
		if startIndex+len(page.Resources) > page.TotalResults {
			// 已经取满声明的总数，收口。
			if len(accumulated) != page.TotalResults {
				return nil, fmt.Errorf("%w: scim %s count mismatch: got %d, declared %d",
					ErrSsoEndpointUnreachable, resource, len(accumulated), page.TotalResults)
			}
			return accumulated, nil
		}
		startIndex += len(page.Resources)
	}
}

func fetchSsoScimDirectory(syncConfig SsoSyncConfigModel) ([]ssoSyncMember, []ssoSyncDept, error) {
	if strings.TrimSpace(syncConfig.ScimBaseUrl) == `` || strings.TrimSpace(syncConfig.ScimToken) == `` {
		return nil, nil, fmt.Errorf("%w: scim_base_url/scim_token", ErrSsoConfigIncomplete)
	}
	userMapping := parseSsoFieldMapping(syncConfig.UserFieldMapping, ssoScimUserDefaults)
	deptMapping := parseSsoFieldMapping(syncConfig.DeptFieldMapping, ssoScimDeptDefaults)

	rawUsers, err := ssoScimList(syncConfig.ScimBaseUrl, `Users`, syncConfig.ScimToken)
	if err != nil {
		return nil, nil, err
	}
	rawGroups, err := ssoScimList(syncConfig.ScimBaseUrl, `Groups`, syncConfig.ScimToken)
	if err != nil {
		return nil, nil, err
	}

	members := make([]ssoSyncMember, 0, len(rawUsers))
	for _, raw := range rawUsers {
		info := SsoUserInfo{
			ExternalId: ssoFirstString(raw, userMapping, `external_id`),
			Username:   ssoFirstString(raw, userMapping, `username`),
			Name:       ssoFirstString(raw, userMapping, `name`),
			Phone:      ssoFirstString(raw, userMapping, `phone`),
			Avatar:     ssoFirstString(raw, userMapping, `avatar`),
			Email:      ssoFirstString(raw, userMapping, `email`),
			JobTitle:   ssoFirstString(raw, userMapping, `job_title`),
			EmployeeNo: ssoFirstString(raw, userMapping, `employee_no`),
		}
		if info.ExternalId == `` {
			// 没有身份标识的记录无法幂等落库，整次同步视为数据不合法。
			return nil, nil, fmt.Errorf("%w: scim user without external id", ErrSsoEndpointUnreachable)
		}
		members = append(members, ssoSyncMember{info: info, department: ssoAllStrings(raw, userMapping, `department`)})
	}

	depts := make([]ssoSyncDept, 0, len(rawGroups))
	for _, raw := range rawGroups {
		extId := ssoFirstString(raw, deptMapping, `external_id`)
		if extId == `` {
			return nil, nil, fmt.Errorf("%w: scim group without external id", ErrSsoEndpointUnreachable)
		}
		depts = append(depts, ssoSyncDept{
			externalId: extId,
			name:       ssoFirstString(raw, deptMapping, `name`),
			parent:     ssoFirstString(raw, deptMapping, `parent_external_id`),
		})
	}
	return members, depts, nil
}

// ======================== 厂商通讯录拉取 ========================

// ssoDingtalkDepartment 拉取钉钉部门列表 API 的单个节点（旧版 topapi，字段名沿用官方）。
type ssoDingtalkDepartment struct {
	DeptId   int    `json:"dept_id"`
	Name     string `json:"name"`
	ParentId int    `json:"parent_id"`
}

func fetchSsoDingtalkDirectory(config SsoConfigModel) ([]ssoSyncMember, []ssoSyncDept, error) {
	appToken, err := ssoDingtalkAppToken(config.AppKey, config.AppSecret)
	if err != nil {
		return nil, nil, err
	}
	depts, err := ssoDingtalkCollectDepartments(appToken, ssoDingtalkRootDeptId)
	if err != nil {
		// 部门树不是成员同步的必需项：读不到部门时退化为只取根部门的直属成员。
		// 卡死整次同步会让「先开成员权限、后开部门权限」的接入方一个人都同步不上。
		logs.Error("sso dingtalk department fetch failed, skip department sync: %s", err.Error())
		depts = nil
	}

	// 要取用户的部门 = 根部门 + 每个子部门。listsub 不返回根部门本身，只遍历子部门会丢掉
	// 「没有被分到任何子部门」的成员——企业只有根部门时，结果直接是 0 人。
	deptIds := make([]int, 0, len(depts)+1)
	deptIds = append(deptIds, ssoDingtalkRootDeptId)
	for _, dept := range depts {
		deptId, _ := strconv.Atoi(dept.externalId)
		deptIds = append(deptIds, deptId)
	}

	members := make([]ssoSyncMember, 0)
	for _, deptId := range deptIds {
		list, err := ssoDingtalkDeptUsers(appToken, deptId)
		if err != nil {
			return nil, nil, err
		}
		for _, user := range list {
			extId := strings.TrimSpace(user.UserId)
			if extId == `` {
				return nil, nil, fmt.Errorf("%w: dingtalk member without userid", ErrSsoEndpointUnreachable)
			}
			members = append(members, ssoSyncMember{
				info: SsoUserInfo{
					ExternalId: extId, Name: strings.TrimSpace(user.Name),
					Phone: strings.TrimSpace(user.Mobile), JobTitle: strings.TrimSpace(user.Title),
					EmployeeNo: strings.TrimSpace(user.JobNumber), Avatar: strings.TrimSpace(user.Avatar),
					Email: strings.TrimSpace(user.Email),
				},
				department: user.deptIds(),
			})
		}
	}
	return dedupeSsoMembers(members), depts, nil
}

// ssoDingtalkCollectDepartments 逐层下钻 listsub（只给直接子级），聚成全量部门树。
// 钉钉根部门 id=1 本身不在 listsub 结果里，其直接子级 parent 置空即映射为本地顶层。
func ssoDingtalkCollectDepartments(appToken string, rootId int) ([]ssoSyncDept, error) {
	accumulated := make([]ssoSyncDept, 0)
	queue := []int{rootId}
	for len(queue) > 0 {
		parentId := queue[0]
		queue = queue[1:]
		var resp struct {
			ErrCode int                     `json:"errcode"`
			ErrMsg  string                  `json:"errmsg"`
			Result  []ssoDingtalkDepartment `json:"result"`
		}
		target := ssoDingtalkDeptListURL + `?access_token=` + url.QueryEscape(appToken)
		if err := ssoVendorCallJSON(http.MethodPost, target, nil, map[string]int{`dept_id`: parentId}, &resp); err != nil {
			return nil, err
		}
		if resp.ErrCode != 0 {
			return nil, fmt.Errorf("%w: dingtalk department list rejected: errcode=%d, errmsg=%s",
				ErrSsoEndpointUnreachable, resp.ErrCode, resp.ErrMsg)
		}
		for _, item := range resp.Result {
			parent := strconv.Itoa(parentId)
			if parentId == rootId {
				parent = `` // 根的直属子级在本地即顶层。
			}
			externalId := strconv.Itoa(item.DeptId)
			accumulated = append(accumulated, ssoSyncDept{externalId: externalId, name: item.Name, parent: parent})
			queue = append(queue, item.DeptId)
		}
	}
	return accumulated, nil
}

type ssoDingtalkUser struct {
	UserId     string `json:"userid"`
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	Title      string `json:"title"`
	JobNumber  string `json:"job_number"`
	Avatar     string `json:"avatar"`
	Email      string `json:"email"`
	DeptIdList []int  `json:"dept_id_list"`
}

// deptIds 把钉钉的 dept_id_list 映射成本地部门的 external_id 列表。
// 根部门在本地没有对应部门，必须剔掉：留着它会让整个部门引用解不出来，
// 该成员的部门关联会被整体跳过（成员仍会建出来，但永远挂不上部门）。
func (u ssoDingtalkUser) deptIds() []string {
	result := make([]string, 0, len(u.DeptIdList))
	for _, id := range u.DeptIdList {
		if id == ssoDingtalkRootDeptId {
			continue
		}
		result = append(result, strconv.Itoa(id))
	}
	return result
}

// ssoDingtalkDeptUsers 拉取一个部门下的直属成员（不含子部门）。v2 接口按游标分页：
// 首屏 cursor=0，之后传上一屏返回的 next_cursor；size 上限 100。
func ssoDingtalkDeptUsers(appToken string, deptId int) ([]ssoDingtalkUser, error) {
	cursor := 0
	size := 100
	accumulated := make([]ssoDingtalkUser, 0, size)
	for {
		target := fmt.Sprintf(`%s?access_token=%s`, ssoDingtalkUserListURL, url.QueryEscape(appToken))
		var resp struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
			Result  struct {
				HasMore    bool              `json:"has_more"`
				NextCursor int               `json:"next_cursor"`
				List       []ssoDingtalkUser `json:"list"`
			} `json:"result"`
		}
		body := map[string]any{`dept_id`: deptId, `cursor`: cursor, `size`: size}
		if err := ssoVendorCallJSON(http.MethodPost, target, nil, body, &resp); err != nil {
			return nil, err
		}
		if resp.ErrCode != 0 {
			// errcode/errmsg 必须带出去：钉钉的「没开通权限(60011)」与「部门不在权限范围内(50004)」
			// 在这里长得一模一样，只报 rejected 就没法判断要去开哪个权限。
			return nil, fmt.Errorf("%w: dingtalk user list rejected: errcode=%d, errmsg=%s",
				ErrSsoEndpointUnreachable, resp.ErrCode, resp.ErrMsg)
		}
		accumulated = append(accumulated, resp.Result.List...)
		if !resp.Result.HasMore {
			return accumulated, nil
		}
		// has_more 却没有数据、或游标原地不动，都会让这个循环一直转下去，按取数失败处理。
		if len(resp.Result.List) == 0 || resp.Result.NextCursor == cursor {
			return nil, fmt.Errorf("%w: dingtalk user list cursor not advancing", ErrSsoEndpointUnreachable)
		}
		cursor = resp.Result.NextCursor
	}
}

func fetchSsoWecomDirectory(config SsoConfigModel) ([]ssoSyncMember, []ssoSyncDept, error) {
	accessToken, err := ssoWecomAccessToken(config.AppKey, config.AppSecret)
	if err != nil {
		return nil, nil, err
	}
	var deptResp struct {
		ssoWecomErrorResponse
		Department []struct {
			Id       int    `json:"id"`
			Name     string `json:"name"`
			ParentId int    `json:"parentid"`
		} `json:"department"`
	}
	deptUrl := fmt.Sprintf(`%s?access_token=%s`, ssoWecomDeptListURL, url.QueryEscape(accessToken))
	if err = ssoVendorCallJSON(http.MethodGet, deptUrl, nil, nil, &deptResp); err != nil {
		return nil, nil, err
	}
	if deptResp.ErrCode != 0 {
		return nil, nil, fmt.Errorf("%w: wecom department list rejected", ErrSsoEndpointUnreachable)
	}
	depts := make([]ssoSyncDept, 0, len(deptResp.Department))
	for _, item := range deptResp.Department {
		parent := ``
		if item.ParentId > 0 && item.Id != 1 {
			parent = strconv.Itoa(item.ParentId)
		}
		depts = append(depts, ssoSyncDept{externalId: strconv.Itoa(item.Id), name: item.Name, parent: parent})
	}

	members := make([]ssoSyncMember, 0)
	for _, dept := range depts {
		list, err := ssoWecomDeptMembers(accessToken, dept.externalId)
		if err != nil {
			return nil, nil, err
		}
		members = append(members, list...)
	}
	return dedupeSsoMembers(members), depts, nil
}

// ssoWecomDeptMembers 拉取企微一个部门的直属成员详情。simplelist 只给 userid，
// 这里按 userid 批量取详情代价过高，先用 simplelist 建身份 + 部门关联，资料字段留空由登录补全。
func ssoWecomDeptMembers(accessToken string, deptId string) ([]ssoSyncMember, error) {
	target := fmt.Sprintf(`%s?access_token=%s&department_id=%s`, ssoWecomUserListURL,
		url.QueryEscape(accessToken), url.QueryEscape(deptId))
	var resp struct {
		ssoWecomErrorResponse
		UserList []struct {
			UserId     string `json:"userid"`
			Name       string `json:"name"`
			Department []int  `json:"department"`
			Position   string `json:"position"`
			Mobile     string `json:"mobile"`
			Email      string `json:"email"`
		} `json:"userlist"`
	}
	if err := ssoVendorCallJSON(http.MethodGet, target, nil, nil, &resp); err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("%w: wecom user list rejected", ErrSsoEndpointUnreachable)
	}
	members := make([]ssoSyncMember, 0, len(resp.UserList))
	for _, user := range resp.UserList {
		if strings.TrimSpace(user.UserId) == `` {
			return nil, fmt.Errorf("%w: wecom member without userid", ErrSsoEndpointUnreachable)
		}
		depts := make([]string, 0, len(user.Department))
		for _, id := range user.Department {
			depts = append(depts, strconv.Itoa(id))
		}
		members = append(members, ssoSyncMember{
			info: SsoUserInfo{
				ExternalId: user.UserId, Name: user.Name, Phone: user.Mobile,
				Email: user.Email, JobTitle: user.Position,
			},
			department: depts,
		})
	}
	return members, nil
}

func fetchSsoFeishuDirectory(config SsoConfigModel) ([]ssoSyncMember, []ssoSyncDept, error) {
	token, err := ssoFeishuTenantToken(config.AppKey, config.AppSecret)
	if err != nil {
		return nil, nil, err
	}
	headers := map[string]string{`Authorization`: `Bearer ` + token}
	depts := make([]ssoSyncDept, 0)
	if err = ssoFeishuEachDepartment(headers, ssoFeishuRootDepartment, &depts); err != nil {
		// 与钉钉同一口径：部门树读不到只跳过部门同步，成员仍按根部门下的直属成员同步。
		logs.Error("sso feishu department fetch failed, skip department sync: %s", err.Error())
		depts = nil
	}

	// 要取用户的部门 = 根部门 + 每个子部门。children 不返回根部门本身，只遍历子部门会丢掉
	// 「没有被分到任何子部门」的成员——企业只有根部门时，结果直接是 0 人（与钉钉同一形态的缺陷）。
	deptIds := make([]string, 0, len(depts)+1)
	deptIds = append(deptIds, ssoFeishuRootDepartment)
	for _, dept := range depts {
		deptIds = append(deptIds, dept.externalId)
	}

	members := make([]ssoSyncMember, 0)
	for _, deptId := range deptIds {
		list, err := ssoFeishuDeptUsers(headers, deptId)
		if err != nil {
			return nil, nil, err
		}
		members = append(members, list...)
	}
	return dedupeSsoMembers(members), depts, nil
}

// ssoFeishuEachDepartment 递归展开飞书部门树（children 接口一次只给直接子级）。
// 根的直属子级 parent 置空即映射为本地顶层。
func ssoFeishuEachDepartment(headers map[string]string, parentId string, out *[]ssoSyncDept) error {
	children, err := ssoFeishuChildDepartments(headers, parentId)
	if err != nil {
		return err
	}
	for _, child := range children {
		parent := ``
		if parentId != `` && parentId != ssoFeishuRootDepartment {
			parent = parentId
		}
		*out = append(*out, ssoSyncDept{externalId: child.externalId, name: child.name, parent: parent})
		if err = ssoFeishuEachDepartment(headers, child.externalId, out); err != nil {
			return err
		}
	}
	return nil
}

// ssoFeishuChildDepartments 拉取一个部门的直接子部门，按 page_token 翻到 has_more 为假。
// 部门 id 走路径参数，与 children 返回的 open_department_id 同属一个 ID 类型（接口默认值）。
func ssoFeishuChildDepartments(headers map[string]string, parentId string) ([]ssoSyncDept, error) {
	pageToken := ``
	accumulated := make([]ssoSyncDept, 0)
	for {
		target := ssoFeishuDepartmentsURL + `/` + url.PathEscape(parentId) + `/children?page_size=50`
		if pageToken != `` {
			target += `&page_token=` + url.QueryEscape(pageToken)
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items []struct {
					OpenDepartmentId string `json:"open_department_id"`
					Name             string `json:"name"`
				} `json:"items"`
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
			} `json:"data"`
		}
		if err := ssoVendorCallJSON(http.MethodGet, target, headers, nil, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			// code/msg 必须带出去：飞书的「无部门权限(40004)」与「无父部门权限(40014)」
			// 在这里长得一模一样，只报 rejected 就没法判断要去开哪个权限。
			return nil, fmt.Errorf("%w: feishu department list rejected: code=%d, msg=%s",
				ErrSsoEndpointUnreachable, resp.Code, resp.Msg)
		}
		for _, item := range resp.Data.Items {
			if item.OpenDepartmentId == `` {
				return nil, fmt.Errorf("%w: feishu department without open_department_id", ErrSsoEndpointUnreachable)
			}
			accumulated = append(accumulated, ssoSyncDept{externalId: item.OpenDepartmentId, name: item.Name})
		}
		if !resp.Data.HasMore {
			return accumulated, nil
		}
		// has_more 却没有数据、或 page_token 原地不动，都会让这个循环一直转下去，按取数失败处理。
		if len(resp.Data.Items) == 0 || resp.Data.PageToken == `` || resp.Data.PageToken == pageToken {
			return nil, fmt.Errorf("%w: feishu department list page token not advancing", ErrSsoEndpointUnreachable)
		}
		pageToken = resp.Data.PageToken
	}
}

// ssoFeishuUser 是飞书通讯录的用户对象。avatar 在飞书是「多尺寸对象」而不是字符串，
// 按字符串解析会让整页 JSON 解析失败（json.Unmarshal 遇到类型不符直接报错），不是只丢头像。
type ssoFeishuUser struct {
	OpenId        string   `json:"open_id"`
	Name          string   `json:"name"`
	Mobile        string   `json:"mobile"`
	JobTitle      string   `json:"job_title"`
	EmployeeNo    string   `json:"employee_no"`
	DepartmentIds []string `json:"department_ids"`
	Avatar        struct {
		Avatar240    string `json:"avatar_240"`
		AvatarOrigin string `json:"avatar_origin"`
	} `json:"avatar"`
}

// avatarUrl 取一个可用尺寸的头像地址：240 优先，原图兜底。
func (u ssoFeishuUser) avatarUrl() string {
	if avatar := strings.TrimSpace(u.Avatar.Avatar240); avatar != `` {
		return avatar
	}
	return strings.TrimSpace(u.Avatar.AvatarOrigin)
}

// deptIds 把用户的 department_ids 映射成本地部门的 external_id 列表。
// 根部门在本地没有对应部门，必须剔掉：留着它会让整个部门引用解不出来，
// 该成员的部门关联会被整体跳过（成员仍会建出来，但永远挂不上部门）。
// department_ids 需要「获取用户组织架构信息」字段权限，缺失时回退成当前查询的部门。
func (u ssoFeishuUser) deptIds(fallback string) []string {
	result := make([]string, 0, len(u.DepartmentIds))
	for _, id := range u.DepartmentIds {
		if id = strings.TrimSpace(id); id == `` || id == ssoFeishuRootDepartment {
			continue
		}
		result = append(result, id)
	}
	if len(result) == 0 && fallback != `` && fallback != ssoFeishuRootDepartment {
		result = append(result, fallback)
	}
	return result
}

// ssoFeishuDeptUsers 拉取一个部门的直属成员，按 page_token 翻到 has_more 为假。
func ssoFeishuDeptUsers(headers map[string]string, deptId string) ([]ssoSyncMember, error) {
	pageToken := ``
	accumulated := make([]ssoSyncMember, 0)
	for {
		target := ssoFeishuUsersURL + `?department_id=` + url.QueryEscape(deptId) + `&page_size=50`
		if pageToken != `` {
			target += `&page_token=` + url.QueryEscape(pageToken)
		}
		var resp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Items     []ssoFeishuUser `json:"items"`
				HasMore   bool            `json:"has_more"`
				PageToken string          `json:"page_token"`
			} `json:"data"`
		}
		if err := ssoVendorCallJSON(http.MethodGet, target, headers, nil, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			// 与部门列表同理：40004（无部门权限）与 41050（用户不在通讯录权限范围）要靠 code 区分。
			return nil, fmt.Errorf("%w: feishu user list rejected: code=%d, msg=%s",
				ErrSsoEndpointUnreachable, resp.Code, resp.Msg)
		}
		for _, user := range resp.Data.Items {
			if user.OpenId == `` {
				return nil, fmt.Errorf("%w: feishu member without open_id", ErrSsoEndpointUnreachable)
			}
			accumulated = append(accumulated, ssoSyncMember{
				info: SsoUserInfo{
					ExternalId: user.OpenId, Name: user.Name, Phone: user.Mobile,
					JobTitle: user.JobTitle, EmployeeNo: user.EmployeeNo, Avatar: user.avatarUrl(),
				},
				department: user.deptIds(deptId),
			})
		}
		if !resp.Data.HasMore {
			return accumulated, nil
		}
		if len(resp.Data.Items) == 0 || resp.Data.PageToken == `` || resp.Data.PageToken == pageToken {
			return nil, fmt.Errorf("%w: feishu user list page token not advancing", ErrSsoEndpointUnreachable)
		}
		pageToken = resp.Data.PageToken
	}
}

// dedupeSsoMembers 按 external_id 去重（一家企业成员可能挂在多个部门下被重复列出）。
// 重复出现时合并部门引用而不是丢掉后来的：飞书按部门逐个拉取，同一成员在每个所属部门下各出现一次，
// 只保留首次出现会丢掉他在其他部门下的归属（部门引用由 syncSsoMember 再按本地 id 去重）。
func dedupeSsoMembers(members []ssoSyncMember) []ssoSyncMember {
	index := make(map[string]int, len(members))
	result := make([]ssoSyncMember, 0, len(members))
	for _, member := range members {
		at, ok := index[member.info.ExternalId]
		if !ok {
			index[member.info.ExternalId] = len(result)
			result = append(result, member)
			continue
		}
		result[at].department = append(result[at].department, member.department...)
	}
	return result
}

// ======================== 落库（阶段二） ========================

// ssoDeptNode 是落库前的部门处理单元，parentResolvable 标记其上级能否解到本地部门。
type ssoDeptNode struct {
	dept     ssoSyncDept
	children []*ssoDeptNode
}

// runSsoSync 执行一次完整同步：取数 → 部门两阶段落库 → 逐成员事务，结果写回 sso_sync_configs。
// 取数阶段任何失败都直接终止，不写业务数据；落库阶段单条成员失败只回滚该条、计入 failed_count。
func runSsoSync(adminId int) error {
	config, err := GetSsoConfig(adminId)
	if err != nil {
		recordSsoSyncResult(adminId, define.SsoSyncStatusFailed, 0, 0, err.Error())
		return err
	}
	syncConfig, err := GetSsoSyncConfig(adminId)
	if err != nil {
		recordSsoSyncResult(adminId, define.SsoSyncStatusFailed, 0, 0, err.Error())
		return err
	}
	if !config.Enabled {
		// 登录总开关关了不自动同步（手动入口在 controller 侧仍受此约束）。
		err = fmt.Errorf("%w: sso disabled", ErrSsoDisabled)
		recordSsoSyncResult(adminId, define.SsoSyncStatusFailed, 0, 0, err.Error())
		return err
	}

	logs.Info("sso sync start: admin_id=%d, provider=%s", adminId, config.Provider)
	members, depts, err := fetchSsoDirectory(config, syncConfig)
	if err != nil {
		logs.Error("sso sync fetch failed: admin_id=%d, provider=%s, err=%s", adminId, config.Provider, err.Error())
		recordSsoSyncResult(adminId, define.SsoSyncStatusFailed, 0, 0, err.Error())
		return err
	}
	// 取数结果必须落日志：成功路径其余位置都不写日志，只凭「同步了 0 人」没法判断
	// 是上游真的没人，还是取数环节漏了（分页、部门范围、响应字段对不上都会表现为 0 人）。
	logs.Info("sso sync directory fetched: admin_id=%d, provider=%s, members=%d, departments=%d",
		adminId, config.Provider, len(members), len(depts))

	extToDeptId, notice, err := syncSsoDepartments(adminId, config.Provider, config.AppKey, depts)
	if err != nil {
		logs.Error("sso sync departments failed: admin_id=%d, err=%s", adminId, err.Error())
		recordSsoSyncResult(adminId, define.SsoSyncStatusFailed, 0, 0, err.Error())
		return err
	}

	roleId, err := enterprisePresetRoleId(adminId, `perm_role_enterprise_member`)
	if err != nil {
		recordSsoSyncResult(adminId, define.SsoSyncStatusFailed, 0, 0, err.Error())
		return err
	}

	processed, failed := 0, 0
	for _, member := range members {
		processed++
		if err = syncSsoMember(adminId, config.Provider, config.AppKey, roleId, member, extToDeptId); err != nil {
			failed++
			logs.Error("sso sync member failed: admin_id=%d, external_id=%s, err=%s", adminId, member.info.ExternalId, err.Error())
		}
	}

	status := define.SsoSyncStatusSuccess
	if failed > 0 {
		status = define.SsoSyncStatusPartial
	}
	logs.Info("sso sync finished: admin_id=%d, provider=%s, processed=%d, failed=%d, status=%s",
		adminId, config.Provider, processed, failed, status)
	recordSsoSyncResult(adminId, status, processed, failed, notice)
	return nil
}

// syncSsoDepartments 在单个事务里落库全部部门：先清遗留绑定，再按拓扑序建/更新，
// 返回 external_id→本地 department_id 的映射，以及一条可选的可见提示（同名认领等）。
func syncSsoDepartments(adminId int, provider string, appKey string, depts []ssoSyncDept) (map[string]int, string, error) {
	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return nil, ``, err
	}
	defer func() { _ = tx.Rollback() }()

	// 遗留绑定清理：本地部门被硬删后，其 department_sso_bindings 悬空，下次同步要能重建。
	if _, err = msql.RawExec(define.Postgres, `DELETE FROM department_sso_bindings b
		WHERE b.admin_id=$1 AND b.provider=$2 AND b.app_key=$3
		AND NOT EXISTS (SELECT 1 FROM departments d WHERE d.id=b.department_id)`,
		tx, adminId, provider, appKey); err != nil {
		logs.Error("clean orphan department bindings failed: %s", err.Error())
		return nil, ``, err
	}

	// 现有一批绑定，供幂等更新与「父已存在」判定。
	existing, err := loadSsoDepartmentBindingsTx(tx, adminId, provider, appKey)
	if err != nil {
		return nil, ``, err
	}
	extToId := make(map[string]int, len(existing))
	for ext, id := range existing {
		extToId[ext] = id
	}

	// external_id → 节点，重名的取第一个（IdP 侧不应重名，重名属于数据异常，忽略后续）。
	nodes := make(map[string]*ssoDeptNode, len(depts))
	order := make([]string, 0, len(depts))
	for _, dept := range depts {
		if _, ok := nodes[dept.externalId]; ok {
			continue
		}
		nodes[dept.externalId] = &ssoDeptNode{dept: dept}
		order = append(order, dept.externalId)
	}

	notice := ``
	resolved := make(map[string]bool, len(order))
	// Kahn 式逐轮：只处理「无父 / 父已在库 / 父已解出」的节点；一轮推不动就是成环，剩余节点挂顶层。
	for len(order) > 0 {
		progress := false
		remaining := make([]string, 0, len(order))
		for _, ext := range order {
			node := nodes[ext]
			parentLocal := 0
			if node.dept.parent != `` {
				if id, ok := extToId[node.dept.parent]; ok {
					parentLocal = id
				} else {
					remaining = append(remaining, ext)
					continue
				}
			}
			id, claimed, err := upsertSsoDepartmentTx(tx, adminId, provider, appKey, node.dept, parentLocal, extToId)
			if err != nil {
				return nil, ``, err
			}
			extToId[ext] = id
			resolved[ext] = true
			if claimed {
				notice = fmt.Sprintf("本地已存在同名部门「%s」，已认领并绑定", node.dept.name)
			}
			progress = true
		}
		if !progress {
			// 环或父缺失：把剩余节点挂顶层落库，避免整个部门批次卡死。
			for _, ext := range remaining {
				id, claimed, err := upsertSsoDepartmentTx(tx, adminId, provider, appKey, nodes[ext].dept, 0, extToId)
				if err != nil {
					return nil, ``, err
				}
				extToId[ext] = id
				resolved[ext] = true
				if claimed && notice == `` {
					notice = fmt.Sprintf("本地已存在同名部门「%s」，已认领并绑定", nodes[ext].dept.name)
				}
			}
			break
		}
		order = remaining
	}

	if err = tx.Commit(); err != nil {
		return nil, ``, err
	}
	return extToId, notice, nil
}

// upsertSsoDepartmentTx 建或更新一个部门并维护其绑定，返回 (本地id, 是否同名认领, 错误)。
// 已绑定 → 覆盖 name/parent_id；未绑定 → 新建，撞同名唯一键时认领既有本地部门。
func upsertSsoDepartmentTx(tx *sql.Tx, adminId int, provider, appKey string, dept ssoSyncDept,
	parentId int, extToId map[string]int) (int, bool, error) {

	name, err := NormalizeDepartmentName(dept.name)
	if err != nil {
		name = dept.externalId // 名称非法（空/超长）时退化到外部标识，保证部门可落库不丢人。
	}
	now := tool.Time2Int()

	if localId, ok := extToId[dept.externalId]; ok {
		if err = updateDepartmentParentNameTx(tx, adminId, localId, name, parentId); err != nil {
			return 0, false, err
		}
		return localId, false, nil
	}

	// 先按 (parent,name) 找同名本地部门：命中且它尚无绑定 → 认领；否则新建。
	colliding, err := findDepartmentByNameTx(tx, adminId, parentId, name)
	if err != nil {
		return 0, false, err
	}
	claimed := false
	departmentId := colliding
	if departmentId == 0 {
		if departmentId, err = insertDepartmentTx(tx, adminId, parentId, name, now); err != nil {
			return 0, false, err
		}
	} else {
		claimed = true
	}
	if err = insertDepartmentBindingTx(tx, adminId, provider, appKey, departmentId, dept.externalId, now); err != nil {
		return 0, false, err
	}
	return departmentId, claimed, nil
}

func loadSsoDepartmentBindingsTx(tx *sql.Tx, adminId int, provider, appKey string) (map[string]int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT external_id, department_id FROM department_sso_bindings
		WHERE admin_id=$1 AND provider=$2 AND app_key=$3`, tx, adminId, provider, appKey)
	if err != nil {
		logs.Error("load sso department bindings failed: %s", err.Error())
		return nil, err
	}
	result := make(map[string]int, len(rows))
	for _, row := range rows {
		result[row[`external_id`]] = cast.ToInt(row[`department_id`])
	}
	return result, nil
}

func findDepartmentByNameTx(tx *sql.Tx, adminId, parentId int, name string) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `SELECT id FROM departments
		WHERE admin_id=$1 AND parent_id=$2 AND name=$3 LIMIT 1`, tx, adminId, parentId, name)
	if err != nil {
		logs.Error("find department by name failed: %s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return cast.ToInt(rows[0][`id`]), nil
}

func insertDepartmentTx(tx *sql.Tx, adminId, parentId int, name string, now int) (int, error) {
	rows, err := msql.RawValues(define.Postgres, `INSERT INTO departments (admin_id,parent_id,name,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$4) RETURNING id`, tx, adminId, parentId, name, now)
	if err != nil {
		logs.Error("insert department failed: %s", err.Error())
		return 0, err
	}
	if len(rows) == 0 {
		return 0, errors.New(`department insert returned no row`)
	}
	return cast.ToInt(rows[0][`id`]), nil
}

func updateDepartmentParentNameTx(tx *sql.Tx, adminId, departmentId int, name string, parentId int) error {
	// 改父级前校验不成环/不超深度：沿 parent 上溯若回到本节点则拒绝改父、仅改名。
	if parentId != 0 && parentId != departmentId {
		forms, err := departmentParentFormsCycleTx(tx, adminId, departmentId, parentId)
		if err != nil {
			return err
		}
		if forms {
			parentId = cast.ToInt(currentDepartmentParentTx(tx, departmentId))
		}
	}
	_, err := msql.RawExec(define.Postgres, `UPDATE departments SET name=$1, parent_id=$2, updated_at=$3
		WHERE admin_id=$4 AND id=$5`, tx, name, parentId, tool.Time2Int(), adminId, departmentId)
	if err != nil {
		logs.Error("update department failed: %s", err.Error())
	}
	return err
}

func currentDepartmentParentTx(tx *sql.Tx, departmentId int) int {
	rows, err := msql.RawValues(define.Postgres, `SELECT parent_id FROM departments WHERE id=$1 LIMIT 1`, tx, departmentId)
	if err != nil || len(rows) == 0 {
		return 0
	}
	return cast.ToInt(rows[0][`parent_id`])
}

// departmentParentFormsCycleTx 判断把 departmentId 挂到 newParent 下是否会成环：从 newParent 上溯能否回到 departmentId。
func departmentParentFormsCycleTx(tx *sql.Tx, adminId, departmentId, newParent int) (bool, error) {
	current := newParent
	for depth := 0; current != 0 && depth <= DepartmentMaxDepth+1; depth++ {
		if current == departmentId {
			return true, nil
		}
		rows, err := msql.RawValues(define.Postgres, `SELECT parent_id FROM departments
			WHERE admin_id=$1 AND id=$2 LIMIT 1`, tx, adminId, current)
		if err != nil {
			return false, err
		}
		if len(rows) == 0 {
			return false, nil
		}
		current = cast.ToInt(rows[0][`parent_id`])
	}
	return false, nil
}

func insertDepartmentBindingTx(tx *sql.Tx, adminId int, provider, appKey string, departmentId int, externalId string, now int) error {
	// 一个本地部门在一个应用下只能有一个上游；ON CONFLICT DO NOTHING 挡住「同批两条指同一部门」。
	_, err := msql.RawExec(define.Postgres, `INSERT INTO department_sso_bindings
		(admin_id,provider,app_key,department_id,external_id,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$6) ON CONFLICT (department_id,provider,app_key) DO NOTHING`,
		tx, adminId, provider, appKey, departmentId, externalId, now)
	if err != nil {
		logs.Error("insert department binding failed: %s", err.Error())
	}
	return err
}

// syncSsoMember 处理单个成员：命中绑定覆盖资料，未命中 JIT 建号；部门引用全部解得出才替换关联。
// 单条一个事务，失败整体回滚，不影响同批其他成员。
func syncSsoMember(adminId int, provider, appKey string, roleId int, member ssoSyncMember, extToDeptId map[string]int) error {
	info := ssoTrimUserInfo(member.info)
	if info.ExternalId == `` {
		return errors.New(`member without external id`)
	}

	// 全部部门引用能否解到本地 id；有一个解不出就跳过部门替换（保留现有关联，绝不清空）。
	departmentIds := make([]int, 0, len(member.department))
	resolvable := true
	seen := make(map[int]bool, len(member.department))
	for _, ext := range member.department {
		localId, ok := extToDeptId[ext]
		if !ok || localId <= 0 {
			resolvable = false
			break
		}
		if !seen[localId] {
			seen[localId] = true
			departmentIds = append(departmentIds, localId)
		}
	}

	tx, err := msql.Begin(define.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	userId, found, err := getSsoBindingUserIdTx(tx, adminId, provider, appKey, info.ExternalId)
	if err != nil {
		return err
	}
	if !found {
		if userId, err = ssoSyncCreateUserTx(tx, adminId, roleId, info); err != nil {
			return err
		}
	} else if err = updateSsoMemberProfileTx(tx, adminId, userId, info); err != nil {
		return err
	}

	if err = updateSsoBindingProfileTx(tx, adminId, provider, appKey, userId, info, found); err != nil {
		return err
	}
	if resolvable {
		if err = replaceUserDepartmentsTx(tx, adminId, userId, departmentIds); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ssoSyncCreateUserTx 在同步事务里建号：锁配额 → 建 users 行 + 头像。返回新 user_id。
// 随机 username 撞唯一索引会把事务打成 aborted、同事务不能续写；这条成员由上层计为失败、下次同步再建，
// 不在事务内重试（登录侧的 ssoJitProvision 才每重试开新事务，那是交互链路、需要当场成功）。
func ssoSyncCreateUserTx(tx *sql.Tx, adminId int, roleId int, info SsoUserInfo) (int, error) {
	if err := CheckMemberQuotaTx(tx, adminId); err != nil {
		return 0, err
	}
	password, err := randomOauthToken(ssoJitPasswordBytes)
	if err != nil {
		return 0, err
	}
	name := info.Name
	if name == `` {
		name = info.Username
	}
	username, err := NewSsoJitUsername()
	if err != nil {
		return 0, err
	}
	userId, err := createUserTx(tx, CreateUserModel{
		Username: username, Password: password, Name: name, Phone: info.Phone,
		AdminId: adminId, RoleId: roleId,
	}, 0)
	if err != nil {
		return 0, err
	}
	if info.Avatar != `` {
		if _, err = msql.RawExec(define.Postgres, `UPDATE users SET avatar=$1, updated_at=$2 WHERE id=$3`,
			tx, info.Avatar, tool.Time2Int(), userId); err != nil {
			logs.Error("set sso sync avatar failed: %s", err.Error())
			return 0, err
		}
	}
	return userId, nil
}

// updateSsoMemberProfileTx 覆盖本地成员资料（姓名 / 手机号 / 头像），绝不触碰 status / deleted_at。
func updateSsoMemberProfileTx(tx *sql.Tx, adminId, userId int, info SsoUserInfo) error {
	if _, err := msql.RawExec(define.Postgres, `UPDATE users SET name=$1, tel_number=$2, avatar=$3, updated_at=$4
		WHERE admin_id=$5 AND id=$6 AND deleted_at=0`,
		tx, info.Name, info.Phone, info.Avatar, tool.Time2Int(), adminId, userId); err != nil {
		logs.Error("update sso member profile failed: %s", err.Error())
		return err
	}
	return nil
}

// updateSsoBindingProfileTx 覆盖绑定行的资料字段。found=false 表示成员是本次新建，绑定行尚未存在 → INSERT；
// 已存在则 UPDATE 资料列。
func updateSsoBindingProfileTx(tx *sql.Tx, adminId int, provider, appKey string, userId int, info SsoUserInfo, bound bool) error {
	now := tool.Time2Int()
	if !bound {
		return insertSsoBindingTx(tx, adminId, userId, provider, appKey, info)
	}
	_, err := msql.RawExec(define.Postgres, `UPDATE user_sso_bindings SET username=$1, email=$2, job_title=$3,
		employee_no=$4, nickname=$5, avatar=$6, updated_at=$7
		WHERE admin_id=$8 AND provider=$9 AND app_key=$10 AND user_id=$11`,
		tx, info.Username, info.Email, info.JobTitle, info.EmployeeNo, info.Name, info.Avatar,
		now, adminId, provider, appKey, userId)
	if err != nil {
		logs.Error("update sso binding profile failed: %s", err.Error())
	}
	return err
}

// recordSsoSyncResult 把同步结果写回 sso_sync_configs，供配置页展示最近一次状态。
// notice 非空时作为 last_error 提示（如同名认领）；纯成功时清空。
func recordSsoSyncResult(adminId int, status string, processed, failed int, notice string) {
	now := tool.Time2Int()
	message := notice
	if status == define.SsoSyncStatusSuccess {
		message = ``
	}
	message = truncateOauthText(message, define.SsoSyncLastErrorLimit)
	_, err := msql.Model(define.TableSsoSyncConfigs, define.Postgres).Where(`admin_id`, cast.ToString(adminId)).
		Update(msql.Datas{
			`last_synced_at`: now, `last_synced_count`: processed,
			`last_sync_status`: status, `last_failed_count`: failed, `last_error`: message,
			`updated_at`: now,
		})
	if err != nil {
		logs.Error("record sso sync result failed: admin_id=%d, err=%s", adminId, err.Error())
	}
}

// ======================== 触发入口 ========================

// SsoSyncHandler 是 scheduled_tasks 的处理函数：自动与手动都经它，从 task.AdminId 取租户。
func SsoSyncHandler(task ScheduledTask) error {
	if task.AdminId <= 0 {
		return fmt.Errorf("%w: missing admin_id", ErrSsoSyncUnsupported)
	}
	return runSsoSync(task.AdminId)
}

// TriggerSsoSync 手动触发一次同步：原子领取租约后异步跑，立即返回。
// 正在执行返回 ErrSchedulerTaskBusy，调度器已停返回 ErrSchedulerStopped。
func TriggerSsoSync(adminId int) error {
	return TriggerScheduledTask(define.SsoSyncTaskKey, SsoSyncScopeKey(adminId))
}
