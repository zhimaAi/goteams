import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

# Windows 控制台/管道的默认编码不是 UTF-8，中文 JSON 会编不出来；能改就改成 UTF-8。
# The default encoding of a Windows console/pipe is not UTF-8; switch it when possible.
for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8")
    except (AttributeError, ValueError):
        pass

SITE_URL = "{{site_url}}"
API_KEY = "{{api_key}}"

# 打包时按下载者的界面语言写成 zh-CN 或 en-US，脚本里的文案跟着它走。
# The packager writes zh-CN or en-US here according to the downloader's UI language.
LANG = "{{lang}}"

MESSAGES = {
    "zh-CN": {
        "doc_client": """GoTeams 开放接口公用客户端。

站点地址和 API Key 已经写在下面，只依赖 Python 标准库，拷到哪都能跑。
这个文件含有明文 API Key，不要分享给任何人。
""",
        "err_request_failed": "请求 %s 失败：%s",
        "err_upload_file_missing": "找不到要上传的文件：%s",
        "err_key_value_form": "参数需要 key=value 形式：%s",
        "help_ws_id": "项目 ID（wsId）",
        "help_item_id": "工作项 ID（itemId）",
        "help_file": "本地文件路径",
        "help_json": "完整请求体的 JSON 字符串，或 @文件名；与上面的选项合并，同名以它为准",
        "help_page": "页码，从 1 开始",
        "help_size": "每页条数，最大 100",
        "help_title": "标题，不超过 500 字",
        "help_description": "正文内容",
        "help_description_mode": "Web 端打开正文用的编辑器，默认 rich-text；正文内容始终是 HTML",
        "help_description_mode_update": "Web 端打开正文用的编辑器",
        "help_status_id": "状态 ID",
        "help_priority_id": "优先级 ID",
        "help_parent_id": "父需求 ID",
        "help_version_id": "版本 ID",
        "help_assignee_id": "负责人 ID，可重复传多次",
        "help_assignee_id_replace": "负责人 ID，可重复传多次，传了就整体替换",
        "help_clear_assignees": "清空负责人",
        "help_planned_start_date": "预计开始，Unix 秒",
        "help_planned_end_date": "预计结束，Unix 秒",
        "help_estimated_work_hours": "预计工时（小时）",
        "help_custom_field": "自定义字段，key=value，可重复传多次",
        "help_image_file_id_create": "正文里已上传素材的 ID（upload_*.py 返回的 file_id），可重复传多次；先上传再新建才不会丢图",
        "help_image_file_id_update": "本次新增素材的 ID（upload_*.py 返回的 file_id），可重复传多次；要跟其它字段一起传才会生效",
        "help_type_filter": "只看需求或缺陷，默认不限",
        "help_type_create": "工作项类型，默认 requirement",
        "help_src": "素材地址：详情 description 里的图片/文件/视频地址，原样复制",
        "help_out": "保存位置：目录或完整文件路径，默认存到当前目录",
        "help_content": "评论内容，不能为空",
        "desc_my_work": "查询我负责的工作项",
        "desc_list_workspaces": "查询我可见的项目列表",
        "desc_list_work_items": "查询项目内的工作项列表",
        "desc_get_work_item": "查询工作项详情",
        "desc_create_work_item": "新建工作项",
        "desc_update_work_item": "修改工作项",
        "desc_create_comment": "给工作项新增一条评论",
        "desc_download_file": "下载工作项里的图片/文件/视频",
        "desc_upload_image": "上传图片（富文本素材）",
        "desc_upload_file": "上传文件（富文本素材）",
        "desc_upload_video": "上传视频（富文本素材）",
        "err_missing_title": "缺少 --title：标题必填（或用 --json 传完整请求体）",
        "err_nothing_to_update": "没有要修改的字段：至少传一个选项（或用 --json 传请求体）",
        "err_image_only": "--image-file-id 必须跟其它字段一起传：只传素材 ID 时没有任何可改字段，接口会返回 400「参数无效」；脚本在这里提前拦下，不消耗调用额度",
        "err_content_blank": "--content 不能为空",
        "doc_my_work": """GET /api/v1/my-work —— 查询我负责的工作项（跨项目，分页）。

示例：
  python3 scripts/my_work.py
  python3 scripts/my_work.py --page 2 --size 50
""",
        "doc_list_workspaces": """GET /api/v1/workspaces —— 我可见的项目列表。

返回里每条记录的 id 就是后面所有接口要用的 wsId。
""",
        "doc_list_work_items": """GET /api/v1/workspaces/{wsId}/work-items —— 项目内工作项列表（分页）。

wsId 先用 list_workspaces.py 拿。type 留空表示需求和缺陷都要。

示例：
  python3 scripts/list_work_items.py --ws-id 12
  python3 scripts/list_work_items.py --ws-id 12 --type defect --size 100
""",
        "doc_get_work_item": """GET /api/v1/workspaces/{wsId}/work-items/{itemId} —— 工作项详情。

比列表多返回 description（正文）、description_mode、comment_count、creator_name、custom_fields；
要设置状态、优先级、版本、负责人、自定义字段时，先用本接口看当前值。

示例：
  python3 scripts/get_work_item.py --ws-id 12 --item-id 1001
""",
        "doc_create_work_item": """POST /api/v1/workspaces/{wsId}/work-items —— 新建工作项（需求或缺陷）。

title 必填；其余都不传就是默认值。所有 *_id 都是数字 ID，先从列表或详情接口拿到再传。
时间戳是 Unix 秒（10 位）。正文里含先上传的图片/文件/视频时，必须用 --image-file-id 把它们的
ID 一并传上，否则文件不会挂到这条工作项上。成功后返回 {"id": 新建工作项的 ID}。
正文（--description）一律是 HTML 片段，Markdown 源文要先转成 HTML 再传；--description-mode
只决定 Web 端用哪种编辑器打开它，不改变内容格式。

示例：
  python3 scripts/create_work_item.py --ws-id 12 --title "登录页文案微调"
  python3 scripts/create_work_item.py --ws-id 12 --title "导出慢" --type defect \\
      --description "<h1>导出慢</h1><p>导出 1 万条要 3 分钟</p>" --description-mode markdown \\
      --estimated-work-hours 4
  python3 scripts/upload_image.py --ws-id 12 --file ./shot.png   # 先上传拿 file_id
  python3 scripts/create_work_item.py --ws-id 12 --title "截图" \\
      --description '<p><img src="<素材地址>"></p>' --image-file-id <上传返回的 file_id>
  python3 scripts/create_work_item.py --ws-id 12 --json @payload.json
""",
        "doc_update_work_item": """PATCH /api/v1/workspaces/{wsId}/work-items/{itemId} —— 修改工作项。

只改命令行里传了的字段，没传的保持原样；type 不可修改。
status_id / priority_id / parent_id / version_id 传 0（或不传）等于不修改。
负责人用 --assignee-id 重设整份名单；要清空用 --clear-assignees。
本次新增的素材用 --image-file-id 传 ID 一起提交，但**必须同时改其它字段**（例如 --description），
只传 --image-file-id 接口会返回 400「参数无效」；脚本会提前拦下这种请求，不消耗调用额度。
成功后 data 为 null。

示例：
  python3 scripts/update_work_item.py --ws-id 12 --item-id 1001 --title "新标题"
  python3 scripts/update_work_item.py --ws-id 12 --item-id 1001 \\
      --status-id 101 --estimated-work-hours 8
  python3 scripts/update_work_item.py --ws-id 12 --item-id 1001 --clear-assignees
""",
        "doc_create_comment": """POST /api/v1/workspaces/{wsId}/work-items/{itemId}/comments —— 新增工作项评论。

content 去空格后不能为空。成功后返回 {"id": 评论的 ID}。

示例：
  python3 scripts/create_comment.py --ws-id 12 --item-id 1001 --content "已复现，正在查"
""",
        "doc_download_file": """下载工作项里的图片/文件/视频。

素材地址就是工作项详情里 description 的 <img src> / 视频 / 文件链接（附件列表里的 url 也是同一种值），
原样传给 --src 即可；脚本按「项目 + 工作项 + 地址」把文件取回来，不用自己拼域名、不用加鉴权头。

示例：
  python3 scripts/download_file.py --ws-id 12 --item-id 1001 --src <素材地址>
  python3 scripts/download_file.py --ws-id 12 --item-id 1001 --src "https://<站点>/..." --out ./assets
""",
        "doc_upload_image": """上传图片到工作项素材库（与界面富文本编辑器上传走同一条链路）。

上传后：返回的 url 写进详情正文，返回的 file_id 要用 --image-file-id 跟新建或修改工作项一起提交，
文件才会挂到那条工作项上；没一起传的素材之后下载不到。

每次上传算 1 次接口调用。

示例：
  python3 scripts/upload_image.py --ws-id 12 --file ./screenshot.png
""",
        "doc_upload_file": """上传文件到工作项素材库（与界面富文本编辑器上传走同一条链路）。

支持 pdf/doc/docx/xls/xlsx/ppt/pptx/txt/csv/md/zip/rar/7z/html/htm，单个不超过 200MB。
上传后：url 写进详情正文，file_id 要用 --image-file-id 跟新建或修改工作项一起提交，文件才会挂到工作项上。

每次上传算 1 次接口调用。

示例：
  python3 scripts/upload_file.py --ws-id 12 --file ./设计稿.pdf
""",
        "doc_upload_video": """上传视频到工作项素材库（与界面富文本编辑器上传走同一条链路）。

只支持 mp4 与 webm，单个不超过 200MB。
上传后：url 写进详情正文，file_id 要用 --image-file-id 跟新建或修改工作项一起提交，文件才会挂到工作项上。

每次上传算 1 次接口调用。

示例：
  python3 scripts/upload_video.py --ws-id 12 --file ./复现.mp4
""",
        "doc_fail": "把失败原因打到标准错误并退出（退出码 1）。",
        "doc_msg": "取当前语言的文案；该 key 缺失时退回另一份语言，保证脚本不会因为少一条文案就崩。",
        "doc_api": """调用开放接口并返回响应里的 data。

params 里值为 None 或空字符串的项会被当作「未提供」丢掉；
body 非空时按 JSON 发送。失败时直接退出，不会返回半个结果。
""",
        "doc_show": "把接口返回的 data 以 JSON 打印到标准输出。",
        "doc_safe_filename": "只保留纯文件名：路径分隔符、空名、. 和 .. 都退回默认名，避免写出目录外。",
        "doc_attachment_filename": "取下载文件名：优先响应头 Content-Disposition，其次地址末段。",
        "doc_download": """下载二进制内容（图片/文件/视频），打印并返回保存结果。

destination 可以是完整文件路径、已存在的目录，或省略（存到当前目录，
文件名取响应头或地址里的名字）。
""",
        "doc_upload": """以 multipart/form-data 上传一个文件，返回接口的 data。

富文本素材走这里：返回里的 url 写进正文，file_id 要在新建/修改工作项时
用 image_file_ids 一起传，文件才算真正挂到那条工作项上。
""",
        "doc_json_argument": "解析 --json：直接是 JSON 字符串，或 @文件名 从文件读。",
        "doc_key_value_list": "把 --custom-field key=value 这类重复参数收成字典。",
    },
    "en-US": {
        "doc_client": """GoTeams Open API shared client.

The site URL and API key are written in below. Only the standard library is used, so the
package runs anywhere. This file contains a plaintext API key -- do not share it.
""",
        "err_request_failed": "Request %s failed: %s",
        "err_upload_file_missing": "File to upload not found: %s",
        "err_key_value_form": "Expected key=value form, got: %s",
        "help_ws_id": "Project ID (wsId)",
        "help_item_id": "Work item ID (itemId)",
        "help_file": "Local file path",
        "help_json": "Full request body as a JSON string, or @filename; merged over the options above, and it wins on conflicts",
        "help_page": "Page number, starting at 1",
        "help_size": "Items per page, at most 100",
        "help_title": "Title, at most 500 characters",
        "help_description": "Body content",
        "help_description_mode": "Editor the web app opens the body with, defaults to rich-text; the body content is always HTML",
        "help_description_mode_update": "Editor the web app opens the body with",
        "help_status_id": "Status ID",
        "help_priority_id": "Priority ID",
        "help_parent_id": "Parent requirement ID",
        "help_version_id": "Version ID",
        "help_assignee_id": "Assignee ID; repeat the option for more",
        "help_assignee_id_replace": "Assignee ID; repeat the option for more. Passing any replaces the whole list",
        "help_clear_assignees": "Clear the assignees",
        "help_planned_start_date": "Planned start, Unix seconds",
        "help_planned_end_date": "Planned end, Unix seconds",
        "help_estimated_work_hours": "Estimated hours",
        "help_custom_field": "Custom field as key=value; repeat the option for more",
        "help_image_file_id_create": "IDs of assets already uploaded for the body (the file_id returned by upload_*.py); repeat the option for more. Upload first, then create, or the files are lost",
        "help_image_file_id_update": "IDs of assets added in this call (the file_id returned by upload_*.py); repeat the option for more. They only take effect when sent together with other fields",
        "help_type_filter": "Only requirements or only defects; unlimited by default",
        "help_type_create": "Work item type, defaults to requirement",
        "help_src": "Asset URL: the image/file/video URL from the detail's description, copied verbatim",
        "help_out": "Where to save: a directory or a full file path; defaults to the current directory",
        "help_content": "Comment text; must not be blank",
        "desc_my_work": "List the work items assigned to me",
        "desc_list_workspaces": "List the projects visible to me",
        "desc_list_work_items": "List the work items in a project",
        "desc_get_work_item": "Get work item detail",
        "desc_create_work_item": "Create a work item",
        "desc_update_work_item": "Update a work item",
        "desc_create_comment": "Add a comment to a work item",
        "desc_download_file": "Download images/files/videos from a work item",
        "desc_upload_image": "Upload an image (body asset)",
        "desc_upload_file": "Upload a file (body asset)",
        "desc_upload_video": "Upload a video (body asset)",
        "err_missing_title": "Missing --title: the title is required (or pass the full body with --json)",
        "err_nothing_to_update": "Nothing to update: pass at least one option (or a body with --json)",
        "err_image_only": "--image-file-id must be sent together with other fields: sending only asset IDs leaves nothing to update and the endpoint returns 400 \"参数无效\" (invalid parameter); the script blocks it here so no API call is spent",
        "err_content_blank": "--content must not be blank",
        "doc_my_work": """GET /api/v1/my-work -- list the work items assigned to me (across projects, paginated).

Examples:
  python3 scripts/my_work.py
  python3 scripts/my_work.py --page 2 --size 50
""",
        "doc_list_workspaces": """GET /api/v1/workspaces -- the projects visible to me.

Each record's id is the wsId every other endpoint needs.
""",
        "doc_list_work_items": """GET /api/v1/workspaces/{wsId}/work-items -- the work items in a project (paginated).

Obtain wsId with list_workspaces.py first. Leaving type empty returns both requirements and defects.

Examples:
  python3 scripts/list_work_items.py --ws-id 12
  python3 scripts/list_work_items.py --ws-id 12 --type defect --size 100
""",
        "doc_get_work_item": """GET /api/v1/workspaces/{wsId}/work-items/{itemId} -- work item detail.

Returns description (the body), description_mode, comment_count, creator_name and custom_fields
on top of the list fields. To set status, priority, version, assignees or custom fields, read the
current values here first.

Example:
  python3 scripts/get_work_item.py --ws-id 12 --item-id 1001
""",
        "doc_create_work_item": """POST /api/v1/workspaces/{wsId}/work-items -- create a work item (requirement or defect).

title is required; everything else defaults when omitted. Every *_id is a numeric ID, so fetch it from a
list or detail endpoint first. Timestamps are Unix seconds (10 digits). When the body references
images/files/videos, upload them first and pass their IDs with --image-file-id, otherwise the files
are not attached to the work item. Returns {"id": <the new work item ID>} on success.
The body (--description) is always an HTML fragment: convert Markdown source to HTML before sending.
--description-mode only picks the editor the web app opens it with; it does not change the content format.

Examples:
  python3 scripts/create_work_item.py --ws-id 12 --title "Login page copy tweak"
  python3 scripts/create_work_item.py --ws-id 12 --title "Export is slow" --type defect \\
      --description "<h1>Export is slow</h1><p>Exporting 10k rows takes 3 minutes</p>" \\
      --description-mode markdown --estimated-work-hours 4
  python3 scripts/upload_image.py --ws-id 12 --file ./shot.png   # upload first to get file_id
  python3 scripts/create_work_item.py --ws-id 12 --title "Screenshot" \\
      --description '<p><img src="<asset url>"></p>' --image-file-id <file_id returned by the upload>
  python3 scripts/create_work_item.py --ws-id 12 --json @payload.json
""",
        "doc_update_work_item": """PATCH /api/v1/workspaces/{wsId}/work-items/{itemId} -- update a work item.

Only the fields passed on the command line are changed; the rest stay as they are. type cannot be
changed. A value of 0 (or omitting the field) for status_id / priority_id / parent_id / version_id means "do not change".
Use --assignee-id to replace the whole assignee list, and --clear-assignees to empty it.
Assets added in this call are passed as IDs with --image-file-id, but they **must accompany other
fields** (for example --description); sending only --image-file-id makes the endpoint return 400
"参数无效" (invalid parameter), so the script blocks such a call here and spends no API call.
On success data is null.

Examples:
  python3 scripts/update_work_item.py --ws-id 12 --item-id 1001 --title "New title"
  python3 scripts/update_work_item.py --ws-id 12 --item-id 1001 \\
      --status-id 101 --estimated-work-hours 8
  python3 scripts/update_work_item.py --ws-id 12 --item-id 1001 --clear-assignees
""",
        "doc_create_comment": """POST /api/v1/workspaces/{wsId}/work-items/{itemId}/comments -- add a work item comment.

content must not be blank after trimming. Returns {"id": <the comment ID>} on success.

Example:
  python3 scripts/create_comment.py --ws-id 12 --item-id 1001 --content "Reproduced, investigating"
""",
        "doc_download_file": """Download images/files/videos from a work item.

The asset URL is the <img src> / video / file link in the work item detail's description (the url in
an attachment list is the same kind of value). Pass it to --src verbatim; the script resolves the file
by project + work item + URL, so there is no domain to assemble and no auth header to add.

Examples:
  python3 scripts/download_file.py --ws-id 12 --item-id 1001 --src <asset url>
  python3 scripts/download_file.py --ws-id 12 --item-id 1001 --src "https://<site>/..." --out ./assets
""",
        "doc_upload_image": """Upload an image to the work item asset library (the same path the rich-text editor uses).

Afterwards: put the returned url in the body, and submit the returned file_id with --image-file-id
along with the create or update call, or the file is not attached to that work item and cannot be
downloaded later.

Every upload counts as one API call.

Example:
  python3 scripts/upload_image.py --ws-id 12 --file ./screenshot.png
""",
        "doc_upload_file": """Upload a file to the work item asset library (the same path the rich-text editor uses).

Supports pdf/doc/docx/xls/xlsx/ppt/pptx/txt/csv/md/zip/rar/7z/html/htm, up to 200MB each.
Afterwards: put url in the body and submit file_id with --image-file-id along with the create or update call, or the file is not attached.

Every upload counts as one API call.

Example:
  python3 scripts/upload_file.py --ws-id 12 --file ./design-spec.pdf
""",
        "doc_upload_video": """Upload a video to the work item asset library (the same path the rich-text editor uses).

Only mp4 and webm are supported, up to 200MB each.
Afterwards: put url in the body and submit file_id with --image-file-id along with the create or update call, or the file is not attached.

Every upload counts as one API call.

Example:
  python3 scripts/upload_video.py --ws-id 12 --file ./repro.mp4
""",
        "doc_fail": "Write the failure reason to standard error and exit (status 1).",
        "doc_msg": "Return the text for the current language; when a key is missing it falls back to the other language so a script never breaks over one missing string.",
        "doc_api": """Call an open endpoint and return the data from the response.

Items in params whose value is None or an empty string count as "not provided" and are dropped.
A non-empty body is sent as JSON. On failure it exits immediately instead of returning half a result.
""",
        "doc_show": "Print the data returned by an endpoint to standard output as JSON.",
        "doc_safe_filename": "Keep only a bare filename: path separators, an empty name, . and .. all fall back to a default so nothing is written outside the target directory.",
        "doc_attachment_filename": "Pick the download filename: prefer the Content-Disposition header, then the last segment of the URL.",
        "doc_download": """Download binary content (image/file/video) and print and return the saved result.

destination may be a full file path, an existing directory, or omitted (save to the current
directory under the name from the response header or the URL).
""",
        "doc_upload": """Upload one file as multipart/form-data and return the endpoint's data.

This is the path for body assets: the returned url goes into the body, and file_id must be passed
along as image_file_ids when creating or updating a work item for the file to be attached.
""",
        "doc_json_argument": "Parse --json: either a JSON string, or @filename to read from a file.",
        "doc_key_value_list": "Collect repeated --custom-field key=value arguments into a dict.",
    },
}


def msg(key, *args):
    table = MESSAGES.get(LANG) or MESSAGES.get("en-US") or {}
    text = table.get(key) or MESSAGES.get("zh-CN", {}).get(key) or key
    return text % args if args else text


__doc__ = msg("doc_client")


def fail(message):
    print(message, file=sys.stderr)
    sys.exit(1)


def api(method, path, params=None, body=None):
    url = SITE_URL.rstrip("/") + path
    if params:
        query = {key: value for key, value in params.items() if value not in (None, "")}
        if query:
            url += "?" + urllib.parse.urlencode(query)
    headers = {"Authorization": "Bearer " + API_KEY, "Accept": "application/json"}
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    request = urllib.request.Request(url, data=data, headers=headers, method=method.upper())
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", "replace")
        try:
            detail = json.loads(detail).get("message") or detail
        except ValueError:
            pass
        return fail("HTTP %s: %s" % (error.code, detail))
    except urllib.error.URLError as error:
        return fail(msg("err_request_failed", url, error.reason))
    if payload.get("code") != 200:
        return fail("HTTP %s: %s" % (payload.get("code"), payload.get("message")))
    return payload.get("data")


def show(data):
    print(json.dumps(data, ensure_ascii=False, indent=2))


def safe_filename(name):
    name = (name or "").replace("\\", "/")
    name = name.split("/")[-1].strip()
    return name if name not in ("", ".", "..") else "download.bin"


def attachment_filename(headers, url):
    disposition = headers.get("Content-Disposition", "") or ""
    match = re.search(r"filename\*=UTF-8''([^;]+)", disposition, re.IGNORECASE)
    if match:
        return safe_filename(urllib.parse.unquote(match.group(1)))
    match = re.search(r'filename="?([^";]+)"?', disposition, re.IGNORECASE)
    if match:
        return safe_filename(match.group(1))
    return safe_filename(os.path.basename(urllib.parse.urlparse(url).path))


def download(path, params=None, destination=None):
    url = SITE_URL.rstrip("/") + path
    if params:
        query = {key: value for key, value in params.items() if value not in (None, "")}
        if query:
            url += "?" + urllib.parse.urlencode(query)
    request = urllib.request.Request(url, headers={"Authorization": "Bearer " + API_KEY})
    try:
        with urllib.request.urlopen(request, timeout=120) as response:
            headers = dict(response.headers)
            data = response.read()
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", "replace")
        try:
            detail = json.loads(detail).get("message") or detail
        except ValueError:
            pass
        return fail("HTTP %s: %s" % (error.code, detail))
    except urllib.error.URLError as error:
        return fail(msg("err_request_failed", url, error.reason))
    target = destination
    if not target or os.path.isdir(target):
        target = os.path.join(target or ".", attachment_filename(headers, url))
    os.makedirs(os.path.dirname(os.path.abspath(target)), exist_ok=True)
    with open(target, "wb") as handle:
        handle.write(data)
    result = {"path": target, "bytes": len(data), "content_type": headers.get("Content-Type", "")}
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return result


def upload(path, file_path, field="file"):
    if not file_path or not os.path.isfile(file_path):
        return fail(msg("err_upload_file_missing", file_path))
    with open(file_path, "rb") as handle:
        content = handle.read()
    boundary = "----goteams" + uuid.uuid4().hex
    name = os.path.basename(file_path).replace('"', "").replace("\r", "").replace("\n", "") or "upload.bin"
    head = (
        "--%(boundary)s\r\n"
        'Content-Disposition: form-data; name="%(field)s"; filename="%(name)s"\r\n'
        "Content-Type: application/octet-stream\r\n\r\n"
    ) % {"boundary": boundary, "field": field, "name": name}
    body = head.encode("utf-8") + content + ("\r\n--%s--\r\n" % boundary).encode("utf-8")
    url = SITE_URL.rstrip("/") + path
    request = urllib.request.Request(
        url,
        data=body,
        headers={
            "Authorization": "Bearer " + API_KEY,
            "Accept": "application/json",
            "Content-Type": "multipart/form-data; boundary=" + boundary,
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            payload = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", "replace")
        try:
            detail = json.loads(detail).get("message") or detail
        except ValueError:
            pass
        return fail("HTTP %s: %s" % (error.code, detail))
    except urllib.error.URLError as error:
        return fail(msg("err_request_failed", url, error.reason))
    if payload.get("code") != 200:
        return fail("HTTP %s: %s" % (payload.get("code"), payload.get("message")))
    data = payload.get("data")
    print(json.dumps(data, ensure_ascii=False, indent=2))
    return data


def json_argument(value):
    if value.startswith("@"):
        with open(value[1:], "r", encoding="utf-8") as handle:
            return json.load(handle)
    return json.loads(value)


def key_value_list(values):
    result = {}
    for item in values or []:
        if "=" not in item:
            fail(msg("err_key_value_form", item))
        key, _, value = item.partition("=")
        result[key.strip()] = value
    return result


# 函数说明也走消息表，保证交付包里不出现另一种语言。
# Function docs come from the message table too, so the delivered package carries only one language.
fail.__doc__ = msg("doc_fail")
msg.__doc__ = msg("doc_msg")
api.__doc__ = msg("doc_api")
show.__doc__ = msg("doc_show")
safe_filename.__doc__ = msg("doc_safe_filename")
attachment_filename.__doc__ = msg("doc_attachment_filename")
download.__doc__ = msg("doc_download")
upload.__doc__ = msg("doc_upload")
json_argument.__doc__ = msg("doc_json_argument")
key_value_list.__doc__ = msg("doc_key_value_list")
