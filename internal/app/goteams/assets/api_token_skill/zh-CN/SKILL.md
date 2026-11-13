---
name: {{skill_name}}
description: 通过 GoTeams 开放 API 以个人身份查看与维护工作项（需求、缺陷）。本 Key 已开通：{{permissions}}。当用户需要读取或修改 GoTeams 上的工作项时使用。
---

# GoTeams 开放 API

- 站点地址：{{site_url}}
- 鉴权：每个请求都要带请求头 `Authorization: Bearer {{api_key}}`。本 Key 已内置在脚本里，不要再向用户索取。
- 本 Key 可用的功能：{{permissions}}。没列出的功能调用会返回 403。
<!--if:missing-->
- **本 Key 未开通的功能**：{{missing_permissions}}。这些功能的接口和脚本都不在本安装包里。用户需要时，让他在 GoTeams「个人设置 → API Token」里编辑本 Key 补上勾选，然后重新下载安装包。
- **缺功能时不要硬试**：拿不到 ID、或某个字段改不动时，直接告诉用户缺哪个功能、让他补齐；不要换着参数反复重试——权限不足的请求永远不会成功。
<!--endif-->
- 请求体和响应体都是 JSON（UTF-8）；写接口必须带 `Content-Type: application/json`。
- 成功响应外壳固定为 `{"code": 200, "message": "操作成功", "data": ...}`，真正的内容在 `data` 里，脚本已自动取出，只打印 `data`。
- 失败看 HTTP 状态码，不看 `code`：400 参数或 ID 非法；401 Key 无效；403 没有该功能、Key 已停用、不是该项目成员或没有角色权限；404 不存在或对你不可见；429 当日调用次数已达上限；500 服务异常。失败响应的 `message` 是可读原因。
- 路径里的 ID 不是十进制正整数时返回 400「ID 无效」。ID 是正整数时：项目不存在返回 404「工作空间不存在」；工作项不存在、或不在你的可见范围内返回 404「需求不存在」（两者刻意不区分，避免靠状态码探测别人的数据）；项目存在但你不是它的成员返回 403。判断时状态码和 `message` 一起看。
- 未开放的方法返回 405（如 `DELETE` 工作项），不是 403。
<!--if:perm=work_item:create|work_item:update-->
- 跨项目的 `status_id` / `priority_id` 会被拒（400「状态配置不存在」/「ID 无效」），候选值必须取自本项目的 `metadata`。
<!--endif-->
<!--if:perm=work_item:create-->
- 新建缺 `title` 时返回的是 Go 结构体校验原文（`Key: 'createApiWorkItemRequest.Title' ...`），不是中文提示。
<!--endif-->

## 名词与 ID 获取方式

- 项目 = 工作空间（workspace），同一个东西。接口里写作 `wsId`，是项目的 ID。
- 工作项（work item）= 需求（`type` 为 requirement）和缺陷（`type` 为 defect）。两者存在同一张表，对外统一叫工作项；接口里写作 `itemId`，是工作项的 ID。
- ID 一律从接口返回里取，**不要自己拼、不要自己猜、不要按顺序推算**。ID 写成非数字会得到 400「ID 无效」，ID 不存在会得到 404。
- 所有 ID 都是十进制整数（JSON 里是 number，路径里直接写数字），即各表自己的主键；接口不对 ID 做任何编码或加密。
- **用户贴的界面链接里就带着 ID**：界面地址形如 `<站点>/workspace/<wsId>/work-items/<itemId>`，例如 `{{site_url}}/workspace/13/work-items/95` 里 `13` 就是 `wsId`、`95` 就是 `itemId`。拿到链接直接取这两段数字，不必再向用户确认，也不必为此多花一次列表调用。同级还有 `/workspace/<wsId>/board`、`/versions`、`/settings` 这类只带 `wsId`、不带 `itemId` 的地址。链接里的域名和端口是用户当前用的界面，跟接口的站点地址不是一回事，取完 ID 就丢掉它，不要拿它去拼接口地址。
<!--if:perm=workspace:list-->
- `wsId` 从哪来：先调 `GET /api/v1/workspaces` 拿项目列表，返回里每条记录的 `id` 就是 `wsId`，`name` 是项目名；按名字挑出用户说的那个项目。
<!--endif-->
<!--if:noperm=workspace:list-->
- `wsId` 从哪来：本 Key **没有「查看项目列表」功能**，你取不到 `wsId`。需要时直接向用户索取项目 ID，不要去调 `/api/v1/workspaces`（会返回 403）。
<!--endif-->
<!--if:perm=my_work:list|work_item:list-->
- `itemId` 从哪来：项目内工作项列表 `GET /api/v1/workspaces/{wsId}/work-items` 或我的工作 `GET /api/v1/my-work`，返回里每条记录的 `id` 就是 `itemId`。
<!--endif-->
<!--if:noperm=my_work:list|work_item:list-->
- `itemId` 从哪来：本 Key 既不能查「我的工作」，也不能查项目内工作项列表，你取不到 `itemId`。需要时直接向用户索取工作项 ID。
<!--endif-->
<!--if:perm=work_item:view-->
- `status_id`（状态）、`priority_id`（优先级）、`version_id`（版本）、`parent_id`（父需求）、`assignee_ids`（负责人）也是 ID，取法与 `itemId` 相同：详情、项目内工作项列表、我的工作都会回显这些字段，回显出来的就是该工作项当前在用的有效值。要设置成别的值时，先看同类工作项在用什么值，再用回显出来的原值，不要自己构造。
- 不同字段的取值不能互借：状态、优先级、版本、成员各自从对应接口取。ID 是各表自己的主键，把某个字段回显出来的值用到别的字段上，可能指向完全不同的记录，也可能直接被判为不存在。
<!--endif-->
<!--if:perm=workspace:list-->
- 要**改**工作项字段时，候选值从项目列表取：`GET /api/v1/workspaces` 的每条项目都带 `metadata`，含该项目需求与缺陷的全部状态、优先级、自定义字段定义，以及迭代与成员，用的就是工作项字段那一套 ID。工作项里没出现过的值（例如某个项目从没用过「紧急」）只能从这里拿。
<!--endif-->
<!--if:perm=work_item:view|work_item:create|work_item:update-->
- `custom_fields` 是对象，键是自定义字段 ID 的十进制字符串（JSON 对象的键只能是字符串），值按字段类型给字符串；键同样来自详情接口返回的 `custom_fields`。各类型的取值格式见「自定义字段取值格式」。
<!--endif-->
- 时间戳：`planned_start_date`、`planned_end_date`、`created_at`、`updated_at` 都是 **Unix 秒**（10 位整数），`0` 表示未设置，不要传毫秒。
- 分页：`page` 从 1 开始；`size` 默认 20、最大 100。列表返回 `{"list": [...], "total": 总数, "page": 页码, "size": 每页条数}`。
<!--if:perm=work_item:view-->
- 素材地址：正文里的图片、文件、视频，地址就在详情的 `description` 里（可能是相对路径，也可能是完整网址）。下载时把它原样交给下载脚本，不要自己拼站点域名。
<!--endif-->

## 脚本

`{{skill_name}}/scripts/` 下每个可用接口一个可直接运行的 Python 3 脚本，只用标准库、无需安装依赖，站点地址和 Key 已内置。在解压后的 `{{skill_name}}/` 目录下执行即可。包里只有本 Key 已开通功能对应的脚本。

| 脚本 | 对应接口 |
| --- | --- |
<!--if:perm=my_work:list-->
| `python3 scripts/my_work.py --page 1 --size 20` | 我的工作列表 |
<!--endif-->
<!--if:perm=workspace:list-->
| `python3 scripts/list_workspaces.py` | 项目列表（取 wsId） |
<!--endif-->
<!--if:perm=work_item:list-->
| `python3 scripts/list_work_items.py --ws-id <wsId> --type requirement --page 1 --size 20` | 项目内工作项列表 |
<!--endif-->
<!--if:perm=work_item:view-->
| `python3 scripts/get_work_item.py --ws-id <wsId> --item-id <itemId>` | 工作项详情 |
| `python3 scripts/download_file.py --ws-id <wsId> --item-id <itemId> --src <素材地址>` | 下载工作项里的图片/文件/视频 |
<!--endif-->
<!--if:perm=work_item:create-->
| `python3 scripts/create_work_item.py --ws-id <wsId> --title "标题" [其他选项]` | 新建工作项 |
<!--endif-->
<!--if:perm=work_item:update-->
| `python3 scripts/update_work_item.py --ws-id <wsId> --item-id <itemId> [要改的选项]` | 修改工作项 |
<!--endif-->
<!--if:perm=work_item:comment-->
| `python3 scripts/create_comment.py --ws-id <wsId> --item-id <itemId> --content "评论"` | 新增工作项评论 |
<!--endif-->
<!--if:perm=work_item:create|work_item:update-->
| `python3 scripts/upload_image.py --ws-id <wsId> --file ./shot.png` | 上传图片（正文用） |
| `python3 scripts/upload_file.py --ws-id <wsId> --file ./设计稿.pdf` | 上传文件（正文用） |
| `python3 scripts/upload_video.py --ws-id <wsId> --file ./复现.mp4` | 上传视频（正文用） |
<!--endif-->

脚本把 `data` 以 JSON 打印到标准输出；失败时把状态码和原因打到标准错误，并以非 0 退出码结束。每个脚本都可以加 `--help` 看全部参数。
<!--if:perm=work_item:create|work_item:update-->
新建与修改脚本支持 `--json '<对象>'` 或 `--json @payload.json`，用来传脚本选项没覆盖到的字段。
<!--endif-->

## 接口一览

| 方法 | 路径 | 需要的功能 | 说明 |
| --- | --- | --- | --- |
<!--if:perm=my_work:list-->
| GET | `/api/v1/my-work` | 查询我的工作列表（my_work:list） | 我负责的工作项，跨项目，分页 |
<!--endif-->
<!--if:perm=workspace:list-->
| GET | `/api/v1/workspaces` | 查看项目列表（workspace:list） | 我可见的项目列表；用返回的 `id` 作 wsId，`metadata` 给出该项目全部字段候选值 |
<!--endif-->
<!--if:perm=work_item:list-->
| GET | `/api/v1/workspaces/{wsId}/work-items` | 查看项目下工作项列表（work_item:list） | 项目内工作项，分页，`type` 可筛 requirement/defect |
<!--endif-->
<!--if:perm=work_item:view-->
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | 查看工作项详情（work_item:view） | 单条详情，多返回正文与自定义字段 |
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}/files/download` | 查看工作项详情（work_item:view） | 按素材地址下载图片/文件/视频，query 传 `src=素材地址` |
<!--endif-->
<!--if:perm=work_item:create-->
| POST | `/api/v1/workspaces/{wsId}/work-items` | 新增工作项（work_item:create） | 新建需求或缺陷 |
<!--endif-->
<!--if:perm=work_item:update-->
| PATCH | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | 修改工作项（work_item:update） | 只改请求里出现的字段 |
<!--endif-->
<!--if:perm=work_item:comment-->
| POST | `/api/v1/workspaces/{wsId}/work-items/{itemId}/comments` | 新增工作项评论（work_item:comment） | 追加一条评论 |
<!--endif-->
<!--if:perm=work_item:create|work_item:update-->
| POST | `/api/v1/workspaces/{wsId}/assets/image` | 新增或修改工作项（work_item:create / work_item:update） | 上传图片，multipart 字段名 `file` |
| POST | `/api/v1/workspaces/{wsId}/assets/file` | 新增或修改工作项（work_item:create / work_item:update） | 上传文件（文档/压缩包） |
| POST | `/api/v1/workspaces/{wsId}/assets/video` | 新增或修改工作项（work_item:create / work_item:update） | 上传视频 |
<!--endif-->

## 字段参考

<!--if:perm=workspace:list-->
### 项目列表（`GET /api/v1/workspaces`）

每条项目在 `id`、`name` 等基础字段之外带一个 `metadata`，列出该项目所有可写字段的候选值：

```json
{
  "id": 12,
  "name": "官网改版",
  "metadata": {
    "requirement": {
      "statuses": [{"id": 101, "key": "status_pending", "name": "待分配", "color": "#0d68ff", "sort_order": 1, "is_default": true}],
      "priorities": [{"id": 201, "key": "priority_urgent", "label": "紧急", "color": "#ff0000", "sort_order": 1, "is_default": true}],
      "custom_fields": [{"id": 301, "key": "env", "name": "环境", "field_type": "select", "is_required": true, "is_enabled": true, "options": [{"key": "prod", "name": "生产", "color": "#ff0000"}]}]
    },
    "defect": {"statuses": [], "priorities": [], "custom_fields": []},
    "versions": [{"id": 401, "name": "v1.2", "start_date": 1788451200, "end_date": 0, "is_ended": false, "iteration_type": "single"}],
    "members": [{"user_id": 51, "user_name": "Admin", "user_username": "admin"}],
    "settings": {"requirement_initial_status_id": 101, "requirement_final_status_ids": [103]}
  }
}
```

- 需求与缺陷各一组：两边的状态、优先级、自定义字段定义不同，不要混用。
- 对应关系：`statuses[].id` → `status_id`，`priorities[].id` → `priority_id`，`versions[].id` → `version_id`，`members[].user_id` → `assignee_ids`，`custom_fields[].id` → `custom_fields` 对象的键。
- `is_default` 是该组的默认值；`settings` 给出初始状态与终态，终态即流程结束的状态。
- 迭代可能跨项目，同一条迭代在每个所属项目的 `versions` 里各出现一次。
- 该项目没配置过的部分返回空数组；`settings` 的字段可能整体不出现。

<!--endif-->
<!--if:perm=work_item:view|work_item:create|work_item:update-->
### 自定义字段取值格式

`custom_fields` 的值按字段类型区别，写错会整条请求 400，同一请求里其它字段也不会生效：

| field_type | 正确写法 | 错误写法（会 400） |
| --- | --- | --- |
| `select_single` | 选项的 **key**，如 `opt_50644a297797` | 选项名 `A` / `B`（报「选项无效」） |
| `select_multi` | **JSON 数组字符串**，如 `["opt_ca1a050b5087"]` | 逗号分隔 `opt_a,opt_b`（报「值格式不正确」） |
| `date` | `2026-09-20` | — |

选项 key 从 `metadata.<requirement|defect>.custom_fields[].options[].key` 取，不要用 `name`。写入后详情接口原样回显；`select_multi` 回显的是那段 JSON 字符串本身。

<!--endif-->
<!--if:perm=work_item:view-->
### 工作项返回（列表项）

`id`（itemId）、`workspace_id`（wsId）、`workspace_name`、`parent_id`、`title`、`type`（requirement/defect）、`status_id`、`status_name`、`priority_id`、`priority_name`、`version_id`、`version_name`、`assignee_ids`、`assignee_names`、`planned_start_date`、`planned_end_date`、`estimated_work_hours`（预计工时）、`completed_work_hours`（已完成工时）、`created_at`、`updated_at`。未设置的字段按 `omitempty` 整体省略（`version_id`、`version_name`、`workspace_name`、`assignee_ids` 等），不出现即视为未设置；`status_name`、`priority_name` 一定出现，但未设置时是空串，此时以 `*_id` 为准，不要臆造名称。ID 字段（`id`、`workspace_id`、`parent_id`、`status_id`、`priority_id`、`version_id`、`assignee_ids`）都是数字。

### 工作项返回（详情）

详情 = 上面全部字段（**不含 `workspace_name`**），再加 `description`（正文，HTML）、`description_mode`（Web 端打开正文用的编辑器）、`comment_count`、`creator_name`、`custom_fields`。`creator_name` 由创建人账号实时关联得出，账号已不存在时为空串。

<!--endif-->
<!--if:perm=work_item:create-->
### 新建工作项请求体

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| title | string | 是 | 标题，不超过 500 字 |
| description | string | 否 | 正文，内容一律是 HTML（见下方「正文格式」） |
| description_mode | string | 否 | Web 端用哪种编辑器打开正文：`rich-text`（默认）或 `markdown`；不改变正文的内容格式 |
| type | string | 否 | `requirement`（默认）或 `defect`；新建后不可改 |
| status_id | int | 否 | 状态 ID |
| priority_id | int | 否 | 优先级 ID |
| parent_id | int | 否 | 父需求 ID（建子需求时用） |
| version_id | int | 否 | 版本 ID |
| assignee_ids | int[] | 否 | 负责人 ID 列表 |
| planned_start_date | int | 否 | 预计开始，Unix 秒 |
| planned_end_date | int | 否 | 预计结束，Unix 秒 |
| estimated_work_hours | number | 否 | 预计工时（小时），≥ 0，最多两位小数；**缺陷会被静默归零** |
| custom_fields | object | 否 | `{"自定义字段 ID": "值"}`；键是字段 ID 的十进制字符串 |
| image_file_ids | int[] | 否 | 正文里先上传的素材 ID（上传接口返回的 file_id），建工作项时一起传，文件才会挂到本工作项 |

成功后返回 `{"id": 新建工作项的 ID}`。

**只有需求有预计工时**：`type` 为 `defect` 时，传 `estimated_work_hours` 不报错，但服务端会把它归零——建完或改完再读详情，`estimated_work_hours` 都是 `0`。所以别拿请求里传的值当结果，是否生效要看详情返回。

**正文格式**：`description` 的内容一律是 HTML，与 `description_mode` 传什么无关。`description_mode` 只声明 Web 端用哪种编辑器打开它——`rich-text`（默认）用富文本编辑器，`markdown` 用 Markdown 编辑器；两种编辑器都以 HTML 作为读写格式，服务端不对正文做任何转换。

所以要用 Markdown 语法写正文，得先把 Markdown 转成 HTML 再放进 `description`：

| Markdown 源文 | 对应的 `description` 内容 |
| --- | --- |
| `# 背景` | `<h1>背景</h1>` |
| `- 要点一` | `<ul><li>要点一</li></ul>` |
| `**重点**` | `<strong>重点</strong>` |
| `![界面截图](https://.../a.png)` | `<img src="https://.../a.png" alt="界面截图">` |

```bash
python3 scripts/create_work_item.py --ws-id <wsId> --title "标题" \
    --description "<h1>背景</h1><ul><li>要点一</li></ul>" --description-mode markdown
```

**不要把 Markdown 源文直接写进 `description`**：那样它只是一段普通文本，Web 详情页打开后行首的 `#`、`[`、`]` 等符号前会多出反斜杠（`## 标题` 显示成 `\## 标题`）。不传 `--description-mode` 时按 `rich-text` 存，正文同样要写 HTML。

<!--endif-->
<!--if:perm=work_item:update-->
### 修改工作项请求体

字段与新建一致，但**只更新请求里出现的字段**，没传的保持原样。注意：

- `type` 不可修改，传了也会被忽略。
- `image_file_ids` 是本次新增素材的 ID（上传接口返回的 file_id），**必须跟其它字段一起传**：只传素材 ID、不改任何列时接口会返回 400「参数无效」，文件也不会挂上去。
- `status_id`、`priority_id`、`parent_id`、`version_id` 传 `0`（或不传）等于不修改；本接口不支持把这几项置空。
- `assignee_ids` 传 `[]` 表示清空负责人；完全不传表示不动。
- `estimated_work_hours` 对缺陷会被静默归零：不报错，也不生效。
- `description_mode` 只决定 Web 端用哪种编辑器打开正文，`description` 的内容始终是 HTML；要换编辑器就传它，不换可以不传。
- 成功后 `data` 为 `null`。

<!--endif-->
<!--if:perm=work_item:comment-->
### 新增评论请求体

`{"content": "评论内容"}`，`content` 去空格后不能为空。成功后返回 `{"id": 评论的 ID}`。

<!--endif-->
<!--if:perm=work_item:create|work_item:update-->
### 上传图片 / 文件 / 视频（正文素材）

正文里要放图片、文件或视频时，先把文件传上来，再带着返回的 ID 提交工作项。顺序不能反：

```bash
# 1. 上传，拿到 file_id（ID）与 url
python3 scripts/upload_image.py --ws-id <wsId> --file ./shot.png
# 2. 把 url 写进正文，把 file_id 一起提交
<!--if:perm=work_item:create-->
python3 scripts/create_work_item.py --ws-id <wsId> --title "登录页错位" \
    --description '<p><img src="<素材地址>"></p>' --image-file-id <file_id>
<!--endif-->
<!--if:perm=work_item:update-->
python3 scripts/update_work_item.py --ws-id <wsId> --item-id <itemId> \
    --description '<p>补充截图</p><p><img src="<素材地址>"></p>' --image-file-id <file_id2>
<!--endif-->
```

- 图片限 jpg/jpeg/png/gif/webp 且不超过 20MB；文件限 pdf/doc/docx/xls/xlsx/ppt/pptx/txt/csv/md/zip/rar/7z/html/htm 且不超过 200MB；视频限 mp4/webm 且不超过 200MB。
- 只按扩展名与大小校验，**不校验文件内容**：内容与扩展名不符（例如字节全为 0 的 `.mp4`）同样会上传成功，传之前自己确认文件有效。
- 上传返回 `{"file_id": ID, "url": 素材地址, "name": 原名, "size": 字节数}`；`url` 就是正文要用的地址，也是之后下载要传的 `src`。
- 必须一起提交：只上传、不把 `file_id` 一起交上去，文件不会挂到工作项上，后面也下载不到。
- 容量超限会返回 403。

<!--endif-->
<!--if:perm=work_item:view-->
### 下载图片 / 文件 / 视频

正文里的图片、文件、视频地址（详情 `description` 里的链接）直接用下载脚本取：

```bash
python3 scripts/download_file.py --ws-id <wsId> --item-id <itemId> --src "<素材地址>" --out ./assets
```

- `--src` 原样复制地址即可：可能是完整网址，也可能是一段相对路径；两种写法都能下到同一个文件。
- 不给 `--out` 时按接口返回的文件名存到当前目录；`--out` 给目录就存进去，给完整路径就存成那个名字；指向一个**不存在**的路径时按文件名处理，要存进新目录请先建好目录。
- 成功时打印 `{"path": 保存路径, "bytes": 字节数, "content_type": 类型}`；地址不属于该工作项时返回 404。
- 404「附件不存在」有两种成因：地址不属于该工作项，或该素材在存储里已经丢失（老数据里有，直接访问 OSS 也是 404）。遇到时先换本工作项内的新素材确认脚本没问题，不要反复重试同一地址。
- 不要重复下同一个文件，先看本地有没有。

<!--endif-->
