#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import api, msg, show  # noqa: E402

__doc__ = msg("doc_my_work")


def main():
    parser = argparse.ArgumentParser(description=msg("desc_my_work"))
    parser.add_argument("--page", type=int, default=1, help=msg("help_page"))
    parser.add_argument("--size", type=int, default=20, help=msg("help_size"))
    args = parser.parse_args()
    show(api("GET", "/api/v1/my-work", params={"page": args.page, "size": args.size}))


if __name__ == "__main__":
    main()
