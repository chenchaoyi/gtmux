#!/usr/bin/env python3
"""Bind parallel emulator checks to the single QA package built in this CI run."""
import argparse
import hashlib
import json
from pathlib import Path
import re

PACKAGE = Path("mobileapp/android/app/build/outputs/apk/qa")


def identity(folder, source, run, attempt, write=False):
    if not re.fullmatch(r"[0-9a-f]{40}", source) or run < 1 or attempt < 1:
        raise ValueError("Exact source and positive run/attempt required")
    apk, receipt = folder / "app-qa.apk", folder / "CI-PACKAGE.json"
    if apk.is_symlink() or not apk.is_file() or not 0 < apk.stat().st_size <= 256 * 1024**2:
        raise ValueError("Expected one bounded regular QA APK")
    with apk.open("rb") as stream:
        digest = hashlib.file_digest(stream, "sha256").hexdigest()
    expected = {"source": source, "run": run, "attempt": attempt,
                "bytes": apk.stat().st_size, "sha256": digest}
    if write:
        if receipt.exists() or receipt.is_symlink():
            raise ValueError("Refusing to replace an existing package receipt")
        with receipt.open("x") as stream:
            json.dump(expected, stream, sort_keys=True)
    else:
        if receipt.is_symlink() or not receipt.is_file() or receipt.stat().st_size > 4096:
            raise ValueError("Package receipt missing or unsafe")
        recorded = json.loads(receipt.read_text())
        # Failed-only re-runs reuse the successful build job's output, not a new build.
        if (not isinstance(recorded, dict) or type(recorded.get("run")) is not int
                or type(recorded.get("bytes")) is not int or type(recorded.get("attempt")) is not int
                or not 1 <= recorded["attempt"] <= attempt):
            raise ValueError("Package attempt is invalid")
        expected["attempt"] = recorded["attempt"]
        if recorded != expected:
            raise ValueError("QA package source/run/bytes differ from the build receipt")
    return expected


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("write", "verify"))
    parser.add_argument("--source", required=True)
    parser.add_argument("--run", type=int, required=True)
    parser.add_argument("--attempt", type=int, required=True)
    args = parser.parse_args()
    identity(PACKAGE, args.source, args.run, args.attempt, args.command == "write")
    print("QA package source and checksum verified")


if __name__ == "__main__":
    main()
