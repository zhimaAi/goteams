# GoTeams 开放 API 接口文档

本文档描述 GoTeams 开放 API（`/api/v1`）的全部接口。

**文档中不包含任何密钥。** 调用前请先在 GoTeams「个人设置 → API Token」里创建一个 Key，再把它作为请求头 `Authorization: Bearer <API Key>` 传进来。

## 1. 基本信息

| 项 | 值 |
| --- | --- |
| 接口地址 | `{{site_url}}/api/v1` |
| 鉴权方式 | 请求头 `Authorization: Bearer <API Key>` |
| 请求体格式 | JSON（UTF-8），写接口必须带 `Content-Type: application/json` |
| 上传格式 | `multipart/form-data`，文件字段名固定为 `file` |
| 调用身份 | Key 的创建者本人 |

**两层校验**：除了 Key 自身开通的功能，接口还会按调用者身份校验项目成员与角色权限。所以「Key 有该功能」不代表一定能操作某个项目，缺少项目权限时返回 403。

**Key 的能力范围**：一个 Key 只能调用它在创建或编辑时勾选的功能。调用未开通的接口返回 403；需要时在「个人设置 → API Token」里编辑该 Key 补上勾选。

## 2. 通用约定

### 2.1 响应结构

成功时统一返回：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {}
}
```

业务内容都在 `data` 里。**判断成败请看 HTTP 状态码**，不要只看 `code`。

### 2.2 状态码

| 状态码 | 含义 |
| --- | --- |
| 200 | 成功 |
| 400 | 参数错误、ID 非法或字段取值不合法 |
| 401 | Key 无效、已停用，或其归属账号已被禁用/移除 |
| 403 | Key 没有该功能；不是该项目成员；角色权限不足；存储容量超限 |
| 404 | 资源不存在，或对你不可见 |
| 405 | 该路径不支持这个方法 |
| 429 | 当日调用次数已达上限 |
| 500 | 服务异常 |

失败响应的 `message` 是可读原因，可以直接展示给用户。

### 2.3 分页

`page` 从 1 开始；`size` 默认 20、最大 100。列表类接口统一返回：

```json
{
  "list": [],
  "total": 0,
  "page": 1,
  "size": 20
}
```

### 2.4 ID 与取值

- `wsId`（项目）、`itemId`（工作项）、`status_id`、`priority_id`、`version_id`、`parent_id`、`assignee_ids`、`custom_fields` 的键，都是服务端生成的 ID，**只能从接口返回里取，不要自己拼、不要自己猜、不要按顺序推算**。
- 路径里的 ID 不是十进制正整数返回 400「ID 无效」；是正整数、但那条记录不存在或不在你的可见范围内返回 404（项目报「工作空间不存在」，工作项报「需求不存在」，两种情况刻意不区分，避免靠状态码探测别人的数据）。
- 用户贴的界面链接里就带着 ID：界面地址形如 `/workspace/<wsId>/work-items/<itemId>`，例如 `{{site_url}}/workspace/13/work-items/95` 里 `13` 是 `wsId`、`95` 是 `itemId`。同级还有 `/workspace/<wsId>/board`、`/versions`、`/settings` 这类只带 `wsId`、不带 `itemId` 的地址。链接里的域名与端口属于界面，取完 ID 就丢掉，不要拿去拼接口地址。
- 不同字段的取值不能互借：状态、优先级、版本、成员各自从对应接口取。把某个字段回显出来的值用到别的字段上，可能指向完全不同的记录。
- 跨项目的 `status_id` / `priority_id` 会被拒（400「状态配置不存在」或「ID 无效」），候选值必须取自本项目的 `metadata`。

### 2.5 时间

`planned_start_date`、`planned_end_date`、`created_at`、`updated_at` 都是 **Unix 秒**（10 位整数），`0` 表示未设置，不要传毫秒。

### 2.6 本文示例中的占位符

| 占位符 | 说明 |
| --- | --- |
| `<API Key>` | 在「个人设置 → API Token」里创建或复制的 Key 明文 |
| `<wsId>` | 项目 ID，取自 `GET /api/v1/workspaces` 返回的 `id` |
| `<itemId>` | 工作项 ID，取自「我的工作」列表或工作项详情返回的 `id` |
| `<素材地址>` | 工作项正文里图片/文件/视频的地址，取自详情 `description` |

## 3. 接口一览

| 方法 | 路径 | 需要的功能 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/v1/my-work` | 查询我的工作列表（my_work:list） | 我负责的工作项，跨项目，分页 |
| GET | `/api/v1/workspaces` | 查看项目列表（workspace:list） | 我可见的项目；返回的 `id` 即 `wsId`，`metadata` 给出该项目全部字段候选值 |
| GET | `/api/v1/workspaces/{wsId}/work-items` | 查看项目下工作项列表（work_item:list） | 项目内工作项列表，分页，`type` 可筛需求/缺陷 |
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | 查看工作项详情（work_item:view） | 单条详情，多返回正文与自定义字段 |
| POST | `/api/v1/workspaces/{wsId}/work-items` | 新增工作项（work_item:create） | 新建需求或缺陷 |
| PATCH | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | 修改工作项（work_item:update） | 只改请求里出现的字段 |
| POST | `/api/v1/workspaces/{wsId}/work-items/{itemId}/comments` | 新增工作项评论（work_item:comment） | 追加一条评论 |
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}/files/download` | 查看工作项详情（work_item:view） | 按素材地址下载图片/文件/视频 |
| POST | `/api/v1/workspaces/{wsId}/assets/image` | 新增或修改工作项（work_item:create / work_item:update） | 上传图片，multipart 字段名 `file` |
| POST | `/api/v1/workspaces/{wsId}/assets/file` | 新增或修改工作项（work_item:create / work_item:update） | 上传文件（文档/压缩包） |
| POST | `/api/v1/workspaces/{wsId}/assets/video` | 新增或修改工作项（work_item:create / work_item:update） | 上传视频 |

## 4. 接口详情

### 4.1 查询我的工作列表

`GET {{site_url}}/api/v1/my-work`

需要的功能：查询我的工作列表（my_work:list）

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| page | int | 否 | 页码，从 1 开始 |
| size | int | 否 | 每页条数，默认 20，最大 100 |

请求示例：

```bash
curl -X GET "{{site_url}}/api/v1/my-work?page=1&size=20" \
  -H "Authorization: Bearer <API Key>"
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "list": [
      {
        "id": 1024,
        "workspace_id": 37,
        "title": "登录页在 Safari 下错位",
        "type": "requirement",
        "status_id": 12,
        "status_name": "待处理",
        "priority_id": 3,
        "priority_name": "高",
        "version_id": 5,
        "version_name": "v1.2",
        "assignee_ids": [9],
        "assignee_names": ["张三"],
        "planned_start_date": 1788451200,
        "planned_end_date": 0,
        "estimated_work_hours": 8.5,
        "completed_work_hours": 0,
        "workspace_name": "官网改版",
        "created_at": 1788451200,
        "updated_at": 1788454800
      }
    ],
    "total": 1,
    "page": 1,
    "size": 20
  }
}
```

字段说明见「附录 A：工作项字段」。

### 4.2 查看项目列表

`GET {{site_url}}/api/v1/workspaces`

需要的功能：查看项目列表（workspace:list）

无请求参数。返回当前 Key 归属用户可见的全部项目，每条项目除基础字段外带一个 `metadata`，列出该项目所有可写字段的候选值。

请求示例：

```bash
curl -X GET "{{site_url}}/api/v1/workspaces" \
  -H "Authorization: Bearer <API Key>"
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": [
    {
      "id": 37,
      "name": "官网改版",
      "description": "2026 年官网改版项目",
      "color": "#0d68ff",
      "icon": "project",
      "created_at": 1788451200,
      "updated_at": 1788454800,
      "metadata": {
        "requirement": {
          "statuses": [
            {"id": 12, "key": "status_pending", "name": "待处理", "color": "#0d68ff", "sort_order": 1, "is_default": true}
          ],
          "priorities": [
            {"id": 3, "key": "priority_high", "label": "高", "color": "#ff7d00", "sort_order": 1, "is_default": true}
          ],
          "custom_fields": [
            {
              "id": 88,
              "key": "env",
              "name": "环境",
              "field_type": "select_single",
              "is_required": true,
              "is_enabled": true,
              "options": [{"key": "prod", "name": "生产", "color": "#ff0000"}]
            }
          ]
        },
        "defect": {"statuses": [], "priorities": [], "custom_fields": []},
        "versions": [
          {"id": 5, "name": "v1.2", "start_date": 1788451200, "end_date": 0, "is_ended": false, "iteration_type": "single"}
        ],
        "members": [
          {"user_id": 9, "user_name": "张三", "user_username": "zhangsan"}
        ],
        "settings": {
          "requirement_initial_status_id": 12,
          "requirement_final_status_ids": [15]
        }
      }
    }
  ]
}
```

要点：

- 需求与缺陷各一组：两边的状态、优先级、自定义字段定义不同，不要混用。
- 对应关系：`statuses[].id` → `status_id`，`priorities[].id` → `priority_id`，`versions[].id` → `version_id`，`members[].user_id` → `assignee_ids`，`custom_fields[].id` → `custom_fields` 对象的键。
- `is_default` 是该组的默认值；`settings` 给出初始状态与终态，终态即流程结束的状态。
- 迭代可能跨项目，同一条迭代在每个所属项目的 `versions` 里各出现一次。
- 该项目没配置过的部分返回空数组；`settings` 的字段可能整体不出现。
- **要改工作项字段时，候选值就取自这里**：某个项目从没用过的值（例如从没用过「紧急」）不会出现在工作项上，只能从这里拿。

### 4.3 查询项目内工作项列表

`GET {{site_url}}/api/v1/workspaces/{wsId}/work-items`

需要的功能：查看项目下工作项列表（work_item:list）

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| type | string | 否 | `requirement`（需求）或 `defect`（缺陷）；不传表示不限类型 |
| page | int | 否 | 页码，从 1 开始 |
| size | int | 否 | 每页条数，默认 20，最大 100 |

不传 `type` 时按「需求或缺陷任一可见」放行；传了 `type` 则要求该类型对应的查看权限。列表还会按项目角色的数据查看范围收窄，只返回该用户有权查看的工作项。

请求示例：

```bash
curl -X GET "{{site_url}}/api/v1/workspaces/<wsId>/work-items?type=requirement&page=1&size=20" \
  -H "Authorization: Bearer <API Key>"
```

响应结构与 4.1 相同（`list` / `total` / `page` / `size`），`list` 里每条记录多一个 `workspace_name`。

### 4.4 查看工作项详情

`GET {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}`

需要的功能：查看工作项详情（work_item:view）

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |
| itemId | int | 是 | 工作项 ID |

请求示例：

```bash
curl -X GET "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>" \
  -H "Authorization: Bearer <API Key>"
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "id": 1024,
    "workspace_id": 37,
    "parent_id": 1010,
    "title": "登录页在 Safari 下错位",
    "type": "requirement",
    "status_id": 12,
    "status_name": "待处理",
    "priority_id": 3,
    "priority_name": "高",
    "version_id": 5,
    "version_name": "v1.2",
    "assignee_ids": [9],
    "assignee_names": ["张三"],
    "planned_start_date": 1788451200,
    "planned_end_date": 0,
    "estimated_work_hours": 8.5,
    "completed_work_hours": 0,
    "created_at": 1788451200,
    "updated_at": 1788454800,
    "description": "<p>登录页在 Safari 下按钮错位，截图：<img src=\"<素材地址>\"></p>",
    "description_mode": "rich-text",
    "comment_count": 2,
    "creator_name": "李四",
    "custom_fields": {"88": "prod"}
  }
}
```

要点：

- 详情比列表项多返回 `description`（正文，HTML）、`description_mode`（Web 端打开正文用的编辑器）、`comment_count`、`creator_name`、`custom_fields`；**详情不返回 `workspace_name`**。
- `creator_name` 由创建人账号实时关联得出，账号已不存在时为空串。
- 正文里的图片、文件、视频地址就在 `description` 里（可能是相对路径，也可能是完整网址），下载时把它原样交给 4.8 的接口，不要自己拼站点域名。

### 4.5 新建工作项

`POST {{site_url}}/api/v1/workspaces/{wsId}/work-items`

需要的功能：新增工作项（work_item:create）

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |

请求体（JSON）：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| title | string | 是 | 标题，不超过 500 字 |
| description | string | 否 | 正文，内容一律是 HTML（见 4.5 要点中的「正文格式」） |
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
| custom_fields | object | 否 | `{"自定义字段 ID": "值"}`，取值格式见「附录 C」 |
| image_file_ids | int[] | 否 | 正文里先上传的素材 ID（4.9 上传接口返回的 `file_id`），建工作项时一起传，文件才会挂到本工作项 |

请求示例：

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/work-items" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "登录页在 Safari 下错位",
    "type": "requirement",
    "status_id": 12,
    "priority_id": 3,
    "assignee_ids": [9],
    "estimated_work_hours": 8.5,
    "custom_fields": {"88": "prod"}
  }'
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {"id": 1024}
}
```

要点：

- 缺 `title` 时返回的是 Go 结构体校验原文（`Key: 'createApiWorkItemRequest.Title' ...`），不是中文提示。
- **只有需求有预计工时**：`type` 为 `defect` 时传 `estimated_work_hours` 不报错，但服务端会把它归零。是否生效要看详情返回，别拿请求里传的值当结果。
- **正文格式**：`description` 的内容一律是 HTML，与 `description_mode` 传什么无关。`description_mode` 只声明 Web 端用哪种编辑器打开它——`rich-text`（默认）用富文本编辑器，`markdown` 用 Markdown 编辑器；两种编辑器都以 HTML 作为读写格式，服务端不对正文做任何转换。

用 Markdown 语法写正文时，先把 Markdown 转成 HTML 再放进 `description`：

| Markdown 源文 | 对应的 `description` 内容 |
| --- | --- |
| `# 背景` | `<h1>背景</h1>` |
| `- 要点一` | `<ul><li>要点一</li></ul>` |
| `**重点**` | `<strong>重点</strong>` |
| `![界面截图](https://.../a.png)` | `<img src="https://.../a.png" alt="界面截图">` |

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/work-items" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "需求标题",
    "description": "<h1>背景</h1><ul><li>要点一</li></ul>",
    "description_mode": "markdown"
  }'
```

不要把 Markdown 源文直接写进 `description`：那样它只是一段普通文本，Web 详情页打开后行首的 `#`、`[`、`]` 等符号前会多出反斜杠（`## 标题` 显示成 `\## 标题`）。不传 `description_mode` 时按 `rich-text` 存，正文同样要写 HTML。

### 4.6 修改工作项

`PATCH {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}`

需要的功能：修改工作项（work_item:update）

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |
| itemId | int | 是 | 工作项 ID |

请求体字段与 4.5 新建一致，但**只更新请求里出现的字段**，没传的保持原样。

请求示例：

```bash
curl -X PATCH "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{"status_id": 15, "assignee_ids": [9]}'
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功"
}
```

要点：

- `type` 不可修改，传了也会被忽略。
- `image_file_ids` 是本次新增素材的 ID（4.9 上传接口返回的 `file_id`），**必须跟其它字段一起传**：只传素材 ID、不改任何列时，接口会返回 400「参数无效」，文件也不会挂上去。
- `status_id`、`priority_id`、`parent_id`、`version_id` 传空值等于不修改；本接口不支持把这些字段置空。
- `assignee_ids` 传 `[]` 表示清空负责人；完全不传表示不动。
- `estimated_work_hours` 对缺陷会被静默归零：不报错，也不生效。
- `description_mode` 只决定 Web 端用哪种编辑器打开正文，`description` 的内容始终是 HTML；要换编辑器就传它，不换可以不传。
- 成功后 `data` 为空。

### 4.7 新增工作项评论

`POST {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}/comments`

需要的功能：新增工作项评论（work_item:comment）

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |
| itemId | int | 是 | 工作项 ID |

请求体：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| content | string | 是 | 评论内容，去空格后不能为空 |

请求示例：

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>/comments" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{"content": "已在 v1.2 修复"}'
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {"id": 2048}
}
```

权限口径与界面评论一致：需要该工作项的编辑权限。工作项不存在时返回 404「需求不存在」。

### 4.8 下载工作项里的图片 / 文件 / 视频

`GET {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}/files/download`

需要的功能：查看工作项详情（work_item:view）

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |
| itemId | int | 是 | 工作项 ID |

查询参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| src | string | 是 | 素材地址，取自详情 `description` 里的图片/文件/视频地址，原样传入 |

请求示例：

```bash
curl -X GET "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>/files/download?src=<素材地址>" \
  -H "Authorization: Bearer <API Key>" \
  -o ./download.bin
```

响应：成功时直接返回文件流（带 `Content-Disposition` 与 `Content-Type`），响应体不是 JSON。

要点：

- `src` 可能是完整网址，也可能是一段相对路径，两种写法都能下到同一个文件，不要自己拼站点域名。
- 地址不属于该工作项时返回 404「附件不存在」。404 还有另一种成因：素材在存储里已丢失（老数据里有，直接访问存储也是 404）。遇到时先换本工作项内的新素材确认，不要反复重试同一地址。

### 4.9 上传图片 / 文件 / 视频（正文素材）

`POST {{site_url}}/api/v1/workspaces/{wsId}/assets/image`
`POST {{site_url}}/api/v1/workspaces/{wsId}/assets/file`
`POST {{site_url}}/api/v1/workspaces/{wsId}/assets/video`

需要的功能：新增工作项（work_item:create）或修改工作项（work_item:update），开通任意一个即可。

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| wsId | int | 是 | 项目 ID |

请求体：`multipart/form-data`，文件字段名固定为 `file`。

| 类型 | 允许的扩展名 | 大小上限 |
| --- | --- | --- |
| 图片 | jpg、jpeg、png、gif、webp | 20MB |
| 文件 | pdf、doc、docx、xls、xlsx、ppt、pptx、txt、csv、md、zip、rar、7z、html、htm | 200MB |
| 视频 | mp4、webm | 200MB |

请求示例：

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/assets/image" \
  -H "Authorization: Bearer <API Key>" \
  -F "file=@./shot.png"
```

响应示例：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "file_id": 4096,
    "url": "<素材地址>",
    "name": "shot.png",
    "size": 20480
  }
}
```

要点：

- **先上传再提交**：`url` 写进工作项正文（`description`），`file_id` 放进新建或修改的 `image_file_ids`。顺序不能反，只上传而不把 `file_id` 一起交上去，文件不会挂到工作项上，之后也下载不到。
- 只按扩展名与大小校验，**不校验文件内容**：内容与扩展名不符（例如字节全为 0 的 `.mp4`）同样会上传成功，传之前请自行确认文件有效。
- 容量超限返回 403。

## 附录 A：工作项字段

列表项与详情共有的字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | int | 工作项 ID（`itemId`） |
| workspace_id | int | 所属项目 ID（`wsId`） |
| parent_id | int | 父需求 ID，未设置时不返回 |
| title | string | 标题 |
| type | string | `requirement`（需求）或 `defect`（缺陷） |
| status_id | int | 状态 ID，未设置时不返回 |
| status_name | string | 状态名称，一定返回，未设置时是空串 |
| priority_id | int | 优先级 ID，未设置时不返回 |
| priority_name | string | 优先级名称，一定返回，未设置时是空串 |
| version_id | int | 版本 ID，未设置时不返回 |
| version_name | string | 版本名称，未设置时不返回 |
| assignee_ids | int[] | 负责人 ID 列表，未设置时不返回 |
| assignee_names | string[] | 负责人名称列表 |
| planned_start_date | int | 预计开始时间，Unix 秒 |
| planned_end_date | int | 预计结束时间，Unix 秒 |
| estimated_work_hours | number | 预计工时（小时） |
| completed_work_hours | number | 已完成工时（小时） |
| workspace_name | string | 项目名称，**列表有、详情没有** |
| created_at | int | 创建时间，Unix 秒 |
| updated_at | int | 更新时间，Unix 秒 |

详情额外返回：`description`（正文，HTML）、`description_mode`（Web 端打开正文用的编辑器：`rich-text` / `markdown`）、`comment_count`（评论数）、`creator_name`（创建人名称，账号不存在时为空串）、`custom_fields`（自定义字段值，键是字段 ID）。

**未设置的字段会整体省略**（如 `version_id`、`version_name`、`workspace_name`、`assignee_ids`），不出现即视为未设置；`status_name`、`priority_name` 一定出现，但未设置时是空串，此时以 `*_id` 为准，不要臆造名称。

## 附录 B：项目 `metadata` 结构

`GET /api/v1/workspaces` 的每条项目都带 `metadata`，字段含义：

| 字段 | 说明 |
| --- | --- |
| requirement | 需求的状态、优先级、自定义字段定义 |
| defect | 缺陷的状态、优先级、自定义字段定义 |
| versions | 迭代/版本列表，`id` 用于 `version_id` |
| members | 可指派成员，`user_id` 用于 `assignee_ids` |
| settings | 状态配置：初始状态与终态 |

`requirement` 与 `defect` 的结构相同：

| 字段 | 说明 |
| --- | --- |
| statuses | 状态列表，`id` 用于 `status_id` |
| priorities | 优先级列表，`id` 用于 `priority_id` |
| custom_fields | 自定义字段定义，`id` 是 `custom_fields` 对象的键，`options` 是可选值 |

## 附录 C：自定义字段取值格式

`custom_fields` 的值按字段类型区别，**写错会让整条请求返回 400，同一请求里其它字段也不会生效**：

| field_type | 正确写法 | 错误写法（会 400） |
| --- | --- | --- |
| `select_single` | 选项的 **key**，如 `opt_50644a297797` | 选项名 `A` / `B`（报「选项无效」） |
| `select_multi` | **JSON 数组字符串**，如 `["opt_ca1a050b5087"]` | 逗号分隔 `opt_a,opt_b`（报「值格式不正确」） |
| `date` | `2026-09-20` | — |

选项 key 从 `metadata.<requirement|defect>.custom_fields[].options[].key` 取，不要用 `name`。写入后详情接口原样回显；`select_multi` 回显的是那段 JSON 字符串本身。

## 附录 D：常见问题

| 现象 | 原因与处理 |
| --- | --- |
| 401 | Key 无效、已被停用，或其归属账号已被禁用/移出企业，检查请求头里的 Key，或在「个人设置 → API Token」确认它的状态 |
| 403 且提示没有该功能 | 该 Key 没开通这个接口，编辑 Key 补上对应勾选 |
| 403 且提示不是项目成员 | 调用者不在该项目里，先加入项目 |
| 405 | 该路径不支持这个方法，核对「接口一览」里的方法 |
| 400「ID 无效」 | 路径里的 ID 不是十进制正整数，ID 只能从接口返回里取 |
| 400「参数无效」 | 修改时只传了 `image_file_ids`、没有任何可改字段 |
| 400「状态配置不存在」 | `status_id` / `priority_id` 跨项目了，候选值取自本项目 `metadata` |
| 404「工作空间不存在」 | 路径里的 `wsId` 在本企业里不存在 |
| 404「需求不存在」 | `itemId` 不存在，或该工作项对你不可见（越权与不存在统一返回 404） |
| 429 | 当日调用次数已达上限，稍后再试或联系管理员调整套餐 |
