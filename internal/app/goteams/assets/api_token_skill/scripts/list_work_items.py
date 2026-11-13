#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import api, msg, show  # noqa: E402

__doc__ = msg("doc_list_work_items")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_list_work_items"))
    parser.add_argument("--ws-id", required=True, type=int, help=msg("help_ws_id"))
    parser.add_argument("--type", choices=["requirement", "defect"], default="", help=msg("help_type_filter"))
    parser.add_argument("--page", type=int, default=1, help=msg("help_page"))
    parser.add_argument("--size", type=int, default=20, help=msg("help_size"))
    args = parser.parse_args()
    path = "/api/v1/workspaces/%s/work-items" % args.ws_id
    show(api("GET", path, params={"type": args.type, "page": args.page, "size": args.size}))


if __name__ == "__main__":
    main()
