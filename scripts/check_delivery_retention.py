#!/usr/bin/env python3
"""Validate one explicit delivery archive's policy and logical-byte budget.

Read only: never migrate, delete, open payloads, follow links, or infer that an
expired review/hold date authorizes removal. Errors name fields, never values.
"""
import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import stat


def validate_policy(value):
    keys = {"schema", "purpose", "source", "budget_bytes", "retain_until", "review_by", "review_owner", "auto_delete"}
    if not isinstance(value, dict) or set(value) != keys:
        raise ValueError("policy fields are incomplete or unsupported")
    if value["schema"] != "gtmux.delivery-retention.v1" or value["auto_delete"] is not False:
        raise ValueError("schema or auto_delete is invalid")
    purpose = value["purpose"]
    if not isinstance(purpose, str) or not 1 <= len(purpose.strip()) <= 1024:
        raise ValueError("purpose is required")
    source = value["source"]
    if not isinstance(source, dict) or set(source) != {"revision", "channel"}:
        raise ValueError("source fields are invalid")
    if not isinstance(source["revision"], str) or not re.fullmatch(r"[a-f0-9]{7,64}", source["revision"]):
        raise ValueError("source revision is invalid")
    if source["channel"] not in ("release", "store", "engineering"):
        raise ValueError("source channel is invalid")
    if type(value["budget_bytes"]) is not int or value["budget_bytes"] <= 0:
        raise ValueError("budget_bytes must be a positive integer")
    owner = value["review_owner"]
    if not isinstance(owner, str) or not re.fullmatch(r"[a-z][a-z0-9_-]{1,63}", owner):
        raise ValueError("review_owner must identify a responsible role")
    try:
        for field in ("retain_until", "review_by"):
            if not isinstance(value[field], str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value[field]):
                raise ValueError()
        hold = dt.date.fromisoformat(value["retain_until"])
        review = dt.date.fromisoformat(value["review_by"])
    except (ValueError, TypeError):
        raise ValueError("retention/review dates are invalid") from None
    if review > hold:
        raise ValueError("review_by must be no later than retain_until")


def measure(root):
    if Path(root).is_symlink():
        raise ValueError("archive root must not be a symlink")
    total = 0
    for directory, dirs, files in os.walk(root, followlinks=False):
        dirs[:] = [name for name in dirs if not (Path(directory) / name).is_symlink()]
        for name in files:
            info = (Path(directory) / name).lstat()
            if stat.S_ISREG(info.st_mode):
                total += info.st_size
    return total


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    args = parser.parse_args(argv)
    try:
        if args.manifest.is_symlink() or args.manifest.stat().st_size > 16 * 1024:
            raise ValueError("manifest must be a small regular file")
        value = json.loads(args.manifest.read_text())
        validate_policy(value)
        used = measure(args.manifest.parent)
        if used > value["budget_bytes"]:
            raise ValueError("archive exceeds its registered logical-byte budget")
    except (ValueError, OSError):
        print("delivery retention: invalid policy or exceeded budget; no files changed")
        return 1
    print(f"delivery retention: OK ({used} logical bytes); dates require review, never automatic deletion")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
