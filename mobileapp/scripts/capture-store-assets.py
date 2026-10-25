#!/usr/bin/env python3
"""Capture synthetic App Store assets using an already built simulator app."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess

from store_artifacts import GROUPS, checkpoint_group, load_checkpoint, png_record, save, tree_hash, verify_records


def command(args, **kwargs):
    kwargs.setdefault("timeout", 900)
    return subprocess.check_output(args, text=True, **kwargs).strip()


CAPTURE_HARNESS = ("__tests__/appstore-shots.test.ts", "__tests__/appstore-shots-ipad.test.ts",
                   "setup/demoEntry.ts", "setup/capabilities.ts", "setup/global-setup.ts")


def copy_capture_harness(source, harness):
    files = [(harness / name, source / "e2e" / name) for name in CAPTURE_HARNESS]
    if any(not src.is_file() or src.is_symlink() for src, _ in files):
        raise ValueError("Capture harness is incomplete")
    for src, dst in files:
        dst.parent.mkdir(parents=True, exist_ok=True)
        if src.resolve() != dst.resolve():
            shutil.copyfile(src, dst)
    return {name: hashlib.sha256((harness / name).read_bytes()).hexdigest() for name in CAPTURE_HARNESS}


def progress(output, phase, **fields):
    event = {"time": datetime.datetime.now(datetime.timezone.utc).isoformat(), "phase": phase, **fields}
    if output.is_dir():
        line = json.dumps(event)
        print(line, flush=True)
        with (output / "progress.jsonl").open("a") as stream:
            stream.write(line + "\n")


def load_review(output, context):
    path = output / "review.json"
    if not path.exists():
        return {"schema": 1, "context": context, "groups": {}}
    data = json.loads(path.read_text())
    if data.get("schema") != 1 or not isinstance(data.get("groups"), dict):
        raise ValueError("Invalid visual-review checkpoint")
    for key, records in data["groups"].items():
        if key not in GROUPS or {r["file"] for r in records} != {f"review/{key}/{n}.png" for n in GROUPS[key]}:
            raise ValueError("Incomplete visual-review group")
        verify_records(output, records)
    return data if data.get("context") == context else {"schema": 1, "context": context, "groups": {}}


def checkpoint_review(output, context, key, raw):
    data = load_review(output, context)
    records = [png_record(raw / (name + ".png"), raw) for name in GROUPS[key]]
    size = [2752, 2064] if key.startswith("ipad-") else [1320, 2868]
    if any(r["size"] != size for r in records):
        raise ValueError("Wrong visual-review dimensions")
    light = load_checkpoint(output, context)["groups"].get(key, [])
    first = next((r for r in light if Path(r["file"]).stem == "00-demo-entry"), None)
    if not first or first["sha256"] == records[0]["sha256"]:
        raise ValueError("Dark review did not change the first-launch appearance")
    for name in GROUPS[key]:
        target = output / "review" / key / (name + ".png")
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(raw / (name + ".png"), target)
    data["groups"][key] = [png_record(output / "review" / key / (name + ".png"), output) for name in GROUPS[key]]
    save(output / "review.json", data)


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


def wda_project(appium_home, entry):
    home, entry = appium_home.resolve(), entry.resolve()
    if not entry.is_relative_to(home):
        raise ValueError("WDA must resolve inside this run's Appium home")
    for folder in entry.parents:
        if folder == home:
            break
        package = folder / "package.json"
        project = folder / "WebDriverAgent.xcodeproj"
        if package.is_file() and project.is_dir():
            info = json.loads(package.read_text())
            if info.get("name") == "appium-webdriveragent":
                return project, info["version"]
    raise ValueError("No WDA project for the installed XCUITest driver")


def prebuild_wda(appium_home, udid, output, slot, platform_version, run=command):
    driver = appium_home / "node_modules/appium-xcuitest-driver"
    entry = Path(run(["node", "-p", "require.resolve('appium-webdriveragent', {paths: [process.argv[1]]})", str(driver)]))
    project, version = wda_project(appium_home, entry)
    progress(output, "wda-build", slot=slot, version=version)
    # WDA 15.0.0 carries build-only exit state into its next readiness check.
    # A separate compiler process leaves the new Appium session with fresh state.
    with (output / f"wda-build-{slot}.log").open("w") as log:
        subprocess.run(["xcodebuild", "build-for-testing", "-project", str(project),
                        "-scheme", "WebDriverAgentRunner", "-derivedDataPath", str(output / "wda"),
                        "-destination", f"id={udid}", "-jobs", "2",
                        f"IPHONEOS_DEPLOYMENT_TARGET={platform_version}",
                        "GCC_TREAT_WARNINGS_AS_ERRORS=0", "COMPILER_INDEX_STORE_ENABLE=NO"],
                       stdout=log, stderr=subprocess.STDOUT, check=True, timeout=900)


def capture_device(source, app, output, slot, device_type, runtime, run=command, context=None):
    checkpoint = load_checkpoint(output, context) if context else {"groups": {}}
    review = load_review(output, context) if context and context.get("review_capture") else None
    languages = [lang for lang in ("en", "zh") if slot + "-" + lang not in checkpoint["groups"] or
                 (review is not None and slot + "-" + lang not in review["groups"])]
    if not languages:
        progress(output, "reused", slot=slot)
        return
    # Only this newly returned device ID is ever shut down or deleted.
    progress(output, "create", slot=slot, runtime=runtime.get("version"))
    udid = run(["xcrun", "simctl", "create", f"gtmux-store-{slot}", device_type["identifier"], runtime["identifier"]])
    try:
        progress(output, "boot", slot=slot)
        run(["xcrun", "simctl", "boot", udid])
        run(["xcrun", "simctl", "bootstatus", udid, "-b"])
        run(["xcrun", "simctl", "install", udid, str(app)])
        prebuild_wda(Path(os.environ["APPIUM_HOME"]), udid, output, slot, runtime["version"])
        env = dict(os.environ, GTMUX_E2E_UDID=udid, GTMUX_E2E_DEVICE=device_type["name"],
                   GTMUX_E2E_OS=runtime["version"], GTMUX_DEMO_SHOTS="1",
                   GTMUX_E2E_HEADLESS="1",
                   GTMUX_E2E_CAPTURE_CI="1",
                   GTMUX_E2E_WDA_DERIVED=str(output / "wda"))
        test = "appstore-shots-ipad.test.ts" if slot == "ipad" else "appstore-shots.test.ts"
        for language in languages:
            key = slot + "-" + language
            for appearance in (("light", "dark") if review is not None else ("light",)):
                saved = review if appearance == "dark" else checkpoint
                if context and key in saved["groups"]:
                    continue
                # Select appearance before the harness cold-launches the app,
                # matching the existing usage-clarity test. Mid-page simctl
                # changes did not refresh React Native's current color scheme.
                if review is not None:
                    run(["xcrun", "simctl", "ui", udid, "appearance", appearance])
                progress(output, "capture", slot=slot, language=language, appearance=appearance)
                env["GTMUX_SHOTS_LANG"] = language
                env["GTMUX_STORE_APPEARANCE"] = appearance
                subprocess.run(["npm", "run", "test:e2e", "--", test], cwd=source, env=env, check=True, timeout=1200)
                if context:
                    folder = ("ipad-" if slot == "ipad" else "") + language
                    raw = source / ".e2e-artifacts/appstore" / folder
                    if appearance == "dark":
                        checkpoint_review(output, context, key, raw / "review")
                    else:
                        checkpoint_group(output, context, key, raw,
                                         origin={"run_id": os.environ.get("GITHUB_RUN_ID"), "workflow_commit": os.environ.get("WORKFLOW_COMMIT")})
                    progress(output, "checkpoint", slot=slot, language=language, appearance=appearance)
    finally:
        for action in ("shutdown", "delete"):
            try:
                run(["xcrun", "simctl", action, udid])
            except subprocess.CalledProcessError:
                pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--app", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--harness", required=True, type=Path)
    args = parser.parse_args()
    source, app, output = args.source.resolve(), args.app.resolve(), args.output.resolve()
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise ValueError("This headless helper requires an isolated Actions runner; use the local e2e workflow on a shared Mac")
    if not app.is_dir():
        raise ValueError("Need a verified simulator app")
    actual_commit = command(["git", "rev-parse", "HEAD"], cwd=source)
    info = plistlib.loads((app / "Info.plist").read_bytes())
    if actual_commit != os.environ["STORE_COMMIT"] or info.get("CFBundleShortVersionString") != os.environ["STORE_TAG"].removeprefix("v"):
        raise ValueError("Source or simulator app version differs from the requested release")
    output.mkdir(parents=True, exist_ok=True)
    runtime, devices = simulator_plan(
        json.loads(command(["xcrun", "simctl", "list", "runtimes", "--json"]))["runtimes"],
        json.loads(command(["xcrun", "simctl", "list", "devicetypes", "--json"]))["devicetypes"])
    harness_files = copy_capture_harness(source, args.harness.resolve())
    context = {"release_tag": os.environ["STORE_TAG"], "source_commit": os.environ["STORE_COMMIT"],
               "workflow_commit": os.environ["WORKFLOW_COMMIT"], "capture_harness_commit": os.environ["WORKFLOW_COMMIT"],
               "capture_harness_files": harness_files, "runtime": runtime["version"],
               "xcode": command(["xcodebuild", "-version"]), "app_sha256": tree_hash(app),
               "runtime_identity": {key: runtime.get(key) for key in ("identifier", "version", "buildversion")},
               "devices": {slot: device["identifier"] for slot, device in devices},
               "capture_tool_files": {path.name: hashlib.sha256(path.read_bytes()).hexdigest()
                                      for path in (Path(__file__), Path(__file__).with_name("store_artifacts.py"))},
               "synthetic_demo_only": True, "review_capture": True, "submitted": False}
    # Run/workflow identity is evidence, not a capture input. Only matching app,
    # harness, runtime and Xcode can reuse completed groups.
    identity = {key: value for key, value in context.items() if key not in ("workflow_commit", "capture_harness_commit")}
    checkpoint = load_checkpoint(output, identity)
    checkpoint.setdefault("attempts", []).append({**context, "run_id": os.environ.get("GITHUB_RUN_ID")})
    save(output / "checkpoint.json", checkpoint)
    save(output / "capture-context.json", context)
    for slot, device_type in devices:
        capture_device(source, app, output, slot, device_type, runtime, context=identity)
    progress(output, "capture-complete")


if __name__ == "__main__":
    main()
