---
name: {{skill_name}}
description: View and maintain work items (requirements and defects) on GoTeams as an individual through the GoTeams Open API. Permissions granted to this key: {{permissions}}. Use when the user needs to read or modify work items on GoTeams.
---

# GoTeams Open API

- Site URL: {{site_url}}
- Auth: every request must carry the header `Authorization: Bearer {{api_key}}`. The key is already embedded in the scripts; never ask the user for it.
- Permissions granted to this key: {{permissions}}. Calling anything not listed returns 403.
<!--if:missing-->
- **Permissions not granted to this key**: {{missing_permissions}}. Neither the endpoints nor the scripts for those features are included in this package. If the user needs one, ask them to edit this key under "Personal Settings → API Token" in GoTeams, tick the missing permission, and download the package again.
- **Do not retry when a permission is missing**: if you cannot obtain an ID, or a field will not change, tell the user which permission is missing and let them grant it. Do not retry with different parameters — a request that lacks permission will never succeed.
<!--endif-->
- Request and response bodies are JSON (UTF-8); write endpoints must send `Content-Type: application/json`.
- A successful response is always wrapped as `{"code": 200, "message": "操作成功", "data": ...}`. The payload is in `data`; the scripts already unwrap it and print only `data`.
- Judge failure by the HTTP status code, not by `code`: 400 invalid parameter or ID; 401 invalid key; 403 the permission is missing, the key is disabled, you are not a member of that project, or your role lacks the permission; 404 it does not exist or is not visible to you; 429 the daily call limit is reached; 500 server error. The `message` in a failure response is a readable reason.
- An ID in the path that is not a positive decimal integer returns 400 "ID 无效". For a positive integer: a project that does not exist returns 404 "工作空间不存在"; a work item that does not exist, or is outside what you can see, returns 404 "需求不存在" (the two are deliberately indistinguishable so nobody can probe other people's data by status code); a project that exists but you are not a member of returns 403. Read the status code and `message` together.
- An unsupported method returns 405 (for example `DELETE` on a work item), not 403.
<!--if:perm=work_item:create|work_item:update-->
- A cross-project `status_id` / `priority_id` is rejected (400 "状态配置不存在" / "ID 无效"). Candidate values must come from that project's `metadata`.
<!--endif-->
<!--if:perm=work_item:create-->
- Creating without `title` returns the raw Go struct validation text (`Key: 'createApiWorkItemRequest.Title' ...`), not a readable message.
<!--endif-->

## Terms and how to obtain IDs

- Project = workspace; the two are the same thing. The API writes it as `wsId`, which is the project ID.
- Work item = requirement (`type` is `requirement`) and defect (`type` is `defect`). Both live in the same table and are collectively called work items. The API writes it as `itemId`, which is the work item ID.
- Always take IDs from API responses. **Never assemble, guess, or derive them by position.** A non-numeric ID returns 400 "ID 无效"; an ID that does not exist returns 404.
- Every ID is a decimal integer (a JSON number; write the digits directly in the path) — the primary key of its own table. The API applies no encoding or encryption to IDs.
- **A UI link the user pastes already carries the IDs**: UI addresses look like `<site>/workspace/<wsId>/work-items/<itemId>` — in `{{site_url}}/workspace/13/work-items/95`, `13` is the `wsId` and `95` is the `itemId`. Take those two numbers straight from the link; there is no need to ask the user, and no need to spend a list call on it. Sibling paths such as `/workspace/<wsId>/board`, `/versions` and `/settings` carry only the `wsId`, no `itemId`. The host and port in the link belong to the UI the user is running, not to the API — read the IDs and drop the rest, never build an API URL from it.
<!--if:perm=workspace:list-->
- Where `wsId` comes from: call `GET /api/v1/workspaces` to list projects. Each record's `id` is the `wsId` and `name` is the project name; pick the project the user meant by its name.
<!--endif-->
<!--if:noperm=workspace:list-->
- Where `wsId` comes from: this key **does not have the "list projects" permission**, so you cannot obtain a `wsId` yourself. Ask the user for the project ID instead of calling `/api/v1/workspaces` (it returns 403).
<!--endif-->
<!--if:perm=my_work:list|work_item:list-->
- Where `itemId` comes from: the project work item list `GET /api/v1/workspaces/{wsId}/work-items` or the my work list `GET /api/v1/my-work`. Each record's `id` is the `itemId`.
<!--endif-->
<!--if:noperm=my_work:list|work_item:list-->
- Where `itemId` comes from: this key can neither list my work nor list the work items in a project, so you cannot obtain an `itemId` yourself. Ask the user for the work item ID.
<!--endif-->
<!--if:perm=work_item:view-->
- `status_id` (status), `priority_id` (priority), `version_id` (version), `parent_id` (parent requirement) and `assignee_ids` (assignees) are IDs too, obtained exactly like `itemId`: the detail endpoint, the project work item list and my work all echo these fields, and whatever they echo is a valid value currently in use on that work item. To set a different value, first look at what comparable work items use, then reuse an echoed value verbatim — never construct one.
- Values are not interchangeable across fields: status, priority, version and member values each come from their own endpoint. An ID is the primary key of its own table, so reusing a value echoed for one field on another field can point at a completely different record — or be rejected as non-existent. Use each value strictly according to its field.
<!--endif-->
<!--if:perm=workspace:list-->
- To **change** a work item field, take candidates from the project list: every project in `GET /api/v1/workspaces` carries a `metadata` block holding all statuses, priorities and custom field definitions for that project's requirements and defects, plus iterations and members — all using the same ID scheme as the work item fields. A value that never appeared on a work item (for example a project that has never used "Urgent") is only available here.
<!--endif-->
<!--if:perm=work_item:view|work_item:create|work_item:update-->
- `custom_fields` is an object whose keys are custom field IDs written as decimal strings (JSON object keys can only be strings) and whose values are strings chosen by field type; the keys also come from the `custom_fields` returned by the detail endpoint. See "Custom field value formats" for each type.
<!--endif-->
- Timestamps: `planned_start_date`, `planned_end_date`, `created_at` and `updated_at` are all **Unix seconds** (10-digit integers). `0` means unset. Do not send milliseconds.
- Pagination: `page` starts at 1; `size` defaults to 20 and maxes out at 100. A list returns `{"list": [...], "total": <count>, "page": <page>, "size": <page size>}`.
<!--if:perm=work_item:view-->
- Asset URLs: images, files and videos in the body have their URLs in the detail's `description` (either a relative path or a full URL). Hand them to the download script verbatim; do not build the site domain yourself.
<!--endif-->

## Scripts

Under `{{skill_name}}/scripts/` there is one directly runnable Python 3 script per available endpoint. They use only the standard library, need no installation, and already embed the site URL and the key. Run them from the unpacked `{{skill_name}}/` directory. The package contains only the scripts for the permissions this key has.

| Script | Endpoint |
| --- | --- |
<!--if:perm=my_work:list-->
| `python3 scripts/my_work.py --page 1 --size 20` | My work list |
<!--endif-->
<!--if:perm=workspace:list-->
| `python3 scripts/list_workspaces.py` | Project list (to obtain wsId) |
<!--endif-->
<!--if:perm=work_item:list-->
| `python3 scripts/list_work_items.py --ws-id <wsId> --type requirement --page 1 --size 20` | Project work item list |
<!--endif-->
<!--if:perm=work_item:view-->
| `python3 scripts/get_work_item.py --ws-id <wsId> --item-id <itemId>` | Work item detail |
| `python3 scripts/download_file.py --ws-id <wsId> --item-id <itemId> --src <asset url>` | Download images/files/videos from a work item |
<!--endif-->
<!--if:perm=work_item:create-->
| `python3 scripts/create_work_item.py --ws-id <wsId> --title "Title" [other options]` | Create a work item |
<!--endif-->
<!--if:perm=work_item:update-->
| `python3 scripts/update_work_item.py --ws-id <wsId> --item-id <itemId> [options to change]` | Update a work item |
<!--endif-->
<!--if:perm=work_item:comment-->
| `python3 scripts/create_comment.py --ws-id <wsId> --item-id <itemId> --content "Comment"` | Add a work item comment |
<!--endif-->
<!--if:perm=work_item:create|work_item:update-->
| `python3 scripts/upload_image.py --ws-id <wsId> --file ./shot.png` | Upload an image (for the body) |
| `python3 scripts/upload_file.py --ws-id <wsId> --file ./design-spec.pdf` | Upload a file (for the body) |
| `python3 scripts/upload_video.py --ws-id <wsId> --file ./repro.mp4` | Upload a video (for the body) |
<!--endif-->

The scripts print `data` as JSON to standard output; on failure they print the status code and reason to standard error and exit non-zero. Every script accepts `--help` for the full option list.
<!--if:perm=work_item:create|work_item:update-->
The create and update scripts accept `--json '<object>'` or `--json @payload.json` for fields the script options do not cover.
<!--endif-->

## Endpoints

| Method | Path | Required permission | Description |
| --- | --- | --- | --- |
<!--if:perm=my_work:list-->
| GET | `/api/v1/my-work` | List my work (my_work:list) | Work items assigned to me, across projects, paginated |
<!--endif-->
<!--if:perm=workspace:list-->
| GET | `/api/v1/workspaces` | List workspaces (workspace:list) | Projects visible to me; use the returned `id` as wsId, and `metadata` gives every candidate field value for that project |
<!--endif-->
<!--if:perm=work_item:list-->
| GET | `/api/v1/workspaces/{wsId}/work-items` | List the work items in a project (work_item:list) | Work items in a project, paginated; `type` filters requirement/defect |
<!--endif-->
<!--if:perm=work_item:view-->
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | View work item (work_item:view) | A single item, additionally returning the body and custom fields |
| GET | `/api/v1/workspaces/{wsId}/work-items/{itemId}/files/download` | View work item (work_item:view) | Download images/files/videos by asset URL; pass `src=<asset url>` as a query parameter |
<!--endif-->
<!--if:perm=work_item:create-->
| POST | `/api/v1/workspaces/{wsId}/work-items` | Create work item (work_item:create) | Create a requirement or defect |
<!--endif-->
<!--if:perm=work_item:update-->
| PATCH | `/api/v1/workspaces/{wsId}/work-items/{itemId}` | Update work item (work_item:update) | Only the fields present in the request are changed |
<!--endif-->
<!--if:perm=work_item:comment-->
| POST | `/api/v1/workspaces/{wsId}/work-items/{itemId}/comments` | Comment on work item (work_item:comment) | Append a comment |
<!--endif-->
<!--if:perm=work_item:create|work_item:update-->
| POST | `/api/v1/workspaces/{wsId}/assets/image` | Create or update work item (work_item:create / work_item:update) | Upload an image; multipart field name `file` |
| POST | `/api/v1/workspaces/{wsId}/assets/file` | Create or update work item (work_item:create / work_item:update) | Upload a file (document/archive) |
| POST | `/api/v1/workspaces/{wsId}/assets/video` | Create or update work item (work_item:create / work_item:update) | Upload a video |
<!--endif-->

## Field reference

<!--if:perm=workspace:list-->
### Project list (`GET /api/v1/workspaces`)

Beyond basic fields such as `id` and `name`, every project carries a `metadata` block listing all writable field candidates for that project:

```json
{
  "id": 12,
  "name": "Website revamp",
  "metadata": {
    "requirement": {
      "statuses": [{"id": 101, "key": "status_pending", "name": "Unassigned", "color": "#0d68ff", "sort_order": 1, "is_default": true}],
      "priorities": [{"id": 201, "key": "priority_urgent", "label": "Urgent", "color": "#ff0000", "sort_order": 1, "is_default": true}],
      "custom_fields": [{"id": 301, "key": "env", "name": "Environment", "field_type": "select", "is_required": true, "is_enabled": true, "options": [{"key": "prod", "name": "Production", "color": "#ff0000"}]}]
    },
    "defect": {"statuses": [], "priorities": [], "custom_fields": []},
    "versions": [{"id": 401, "name": "v1.2", "start_date": 1788451200, "end_date": 0, "is_ended": false, "iteration_type": "single"}],
    "members": [{"user_id": 51, "user_name": "Admin", "user_username": "admin"}],
    "settings": {"requirement_initial_status_id": 101, "requirement_final_status_ids": [103]}
  }
}
```

- Requirements and defects each have their own group: the statuses, priorities and custom field definitions differ between the two, so never mix them.
- Mapping: `statuses[].id` → `status_id`, `priorities[].id` → `priority_id`, `versions[].id` → `version_id`, `members[].user_id` → `assignee_ids`, `custom_fields[].id` → a key of the `custom_fields` object.
- `is_default` is that group's default value; `settings` gives the initial and final statuses, where the final status ends the flow.
- Iterations can span projects; the same iteration appears once in the `versions` of every project it belongs to.
- Parts the project has never configured come back as empty arrays; `settings` fields may be omitted entirely.

<!--endif-->
<!--if:perm=work_item:view|work_item:create|work_item:update-->
### Custom field value formats

A `custom_fields` value depends on the field type. Getting it wrong fails the whole request with 400, and the other fields in the same request are not written either:

| field_type | Correct | Wrong (returns 400) |
| --- | --- | --- |
| `select_single` | the option **key**, e.g. `opt_50644a297797` | the option name `A` / `B` (reports "选项无效") |
| `select_multi` | a **JSON array string**, e.g. `["opt_ca1a050b5087"]` | comma-separated `opt_a,opt_b` (reports "值格式不正确") |
| `date` | `2026-09-20` | — |

Take option keys from `metadata.<requirement|defect>.custom_fields[].options[].key`, not from `name`. After a write the detail endpoint echoes the value verbatim; `select_multi` echoes that JSON string itself.

<!--endif-->
<!--if:perm=work_item:view-->
### Work item response (list item)

`id` (itemId), `workspace_id` (wsId), `workspace_name`, `parent_id`, `title`, `type` (requirement/defect), `status_id`, `status_name`, `priority_id`, `priority_name`, `version_id`, `version_name`, `assignee_ids`, `assignee_names`, `planned_start_date`, `planned_end_date`, `estimated_work_hours`, `completed_work_hours`, `created_at`, `updated_at`. Unset fields are omitted entirely via `omitempty` (`version_id`, `version_name`, `workspace_name`, `assignee_ids` and others); absence means unset. `status_name` and `priority_name` are always present but are empty strings when unset — trust the `*_id` in that case and do not invent a name. The ID fields (`id`, `workspace_id`, `parent_id`, `status_id`, `priority_id`, `version_id`, `assignee_ids`) are numbers.

### Work item response (detail)

Detail = all of the fields above (**without `workspace_name`**), plus `description` (the body, HTML), `description_mode` (the editor the web app opens the body with), `comment_count`, `creator_name` and `custom_fields`. `creator_name` is resolved live from the creator's account and is an empty string when that account no longer exists.

<!--endif-->
<!--if:perm=work_item:create-->
### Create work item request body

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| title | string | yes | Title, at most 500 characters |
| description | string | no | Body; the content is always HTML (see "Body format" below) |
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
| custom_fields | object | no | `{"custom field ID": "value"}`; the key is the field ID as a decimal string |
| image_file_ids | int[] | no | IDs of assets uploaded earlier for the body (the file_id returned by the upload endpoint). Pass them when creating the item or the files will not be attached |

On success it returns `{"id": <the new work item ID>}`.

**Only requirements have estimated hours**: when `type` is `defect`, sending `estimated_work_hours` does not error but the server zeroes it — read the detail after creating or updating and `estimated_work_hours` is `0`. So never treat the value you sent as the result; check the detail response.

**Body format**: `description` is always HTML, whatever `description_mode` is set to. `description_mode` only declares which editor the web app opens the body with — `rich-text` (default) uses the rich-text editor, `markdown` uses the Markdown editor. Both editors read and write HTML, and the server never converts the body.

So to write with Markdown syntax, convert the Markdown to HTML first and put that HTML in `description`:

| Markdown source | Value for `description` |
| --- | --- |
| `# Background` | `<h1>Background</h1>` |
| `- point one` | `<ul><li>point one</li></ul>` |
| `**important**` | `<strong>important</strong>` |
| `![screenshot](https://.../a.png)` | `<img src="https://.../a.png" alt="screenshot">` |

```bash
python3 scripts/create_work_item.py --ws-id <wsId> --title "Title" \
    --description "<h1>Background</h1><ul><li>point one</li></ul>" --description-mode markdown
```

**Never put raw Markdown source in `description`**: it is then just plain text, and the Markdown editor on the web detail page adds backslashes before leading `#`, `[`, `]` and similar characters (`## Title` shows up as `\## Title`). Without `--description-mode` the body is stored as `rich-text` — still HTML.

<!--endif-->
<!--if:perm=work_item:update-->
### Update work item request body

The fields match create, but **only the fields present in the request are updated**; everything else stays as it was. Note:

- `type` cannot be changed; sending it is ignored.
- `image_file_ids` holds the IDs of assets added in this call (the file_id returned by the upload endpoint) and **must be sent together with other fields**: sending only asset IDs without changing any column makes the endpoint return 400 "参数无效" (invalid parameter), and the files are not attached.
- Sending `0` (or omitting the field) for `status_id`, `priority_id`, `parent_id` or `version_id` means "do not change". This endpoint cannot clear those fields.
- `assignee_ids` set to `[]` clears the assignees; omitting it entirely leaves them alone.
- `estimated_work_hours` is silently zeroed for defects: no error, no effect.
- `description_mode` only picks which editor the web app opens the body with; `description` is always HTML. Send it to switch editors, omit it to leave the editor unchanged.
- On success `data` is `null`.

<!--endif-->
<!--if:perm=work_item:comment-->
### Add comment request body

`{"content": "comment text"}`; `content` must not be blank after trimming. On success it returns `{"id": <the comment ID>}`.

<!--endif-->
<!--if:perm=work_item:create|work_item:update-->
### Uploading images / files / videos (body assets)

To put an image, file or video in the body, upload it first and then submit the work item with the returned ID. The order cannot be reversed:

```bash
# 1. Upload and get file_id (the ID) plus url
python3 scripts/upload_image.py --ws-id <wsId> --file ./shot.png
# 2. Put url in the body and submit file_id along with it
<!--if:perm=work_item:create-->
python3 scripts/create_work_item.py --ws-id <wsId> --title "Login page misaligned" \
    --description '<p><img src="<asset url>"></p>' --image-file-id <file_id>
<!--endif-->
<!--if:perm=work_item:update-->
python3 scripts/update_work_item.py --ws-id <wsId> --item-id <itemId> \
    --description '<p>Added a screenshot</p><p><img src="<asset url>"></p>' --image-file-id <file_id2>
<!--endif-->
```

- Images: jpg/jpeg/png/gif/webp up to 20MB. Files: pdf/doc/docx/xls/xlsx/ppt/pptx/txt/csv/md/zip/rar/7z/html/htm up to 200MB. Videos: mp4/webm up to 200MB.
- Only the extension and size are validated, **never the content**: a file whose content does not match its extension (for example an `.mp4` of all zero bytes) still uploads successfully, so confirm the file is valid before sending it.
- Upload returns `{"file_id": <ID>, "url": <asset url>, "name": <original name>, "size": <bytes>}`; `url` is what the body should use and also the `src` to pass when downloading later.
- Both steps are required: uploading without submitting the `file_id` leaves the file unattached and later undownloadable.
- Exceeding the storage limit returns 403.

<!--endif-->
<!--if:perm=work_item:view-->
### Downloading images / files / videos

Take the image, file or video URLs in the body (the links in the detail's `description`) straight to the download script:

```bash
python3 scripts/download_file.py --ws-id <wsId> --item-id <itemId> --src "<asset url>" --out ./assets
```

- Copy `--src` verbatim: it may be a full URL or a relative path, and both resolve to the same file.
- Without `--out` the file is saved to the current directory under the name the endpoint returns. A directory saves it inside; a full path saves it under that name. A path that **does not exist** is treated as a filename, so create the directory first if you want the file inside a new one.
- On success it prints `{"path": <saved path>, "bytes": <bytes>, "content_type": <type>}`; a URL that does not belong to that work item returns 404.
- A 404 "attachment not found" has two causes: the URL does not belong to that work item, or the asset is gone from storage (it exists in old data, but hitting OSS directly is also 404). When that happens, try a fresh asset from the same work item to confirm the script works, and do not retry the same URL.
- Do not download the same file twice — check locally first.

<!--endif-->
