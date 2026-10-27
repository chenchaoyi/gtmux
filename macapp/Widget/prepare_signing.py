#!/usr/bin/env python3
"""Create narrowly scoped Mac widget entitlements from the actual CLI signer."""
import argparse
from pathlib import Path
import plistlib
import re


def entitlements(team, parent):
    if team and re.fullmatch(r"[A-Z0-9]{10}", team) is None:
        raise ValueError("Invalid signing team")
    app = dict(parent)
    widget = {"com.apple.security.app-sandbox": True}
    if team:
        group = [team + ".com.gtmux.menubar.usage"]
        app["com.apple.security.application-groups"] = group
        widget["com.apple.security.application-groups"] = group
    return app, widget


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--team", default="")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    parent = plistlib.loads((Path(__file__).resolve().parents[1] / "Gtmux.entitlements").read_bytes())
    app, widget = entitlements(args.team, parent)
    args.output.mkdir(parents=True, exist_ok=True)
    for name, values in [("parent", app), ("widget", widget)]:
        (args.output / (name + ".entitlements")).write_bytes(plistlib.dumps(values))


if __name__ == "__main__":
    main()
