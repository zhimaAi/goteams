#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import msg, upload  # noqa: E402

__doc__ = msg("doc_upload_video")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_upload_video"))
    parser.add_argument("--ws-id", required=True, type=int, help=msg("help_ws_id"))
    parser.add_argument("--file", required=True, help=msg("help_file"))
    args = parser.parse_args()
    upload("/api/v1/workspaces/%s/assets/video" % args.ws_id, args.file)


if __name__ == "__main__":
    main()
