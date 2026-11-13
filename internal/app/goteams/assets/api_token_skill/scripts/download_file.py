#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import download, msg  # noqa: E402

__doc__ = msg("doc_download_file")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_download_file"))
    parser.add_argument("--ws-id", required=True, type=int, help=msg("help_ws_id"))
    parser.add_argument("--item-id", required=True, type=int, help=msg("help_item_id"))
    parser.add_argument("--src", required=True, help=msg("help_src"))
    parser.add_argument("--out", help=msg("help_out"))
    args = parser.parse_args()
    path = "/api/v1/workspaces/%s/work-items/%s/files/download" % (args.ws_id, args.item_id)
    download(path, {"src": args.src}, args.out)


if __name__ == "__main__":
    main()
