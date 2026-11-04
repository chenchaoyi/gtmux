#!/usr/bin/env python3
"""Retrieve one exact-source encrypted CI result. GET-only; no decryption or publication."""
import argparse
import datetime
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import socket
import ssl
import stat
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile

from delivery_source import DeliveryError, validate_ci

REPO = "chenchaoyi/gtmux"
API = f"https://api.github.com/repos/{REPO}/"
LIMIT = 1024**3
PLATFORMS = {"ios": ("ios-test-delivery.yml", {"trusted-source", "delivery"}),
             "android": ("android-test-delivery.yml", {"preflight", "delivery"})}


def require(value, reason):
    if not value:
        raise DeliveryError(reason)


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, newurl):
        return None


class Transport:
    def __init__(self, token, attempts=3, pause=time.sleep):
        require(bool(token) and not any(c in token for c in "\r\n"), "GitHub authentication unavailable")
        require(1 <= attempts <= 3, "At most three GET attempts are allowed")
        self.token, self.attempts, self.pause = token, attempts, pause
        # Default CA and hostname verification remain enabled for every request.
        self.opener = urllib.request.build_opener(NoRedirect)

    def get(self, url, consume, *, authenticated=True, redirect=False):
        if authenticated:
            require(url.startswith(API), "Authentication is restricted to this repository API")
        else:
            target = urllib.parse.urlsplit(url)
            require(target.scheme == "https" and not target.username and not target.password
                    and target.port in (None, 443) and not target.fragment
                    and any((target.hostname or "").endswith(suffix) for suffix in
                            (".blob.core.windows.net", ".actions.githubusercontent.com")),
                    "Unexpected artifact download origin")
        headers = {"User-Agent": "gtmux-delivery-reader/1", "Accept": "application/vnd.github+json"}
        if authenticated:
            headers["Authorization"] = "Bearer " + self.token
        for attempt in range(self.attempts):
            try:
                with self.opener.open(urllib.request.Request(url, headers=headers), timeout=60) as response:
                    require(response.status == 200, "Unexpected HTTP result")
                    return consume(response)
            except urllib.error.HTTPError as error:
                if error.code == 302 and redirect:
                    location = error.headers.get("Location")
                    error.close()
                    return location
                retry = error.code in {429, 502, 503, 504}
                failure = f"HTTP {error.code}"
                error.close()
            except (urllib.error.URLError, TimeoutError, ConnectionError, ssl.SSLError, http.client.IncompleteRead):
                retry, failure = True, "transport failure"
            if not retry or attempt + 1 == self.attempts:
                # Never print signed download URLs, tokens, response bodies or raw exceptions.
                raise DeliveryError(f"Read failed ({failure}); successful build state is unchanged")
            self.pause(attempt + 1)

    def json(self, path):
        def read(response):
            data = response.read(2 * 1024**2 + 1)
            require(len(data) <= 2 * 1024**2, "API response exceeds the metadata budget")
            return json.loads(data)
        return self.get(API + path, read)


def validate(platform, source, run, attempt, artifact_id, metadata, jobs, artifact):
    workflow, required = PLATFORMS[platform]
    require(re.fullmatch(r"[0-9a-f]{40}", source) and min(run, attempt, artifact_id) > 0, "Exact source/run/attempt/artifact required")
    require(all(metadata.get(key) == value for key, value in {
        "id": run, "run_attempt": attempt, "head_sha": source, "head_branch": "main",
        "event": "workflow_dispatch", "path": ".github/workflows/" + workflow,
        "status": "completed", "conclusion": "success"}.items()), "Selected latest workflow attempt is not a successful exact-source delivery")
    require(metadata.get("repository", {}).get("full_name") == REPO, "Wrong source repository")
    rows = jobs.get("jobs", [])
    require(isinstance(rows, list) and len(rows) == jobs.get("total_count") == len(required)
            and {row.get("name") for row in rows} == required
            and all(row.get("status") == "completed" and row.get("conclusion") == "success"
                    and row.get("head_sha") == source and row.get("run_id") == run for row in rows),
            "Complete successful delivery jobs are required")
    require(artifact.get("id") == artifact_id and artifact.get("name") == f"{platform}-delivery-encrypted-{run}-{attempt}"
            and artifact.get("expired") is False
            and artifact.get("workflow_run", {}).get("id") == run
            and artifact["workflow_run"].get("head_sha") == source, "Artifact identity or expiry differs")
    require(type(artifact.get("size_in_bytes")) is int and 0 < artifact["size_in_bytes"] <= LIMIT
            and re.fullmatch(r"sha256:[0-9a-f]{64}", artifact.get("digest", "")), "Bounded artifact length and published SHA256 required")
    expiry = datetime.datetime.fromisoformat(artifact["expires_at"].replace("Z", "+00:00"))
    require(expiry > datetime.datetime.now(datetime.timezone.utc), "Artifact has expired")


def private_directory(folder):
    info = folder.lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.getuid()
            and info.st_mode & 0o077 == 0, "Use an existing owner-only output directory")


def bounded_download(response, destination, artifact):
    with destination.open("wb") as output:
        size = 0
        while data := response.read(1024 * 1024):
            size += len(data)
            require(size <= artifact["size_in_bytes"], "Download exceeds the published length")
            output.write(data)
    require(size == artifact["size_in_bytes"] and "sha256:" + digest(destination) == artifact["digest"],
            "Downloaded artifact length or SHA256 differs")


def install_private(temp, target):
    # Hard-link creation is atomic and refuses to overwrite any existing path/symlink.
    os.link(temp, target)


def fetch(platform, source, run, attempt, artifact_id, folder, transport):
    private_directory(folder)
    require(platform in PLATFORMS and re.fullmatch(r"[0-9a-f]{40}", source)
            and min(run, attempt, artifact_id) > 0, "Exact delivery selection required")
    member = f"{platform}-delivery.tar.gz.age"
    allowed = {"artifact.zip", member, "VERIFIED.json"}
    require(all(path.name in allowed and path.is_file() and not path.is_symlink()
                and path.stat().st_mode & 0o077 == 0 for path in folder.iterdir()),
            "Output contains unknown or unsafe files; use a new private directory")
    metadata = transport.json(f"actions/runs/{run}")
    jobs = transport.json(f"actions/runs/{run}/attempts/{attempt}/jobs?per_page=100")
    artifact = transport.json(f"actions/artifacts/{artifact_id}")
    validate(platform, source, run, attempt, artifact_id, metadata, jobs, artifact)
    ci = transport.json(f"actions/workflows/ci.yml/runs?head_sha={source}&event=push&per_page=100")
    require(isinstance(ci.get("workflow_runs"), list), "CI evidence missing")
    validate_ci(source, ci["workflow_runs"])
    archive = folder / "artifact.zip"
    reused = archive.exists()
    if reused:
        require(archive.stat().st_size == artifact["size_in_bytes"] and "sha256:" + digest(archive) == artifact["digest"],
                "Cached artifact differs; preserve it and select a fresh private output directory")
    else:
        with tempfile.NamedTemporaryFile(dir=folder, prefix=".download-") as temp:
            location = transport.get(API + f"actions/artifacts/{artifact_id}/zip", lambda response: None, redirect=True)
            require(isinstance(location, str), "Artifact redirect missing")
            transport.get(location, lambda response: bounded_download(response, Path(temp.name), artifact), authenticated=False)
            install_private(temp.name, archive)
    with zipfile.ZipFile(archive) as package:
        entries = package.infolist()
        require(len(entries) == 1 and entries[0].filename == member
                and stat.S_IFMT(entries[0].external_attr >> 16) in (0, stat.S_IFREG)
                and 20 < entries[0].file_size <= LIMIT and not entries[0].flag_bits & 1,
                "Expected one bounded regular encrypted result, with no extra archive members")
        with tempfile.NamedTemporaryFile(dir=folder, prefix=".encrypted-") as temp, package.open(entries[0]) as encrypted:
            with open(temp.name, "wb") as output:
                size = 0
                while data := encrypted.read(1024 * 1024):
                    size += len(data)
                    require(size <= entries[0].file_size, "Encrypted member exceeds its declared length")
                    output.write(data)
            require(size == entries[0].file_size, "Incomplete encrypted member")
            with open(temp.name, "rb") as stream:
                header = b"age-encryption.org/v1\n"
                require(stream.read(len(header)) == header, "Result is not an age encrypted file")
            encrypted_sha = digest(Path(temp.name))
            target = folder / member
            if target.exists():
                require(digest(target) == encrypted_sha, "Existing encrypted output differs")
            else:
                install_private(temp.name, target)
    receipt = {"platform": platform, "source": source, "run": run, "attempt": attempt,
               "artifact": artifact_id, "bytes": artifact["size_in_bytes"], "digest": artifact["digest"],
               "expires_at": artifact["expires_at"], "encrypted_sha256": encrypted_sha,
               "cache_reused": reused, "decrypted": False, "installation": "not checked"}
    with tempfile.NamedTemporaryFile(dir=folder, mode="w", prefix=".receipt-") as temp:
        json.dump(receipt, temp, indent=2); temp.write("\n"); temp.flush()
        path = folder / "VERIFIED.json"
        if path.exists():
            old = json.loads(path.read_text())
            require({k: v for k, v in old.items() if k != "cache_reused"} == {k: v for k, v in receipt.items() if k != "cache_reused"}, "Existing verification receipt differs")
        else:
            install_private(temp.name, path)
    return receipt


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform", choices=PLATFORMS, required=True)
    for field in ("run", "attempt", "artifact"):
        parser.add_argument("--" + field, required=True, type=int)
    parser.add_argument("--source", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--ipv4", action="store_true", help="Process-only IPv4; preserves normal TLS verification")
    args = parser.parse_args()
    os.umask(0o077)
    if args.ipv4:
        resolve = socket.getaddrinfo
        socket.getaddrinfo = lambda host, port, family=0, type=0, proto=0, flags=0: resolve(
            host, port, socket.AF_INET if family in (0, socket.AF_UNSPEC) else family, type, proto, flags)
    try:
        token = os.environ.get("GH_TOKEN") or subprocess.check_output(["gh", "auth", "token"], text=True, stderr=subprocess.DEVNULL).strip()
        result = fetch(args.platform, args.source, args.run, args.attempt, args.artifact, args.output, Transport(token))
        print(json.dumps(result, indent=2))
    except (DeliveryError, OSError, ValueError, KeyError, TypeError, zipfile.BadZipFile, subprocess.CalledProcessError) as error:
        reason = str(error) if isinstance(error, DeliveryError) else type(error).__name__
        print("Encrypted result retrieval failed: " + reason + ". No build, signing or publication action was taken.")
        raise SystemExit(1)


if __name__ == "__main__":
    main()
