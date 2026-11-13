# GoTeams Open API Reference

This document describes every endpoint of the GoTeams Open API (`/api/v1`).

**It contains no key.** Before calling anything, create a key under "Personal Settings → API Token" in GoTeams and send it as the `Authorization: Bearer <API Key>` header.

## 1. Basics

| Item | Value |
| --- | --- |
| Base URL | `{{site_url}}/api/v1` |
| Auth | Request header `Authorization: Bearer <API Key>` |
| Request body | JSON (UTF-8); write endpoints must send `Content-Type: application/json` |
| Uploads | `multipart/form-data`, with the file field named `file` |
| Acting as | The key's creator |

**Two layers of checks**: besides the features granted to the key, every endpoint also checks project membership and role permissions for the caller. So "the key has this feature" does not mean it can act on a given project — missing project permission returns 403.

**What a key can do**: a key can only call the features ticked when it was created or last edited. Calling an endpoint whose feature is not granted returns 403; edit the key under "Personal Settings → API Token" to grant it.

## 2. Conventions

### 2.1 Response shape

A successful call always returns:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {}
}
```

The payload is in `data`. **Judge success by the HTTP status code**, not by `code`.

### 2.2 Status codes

| Status | Meaning |
| --- | --- |
| 200 | Success |
| 400 | Invalid parameter, invalid ID, or an illegal field value |
| 401 | The key is invalid or disabled, or the account it belongs to has been disabled or removed |
| 403 | The key lacks this feature; the caller is not a project member; the role lacks the permission; the storage quota is exceeded |
| 404 | The resource does not exist, or is not visible to you |
| 405 | This method is not supported on this path |
| 429 | The daily call limit has been reached |
| 500 | Server error |

The `message` of a failure response is a readable reason and can be shown to the user as is.

### 2.3 Pagination

`page` starts at 1; `size` defaults to 20 and maxes out at 100. List endpoints all return:

```json
{
  "list": [],
  "total": 0,
  "page": 1,
  "size": 20
}
```

### 2.4 IDs and values

- `wsId` (project), `itemId` (work item), `status_id`, `priority_id`, `version_id`, `parent_id`, `assignee_ids` and the keys of `custom_fields` are all server-generated IDs. **Take them from API responses only — never assemble, guess, or derive them by position.**
- An ID in the path that is not a positive decimal integer returns 400 "ID 无效". For a positive integer: a record that does not exist, or is outside what you can see, returns 404 (a project reports "工作空间不存在", a work item reports "需求不存在"; the two are deliberately indistinguishable so nobody can probe other people's data by status code).
- A UI link the user pastes already carries the IDs: UI addresses look like `/workspace/<wsId>/work-items/<itemId>` — in `{{site_url}}/workspace/13/work-items/95`, `13` is the `wsId` and `95` is the `itemId`. Sibling paths such as `/workspace/<wsId>/board`, `/versions` and `/settings` carry only the `wsId`. The host and port in the link belong to the UI, not to the API — read the IDs and drop the rest, never build an API URL from it.
- Values are not interchangeable across fields: status, priority, version and member values each come from their own endpoint. Reusing a value echoed for one field on another field can point at a completely different record.
- A cross-project `status_id` / `priority_id` is rejected (400 "状态配置不存在" or "ID 无效"). Candidate values must come from that project's `metadata`.

### 2.5 Time

`planned_start_date`, `planned_end_date`, `created_at` and `updated_at` are all **Unix seconds** (10-digit integers). `0` means unset. Do not send milliseconds.

### 2.6 Placeholders used in the examples

| Placeholder | Meaning |
| --- | --- |
| `<API Key>` | The key plaintext created or copied under "Personal Settings → API Token" |
| `<wsId>` | Project ID, taken from the `id` returned by `GET /api/v1/workspaces` |
| `<itemId>` | Work item ID, taken from the `id` returned by the my work list or a work item detail response |
| `<asset url>` | The URL of an image/file/video in a work item body, taken from `description` |

## 3. Endpoint overview

| Method | Path | Required feature | Description |
| --- | --- | --- | --- |
| GET | `/api/v1/my-work` | List my work (my_work:list) | Work items assigned to me, across projects, paginated |
| GET | `/api/v1/workspaces` | List workspaces (workspace:list) | Projects visible to me; the returned `id` is the `wsId`, and `metadata` gives every candidate field value for that project |
| GET | `/api/v1/workspaces/{wsId}/work-items` | List the work items in a project (work_item:list) | Work items in a project, paginated; `type` filters requirements/defects |
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | View work item (work_item:view) | A single item, additionally returning the body and custom fields |
| POST | `/api/v1/workspaces/{wsId}/work-items` | Create work item (work_item:create) | Create a requirement or defect |
| PATCH | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | Update work item (work_item:update) | Only the fields present in the request are changed |
| POST | `/api/v1/workspaces/{wsId}/work-items/{itemId}/comments` | Comment on work item (work_item:comment) | Append a comment |
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}/files/download` | View work item (work_item:view) | Download images/files/videos by asset URL |
| POST | `/api/v1/workspaces/{wsId}/assets/image` | Create or update work item (work_item:create / work_item:update) | Upload an image; multipart field name `file` |
| POST | `/api/v1/workspaces/{wsId}/assets/file` | Create or update work item (work_item:create / work_item:update) | Upload a file (document/archive) |
| POST | `/api/v1/workspaces/{wsId}/assets/video` | Create or update work item (work_item:create / work_item:update) | Upload a video |

## 4. Endpoint details

### 4.1 List my work

`GET {{site_url}}/api/v1/my-work`

Required feature: List my work (my_work:list)

Query parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| page | int | no | Page number, starting at 1 |
| size | int | no | Page size, default 20, max 100 |

Request example:

```bash
curl -X GET "{{site_url}}/api/v1/my-work?page=1&size=20" \
  -H "Authorization: Bearer <API Key>"
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "list": [
      {
        "id": 1024,
        "workspace_id": 37,
        "title": "Login page misaligned in Safari",
        "type": "requirement",
        "status_id": 12,
        "status_name": "Unassigned",
        "priority_id": 3,
        "priority_name": "High",
        "version_id": 5,
        "version_name": "v1.2",
        "assignee_ids": [9],
        "assignee_names": ["Zhang San"],
        "planned_start_date": 1788451200,
        "planned_end_date": 0,
        "estimated_work_hours": 8.5,
        "completed_work_hours": 0,
        "workspace_name": "Website revamp",
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

Field notes are in "Appendix A: Work item fields".

### 4.2 List workspaces

`GET {{site_url}}/api/v1/workspaces`

Required feature: List workspaces (workspace:list)

No parameters. Returns every project visible to the key's creator; each project carries a `metadata` block listing all writable field candidates for that project.

Request example:

```bash
curl -X GET "{{site_url}}/api/v1/workspaces" \
  -H "Authorization: Bearer <API Key>"
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": [
    {
      "id": 37,
      "name": "Website revamp",
      "description": "2026 website revamp",
      "color": "#0d68ff",
      "icon": "project",
      "created_at": 1788451200,
      "updated_at": 1788454800,
      "metadata": {
        "requirement": {
          "statuses": [
            {"id": 12, "key": "status_pending", "name": "Unassigned", "color": "#0d68ff", "sort_order": 1, "is_default": true}
          ],
          "priorities": [
            {"id": 3, "key": "priority_high", "label": "High", "color": "#ff7d00", "sort_order": 1, "is_default": true}
          ],
          "custom_fields": [
            {
              "id": 88,
              "key": "env",
              "name": "Environment",
              "field_type": "select_single",
              "is_required": true,
              "is_enabled": true,
              "options": [{"key": "prod", "name": "Production", "color": "#ff0000"}]
            }
          ]
        },
        "defect": {"statuses": [], "priorities": [], "custom_fields": []},
        "versions": [
          {"id": 5, "name": "v1.2", "start_date": 1788451200, "end_date": 0, "is_ended": false, "iteration_type": "single"}
        ],
        "members": [
          {"user_id": 9, "user_name": "Zhang San", "user_username": "zhangsan"}
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

Notes:

- Requirements and defects each have their own group: the statuses, priorities and custom field definitions differ between the two, so never mix them.
- Mapping: `statuses[].id` → `status_id`, `priorities[].id` → `priority_id`, `versions[].id` → `version_id`, `members[].user_id` → `assignee_ids`, `custom_fields[].id` → a key of the `custom_fields` object.
- `is_default` is that group's default value; `settings` gives the initial and final statuses, where the final status ends the flow.
- Iterations can span projects; the same iteration appears once in the `versions` of every project it belongs to.
- Parts the project has never configured come back as empty arrays; `settings` fields may be omitted entirely.
- **To change a work item field, take candidates from here**: a value the project has never used (for example it has never used "Urgent") does not appear on any work item and is only available here.

### 4.3 List work items in a project

`GET {{site_url}}/api/v1/workspaces/{wsId}/work-items`

Required feature: List the work items in a project (work_item:list)

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |

Query parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| type | string | no | `requirement` or `defect`; omitted means any type |
| page | int | no | Page number, starting at 1 |
| size | int | no | Page size, default 20, max 100 |

Without `type` the call is allowed when either requirement or defect is visible; with `type` it requires the view permission of that type. The list is also narrowed by the project role's data view scope, so it only returns work items the user is allowed to see.

Request example:

```bash
curl -X GET "{{site_url}}/api/v1/workspaces/<wsId>/work-items?type=requirement&page=1&size=20" \
  -H "Authorization: Bearer <API Key>"
```

The response shape matches 4.1 (`list` / `total` / `page` / `size`), and each record additionally carries `workspace_name`.

### 4.4 Get a work item

`GET {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}`

Required feature: View work item (work_item:view)

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |
| itemId | int | yes | Work item ID |

Request example:

```bash
curl -X GET "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>" \
  -H "Authorization: Bearer <API Key>"
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "id": 1024,
    "workspace_id": 37,
    "parent_id": 1010,
    "title": "Login page misaligned in Safari",
    "type": "requirement",
    "status_id": 12,
    "status_name": "Unassigned",
    "priority_id": 3,
    "priority_name": "High",
    "version_id": 5,
    "version_name": "v1.2",
    "assignee_ids": [9],
    "assignee_names": ["Zhang San"],
    "planned_start_date": 1788451200,
    "planned_end_date": 0,
    "estimated_work_hours": 8.5,
    "completed_work_hours": 0,
    "created_at": 1788451200,
    "updated_at": 1788454800,
    "description": "<p>The button is misaligned in Safari: <img src=\"<asset url>\"></p>",
    "description_mode": "rich-text",
    "comment_count": 2,
    "creator_name": "Li Si",
    "custom_fields": {"88": "prod"}
  }
}
```

Notes:

- Detail returns `description` (the body, HTML), `description_mode` (the editor the web app opens the body with), `comment_count`, `creator_name` and `custom_fields` on top of the list fields; **detail does not return `workspace_name`**.
- `creator_name` is resolved live from the creator's account and is an empty string when that account no longer exists.
- The URLs of images, files and videos in the body are in `description` (either a relative path or a full URL). Pass them verbatim to the endpoint in 4.8; do not build the site domain yourself.

### 4.5 Create a work item

`POST {{site_url}}/api/v1/workspaces/{wsId}/work-items`

Required feature: Create work item (work_item:create)

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |

Request body (JSON):

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| title | string | yes | Title, at most 500 characters |
| description | string | no | Body; the content is always HTML (see "Body format" in the notes for 4.5) |
| description_mode | string | no | Which editor the web app opens the body with: `rich-text` (default) or `markdown`. It does not change the content format |
| type | string | no | `requirement` (default) or `defect`; cannot be changed after creation |
| status_id | int | no | Status ID |
| priority_id | int | no | Priority ID |
| parent_id | int | no | Parent requirement ID (when creating a sub-requirement) |
| version_id | int | no | Version ID |
| assignee_ids | int[] | no | Assignee ID list |
| planned_start_date | int | no | Planned start, Unix seconds |
| planned_end_date | int | no | Planned end, Unix seconds |
| estimated_work_hours | number | no | Estimated hours, ≥ 0, at most two decimals; **silently zeroed for defects** |
| custom_fields | object | no | `{"custom field ID": "value"}`; see "Appendix C" for value formats |
| image_file_ids | int[] | no | IDs of assets uploaded earlier for the body (the `file_id` returned by 4.9). Pass them when creating the item or the files will not be attached |

Request example:

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/work-items" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Login page misaligned in Safari",
    "type": "requirement",
    "status_id": 12,
    "priority_id": 3,
    "assignee_ids": [9],
    "estimated_work_hours": 8.5,
    "custom_fields": {"88": "prod"}
  }'
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {"id": 1024}
}
```

Notes:

- Creating without `title` returns the raw Go struct validation text (`Key: 'createApiWorkItemRequest.Title' ...`), not a readable message.
- **Only requirements have estimated hours**: when `type` is `defect`, sending `estimated_work_hours` does not error but the server zeroes it. Check the detail response — never treat the value you sent as the result.
- **Body format**: `description` is always HTML, whatever `description_mode` is set to. `description_mode` only declares which editor the web app opens the body with — `rich-text` (default) uses the rich-text editor, `markdown` uses the Markdown editor. Both editors read and write HTML, and the server never converts the body.

To write with Markdown syntax, convert the Markdown to HTML first and put that HTML in `description`:

| Markdown source | Value for `description` |
| --- | --- |
| `# Background` | `<h1>Background</h1>` |
| `- point one` | `<ul><li>point one</li></ul>` |
| `**important**` | `<strong>important</strong>` |
| `![screenshot](https://.../a.png)` | `<img src="https://.../a.png" alt="screenshot">` |

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/work-items" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Requirement title",
    "description": "<h1>Background</h1><ul><li>point one</li></ul>",
    "description_mode": "markdown"
  }'
```

Never put raw Markdown source in `description`: it is then just plain text, and the Markdown editor on the web detail page adds backslashes before leading `#`, `[`, `]` and similar characters (`## Title` shows up as `\## Title`). Without `description_mode` the body is stored as `rich-text` — still HTML.

### 4.6 Update a work item

`PATCH {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}`

Required feature: Update work item (work_item:update)

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |
| itemId | int | yes | Work item ID |

The body fields match 4.5, but **only the fields present in the request are updated**; everything else stays as it was.

Request example:

```bash
curl -X PATCH "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{"status_id": 15, "assignee_ids": [9]}'
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功"
}
```

Notes:

- `type` cannot be changed; sending it is ignored.
- `image_file_ids` holds the IDs of assets added in this call (the `file_id` returned by 4.9) and **must be sent together with other fields**: sending only asset IDs without changing any column makes the endpoint return 400 "参数无效" (invalid parameter), and the files are not attached.
- Sending an empty value for `status_id`, `priority_id`, `parent_id` or `version_id` means "do not change". This endpoint cannot clear those fields.
- `assignee_ids` set to `[]` clears the assignees; omitting it entirely leaves them alone.
- `estimated_work_hours` is silently zeroed for defects: no error, no effect.
- `description_mode` only picks which editor the web app opens the body with; `description` is always HTML. Send it to switch editors, omit it to leave the editor unchanged.
- On success `data` is empty.

### 4.7 Add a work item comment

`POST {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}/comments`

Required feature: Comment on work item (work_item:comment)

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |
| itemId | int | yes | Work item ID |

Request body:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| content | string | yes | Comment text; must not be blank after trimming |

Request example:

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>/comments" \
  -H "Authorization: Bearer <API Key>" \
  -H "Content-Type: application/json" \
  -d '{"content": "Fixed in v1.2"}'
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {"id": 2048}
}
```

The permission rule matches commenting in the UI: the caller needs edit permission on that work item. A non-existent work item returns 404 "需求不存在".

### 4.8 Download images / files / videos from a work item

`GET {{site_url}}/api/v1/workspaces/{wsId}/work-items/{itemId}/files/download`

Required feature: View work item (work_item:view)

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |
| itemId | int | yes | Work item ID |

Query parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| src | string | yes | Asset URL taken from the image/file/video URL in `description`, passed verbatim |

Request example:

```bash
curl -X GET "{{site_url}}/api/v1/workspaces/<wsId>/work-items/<itemId>/files/download?src=<asset url>" \
  -H "Authorization: Bearer <API Key>" \
  -o ./download.bin
```

Response: on success the raw file stream is returned (with `Content-Disposition` and `Content-Type`); the body is not JSON.

Notes:

- `src` may be a full URL or a relative path; both resolve to the same file. Do not build the site domain yourself.
- A URL that does not belong to that work item returns 404 "附件不存在". Another cause of 404 is that the asset is gone from storage (it exists in old data, but hitting the storage directly is also 404). When that happens, try a fresh asset from the same work item to confirm, and do not retry the same URL.

### 4.9 Upload images / files / videos (body assets)

`POST {{site_url}}/api/v1/workspaces/{wsId}/assets/image`
`POST {{site_url}}/api/v1/workspaces/{wsId}/assets/file`
`POST {{site_url}}/api/v1/workspaces/{wsId}/assets/video`

Required feature: Create work item (work_item:create) or Update work item (work_item:update) — either one is enough.

Path parameters:

| Parameter | Type | Required | Notes |
| --- | --- | --- | --- |
| wsId | int | yes | Project ID |

Request body: `multipart/form-data`, with the file field named `file`.

| Kind | Allowed extensions | Size limit |
| --- | --- | --- |
| Image | jpg, jpeg, png, gif, webp | 20MB |
| File | pdf, doc, docx, xls, xlsx, ppt, pptx, txt, csv, md, zip, rar, 7z, html, htm | 200MB |
| Video | mp4, webm | 200MB |

Request example:

```bash
curl -X POST "{{site_url}}/api/v1/workspaces/<wsId>/assets/image" \
  -H "Authorization: Bearer <API Key>" \
  -F "file=@./shot.png"
```

Response example:

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {
    "file_id": 4096,
    "url": "<asset url>",
    "name": "shot.png",
    "size": 20480
  }
}
```

Notes:

- **Upload first, then submit**: put `url` into the work item body (`description`) and `file_id` into `image_file_ids` of the create or update call. The order cannot be reversed — uploading without submitting the `file_id` leaves the file unattached and later undownloadable.
- Only the extension and size are validated, **never the content**: a file whose content does not match its extension (for example an `.mp4` of all zero bytes) still uploads successfully, so confirm the file is valid before sending it.
- Exceeding the storage quota returns 403.

## Appendix A: Work item fields

Fields shared by list items and detail:

| Field | Type | Notes |
| --- | --- | --- |
| id | int | Work item ID (`itemId`) |
| workspace_id | int | Owning project ID (`wsId`) |
| parent_id | int | Parent requirement ID; omitted when unset |
| title | string | Title |
| type | string | `requirement` or `defect` |
| status_id | int | Status ID; omitted when unset |
| status_name | string | Status name; always present, an empty string when unset |
| priority_id | int | Priority ID; omitted when unset |
| priority_name | string | Priority name; always present, an empty string when unset |
| version_id | int | Version ID; omitted when unset |
| version_name | string | Version name; omitted when unset |
| assignee_ids | int[] | Assignee ID list; omitted when unset |
| assignee_names | string[] | Assignee name list |
| planned_start_date | int | Planned start, Unix seconds |
| planned_end_date | int | Planned end, Unix seconds |
| estimated_work_hours | number | Estimated hours |
| completed_work_hours | number | Completed hours |
| workspace_name | string | Project name; **present in lists, absent in detail** |
| created_at | int | Creation time, Unix seconds |
| updated_at | int | Update time, Unix seconds |

Detail additionally returns: `description` (the body, HTML), `description_mode` (the editor the web app opens the body with: `rich-text` / `markdown`), `comment_count`, `creator_name` (empty string when the account no longer exists) and `custom_fields` (keys are field IDs).

**Unset fields are omitted entirely** (`version_id`, `version_name`, `workspace_name`, `assignee_ids` and others); absence means unset. `status_name` and `priority_name` are always present but are empty strings when unset — trust the `*_id` in that case and do not invent a name.

## Appendix B: The project `metadata` block

Every project in `GET /api/v1/workspaces` carries `metadata`:

| Field | Notes |
| --- | --- |
| requirement | Statuses, priorities and custom field definitions for requirements |
| defect | Statuses, priorities and custom field definitions for defects |
| versions | Iterations/versions; `id` is used as `version_id` |
| members | Assignable members; `user_id` is used in `assignee_ids` |
| settings | Status configuration: initial statuses and final statuses |

`requirement` and `defect` share the same shape:

| Field | Notes |
| --- | --- |
| statuses | Status list; `id` is used as `status_id` |
| priorities | Priority list; `id` is used as `priority_id` |
| custom_fields | Custom field definitions; `id` is the key of the `custom_fields` object and `options` are the allowed values |

## Appendix C: Custom field value formats

A `custom_fields` value depends on the field type. **Getting it wrong fails the whole request with 400, and the other fields in the same request are not written either**:

| field_type | Correct | Wrong (returns 400) |
| --- | --- | --- |
| `select_single` | the option **key**, e.g. `opt_50644a297797` | the option name `A` / `B` (reports "选项无效") |
| `select_multi` | a **JSON array string**, e.g. `["opt_ca1a050b5087"]` | comma-separated `opt_a,opt_b` (reports "值格式不正确") |
| `date` | `2026-09-20` | — |

Take option keys from `metadata.<requirement|defect>.custom_fields[].options[].key`, not from `name`. After a write the detail endpoint echoes the value verbatim; `select_multi` echoes that JSON string itself.

## Appendix D: Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| 401 | The key is invalid or disabled, or the account it belongs to has been disabled or removed from the enterprise. Check the header, or its state under "Personal Settings → API Token" |
| 403 saying the feature is missing | This key was not granted that endpoint; edit the key and tick it |
| 403 saying you are not a project member | The caller is not in that project; join it first |
| 405 | This method is not supported on that path; check the method in "Endpoint overview" |
| 400 "ID 无效" | The ID in the path is not a positive decimal integer; take IDs from API responses only |
| 400 "参数无效" | The update sent only `image_file_ids` and has no field to change |
| 400 "状态配置不存在" | `status_id` / `priority_id` belongs to another project; take candidates from this project's `metadata` |
| 404 "工作空间不存在" | The `wsId` in the path does not exist in this enterprise |
| 404 "需求不存在" | The `itemId` does not exist, or that work item is not visible to you (out-of-scope and non-existent both return 404) |
| 429 | The daily call limit has been reached; retry later or ask an administrator to adjust the plan |
