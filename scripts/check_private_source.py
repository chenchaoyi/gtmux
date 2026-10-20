#!/usr/bin/env python3
"""Check tracked text against an operator-private policy without logging its values."""
import json
import os
from pathlib import Path
import subprocess
import sys


def load_terms(env):
    raw = env.get("GTMUX_PRIVATE_TERMS", "")
    if not raw and env.get("GTMUX_PRIVATE_TERMS_FILE"):
        raw = Path(env["GTMUX_PRIVATE_TERMS_FILE"]).read_text(encoding="utf-8")
    if not raw:
        if env.get("GTMUX_PRIVATE_POLICY_REQUIRED") == "true":
            raise ValueError("Required private policy is unavailable")
        return None
    terms = json.loads(raw)
    if not isinstance(terms, list) or not terms or any(
        not isinstance(term, str) or len(term.strip()) < 3 for term in terms
    ):
        raise ValueError("Private policy must be a nonempty list of terms")
    return list(dict.fromkeys(term.casefold() for term in terms))


def check_paths(root, paths, terms, output):
    hits = 0
    for name in paths:
        path = root / name
        private_name = any(term in name.casefold() for term in terms)
        shown_name = "[private path]" if private_name else json.dumps(name, ensure_ascii=False)
        if private_name:
            print("[private path]: private-policy match (path withheld)", file=output)
            hits += 1
        # Never follow a tracked symlink into private files outside the checkout.
        if path.is_symlink():
            continue
        data = path.read_bytes()
        if b"\0" in data:
            continue
        for number, line in enumerate(data.decode("utf-8", errors="replace").splitlines(), 1):
            if any(term in line.casefold() for term in terms):
                print(f"{shown_name}:{number}: private-policy match (content withheld)", file=output)
                hits += 1
    return hits


def main():
    try:
        terms = load_terms(os.environ)
        if terms is None:
            print("privacy-check: private policy unavailable; exact-term coverage skipped")
            return 0
        files = subprocess.check_output(["git", "ls-files", "-z"]).decode("utf-8").split("\0")
        hits = check_paths(Path.cwd(), [name for name in files if name], terms, sys.stderr)
    except (OSError, ValueError, subprocess.CalledProcessError):
        # Exception details can contain policy values or operator-local paths.
        print("privacy-check: cannot load policy or inspect tracked files", file=sys.stderr)
        return 2
    print("privacy-check: FAILED" if hits else "privacy-check: OK")
    return int(hits > 0)


if __name__ == "__main__":
    raise SystemExit(main())
