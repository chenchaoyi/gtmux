#!/usr/bin/env python3
"""Read-only version/notes preflight; run before the first release PR, not after CI."""
import argparse
import json
from pathlib import Path
import re
import subprocess


def check(root, version, platforms):
    if not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version):
        raise ValueError("Use a stable version X.Y.Z")
    if not platforms or not set(platforms) <= {"ios", "android", "none"} or ("none" in platforms and len(platforms) != 1):
        raise ValueError("Select ios/android, or none for a CLI/Mac-only release")
    if platforms == ["none"]:
        return {"version": version, "platforms": platforms, "mobile_build": "not selected"}
    mobile = root / "mobileapp"
    if f"export const APP_VERSION = '{version}';" not in (mobile / "src/version.ts").read_text().splitlines():
        raise ValueError("Stamp the selected mobile version before opening the release PR")
    if "ios" in platforms:
        native = re.findall(r"MARKETING_VERSION = ([0-9.]+);", (mobile / "ios/GtmuxMobile.xcodeproj/project.pbxproj").read_text())
        if not native or set(native) != {version}:
            raise ValueError("Native Apple marketing versions differ from the selected version")
        for language, store in [("en", "en-US"), ("zh", "zh-Hans")]:
            archived = (mobile / f"release-notes/{version}.{language}.txt").read_text()
            if not archived.strip() or archived != (mobile / f"fastlane/metadata/{store}/release_notes.txt").read_text():
                raise ValueError("Archive this version's bilingual Apple notes before the release PR")
        generated = subprocess.check_output(["bash", str(mobile / "scripts/gen-release-notes.sh")])
        if generated != (mobile / "src/releaseNotes.ts").read_bytes():
            raise ValueError("Regenerate the bundled Apple release history")
        current_hash = subprocess.check_output(["bash", str(mobile / "scripts/appsrc-hash.sh")]).decode().strip()
        if current_hash != (mobile / f"release-notes/{version}.srchash").read_text().strip():
            raise ValueError("Apple notes fingerprint differs from the current app sources")
    if "android" in platforms:
        notes = json.loads((mobile / "release-notes/android.json").read_text())
        if not isinstance(notes, list):
            raise ValueError("Android notes must be a versioned list")
        rows = [row for row in notes if isinstance(row, dict) and row.get("version") == version]
        if len(rows) != 1 or any(not isinstance(rows[0].get(lang), list) or not rows[0][lang]
                                 or any(not isinstance(line, str) or not line.strip() for line in rows[0][lang])
                                 for lang in ("en", "zh")):
            raise ValueError("This Android version needs its own nonempty bilingual release notes")
    return {"version": version, "platforms": sorted(set(platforms)), "version_and_notes": "ready",
            "boundary": "Preparation only; latest-main CI, delivery scope, signing and installation checks still required"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--version", required=True)
    parser.add_argument("--platform", choices=("ios", "android", "none"), action="append", required=True)
    args = parser.parse_args()
    try:
        print(json.dumps(check(args.root, args.version, args.platform), indent=2))
    except ValueError as error:
        raise SystemExit("Release preparation incomplete: " + str(error) + " (no files changed).")
    except (OSError, subprocess.CalledProcessError):
        raise SystemExit("Release preparation incomplete: a required version/notes input or generator is unavailable (no files changed).")


if __name__ == "__main__":
    main()
