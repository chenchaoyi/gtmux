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
import struct
import subprocess


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


def capture_device(source, app, output, slot, device_type, runtime, run=command):
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
        for language in ("en", "zh"):
            progress(output, "capture", slot=slot, language=language)
            env["GTMUX_SHOTS_LANG"] = language
            subprocess.run(["npm", "run", "test:e2e", "--", test], cwd=source, env=env, check=True, timeout=1200)
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
    parser.add_argument("--harness", required=True, type=Path)
    args = parser.parse_args()
    source, app, output = args.source.resolve(), args.app.resolve(), args.output.resolve()
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise ValueError("This headless helper requires an isolated Actions runner; use the local e2e workflow on a shared Mac")
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
    harness_files = copy_capture_harness(source, args.harness.resolve())
    context = {"release_tag": os.environ["STORE_TAG"], "source_commit": os.environ["STORE_COMMIT"],
               "workflow_commit": os.environ["WORKFLOW_COMMIT"], "capture_harness_commit": os.environ["WORKFLOW_COMMIT"],
               "capture_harness_files": harness_files, "runtime": runtime["version"],
               "xcode": command(["xcodebuild", "-version"]), "synthetic_demo_only": True, "submitted": False}
    (output / "capture-context.json").write_text(json.dumps(context, indent=2) + "\n")
    for slot, device_type in devices:
        capture_device(source, app, output, slot, device_type, runtime)
    for language, locale in (("en", "en-US"), ("zh", "zh-Hans")):
        progress(output, "frame", language=language)
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
    provenance = {**context, "images": images}
    (output / "manifest.json").write_text(json.dumps(provenance, indent=2) + "\n")


if __name__ == "__main__":
    main()
