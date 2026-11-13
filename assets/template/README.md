# 模板包 `template.tar.gz`

私有化部署的数据库里没有模板账号，"从模板创建项目"因此无源可复制。模板包把模板账号的
全部可复制配置与图片打成一个离线文件，随代码进入每个私有化环境。

- 包路径：`assets/template/template.tar.gz`（`define.TemplatePackagePath`，相对进程工作目录）
- 内嵌副本：`assets/template/embed.go`（`//go:embed template.tar.gz`，包 `templateassets`）把同一文件编进二进制；
  运行时优先读磁盘文件，文件不存在时回退到内嵌副本，因此只带二进制、不带 `assets/` 目录的开源版也有默认模板包
- 唯一读者：`internal/app/goteams/business/template_package.go`
- 唯一写者：`internal/app/goteams/business/template_export.go`（导出接口 `GET /api/workspace-templates/export`）
- 列清单的唯一正文：`internal/app/goteams/business/template_source.go` 的 `templateTables`

**替换包必须走 git。** 包是仓库里的分发物，运维不手改服务器文件：私有化环境 `git pull` 即拿到新包，
任何服务器侧的手工覆盖都会在下次发布时消失，且无法追溯是谁在什么条件下产出的。

## 1. 包结构

```
template.tar.gz
├── manifest.json                    # generated_at / template_admin_id / workspace_ids / asset_count / missing_asset_count
├── workspaces.json                  # 模板账号未结束项目，顺序 = sort_order ASC, id DESC
├── work_item_templates.json         # 租户级池模板，跨项目共享，单份（可缺省）
├── data/<workspace_id>/<源键>.json   # 26 个源键，每个项目一组，空源也要写空数组
└── assets/
    ├── index.json                   # map[包内路径] → 对象元数据
    └── <sha256 前 32 位>.<ext>       # 一个对象一文件，内容相同天然去重
```

每个 `*.json` 都是 `[]msql.Params`（`map[string]string`），NULL 记为空串，数组顺序即导出顺序。
**导入侧按下标遍历，不得重排**：自动化规则依赖 `created_at ASC, id ASC`，规划行依赖
`group_id, sort_order, id`，版本名称去重依赖 `id ASC`，仪表盘卡片按盘面顺序内依赖
`grid_y, grid_x, id`。

**包内没有产出环境的地址。** 版式 v2 起，`assets/index.json` 的键就是资产在包里的路径
（`assets/<sha256 前 32 位>.<ext>`）：`workspaces.icon`、两张邀请表的 `theme_image`、
`requirements.description`、知识正文与仪表盘文本卡/公告正文里的图片引用（含 markdown 的 `![]()`）在导出时统一改写成
这个路径，复制时再逐条上传到目标环境。所以包完全离线自足——`grep` 全包找不到模板账号的存储域名；
唯一例外是正文里原本就指向外部站点的链接（`source_url` 等），它们不是资产引用，原样保留。

导不出来的对象（模板侧已删、读取失败）会把该处引用清空并在 `missing_asset_count` 计数，等于当场
摘掉坏了的外链，而不是把产出环境的 URL 留在包里。同一个对象被多处引用只落一个文件，因此
`index` 条目数 = `assets/` 下文件数 = 去重后的对象数。

## 2. 产出包

只能由**配置了模板账号的环境**产出（生产 `[template] workspace_template_id = 2`），且调用者要带一个用该环境 `[template] export_secret`
签出的 token：`GET /api/workspace-templates/export?token=<到期时间戳>.<HMAC-SHA256>`。接口不看登录态——
它要能从浏览器地址栏和无头 curl 直接调用，两者都带不了 `Authorization` 头；**拿到有效 token 的任何人**
都能下载整包，token 会明文进入访问日志与浏览器历史，所以默认只签 5 分钟。

一条命令导出并落位（域名三选一：dev7 / goteams.cn / goteams.ai）。Windows 上双击 `template-export.bat`
即可走交互，命令行用法相同（逻辑在 `scripts/ps1/template-export.ps1`，Windows PowerShell 5.1 起）：

```bat
:: 交互选域名，下载后自动覆盖 assets/template/template.tar.gz；域名默认 dev7
scripts\template-export.bat

:: 非交互：直接指定域名与有效期（秒）
scripts\template-export.bat --host https://goteams.cn --ttl 600

:: 新环境或轮换密钥：随机密钥写入 configs/goteams/config_<环境>.ini，提交部署后服务端生效
scripts\template-export.bat --gen-secret --env pro
```

脚本覆盖前会校验响应确实是 gzip，403/500 一律保留原包不动。非线上环境（`config_pro.ini` 以外）
导出的包会在根目录追加一个 `.dev` 标记：`scripts/publish.sh` 拒绝部署这种包，避免把开发环境的
配置带到客户环境；读取侧忽略该标记，因此开发包仍能在本地加载验证。随后做自检与公开性审查
（见第 4、5 节），再提交，由发布流程带到各环境。

导出失败只有四种：token 缺失/过期/验签失败（裸 403）、环境未配置模板账号（裸 403）、体积超上限
（500，文案 `template_package_too_large`）、查库或读对象出错（500）。导出在内存里完成后回灌一遍读取器，
**本二进制读不懂的包不会被你拿到**；超上限是明确失败，不裁剪、不静默丢内容。

体积上限（`define`）：整包 64 MiB，单文件 32 MiB，解包后累计 512 MiB，条目数 65536。条目是逐个
整体读进内存的，字节配额要读完才计得到，所以海量小条目由条目数封顶。

以下任一情况都算「包损坏」，整包按不可用降级：条目名不在格式内、`data/<wsId>` 不是规范的十进制
目录名、某个源的 `data/<wsId>/<源键>.json` 缺失、`workspaces.json` 里有列表接口无法转换的行。

## 3. 包与二进制的列差异

包不做版本校验，行按列名读取：包里的列少于 `templateTables` 声明时，缺的那列读成空串；多出来的列
忽略。所以旧包在新二进制上照样能加载，只有新增的那几列取默认值——`guide_enabled` 缺失按开启处理，
`recall_enabled`/`recall_status_id` 缺失等于撤回关闭（见 `copyIssueInviteTemplates`）。

各版式新增过什么（留档，不再作为拒绝加载的理由）：`workspace_issue_item_templates` 先是从
`source_template_id` 改成邀填模板自己的列（`type_key`、`name`、`title`、`description`、
`description_mode`、`sort_order`，邀填模板已是独立实体，复制要搬整行），后加了 `recall_enabled`、
`recall_status_id`（撤回状态是项目自己的状态 id，复制时跟着状态映射换成本项目的 id，映射不到就把
该模板的撤回关掉）；`workspaces_settings` 加过 `requirement_aging_control`；更早的一次把资产引用从
原始 stored 串改成包内路径 `assets/<sha256 前 32 位>.<ext>`，那种老包的资产引用对不上，对象读不出来。

私有化环境升级顺序不变：先升级二进制、再 `git pull` 到新包。

## 4. 解压自检

命令在 Git Bash / Linux 下均可执行；`jq` 缺失时用 `python -m json.tool` 代替阅读。

```bash
PKG='<刚导出的 template.tar.gz 路径>'
mkdir -p /tmp/tpl && tar -xzf "$PKG" -C /tmp/tpl && cd /tmp/tpl

# 4.1 不得出现绝对路径、.. 、反斜杠或盘符条目（为空才算通过）
tar -tzf "$PKG" | grep -E '^(/|.*\.\.|.*\\|.*:)'

# 4.2 三方一致性：manifest 的项目数 = data/ 目录数，且每个项目都有 22 个源文件
jq '.workspace_ids | length' manifest.json
ls data | wc -l
for d in data/*; do [ "$(ls "$d" | wc -l)" -eq 26 ] || echo "源文件不足: $d"; done

# 4.3 计数：asset_count = index 键数 = assets/ 下的文件数（键就是文件路径）
jq '.asset_count' manifest.json
jq 'length' assets/index.json
ls assets | grep -cv '^index.json$'

# 4.4 内容哈希校验：任何一行 FAIL 都说明对象被替换过或导出写坏了
jq -r 'to_entries[] | "\(.value.sha256)  \(.key)"' assets/index.json | sha256sum -c -

# 4.5 抽查图片确实进了包（file 判出的类型要与 content_type 一致）
file assets/* | head

# 4.6 离线自足：数据行里只能出现包内路径，出现产出环境的存储地址即为缺陷
jq -r '.[] | .icon' workspaces.json | grep -Ev '^(assets/[0-9a-f]{32}\.[a-z0-9]+)?$' && echo 'icon 引用了包外地址'
grep -rlE 'https?://[^"]*/(uploads/|workspace/|requirement/|knowledge_base/)' data/ workspaces.json && echo '正文里残留产出环境地址'
```

## 5. 提交前公开性审查

包会进入 git 并被所有私有化环境拉取，**视同公开发布**。导出不是自动脱敏器，下面这些必须人看：

- [ ] **`missing_asset_count` 为 0**。不为 0 说明模板侧有对象读不到，那些引用在包里已被清空
      （图片位置会空着或显示文字），新项目不会指向任何产出环境；要么修数据重导，要么在提交说明里
      写清楚接受了哪些缺失。
- [ ] **富文本与知识正文里的外链**。`knowledge_link_contents.source_url` 与 `requirements.description`
      里的绝对外链原样入包，私有化环境会请求产出环境的地址；模板内容里不该留这种链接。
- [ ] **附件原文**。内容扫描只带走被引用的对象，不带走知识文档的源附件之外的文件；若源附件
      （`source_attachment_id`）本身含客户数据，它随 `source_asset` 入包，必须人工确认。
- [ ] **`file_name` 可能含 PII**。索引里的 `file_name` 取上传时的原始文件名（如"张工-需求确认单.png"），
      会随包公开；模板账号上传文件时应使用无含义名称，或接受改名后的历史包。
- [ ] **webhook 动作已全部摘除，且代价已知**。`automation_rules.config_json` 里 `type = webhook` 的动作被整条删除（`url`、`secret`、`body`、`template`、`at_user_ids`、`at_roles` 随之消失），触发条件、其他动作和其余节点照常入包，管理员在新项目里补一步通知即可；因此**只由 webhook 动作构成的节点/分支会一起消失**，一条规则摘到不剩任何节点时整条不入包（导出日志 `carries nothing but webhooks`）。确认下面这条为空即通过：

      ```bash
      grep -l '"type":"webhook"' data/*/automation_rules.json
      ```
- [ ] **角色与权限、自定义字段、视图过滤器**都是模板账号的真实配置，确认没有内部项目名、客户名。
- [ ] **项目图标与两张邀请表的 `theme_image`** 会随包发布（正文与知识文档引用的对象同理），确认图片
      本身可公开。`workspaces.icon` 现在入包：新建项目的模板卡片用它显示头像，私有化环境由本环境按
      包内路径提供（`GET /api/workspace-templates/<id>/icon`），不请求产出环境。

## 6. 附录：源键与采用的表

列清单以 `template_source.go` 为准，本表只标明每个包文件来自哪张物理表、由哪个复制步骤演化而来。

| `data/<wsId>/` 文件 | 物理表 | 对应复制步骤 | 派生字段 |
|---|---|---|---|
| `workspace_statuses.json` | `workspace_statuses` | `copyStatuses` | |
| `workspace_priorities.json` | `workspace_priorities` | `copyPriorities` | |
| `requirement_categories.json` | `requirement_categories` | `copyCategories` | |
| `workspaces_settings.json` | `workspaces_settings` | `copySettings` | |
| `workspace_field_configs.json` | `workspace_field_configs` | `copyFieldConfigs` | |
| `workspace_custom_fields.json` | `workspace_custom_fields` | `copyCustomFields` | |
| `workspace_work_item_attributes.json` | `workspace_work_item_attributes` | `copyWorkItemAttributes` | |
| `workspace_applications.json` | `workspace_applications` | `copyApplications` | |
| `workspace_issue_invites.json` | `workspace_issue_invites` | `copyApplications` 内联 | |
| `workspace_navigation_settings.json` | `workspace_navigation_settings` | `copyNavigationSettings` | |
| `planning_groups.json` | `planning_groups` | `copyPlanningGroups` | |
| `versions.json` | `versions` JOIN `version_workspaces` | `copyVersions` | `is_shared` |
| `workspace_work_item_templates.json` | `workspace_work_item_templates` | `copyWorkItemTemplates` | |
| `workspace_issue_item_templates.json` | `workspace_issue_item_templates` JOIN `work_item_templates` | `copyIssueInviteTemplates` | `fields_json` |
| `workspace_views.json` | `workspace_views`（两条查询的并集） | `copyViews` | |
| `automation_rules.json` | `automation_rules` | `copyAutomationRules` | 摘除 webhook 动作后的 `config_json` |
| `roles.json` | `roles`（`scope='project'`） | `copyRoles` + `syncRolePermissions` | `permissions` |
| `requirements_planning.json` | `requirements`（`type='planning'`） | `copyPlannings` | |
| `requirements.json` | `requirements`（需求 + 缺陷） | `copyWorkItems` | |
| `knowledge_items.json` | `knowledge_items` | `copyKnowledgeItems` | |
| `knowledge_document_contents.json` | `knowledge_document_contents` | `copyKnowledgeDocumentContent` | `source_asset` |
| `knowledge_link_contents.json` | `knowledge_link_contents` | `copyKnowledgeLinkContent` | |
| `workspace_dashboards.json` | `workspace_dashboards`（`deleted_at = 0`） | `copyDashboards`（语义参照 `CopyDashboardProjectTemplateTx`：盘面归属操作人，成员可见归一为公开） | |
| `dashboard_visibility_roles.json` | `dashboard_visibility_roles` | `insertCopiedDashboardRoles`，`role_id` 随角色映射换 id；全部换不到时该盘降级公开 | |
| `dashboard_cards.json` | `dashboard_cards`（`deleted_at = 0`） | `copyCopiedDashboardCards`，`config` 复制侧规范化（图表清状态/优先级筛选，指标清 filters），超单盘上限或解析失败的卡片跳过 | 文本卡正文图片引用改写为包内路径 |
| `dashboard_announcements.json` | `dashboard_announcements`（项目级唯一一行） | `copyCopiedDashboardAnnouncement`；包内无此行时保留新项目的默认公告 | 正文图片引用改写为包内路径 |
| `work_item_templates.json`（根级） | `work_item_templates` | `copyPoolTemplate` | 只带被引用的池模板 |
| `workspaces.json`（根级） | `workspaces` | `ListWorkspaceTemplates` + `toWorkspaceModel` | `icon` 改写为包内资产路径，供模板卡片头像接口读取 |
| `assets/`（根级） | `enterprise_files_0` ~ `enterprise_files_9` | 资产元数据与知识源附件反查 | 取值对象字节；索引键 = 包内路径 |
