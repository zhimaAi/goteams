#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import api, msg, show  # noqa: E402

__doc__ = msg("doc_create_comment")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_create_comment"))
    parser.add_argument("--ws-id", required=True, type=int, help=msg("help_ws_id"))
    parser.add_argument("--item-id", required=True, type=int, help=msg("help_item_id"))
    parser.add_argument("--content", required=True, help=msg("help_content"))
    args = parser.parse_args()
    if not args.content.strip():
        sys.exit(msg("err_content_blank"))
    path = "/api/v1/workspaces/%s/work-items/%s/comments" % (args.ws_id, args.item_id)
    show(api("POST", path, body={"content": args.content}))


if __name__ == "__main__":
    main()
