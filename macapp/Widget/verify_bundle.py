#!/usr/bin/env python3
"""Validate compiled widget packaging, including metadata in both executables."""
import argparse
import json
from pathlib import Path
import plistlib


def verify(app):
    extension = app / "Contents/PlugIns/GtmuxUsageWidget.appex"
    host_info = plistlib.loads((app / "Contents/Info.plist").read_bytes())
    child_info = plistlib.loads((extension / "Contents/Info.plist").read_bytes())
    if not child_info["CFBundleIdentifier"].startswith(host_info["CFBundleIdentifier"] + "."):
        raise ValueError("Widget must belong to its host")
    if child_info["CFBundleShortVersionString"] != host_info["CFBundleShortVersionString"]:
        raise ValueError("Widget and parent release versions must match")
    if child_info["CFBundleVersion"] != host_info["CFBundleVersion"]:
        raise ValueError("Widget and parent build versions must match")
    for bundle in [app, extension]:
        resources = bundle / "Contents/Resources"
        if not (resources / "GtmuxUsageWidgets_GtmuxUsageWidgets.bundle").is_dir():
            raise ValueError("Missing shared widget resources")
        data = (resources / "Metadata.appintents/extract.actionsdata").read_bytes()
        if len(data) > 2 * 1024 * 1024:
            raise ValueError("Unexpectedly large intent metadata")
        action = json.loads(data)["actions"]["MacUsageConfiguration"]
        if "com.apple.link.systemProtocol.WidgetConfiguration" not in action["systemProtocols"]:
            raise ValueError("Missing widget configuration protocol")
        parameter = next(p for p in action["parameters"] if p["name"] == "agent")
        if not parameter.get("isOptional") or not parameter.get("dynamicOptionsSupport"):
            raise ValueError("Agent selection must allow an initial automatic configuration")
    if (extension / "Contents/MacOS/gtmux").exists():
        raise ValueError("The CLI belongs only to the parent")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("app", type=Path)
    verify(parser.parse_args().app)
