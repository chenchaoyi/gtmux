#!/usr/bin/env python3
"""Conservative change selection and fail-closed CI result aggregation."""
import argparse
import json
import os
from pathlib import PurePosixPath
import re
import subprocess
import sys


JOBS = {
    "test": "core", "cross-build": "cross", "mobile": "mobile", "web": "web",
    "menubar": "menubar", "relay-worker": "relay", "tunnel-worker": "tunnel",
    "ios-native": "ios",
}
DOC_FILES = {"README.md", "README.zh.md", "CONTRIBUTING.md", "SECURITY.md", "LICENSE",
             "CLAUDE.md", ".github/pull_request_template.md"}
DOC_IMAGES = {".png", ".jpg", ".jpeg", ".webp", ".gif"}


def document(path):
    p = PurePosixPath(path)
    return (path in DOC_FILES or
            (path.startswith(("docs/", "openspec/")) and
             (p.suffix == ".md" or (path.startswith("docs/") and p.suffix in DOC_IMAGES))) or
            (path.startswith(("mobileapp/", "macapp/", "relay-worker/", "tunnel-worker/")) and p.suffix == ".md") or
            (path.startswith(("mobileapp/fastlane/metadata/", "mobileapp/release-notes/")) and p.suffix == ".txt"))


def classify(paths, force_reason=None):
    selected, reasons = set(), []
    full = bool(force_reason) or not paths
    if full:
        reasons.append(force_reason or "No reliable changed-file list; run everything.")
    for path in paths:
        # Paths are data, never shell fragments. Malformed or unrecognized data broadens.
        if not isinstance(path, str) or not path or ".." in PurePosixPath(path).parts or path.startswith("/"):
            full = True
            reasons.append("Unrecognized path; run everything.")
        elif document(path):
            reasons.append("Documentation: " + json.dumps(path))
        elif path.startswith(("internal/server/web/", "tests/web/")):
            selected.update(("test", "web"))
            reasons.append("Web assets/tests and embedded-server regression tests: " + json.dumps(path))
        elif path.startswith(("cmd/", "internal/", "relay/")) and path.endswith("_test.go"):
            selected.add("test")
            reasons.append("Go tests only: " + json.dumps(path))
        elif path.startswith("mobileapp/src/api/"):
            full = True
            reasons.append("Shared HTTP contract consumer: " + json.dumps(path))
        elif path.startswith(("mobileapp/src/", "mobileapp/e2e/")) and PurePosixPath(path).suffix in {".ts", ".tsx", ".js", ".cjs"}:
            selected.update(("mobile", "web", "ios-native"))
            reasons.append("Phone/iPad code, native compile and shared Web presentation: " + json.dumps(path))
        elif path.startswith("mobileapp/ios/") and PurePosixPath(path).suffix in {".swift", ".m", ".mm", ".h"}:
            selected.update(("mobile", "ios-native"))
            reasons.append("iOS native code and phone consumers: " + json.dumps(path))
        elif path.startswith(("macapp/Sources/", "macapp/Tests/")) and PurePosixPath(path).suffix == ".swift":
            selected.add("menubar")
            reasons.append("Menu-bar implementation/tests: " + json.dumps(path))
        elif path.startswith(("relay-worker/src/", "relay-worker/test/")) and PurePosixPath(path).suffix == ".ts":
            selected.update(("relay-worker", "test", "mobile"))
            reasons.append("Push Worker, Go reference and phone consumer: " + json.dumps(path))
        else:
            # Core/API, tunnel control plane, dependencies, CI/build/install machinery,
            # shared assets, new directories and unknown file types all reach this path.
            full = True
            reasons.append("Shared or uncertain impact; run everything: " + json.dumps(path))
    if full:
        selected = set(JOBS)
    # A Go-only test edit still runs the complete Go suite. Web/relay changes also
    # use that suite: dependency/package guesses must never hide a backend failure.
    return {"selected": sorted(selected), "full": full, "files": len(paths), "reasons": reasons[:30]}


def parse_diff(data):
    """Consume NUL-separated name-status records, including BOTH rename/copy paths."""
    parts = data.decode("utf-8", errors="strict").split("\0")
    if not parts or parts[-1] != "":
        raise ValueError("Incomplete git diff")
    parts.pop()
    paths, i = [], 0
    while i < len(parts):
        status = parts[i]; i += 1
        count = 2 if re.fullmatch(r"[RC][0-9]+", status) else 1
        if count == 1 and status not in {"A", "M", "D", "T"}:
            raise ValueError("Unknown git status")
        if i + count > len(parts) or any(not p for p in parts[i:i + count]):
            raise ValueError("Incomplete git paths")
        paths.extend(parts[i:i + count]); i += count
    return paths


def changed_files(base, head, event):
    if not all(re.fullmatch(r"[0-9a-f]{40}", ref or "") and set(ref) != {"0"} for ref in (base, head)):
        raise ValueError("Missing or invalid diff revisions")
    separator = "..." if event == "pull_request" else ".."
    revision = base + separator + head
    data = subprocess.run(["git", "diff", "--name-status", "-z", "--find-renames", revision], check=True, capture_output=True).stdout
    if len(data) > 1024 * 1024:
        raise ValueError("Diff too large to classify confidently")
    raw = subprocess.run(["git", "diff", "--raw", revision], check=True, capture_output=True).stdout
    if b":120000 " in raw or b" 120000 " in raw:
        raise ValueError("Symbolic-link change requires full checks")
    return parse_diff(data)


def verdict(plan, needs):
    if not isinstance(plan, dict) or not isinstance(plan.get("selected"), list) or not isinstance(needs, dict):
        return ["Missing or malformed CI plan"]
    if not all(isinstance(name, str) for name in plan["selected"]) or not isinstance(plan.get("full"), bool):
        return ["Invalid CI plan"]
    chosen = set(plan["selected"])
    if len(chosen) != len(plan["selected"]) or not chosen <= JOBS.keys() or (plan["full"] and chosen != JOBS.keys()):
        return ["Invalid CI plan"]
    errors = []
    for name in ("scope", "quality", *JOBS):
        job = needs.get(name)
        result = job.get("result") if isinstance(job, dict) else None
        required = name in chosen or name in {"scope", "quality"}
        if (required and result != "success") or (not required and result not in {"success", "skipped"}):
            errors.append(f"{name}: {result or 'missing'} ({'required' if required else 'not selected'})")
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    p = sub.add_parser("plan")
    p.add_argument("--base", default=os.environ.get("CI_BASE", ""))
    p.add_argument("--head", default=os.environ.get("CI_HEAD", ""))
    p.add_argument("--event", default=os.environ.get("CI_EVENT", ""))
    p.add_argument("--force-full", action="store_true")
    sub.add_parser("verify")
    args = parser.parse_args()
    if args.command == "verify":
        try:
            errors = verdict(json.loads(os.environ.get("CI_PLAN", "")), json.loads(os.environ.get("CI_RESULTS", "")))
        except (ValueError, TypeError, AttributeError):
            errors = ["Could not read CI plan/results"]
        for error in errors:
            print("CI failed: " + error, file=sys.stderr)
        return int(bool(errors))
    try:
        if args.force_full or args.event not in {"push", "pull_request"}:
            plan = classify([], "Manual or unsupported event; run everything.")
        else:
            plan = classify(changed_files(args.base, args.head, args.event))
    except (ValueError, UnicodeError, OSError, subprocess.CalledProcessError) as error:
        plan = classify([], "Cannot confirm diff; run everything (" + type(error).__name__ + ").")
    print(json.dumps(plan, indent=2))
    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        with open(output, "a", encoding="utf-8") as f:
            f.write("plan=" + json.dumps(plan, separators=(",", ":")) + "\n")
            for job, flag in JOBS.items():
                f.write(flag + "=" + str(job in plan["selected"]).lower() + "\n")
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as f:
            f.write("## Checks selected\n\nPolicy/docs and workflow validation always run.\n\n")
            f.write("| Check | Selected |\n| --- | --- |\n")
            for job in JOBS:
                f.write(f"| {job} | {'Run' if job in plan['selected'] else 'Not affected'} |\n")
            f.write("\n### Reasons\n\n")
            for reason in plan["reasons"]:
                f.write("- " + reason.replace("|", "\\|") + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
