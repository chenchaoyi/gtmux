#!/usr/bin/env python3
"""Validate pinned OpenSpec output without mislabelling launcher failures."""
import json
from pathlib import Path
import subprocess
import sys

COMMAND = ["npx", "--yes", "@fission-ai/openspec@1.10.0", "validate", "--specs", "--strict", "--json"]


def verdict(returncode, stdout, root):
    try:
        report = json.loads(stdout)
        items = report["items"]
        totals = report["summary"]["totals"]
        selected = Path(report["root"]["path"]).resolve()
        if selected != Path(root).resolve() or not isinstance(items, list) or not items:
            return "tool", report
        if any(item.get("type") != "spec" or type(item.get("valid")) is not bool for item in items):
            return "tool", report
        failed = sum(not item["valid"] for item in items)
        if totals != {"items": len(items), "passed": len(items)-failed, "failed": failed}:
            return "tool", report
        if failed:
            return "invalid", report
        return ("ok" if returncode == 0 else "tool"), report
    except (ValueError, TypeError, KeyError, AttributeError):
        return "tool", None


def main():
    try:
        result = subprocess.run(COMMAND, capture_output=True, text=True, timeout=120)
    except (OSError, subprocess.TimeoutExpired) as error:
        print(f"OpenSpec validator could not run or complete (tool/setup failure): {type(error).__name__}", file=sys.stderr)
        return 2
    status, report = verdict(result.returncode, result.stdout, Path.cwd())
    if status == "ok":
        print(f"OpenSpec: {len(report['items'])} specs passed")
        return 0
    if status == "invalid":
        print("OpenSpec spec validation failed (invalid or drifted specs):", file=sys.stderr)
        for item in report["items"]:
            if not item["valid"]:
                print(json.dumps(item, ensure_ascii=False), file=sys.stderr)
        return 1
    print("OpenSpec validator could not run or complete (tool/setup failure); no valid verdict for this checkout.", file=sys.stderr)
    for output in (result.stdout, result.stderr):
        if output.strip():
            print(output.strip(), file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
