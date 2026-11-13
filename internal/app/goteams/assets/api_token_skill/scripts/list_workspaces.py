#!/usr/bin/env python3
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from goteams_client import api, msg, show  # noqa: E402

__doc__ = msg("doc_list_workspaces")


def main():
    argparse.ArgumentParser(description=msg("desc_list_workspaces")).parse_args()
    show(api("GET", "/api/v1/workspaces"))


if __name__ == "__main__":
    main()
