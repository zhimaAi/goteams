// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"goteams/internal/app/goteams/assets"
	"goteams/internal/app/goteams/common"
	"goteams/internal/app/goteams/define"
	"goteams/internal/pkg/lib_i18n"

	"github.com/spf13/cast"
	"github.com/zhimaAi/go_tools/logs"
	"github.com/zhimaAi/go_tools/msql"
)

// ApiTokenSkillName 是 skill 安装包在生产环境下的技能名，同时作为压缩包内的目录名。
// 测试环境会带 -dev 后缀，见 apiTokenSkillNameForEnv。
const ApiTokenSkillName = `goteams-api`

// apiTokenSkillNameDevSuffix 是测试环境技能名的后缀。
// 加后缀是为了让同一个 Agent 里测试包与生产包共存：两者同名时后装的会静默覆盖先装的，
// 而包里的站点地址与明文 Key 各指向自己的环境，覆盖后很容易拿测试环境的 Key 去调生产。
const apiTokenSkillNameDevSuffix = `-dev`

// apiTokenSkillNameForEnv 返回当前环境使用的技能名：测试环境（define.IsDev，即 dev / dev1~dev7 / loc）带
// -dev 后缀，生产环境不带。技能名同时决定 zip 内的目录名、SKILL.md 的 name 与下载文件名，只有这一个出口。
func apiTokenSkillNameForEnv() string {
	if define.IsDev {
		return ApiTokenSkillName + apiTokenSkillNameDevSuffix
	}
	return ApiTokenSkillName
}

// skill 的素材（文档 + 脚本）统一放在 goteams/assets/api_token_skill 下，与业务代码分开管理。
// 文档按语言分目录存放（zh-CN / en-US），脚本只有一套，语言由渲染时写进去的 LANG 决定。
const (
	apiTokenSkillDir        = `api_token_skill`
	apiTokenSkillDocName    = `SKILL.md`
	apiTokenSkillScriptsDir = `scripts`
)

// 文档模板里的条件标记。标记各占一行，本身永远不进入交付物：
//
//	<!--if:perm=a|b-->    … <!--endif-->  开通了 a 或 b 时保留
//	<!--if:noperm=a|b-->  … <!--endif-->  一个都没开通时保留，用来写降级做法
//	<!--if:missing-->     … <!--endif-->  有任意权限点未开通时保留
//
// 脚本按文件名对应的权限点整份取舍，不用标记。
const (
	apiTokenSkillIfPrefix = `<!--if:`
	apiTokenSkillIfSuffix = `-->`
	apiTokenSkillEndIf    = `<!--endif-->`
)

// apiTokenSkillScriptPerms 是脚本文件对应的权限点，命中任意一个才打包。
// 不在表里的脚本视为公用模块，始终打包；新增接口脚本必须同时补这张表，
// 漏登记由 TestApiTokenSkillLocal 兜住。
var apiTokenSkillScriptPerms = map[string][]string{
	`my_work.py`:          {define.ApiPermMyWorkList},
	`list_workspaces.py`:  {define.ApiPermWorkspaceList},
	`list_work_items.py`:  {define.ApiPermWorkItemList},
	`get_work_item.py`:    {define.ApiPermWorkItemView},
	`download_file.py`:    {define.ApiPermWorkItemView},
	`create_work_item.py`: {define.ApiPermWorkItemCreate},
	`update_work_item.py`: {define.ApiPermWorkItemUpdate},
	`create_comment.py`:   {define.ApiPermWorkItemComment},
	`upload_image.py`:     {define.ApiPermWorkItemCreate, define.ApiPermWorkItemUpdate},
	`upload_file.py`:      {define.ApiPermWorkItemCreate, define.ApiPermWorkItemUpdate},
	`upload_video.py`:     {define.ApiPermWorkItemCreate, define.ApiPermWorkItemUpdate},
}

// apiTokenSkillFile 是打包进 skill 的一项内容。
type apiTokenSkillFile struct {
	// Path 是 zip 内相对技能目录的路径。
	Path string
	// Content 是替换占位符、裁剪权限之后的文件内容。
	Content []byte
	// Mode 是解压后的权限位：脚本要可执行，文档普通可读。
	Mode fs.FileMode
}

// BuildApiTokenSkill 生成该 token 的 Agent skill 安装包：SKILL.md 描述开放接口，scripts/ 下每个接口一个
// 可直接运行的脚本，明文 Key 与站点地址已内置。用户把它交给 AI Agent 后即可按创建者的身份调用 /api/v1。
// lang 决定文档与脚本文案的语言；文档只保留该 token 已开通权限对应的段落，脚本同理。
// 技能名随环境变化（测试环境带 -dev 后缀，见 apiTokenSkillNameForEnv）。
// 返回下载文件名与 zip 内容。
func BuildApiTokenSkill(adminId, userId, id int, siteUrl, lang string) (string, []byte, error) {
	m := msql.Model(define.TableApiTokens, define.Postgres)
	row, err := m.Field(`name, token_plain, permissions`).Where2([][]string{
		{`id`, cast.ToString(id)},
		{`admin_id`, cast.ToString(adminId)},
		{`user_id`, cast.ToString(userId)},
	}).Find()
	if err != nil {
		logs.Error("get api token for skill failed: sql=%s, err=%s", m.GetLastSql(), err.Error())
		return ``, nil, err
	}
	if len(row) == 0 {
		return ``, nil, ErrApiTokenNotFound
	}
	skillName := apiTokenSkillNameForEnv()
	files, err := apiTokenSkillFiles(skillName, row[`token_plain`], common.ParseTextArray(row[`permissions`]), siteUrl, lang)
	if err != nil {
		logs.Error("render api token skill failed: token_id=%d, err=%s", id, err.Error())
		return ``, nil, err
	}
	data, err := apiTokenSkillArchive(skillName, files)
	if err != nil {
		logs.Error("archive api token skill failed: token_id=%d, err=%s", id, err.Error())
		return ``, nil, err
	}
	return apiTokenSkillFilename(skillName, row[`name`]), data, nil
}

// apiTokenSkillFiles 渲染整个安装包：选语言模板、按权限裁段落与脚本，最后统一替换占位符。
// 技能名、站点地址、明文 Key、语言、权限名只有这一处替换。
func apiTokenSkillFiles(skillName, plain string, perms []string, siteUrl, lang string) ([]apiTokenSkillFile, error) {
	perms = define.NormalizeApiTokenPermissions(perms)
	lang = lib_i18n.NormalizeLang(lang)
	granted := make(map[string]bool, len(perms))
	for _, perm := range perms {
		granted[perm] = true
	}
	doc, err := assets.ApiTokenSkill.ReadFile(apiTokenSkillDir + `/` + lang + `/` + apiTokenSkillDocName)
	if err != nil {
		return nil, err
	}
	replace := strings.NewReplacer(
		`{{skill_name}}`, skillName,
		`{{site_url}}`, siteUrl,
		`{{api_key}}`, plain,
		`{{lang}}`, lang,
		`{{permissions}}`, apiTokenPermissionNames(lang, perms),
		`{{missing_permissions}}`, apiTokenPermissionNames(lang, apiTokenMissingPermissions(granted)),
	)
	files := []apiTokenSkillFile{{
		Path:    apiTokenSkillDocName,
		Content: []byte(replace.Replace(apiTokenSkillPruneDoc(string(doc), granted))),
		Mode:    0o644,
	}}
	entries, err := fs.ReadDir(assets.ApiTokenSkill, apiTokenSkillDir+`/`+apiTokenSkillScriptsDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !apiTokenSkillScriptAllowed(entry.Name(), granted) {
			continue
		}
		raw, err := assets.ApiTokenSkill.ReadFile(apiTokenSkillDir + `/` + apiTokenSkillScriptsDir + `/` + entry.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, apiTokenSkillFile{
			Path:    apiTokenSkillScriptsDir + `/` + entry.Name(),
			Content: []byte(replace.Replace(string(raw))),
			Mode:    0o755,
		})
	}
	return files, nil
}

// apiTokenSkillPruneDoc 按条件标记裁剪文档：标记行本身丢弃，条件不成立的整段丢弃。
// 标记成对出现且不嵌套，因此只需一个栈记录当前各层的条件结果。
func apiTokenSkillPruneDoc(doc string, granted map[string]bool) string {
	missingAny := len(granted) < len(define.AllApiTokenPermissions)
	stack := make([]bool, 0, 4)
	lines := strings.Split(doc, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == apiTokenSkillEndIf {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if strings.HasPrefix(trimmed, apiTokenSkillIfPrefix) && strings.HasSuffix(trimmed, apiTokenSkillIfSuffix) {
			condition := strings.TrimSuffix(strings.TrimPrefix(trimmed, apiTokenSkillIfPrefix), apiTokenSkillIfSuffix)
			stack = append(stack, apiTokenSkillConditionMet(condition, granted, missingAny))
			continue
		}
		if apiTokenSkillAllKept(stack) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// apiTokenSkillConditionMet 判定单条标记条件。
func apiTokenSkillConditionMet(condition string, granted map[string]bool, missingAny bool) bool {
	if condition == `missing` {
		return missingAny
	}
	if rest, ok := strings.CutPrefix(condition, `perm=`); ok {
		return apiTokenSkillAnyGranted(rest, granted)
	}
	if rest, ok := strings.CutPrefix(condition, `noperm=`); ok {
		return !apiTokenSkillAnyGranted(rest, granted)
	}
	// 认不出来的条件一律保留，宁可多写一段说明，也不要静默吞掉文档内容。
	return true
}

// apiTokenSkillAnyGranted 报告 `a|b` 形式的权限点里是否有任意一个已开通。
func apiTokenSkillAnyGranted(list string, granted map[string]bool) bool {
	for _, perm := range strings.Split(list, `|`) {
		if granted[strings.TrimSpace(perm)] {
			return true
		}
	}
	return false
}

// apiTokenSkillAllKept 报告当前所有外层条件是否都成立。
func apiTokenSkillAllKept(stack []bool) bool {
	for _, ok := range stack {
		if !ok {
			return false
		}
	}
	return true
}

// apiTokenSkillScriptAllowed 按权限点决定一个脚本是否随包下发。
func apiTokenSkillScriptAllowed(name string, granted map[string]bool) bool {
	required, registered := apiTokenSkillScriptPerms[name]
	if !registered {
		// 公用模块与尚未登记的新脚本都先放行，漏登记由测试兜住。
		return true
	}
	for _, perm := range required {
		if granted[perm] {
			return true
		}
	}
	return false
}

// apiTokenMissingPermissions 按 AllApiTokenPermissions 的顺序给出未开通的权限点。
func apiTokenMissingPermissions(granted map[string]bool) []string {
	missing := make([]string, 0, len(define.AllApiTokenPermissions))
	for _, perm := range define.AllApiTokenPermissions {
		if !granted[perm] {
			missing = append(missing, perm)
		}
	}
	return missing
}

// apiTokenPermissionNames 把权限点按请求语言翻成展示名，写进文档让 Agent 知道这个 Key 能做什么。
func apiTokenPermissionNames(lang string, perms []string) string {
	names := make([]string, 0, len(perms))
	for _, perm := range perms {
		names = append(names, lib_i18n.Show(lang, define.ApiTokenPermissionNameKey(perm)))
	}
	return strings.Join(names, apiTokenPermissionSeparator(lang))
}

// apiTokenPermissionSeparator 是展示名的连接符：中文用顿号，英文用逗号加空格。
func apiTokenPermissionSeparator(lang string) string {
	if lang == lib_i18n.LangEnUs {
		return `, `
	}
	return `、`
}

// apiTokenSkillArchive 把渲染好的文件打包成 zip，压缩包内目录名即技能名。
func apiTokenSkillArchive(skillName string, files []apiTokenSkillFile) ([]byte, error) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range files {
		header := &zip.FileHeader{Name: skillName + `/` + file.Path, Method: zip.Deflate}
		header.SetMode(file.Mode)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		if _, err = entry.Write(file.Content); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// apiTokenSkillFilename 组装下载文件名：备注非空时带上备注，便于同一账号下多个 Key 区分。
func apiTokenSkillFilename(skillName, name string) string {
	if name = strings.TrimSpace(name); name == `` {
		return skillName + `.zip`
	}
	return fmt.Sprintf(`%s-%s.zip`, skillName, name)
}
