#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import api, json_argument, key_value_list, msg, show  # noqa: E402

__doc__ = msg("doc_create_work_item")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_create_work_item"))
    parser.add_argument("--ws-id", required=True, type=int, help=msg("help_ws_id"))
    parser.add_argument("--title", help=msg("help_title"))
    parser.add_argument("--type", choices=["requirement", "defect"], help=msg("help_type_create"))
    parser.add_argument("--description", help=msg("help_description"))
    parser.add_argument("--description-mode", choices=["rich-text", "markdown"], help=msg("help_description_mode"))
    parser.add_argument("--status-id", type=int, help=msg("help_status_id"))
    parser.add_argument("--priority-id", type=int, help=msg("help_priority_id"))
    parser.add_argument("--parent-id", type=int, help=msg("help_parent_id"))
    parser.add_argument("--version-id", type=int, help=msg("help_version_id"))
    parser.add_argument("--assignee-id", type=int, action="append", help=msg("help_assignee_id"))
    parser.add_argument("--planned-start-date", type=int, help=msg("help_planned_start_date"))
    parser.add_argument("--planned-end-date", type=int, help=msg("help_planned_end_date"))
    parser.add_argument("--estimated-work-hours", type=float, help=msg("help_estimated_work_hours"))
    parser.add_argument("--custom-field", action="append", help=msg("help_custom_field"))
    parser.add_argument("--image-file-id", type=int, action="append", help=msg("help_image_file_id_create"))
    parser.add_argument("--json", help=msg("help_json"))
    args = parser.parse_args()

    payload = {}
    if args.title:
        payload["title"] = args.title
    if args.type:
        payload["type"] = args.type
    if args.description is not None:
        payload["description"] = args.description
    if args.description_mode:
        payload["description_mode"] = args.description_mode
    if args.status_id:
        payload["status_id"] = args.status_id
    if args.priority_id:
        payload["priority_id"] = args.priority_id
    if args.parent_id:
        payload["parent_id"] = args.parent_id
    if args.version_id:
        payload["version_id"] = args.version_id
    if args.assignee_id:
        payload["assignee_ids"] = args.assignee_id
    if args.planned_start_date is not None:
        payload["planned_start_date"] = args.planned_start_date
    if args.planned_end_date is not None:
        payload["planned_end_date"] = args.planned_end_date
    if args.estimated_work_hours is not None:
        payload["estimated_work_hours"] = args.estimated_work_hours
    if args.custom_field:
        payload["custom_fields"] = key_value_list(args.custom_field)
    if args.image_file_id:
        payload["image_file_ids"] = args.image_file_id
    if args.json:
        payload.update(json_argument(args.json))

    if not payload.get("title"):
        sys.exit(msg("err_missing_title"))
    path = "/api/v1/workspaces/%s/work-items" % args.ws_id
    show(api("POST", path, body=payload))


if __name__ == "__main__":
    main()
