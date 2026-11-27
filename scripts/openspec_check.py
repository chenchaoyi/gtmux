#!/usr/bin/env python3
"""Validate pinned OpenSpec 2.x output without mislabelling launcher failures."""
import json
from pathlib import Path
import subprocess
import sys

COMMAND = ["npx", "--yes", "@fission-ai/openspec@2.2.0", "validate", "--specs", "--strict", "--json"]
LEGACY_PROSE_LEN_WARNING = (
    "Requirement text is very long (>500 characters). Move examples and edge cases into "
    "scenarios, or split it into separate requirements that each state one behavior."
)


def blocking_issues(item):
    issues = item.get("issues")
    if not isinstance(issues, list):
        return None
    blocking = [
        iss for iss in issues
        if isinstance(iss, dict)
        and iss.get("level") != "INFO"
        and not (iss.get("level") == "WARNING" and iss.get("message") == LEGACY_PROSE_LEN_WARNING)
    ]
    if not item.get("valid") and not issues:
        return [{"level": "ERROR", "path": "file", "message": "Spec marked invalid without issue details"}]
    return blocking


def verdict(returncode, stdout, root):
    try:
        report = json.loads(stdout)
        items = report["items"]
        totals = report["summary"]["totals"]
        selected = Path(report["root"]["path"]).resolve()
        if selected != Path(root).resolve() or not isinstance(items, list) or not items:
            return "tool", report
        if any(item.get("type") != "spec" or type(item.get("valid")) is not bool or blocking_issues(item) is None for item in items):
            return "tool", report
        raw_failed = sum(not item["valid"] for item in items)
        if totals != {"items": len(items), "passed": len(items) - raw_failed, "failed": raw_failed}:
            return "tool", report
        failed = sum(bool(blocking_issues(item)) for item in items)
        if failed:
            return "invalid", report
        expected_rc = 1 if raw_failed else 0
        return ("ok" if returncode == expected_rc else "tool"), report
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
            issues = blocking_issues(item)
            if issues:
                filtered = dict(item, issues=issues)
                print(json.dumps(filtered, ensure_ascii=False), file=sys.stderr)
        return 1
    print("OpenSpec validator could not run or complete (tool/setup failure); no valid verdict for this checkout.", file=sys.stderr)
    for output in (result.stdout, result.stderr):
        if output.strip():
            print(output.strip(), file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
