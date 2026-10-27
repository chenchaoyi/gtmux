#!/usr/bin/env python3
"""Block release builds with Go versions affected by GO-2026-6809."""
import re
import subprocess
import sys


def patched(version):
    match = re.fullmatch(r"go(\d+)\.(\d+)\.(\d+)", version.strip())
    if not match:
        return False
    major, minor, patch = map(int, match.groups())
    return (major, minor) > (1, 27) or major == 1 and (
        minor == 27 and patch >= 3 or minor == 26 and patch >= 10)


def main():
    try:
        version = subprocess.check_output(["go", "env", "GOVERSION"], text=True,
                                          stderr=subprocess.DEVNULL, timeout=10)
    except (OSError, subprocess.SubprocessError):
        print("Cannot verify the Go build toolchain.", file=sys.stderr)
        return 1
    if not patched(version):
        print("Build requires Go 1.27.3 / 1.26.10 or a newer stable release "
              "(GO-2026-6809).", file=sys.stderr)
        return 1
    print("Go certificate security floor verified: " + version.strip())
    return 0


if __name__ == "__main__":
    sys.exit(main())
