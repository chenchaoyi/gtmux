#!/usr/bin/env python3
"""Capture synthetic App Store assets using an already built simulator app."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import struct
import subprocess


def command(args, **kwargs):
    return subprocess.check_output(args, text=True, **kwargs).strip()


def simulator_plan(runtimes, device_types):
    available = [r for r in runtimes if r.get("isAvailable") and r.get("platform") == "iOS"]
    if not available:
        raise ValueError("No available iOS simulator runtime")
    runtime = max(available, key=lambda r: tuple(int(x) for x in r["version"].split(".")))
    types = {d["identifier"]: d for d in device_types}
    wanted = [
        ("phone", "com.apple.CoreSimulator.SimDeviceType.iPhone-17-Pro-Max"),
        ("ipad", "com.apple.CoreSimulator.SimDeviceType.iPad-Pro-13-inch-M5-12GB"),
    ]
    if any(identifier not in types for _, identifier in wanted):
        raise ValueError("Need iPhone 17 Pro Max and iPad Pro 13-inch (M5) device types")
    return runtime, [(slot, types[identifier]) for slot, identifier in wanted]


def capture_device(source, app, output, slot, device_type, runtime, run=command):
    # Only this newly returned device ID is ever shut down or deleted.
    udid = run(["xcrun", "simctl", "create", f"gtmux-store-{slot}", device_type["identifier"], runtime["identifier"]])
    try:
        run(["xcrun", "simctl", "boot", udid])
        run(["xcrun", "simctl", "bootstatus", udid, "-b"])
        run(["xcrun", "simctl", "install", udid, str(app)])
        env = dict(os.environ, GTMUX_E2E_UDID=udid, GTMUX_E2E_DEVICE=device_type["name"],
                   GTMUX_E2E_OS=runtime["version"], GTMUX_DEMO_SHOTS="1",
                   GTMUX_E2E_WDA_DERIVED=str(output / "wda"))
        test = "appstore-shots-ipad.test.ts" if slot == "ipad" else "appstore-shots.test.ts"
        for language in ("en", "zh"):
            env["GTMUX_SHOTS_LANG"] = language
            subprocess.run(["npm", "run", "test:e2e", "--", test], cwd=source, env=env, check=True)
    finally:
        for action in ("shutdown", "delete"):
            try:
                run(["xcrun", "simctl", action, udid])
            except subprocess.CalledProcessError:
                pass


def image_record(path, output):
    data = path.read_bytes()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError(f"Not a PNG: {path.name}")
    return {"file": str(path.relative_to(output)), "sha256": hashlib.sha256(data).hexdigest(),
            "bytes": len(data), "size": list(struct.unpack(">II", data[16:24]))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--app", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    source, app, output = args.source.resolve(), args.app.resolve(), args.output.resolve()
    if not app.is_dir() or output.exists():
        raise ValueError("Need a simulator app and a fresh output directory")
    actual_commit = command(["git", "rev-parse", "HEAD"], cwd=source)
    info = plistlib.loads((app / "Info.plist").read_bytes())
    if actual_commit != os.environ["STORE_COMMIT"] or info.get("CFBundleShortVersionString") != os.environ["STORE_TAG"].removeprefix("v"):
        raise ValueError("Source or simulator app version differs from the requested release")
    output.mkdir(parents=True)
    runtime, devices = simulator_plan(
        json.loads(command(["xcrun", "simctl", "list", "runtimes", "--json"]))["runtimes"],
        json.loads(command(["xcrun", "simctl", "list", "devicetypes", "--json"]))["devicetypes"])
    for slot, device_type in devices:
        capture_device(source, app, output, slot, device_type, runtime)
    for language, locale in (("en", "en-US"), ("zh", "zh-Hans")):
        raw = source / ".e2e-artifacts/appstore" / language
        subprocess.run(["node", "scripts/render-lockscreen.mjs", "--lang", language, "--out", str(raw)], cwd=source, check=True)
        for slot in ("phone", "ipad"):
            name = f"ipad-{language}" if slot == "ipad" else language
            options = ["--prefix", "ipad-"] if slot == "ipad" else []
            subprocess.run(["node", "scripts/frame-shots.mjs", "--slot", slot, "--in", f".e2e-artifacts/appstore/{name}",
                            "--lang", name, "--out", str(output / "screenshots" / locale), *options], cwd=source, check=True)
        subprocess.run(["node", "scripts/frame-creative.mjs", "--lang", language, "--ipad", f".e2e-artifacts/appstore/ipad-{language}",
                        "--phone", f".e2e-artifacts/appstore/{language}", "--out", str(output / "creative" / locale)], cwd=source, check=True)
    for path in (source / ".e2e-artifacts/appstore").glob("*/*.png"):
        destination = output / "raw" / path.parent.name / path.name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, destination)
    shutil.rmtree(output / "wda", ignore_errors=True)
    images = [image_record(p, output) for p in sorted(output.rglob("*.png"))]
    framed = [i for i in images if i["file"].startswith("screenshots/")]
    creative = [i for i in images if i["file"].startswith("creative/")]
    if len(framed) != 22 or len(creative) != 2:
        raise ValueError("Expected 22 screenshots and two creative images")
    for image in framed + creative:
        expected = [5244, 2950] if image in creative else ([2752, 2064] if Path(image["file"]).name.startswith("ipad-") else [1320, 2868])
        if image["size"] != expected:
            raise ValueError(f"Unexpected image dimensions: {image['file']}")
    provenance = {"release_tag": os.environ["STORE_TAG"], "source_commit": os.environ["STORE_COMMIT"],
                  "workflow_commit": os.environ["WORKFLOW_COMMIT"], "runtime": runtime["version"],
                  "xcode": command(["xcodebuild", "-version"]), "images": images,
                  "synthetic_demo_only": True, "submitted": False}
    (output / "manifest.json").write_text(json.dumps(provenance, indent=2) + "\n")


if __name__ == "__main__":
    main()
