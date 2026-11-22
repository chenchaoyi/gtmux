#!/usr/bin/env python3
"""Plan and check a standalone A/B run of the existing Android emulator suite."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import zipfile

from android_ci_package import identity


ROOT = Path(__file__).resolve().parents[1]
CASE_IDS = list(range(1, 32))
DEVICES = {"phone": "pixel_7", "tablet": "pixel_tablet"}
PARTS = [CASE_IDS[:17], CASE_IDS[17:]]


def plan(source, source_tree):
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("Exact comparison source commit required")
    subprocess.run(
        ["git", "diff", "--quiet", source, "--", ".", ":(exclude)scripts/android_ci_compare.py"],
        cwd=source_tree, check=True,
    )
    if subprocess.check_output(
        ["git", "ls-files", "--others", "--exclude-standard", "--", "mobileapp"],
        cwd=source_tree,
    ):
        raise ValueError("Untracked mobile source in the comparison checkout")
    journey = source_tree / "mobileapp/e2e/android/journey.cjs"
    cases = json.loads(subprocess.check_output([
        "node", "-e", "console.log(JSON.stringify(require(process.argv[1]).cases))", str(journey),
    ]))
    if [case["id"] for case in cases] != CASE_IDS:
        raise ValueError("The Android case inventory changed. Review the partition first.")
    if sorted(case_id for part in PARTS for case_id in part) != CASE_IDS:
        raise ValueError("The partition must cover each registered case once")
    for part in PARTS:
        groups = {case["group"] for case in cases if case["id"] in part}
        if any(case["id"] not in part for case in cases if case["group"] in groups):
            raise ValueError("A setup group crosses the partition")
        if any(dep not in part for case in cases if case["id"] in part for dep in case["requires"]):
            raise ValueError("A case prerequisite crosses the partition")
    lanes = []
    for device, profile in DEVICES.items():
        lanes.append({"lane": f"full-{device}", "device": device, "profile": profile,
                      "selection": CASE_IDS, "only": None})
        for number, part in enumerate(PARTS, 1):
            lanes.append({"lane": f"split-{device}-{number}", "device": device, "profile": profile,
                          "selection": part, "only": ",".join(f"{case:02d}" for case in part)})
    return lanes, {case["id"]: case for case in cases}


def unpack(args):
    limits = {"app-qa.apk": 256 * 1024**2, "CI-PACKAGE.json": 4096}
    if (not re.fullmatch(r"[0-9a-f]{64}", args.zip_sha256)
            or args.archive.is_symlink() or not args.archive.is_file()
            or args.archive.stat().st_size != args.zip_bytes):
        raise ValueError("Official QA ZIP identity or length differs")
    with args.archive.open("rb") as archive:
        if hashlib.file_digest(archive, "sha256").hexdigest() != args.zip_sha256:
            raise ValueError("Official QA ZIP checksum differs")
        archive.seek(0)
        with zipfile.ZipFile(archive) as package:
            entries = package.infolist()
            if sorted(entry.filename for entry in entries) != sorted(limits):
                raise ValueError("QA ZIP must contain exactly the APK and its receipt")
            for entry in entries:
                if (entry.is_dir() or stat.S_IFMT(entry.external_attr >> 16) not in (0, stat.S_IFREG)
                        or not 0 < entry.file_size <= limits[entry.filename]):
                    raise ValueError("Unsafe QA ZIP member")
            args.package_dir.mkdir()
            for entry in entries:
                with package.open(entry) as source, (args.package_dir / entry.filename).open("xb") as target:
                    shutil.copyfileobj(source, target)
    result = identity(args.package_dir, args.source, args.package_run, args.package_attempt)
    if result["attempt"] != args.package_attempt:
        raise ValueError("QA build attempt differs from the pinned attempt")
    print(json.dumps(result, indent=2))
    return 0


def read_receipt(path):
    if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= 1024**2:
        raise ValueError(f"Missing or unsafe {path.name}")
    value = json.loads(path.read_text())
    if not isinstance(value, dict):
        raise ValueError(f"{path.name} must be an object")
    return value


def check_lane(directory, lane, suite, apk, metadata):
    if directory.is_symlink() or not directory.is_dir():
        raise ValueError("Missing or unsafe lane directory")
    if read_receipt(directory / "LANE.json") != {
        **metadata, "lane": lane["lane"], "device": lane["device"], "profile": lane["profile"],
    }:
        raise ValueError("Lane, profile, source, artifact or comparison identity differs")
    package = read_receipt(directory / "APK.json")
    if (package.get("file") != "app-qa.apk" or type(package.get("bytes")) is not int
            or package["bytes"] != apk["bytes"] or package.get("sha256") != apk["sha256"]):
        raise ValueError("APK bytes or SHA-256 differ from the verified QA package")
    receipt = read_receipt(directory / "RESULTS.json")
    selected = lane["selection"]
    if (receipt.get("schema") != 2 or receipt.get("package") != "com.gtmux.app.qa"
            or not re.fullmatch(r"emulator-[0-9]+", str(receipt.get("serial")))
            or receipt.get("selection") != selected
            or any(type(case_id) is not int for case_id in receipt["selection"])
            or "first_automation_failure" not in receipt):
        raise ValueError("Result schema, app, emulator or selected cases differ")
    if receipt["first_automation_failure"] is not None:
        raise ValueError("Automation stopped before the lane completed")
    rows = receipt.get("results")
    if not isinstance(rows, list) or len(rows) != len(suite):
        raise ValueError("Expected one result row for each of the 31 registered cases")
    seen, failures = set(), []
    for row in rows:
        match = re.fullmatch(r"([0-9]{2})-[a-z0-9-]+", str(row.get("name"))) if isinstance(row, dict) else None
        if not match or (case_id := int(match[1])) not in suite or case_id in seen:
            raise ValueError("Invalid, unregistered or duplicate case result")
        seen.add(case_id)
        case = suite[case_id]
        if row.get("group") != case["group"] or row.get("capability") != case["capability"]:
            raise ValueError(f"Case {case_id:02d} belongs to another group or capability")
        wanted = (row.get("status") == "passed" if case_id in selected else
                  row.get("status") == "not-run" and row.get("reason") == "Explicit subset")
        if not wanted or "blocked_by" in row:
            failures.append(f"{case_id:02d} ({row.get('status', 'missing')})")
    if failures:
        raise ValueError("Unexpected case results: " + ", ".join(failures))
    return len(selected)


def verify(args, lanes, suite):
    apk = identity(args.package_dir, args.source, args.package_run, args.package_attempt)
    if apk["attempt"] != args.package_attempt:
        raise ValueError("Comparison must pin the exact QA build attempt")
    metadata = {"source": args.source, "package_artifact": args.package_artifact,
                "comparison_run": args.comparison_run, "comparison_attempt": args.comparison_attempt,
                "evidence_kind": args.evidence_kind}
    expected = {lane["lane"] for lane in lanes}
    errors = {}
    if args.evidence.is_symlink() or not args.evidence.is_dir():
        raise ValueError("Missing or unsafe comparison evidence directory")
    unexpected = {path.name for path in args.evidence.iterdir()} - expected
    if unexpected:
        errors["matrix"] = "Unexpected lanes: " + ", ".join(sorted(unexpected))
    checked = {}
    for lane in lanes:
        name = lane["lane"]
        try:
            checked[name] = check_lane(args.evidence / name, lane, suite, apk, metadata)
        except (OSError, ValueError) as error:
            errors[name] = str(error)
    by_device = {device: {"full": checked[f"full-{device}"],
                          "split": sum(checked[f"split-{device}-{part}"] for part in (1, 2))}
                 for device in DEVICES} if not errors else {}
    print(json.dumps({**metadata, "package": apk, "passed_cases_by_lane": checked,
                      "passed_cases_by_device": by_device, "complete": not errors, "errors": errors}, indent=2))
    return int(bool(errors))


def positive(value):
    number = int(value)
    if number < 1:
        raise argparse.ArgumentTypeError("Expected a positive ID or attempt")
    return number


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    matrix = sub.add_parser("plan")
    check = sub.add_parser("verify")
    extract = sub.add_parser("unpack")
    for command in (matrix, check):
        command.add_argument("--source-tree", type=Path, default=ROOT)
    for command in (matrix, check, extract):
        command.add_argument("--source", required=True)
    for command in (check, extract):
        command.add_argument("--package-dir", type=Path, required=True)
        for name in ("package-run", "package-attempt"):
            command.add_argument("--" + name, type=positive, required=True)
    extract.add_argument("--archive", type=Path, required=True)
    extract.add_argument("--zip-sha256", required=True)
    extract.add_argument("--zip-bytes", type=positive, required=True)
    check.add_argument("--evidence", type=Path, required=True)
    check.add_argument("--evidence-kind", choices=("workflow", "offline-projection"), required=True)
    for name in ("package-artifact", "comparison-run", "comparison-attempt"):
        check.add_argument("--" + name, type=positive, required=True)
    args = parser.parse_args()
    try:
        if args.command == "unpack":
            return unpack(args)
        lanes, suite = plan(args.source, args.source_tree.resolve())
        if args.command == "plan":
            print(json.dumps({"source": args.source, "include": lanes}, indent=2))
            return 0
        return verify(args, lanes, suite)
    except (OSError, ValueError, subprocess.SubprocessError, zipfile.BadZipFile) as error:
        print(f"Android comparison rejected: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
