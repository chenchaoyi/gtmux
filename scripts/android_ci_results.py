#!/usr/bin/env python3
"""Select and verify full phone coverage and the two tablet emulator shards."""

import argparse
from datetime import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
DEVICES = {"phone": "pixel_7", "tablet": "pixel_tablet"}
PARTS = (1, 2)


def partition(cases):
    ids = [case["id"] for case in cases]
    if len(ids) < 31 or ids != list(range(1, len(ids) + 1)):
        raise ValueError("Android case inventory is incomplete or out of order")
    selections = {1: ids[:17], 2: ids[17:]}
    for selected in selections.values():
        groups = {case["group"] for case in cases if case["id"] in selected}
        for case in cases:
            if (case["group"] in groups) != (case["id"] in selected):
                raise ValueError("An Android setup group crosses the partition")
            if case["id"] in selected and any(dep not in selected for dep in case["requires"]):
                raise ValueError("An Android prerequisite crosses the partition")
    return {case["id"]: case for case in cases}, selections


def suite():
    journey = ROOT / "mobileapp/e2e/android/journey.cjs"
    return partition(json.loads(subprocess.check_output([
        "node", "-e", "console.log(JSON.stringify(require(process.argv[1]).cases))", str(journey),
    ])))


def execution(job):
    started = datetime.fromisoformat(job["started_at"])
    finished = datetime.fromisoformat(job["completed_at"])
    if not job["runner_id"] or started > finished:
        raise ValueError("Missing or invalid emulator execution timestamps")
    return job["runner_id"], started, finished


def shard_attempts(jobs, run, attempt, job_head):
    """GitHub copies retained successes into the current attempt with new job IDs."""
    if not isinstance(jobs, list) or not all(isinstance(job, dict) for job in jobs):
        raise ValueError("Missing or invalid Android job history")
    result = {}
    for part in PARTS:
        name = f"Android tablet emulator shard {part}"
        history = {}
        for job in (job for job in jobs if job.get("name") == name):
            index = job.get("run_attempt")
            if (job.get("run_id") != run or job.get("head_sha") != job_head
                    or type(index) is not int or not 1 <= index <= attempt or index in history):
                raise ValueError(f"{name}: inconsistent run, head or attempt history")
            history[index] = job
        current = history.get(attempt)
        if not current or current.get("status") != "completed" or current.get("conclusion") != "success":
            state = (current or {}).get("conclusion") or "missing or incomplete"
            raise ValueError(f"{name}: current attempt did not succeed ({state})")
        recorded = execution(current)
        originals = [index for index, job in history.items()
                     if job.get("status") == "completed" and job.get("conclusion") == "success"
                     and execution(job) == recorded
                     and datetime.fromisoformat(job["created_at"]) <= recorded[1]]
        if len(originals) != 1:
            raise ValueError(f"{name}: no unique original execution for the current success")
        result[part] = originals[0]
    return result


def read_receipt(directory, name):
    path = directory / name
    if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= 1024**2:
        raise ValueError(f"Missing or unsafe {name}")
    value = json.loads(path.read_text())
    if not isinstance(value, dict):
        raise ValueError(f"Invalid {name}")
    return value


def lane(source, run, attempt, device, part):
    return {"source": source, "run": run, "attempt": attempt, "device": device,
            "profile": DEVICES[device], "shard": part}


def selected_cases(inventory, device, part):
    cases, selections = inventory
    if device == "phone" and part == 0:
        return list(cases)
    if device == "tablet" and part in PARTS:
        return selections[part]
    raise ValueError("Invalid Android device and shard combination")


def verify_lane(receipt_dir, inventory, package, source, run, attempt, device, part):
    cases, _ = inventory
    selected = selected_cases(inventory, device, part)
    label = "Android phone" if device == "phone" else f"Android tablet shard {part}"
    if (not isinstance(package, dict) or package.get("source") != source or package.get("run") != run
            or type(package.get("attempt")) is not int or package["attempt"] < 1
            or type(package.get("bytes")) is not int or not 0 < package["bytes"] <= 256 * 1024**2
            or not re.fullmatch(r"[0-9a-f]{64}", str(package.get("sha256")))):
        raise ValueError("Missing or invalid native QA package identity")
    if (receipt_dir.is_symlink() or package["attempt"] > attempt
            or read_receipt(receipt_dir, "LANE.json") != lane(source, run, attempt, device, part)):
        raise ValueError(f"{label}: wrong product, device, or attempt")
    apk = read_receipt(receipt_dir, "APK.json")
    if apk != {"file": "app-qa.apk", "bytes": package["bytes"], "sha256": package["sha256"]}:
        raise ValueError(f"{label}: QA APK differs from the native build")
    result = read_receipt(receipt_dir, "RESULTS.json")
    rows = result.get("results")
    if (result.get("schema") != 2 or result.get("package") != "com.gtmux.app.qa"
            or not re.fullmatch(r"emulator-[0-9]+", str(result.get("serial")))
            or result.get("selection") != selected
            or any(type(case_id) is not int for case_id in result["selection"])
            or result.get("first_automation_failure", True) is not None
            or not isinstance(rows, list) or len(rows) != len(cases)):
        raise ValueError(f"{label}: incomplete or mismatched result")
    seen = set()
    for row in rows:
        match = re.fullmatch(r"([0-9]{2})-[a-z0-9-]+", str(row.get("name"))) if isinstance(row, dict) else None
        case_id = int(match[1]) if match else 0
        if case_id not in cases or case_id in seen:
            raise ValueError(f"{label}: unregistered or duplicate case")
        seen.add(case_id)
        case = cases[case_id]
        wanted = (row.get("status") == "passed" if case_id in selected else
                  row.get("status") == "not-run" and row.get("reason") == "Explicit subset")
        if (not wanted or "blocked_by" in row or row.get("group") != case["group"]
                or row.get("capability") != case["capability"]):
            raise ValueError(f"{label}: case {case_id:02d} has invalid coverage ({row.get('status')})")
    return len(selected)


def verify_device(directory, inventory, package, source, run, attempts):
    return sum(verify_lane(directory / str(part), inventory, package, source, run, attempts[part], "tablet", part)
               for part in PARTS)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    select = commands.add_parser("select")
    verify_one = commands.add_parser("verify-lane")
    for command in (select, verify_one):
        command.add_argument("--device", choices=DEVICES, required=True)
        command.add_argument("--shard", type=int, choices=(0, *PARTS), required=True)
        command.add_argument("--evidence", type=Path, required=True)
    resolve = commands.add_parser("resolve")
    resolve.add_argument("--jobs", type=Path, required=True)
    verify = commands.add_parser("verify")
    verify.add_argument("--receipts", type=Path, required=True)
    verify.add_argument("--attempt-1", type=int, required=True)
    verify.add_argument("--attempt-2", type=int, required=True)
    args = parser.parse_args()
    try:
        run, attempt = (int(os.environ[key]) for key in ("GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT"))
        source = os.environ["GITHUB_SHA"]
        if min(run, attempt) < 1 or not re.fullmatch(r"[0-9a-f]{40}", source):
            raise ValueError("Invalid workflow identity")
        if args.command == "resolve":
            jobs = json.loads(args.jobs.read_text())
            for part, origin in shard_attempts(jobs, run, attempt, os.environ["GTMUX_CI_JOB_HEAD"]).items():
                print(f"attempt_{part}={origin}")
        elif args.command == "select":
            selected = selected_cases(suite(), args.device, args.shard)
            args.evidence.mkdir(exist_ok=True)
            with (args.evidence / "LANE.json").open("x") as stream:
                json.dump(lane(source, run, attempt, args.device, args.shard), stream)
            if args.shard:
                print(",".join(f"{case:02d}" for case in selected))
        elif args.command == "verify-lane":
            package = json.loads(os.environ["GTMUX_ANDROID_QA_RECEIPT"])
            count = verify_lane(args.evidence, suite(), package, source, run, attempt, args.device, args.shard)
            print(f"Android {args.device}: {count} assigned cases passed on QA APK {package['sha256']}")
        else:
            attempts = {1: args.attempt_1, 2: args.attempt_2}
            if any(not 1 <= origin <= attempt for origin in attempts.values()):
                raise ValueError("Invalid shard receipt attempt")
            package = json.loads(os.environ["GTMUX_ANDROID_QA_RECEIPT"])
            count = verify_device(args.receipts, suite(), package, source, run, attempts)
            print(f"Android tablet: {count} cases passed once on QA APK {package['sha256']}")
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print(f"Android CI: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
