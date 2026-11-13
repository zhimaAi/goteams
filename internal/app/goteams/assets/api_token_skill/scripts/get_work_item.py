#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import api, msg, show  # noqa: E402

__doc__ = msg("doc_get_work_item")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_get_work_item"))
    parser.add_argument("--ws-id", required=True, type=int, help=msg("help_ws_id"))
    parser.add_argument("--item-id", required=True, type=int, help=msg("help_item_id"))
    args = parser.parse_args()
    path = "/api/v1/workspaces/%s/work-items/%s" % (args.ws_id, args.item_id)
    show(api("GET", path))


if __name__ == "__main__":
    main()
