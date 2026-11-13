// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package business

import (
	"fmt"
	"strings"

	"goteams/internal/app/goteams/assets"
	"goteams/internal/pkg/lib_i18n"

	"github.com/zhimaAi/go_tools/logs"
)

// ApiTokenDocFilename 是接口文档的下载文件名。中英文共用同一个名字：
// 文档内容已按请求语言渲染，文件名再分语言只会让同一份材料出现两种落盘名。
const ApiTokenDocFilename = `goteams-api-doc.md`

// apiTokenDocSiteUrlPlaceholder 是文档模板里唯一允许出现的占位符，渲染时替换成当前部署的站点地址。
// 站点地址由调用方从 site_url 配置取（见 controller.apiTokenSiteBaseURL），
// 与 skill 安装包共用同一个来源：私有化部署时交付物必须指向部署方自己的域名，
// 写死官方域名会让对接方照抄后调不通。
const apiTokenDocSiteUrlPlaceholder = `{{site_url}}`

// 接口文档的素材目录，按语言分子目录，与 skill 安装包（api_token_skill）分开维护：
// 前者面向对接开发者，描述全部接口；后者面向 AI Agent，只描述该 Key 已开通的接口。
const (
	apiTokenDocDir  = `api_token_doc`
	apiTokenDocName = `API.md`
)

// BuildApiTokenDoc 渲染开放接口文档（markdown）：描述 /api/v1 的全部接口，
// 站点地址用调用方传入的 siteUrl（当前部署的 site_url），**不含任何密钥**，
// 因此与具体 token 无关，无需查询数据库。
// lang 决定文档语言；返回下载文件名与文件内容。
func BuildApiTokenDoc(siteUrl, lang string) (string, []byte, error) {
	if siteUrl == `` {
		// 文档里每个接口地址都以站点地址开头，缺了它整份文档调不通，宁可报错也不交付半成品。
		logs.Error("empty site url for api token doc: lang=%s", lang)
		return ``, nil, fmt.Errorf(`empty site url for api token doc (%s)`, lang)
	}
	lang = lib_i18n.NormalizeLang(lang)
	raw, err := assets.ApiTokenDoc.ReadFile(apiTokenDocDir + `/` + lang + `/` + apiTokenDocName)
	if err != nil {
		logs.Error("read api token doc template failed: lang=%s, err=%s", lang, err.Error())
		return ``, nil, err
	}
	content := strings.ReplaceAll(string(raw), apiTokenDocSiteUrlPlaceholder, siteUrl)
	// 模板里只允许 site_url 一个占位符，且必须被替换掉：漏替换的 {{...}} 会原样出现在用户下载的文档里，
	// 既看不懂又像是系统故障，所以这里宁可报错也不交付半成品。
	if strings.Contains(content, `{{`) {
		logs.Error("unreplaced placeholder in api token doc: lang=%s", lang)
		return ``, nil, fmt.Errorf(`unreplaced placeholder in api token doc template (%s)`, lang)
	}
	return ApiTokenDocFilename, []byte(content), nil
}
