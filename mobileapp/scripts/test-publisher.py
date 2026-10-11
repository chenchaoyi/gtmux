#!/usr/bin/env python3
"""Load private publisher settings without putting credentials in shell commands.

This is a maintainer adapter for ccy.dev's uploader, not a gtmux product feature.
Only the existing uploader performs IPA inspection, publication and link issuance.
"""
import argparse
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
from urllib.parse import urlsplit

FIELDS = (
    "BUILDS_UPLOAD_KEY", "BUILDS_R2_ACCOUNT_ID", "BUILDS_R2_BUCKET",
    "BUILDS_R2_ACCESS_KEY_ID", "BUILDS_R2_SECRET_ACCESS_KEY",
)
LIMIT = 16 * 1024
DEFAULT_ORIGIN = "https://builds.ccy.dev"


class ConfigError(Exception):
    pass


def config_path(environ):
    explicit = environ.get("BUILDS_PUBLISHER_CONFIG")
    if explicit:
        return Path(explicit).expanduser()
    root = environ.get("XDG_CONFIG_HOME") or str(Path.home() / ".config")
    return Path(root).expanduser() / "ccy-builds/gtmux-publisher.json"


def validate(data):
    if not isinstance(data, dict) or set(data) - set(FIELDS) - {"origin"}:
        raise ConfigError("Unsupported publisher configuration fields.")
    for key, value in data.items():
        if not isinstance(value, str) or not value.strip() or any(c in value for c in "\r\n\0"):
            raise ConfigError(f"Invalid configuration field: {key}.")
    origin = data.get("origin", DEFAULT_ORIGIN)
    try:
        url = urlsplit(origin)
    except ValueError:
        raise ConfigError("Publisher origin is invalid.") from None
    if url.scheme != "https" or not url.hostname or url.username or url.password or url.path not in ("", "/") or url.query or url.fragment:
        raise ConfigError("Publisher origin must be an HTTPS origin without credentials or a path.")
    return data


def decode(raw):
    if len(raw) > LIMIT:
        raise ConfigError("Publisher configuration is too large.")
    try:
        return validate(json.loads(raw))
    except (ValueError, UnicodeError):
        raise ConfigError("Publisher configuration is not valid JSON.") from None


def read_config(path):
    try:
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    except OSError:
        raise ConfigError("Cannot read publisher configuration; check its path and permissions.") from None
    with os.fdopen(fd, "rb") as source:
        info = os.fstat(source.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o600:
            raise ConfigError("Publisher configuration must be an owner-owned regular file with mode 0600.")
        return decode(source.read(LIMIT + 1))


def settings(path, environ):
    # Environment values take priority, including complete CI-secret setups with no file.
    data = {} if all(environ.get(k, "").strip() for k in FIELDS) else (read_config(path) if path.exists() else {})
    env = environ.copy()
    for key in FIELDS:
        if not env.get(key, "").strip() and key in data:
            env[key] = data[key]
    missing = [key for key in FIELDS if not env.get(key, "").strip()]
    if missing:
        raise ConfigError("Missing settings: " + ", ".join(missing) + ". Import an existing authorized configuration over verified SSH, or use the configured hosted workflow/local secrets; see mobileapp/DISTRIBUTION.md.")
    resolved = {key: env[key] for key in FIELDS}
    resolved["origin"] = environ.get("BUILDS_ORIGIN") or data.get("origin", DEFAULT_ORIGIN)
    validate(resolved)
    return env, resolved["origin"]


def import_config(path, source):
    data = decode(source.read(LIMIT + 1))
    if any(k not in data for k in FIELDS):
        raise ConfigError("Import needs all five publisher settings; existing file was not changed.")
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except OSError:
        raise ConfigError("Cannot create configuration; destination must not already exist.") from None
    try:
        with os.fdopen(fd, "w") as dest:
            os.fchmod(dest.fileno(), 0o600)
            json.dump(data, dest)
            dest.write("\n")
    except BaseException:
        path.unlink()
        raise


def private_output(path):
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except OSError:
        raise ConfigError("Cannot create result file; use a new private output path.") from None
    os.fchmod(fd, 0o600)
    return os.fdopen(fd, "w")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, help="private JSON path; overrides BUILDS_PUBLISHER_CONFIG and XDG_CONFIG_HOME")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("import-config", help="read private JSON from stdin, create mode-0600 file; never overwrite")
    check = commands.add_parser("check", help="local preflight only; no network calls or secret output")
    check.add_argument("--uploader", type=Path, help="ccy.dev distribution/scripts/builds.py path")
    run = commands.add_parser("run", help="delegate to the existing uploader, saving private stdout/stderr")
    run.add_argument("--uploader", type=Path, required=True)
    run.add_argument("--output", type=Path, required=True, help="new private stdout result path")
    run.add_argument("--log", type=Path, required=True, help="new private stderr log path")
    run.add_argument("args", nargs=argparse.REMAINDER, help="after --: publish IPA --receipt PATH, or link --build ID")
    args = parser.parse_args(argv)
    path = args.config.expanduser() if args.config else config_path(os.environ)
    try:
        if args.command == "import-config":
            import_config(path, sys.stdin.buffer)
            print("Publisher configuration imported; owner-only permissions set.")
            return 0
        env, origin = settings(path, os.environ)
        uploader = args.uploader.expanduser().resolve() if args.uploader else None
        if uploader and not uploader.is_file():
            raise ConfigError("Uploader not found. Set its path to distribution/scripts/builds.py in a current ccy.dev checkout.")
        if args.command == "check":
            print("Local publisher settings are complete. No credentials displayed; remote access and signing have not been tested.")
            if uploader:
                result = subprocess.run([sys.executable, "-c", "import boto3"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                if result.returncode:
                    raise ConfigError("Uploader dependency missing: install its requirements in this Python environment.")
                print("Uploader and Python dependency found.")
            return 0
        forwarded = args.args[1:] if args.args[:1] == ["--"] else args.args
        if not forwarded or forwarded[0] not in ("publish", "link") or any(x in forwarded for x in ("--server", "--app")):
            raise ConfigError("Supply publish or link arguments after --; origin and App are set by the adapter.")
        if args.output.resolve() == args.log.resolve():
            raise ConfigError("Use distinct new output and log paths.")
        with private_output(args.output) as out, private_output(args.log) as log:
            result = subprocess.run([sys.executable, str(uploader), "--server", origin, "--app", "gtmux", *forwarded], env=env, stdout=out, stderr=log)
        print("Uploader finished. Read the private result and log locally; do not publish installation URLs.")
        if result.returncode:
            print("Uploader failed; preserve its IPA and receipt before retrying.", file=sys.stderr)
        return result.returncode
    except (ConfigError, OSError) as error:
        message = str(error) if isinstance(error, ConfigError) else "Local file or process operation failed; check paths and permissions."
        print("Error: " + message, file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
