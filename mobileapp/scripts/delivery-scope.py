#!/usr/bin/env python3
"""Decide whether a release changes iOS build inputs, before signing is requested."""
import argparse
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess


def git(root, *args):
    return subprocess.check_output(["git", "-C", str(root), *args], stderr=subprocess.DEVNULL)


def version(tag):
    if not isinstance(tag, str) or not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        return None
    return tuple(map(int, tag[1:].split(".")))


def previous_release(tag, releases):
    current = version(tag)
    if current is None or not isinstance(releases, list):
        raise ValueError("A stable tag and an official release list are required.")
    candidates = [r["tag_name"] for r in releases
                  if isinstance(r, dict) and version(r.get("tag_name")) is not None
                  and not r.get("draft") and not r.get("prerelease")
                  and version(r["tag_name"]) < current]
    return max(candidates, key=version, default=None)


def shipping_input(path):
    p = PurePosixPath(path)
    parts = p.parts[1:]
    if not parts or p.parts[0] != "mobileapp":
        return False
    if parts[0] in {"android", "e2e", "__tests__", "scripts", "fastlane", "release-notes"}:
        return False
    if "__tests__" in parts or re.search(r"\.(test|spec)\.[cm]?[jt]sx?$", p.name):
        return False
    if len(parts) == 1 and (p.suffix == ".md" or p.name in {
            "jest.config.js", "jest.setup.js", "tsconfig.json", ".eslintrc.js"}):
        return False
    return path != "mobileapp/src/version.ts"


def build_inputs_changed(root, baseline, commit):
    if not re.fullmatch(r"[0-9a-f]{40}", commit) or version(baseline) is None:
        raise ValueError("Use an exact commit and stable baseline tag.")
    base = git(root, "rev-parse", "--verify", f"refs/tags/{baseline}^{{commit}}").decode().strip()
    git(root, "merge-base", "--is-ancestor", base, commit)
    paths = git(root, "diff", "--no-renames", "--name-only", "-z", base, commit, "--", "mobileapp").decode().split("\0")
    changed = []
    for path in filter(shipping_input, paths):
        if path.endswith(".pbxproj"):
            # Keep every native setting except the generated marketing-version stamp.
            try:
                before, after = [git(root, "show", f"{ref}:{path}") for ref in (base, commit)]
            except subprocess.CalledProcessError:
                pass  # An added/deleted project is a real build-input change.
            else:
                normalize = lambda data: re.sub(rb"MARKETING_VERSION = [0-9.]+;", b"MARKETING_VERSION = <stamp>;", data)
                if normalize(before) == normalize(after):
                    continue
        changed.append(path)
    return changed


def decide(root, purpose, tag, commit, releases):
    if purpose == "ui-preview":
        return {"build_required": True, "baseline": None, "reason": "Explicit UI preview"}
    if purpose != "release-test" or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("Invalid delivery purpose or commit.")
    baseline = previous_release(tag, releases)
    exact = git(root, "rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}").decode().strip()
    if exact != commit:
        raise ValueError("Release tag does not match the validated commit.")
    if baseline is None:
        return {"build_required": True, "baseline": None, "reason": "No earlier published stable release in the source list"}
    changed = build_inputs_changed(root, baseline, commit)
    return {"build_required": bool(changed), "baseline": baseline, "changed_inputs": changed,
            "reason": "iOS build inputs changed" if changed else "iOS inputs unchanged; version stamps do not require a package"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    parser.add_argument("--purpose", choices=("release-test", "ui-preview"), required=True)
    parser.add_argument("--tag", default="")
    parser.add_argument("--commit", required=True)
    parser.add_argument("--releases", type=Path)
    args = parser.parse_args()
    releases = json.loads(args.releases.read_text()) if args.releases else []
    result = decide(args.root, args.purpose, args.tag, args.commit, releases)
    print(json.dumps(result, sort_keys=True))
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as output:
            output.write("build_required=" + str(result["build_required"]).lower() + "\n")
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write("### iOS package scope\n\n" + result["reason"] + ".\n")
            if result["baseline"]:
                summary.write("Compared with published stable release " + result["baseline"] + ".\n")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError):
        raise SystemExit("Cannot establish iOS package scope; stop before signing and inspect source/baseline.")
