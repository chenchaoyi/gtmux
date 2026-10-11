#!/usr/bin/env python3
"""Private runner setup and verification for the opt-in iOS delivery workflow."""
import argparse
import base64
import datetime
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import sys
import tarfile

BUNDLES = {
    "com.gtmux.app": "IOS_PROFILE_APP",
    "com.gtmux.app.widget": "IOS_PROFILE_WIDGET",
    "com.gtmux.app.notificationservice": "IOS_PROFILE_NOTIFICATION",
}
SIGNING_FIELDS = ("IOS_CERT_P12", "IOS_CERT_PASSWORD", *BUNDLES.values())


class DeliveryError(Exception):
    pass


def run(command, log, **kwargs):
    # Child output may contain profiles, signing identities or bearer links.
    with log.open("ab") as output:
        result = subprocess.run(command, stdout=output, stderr=output, **kwargs)
    if result.returncode:
        raise DeliveryError("Runner command failed; inspect encrypted diagnostics.")


def validate_ci(commit, ci_runs):
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise DeliveryError("Use an exact commit ID.")
    runs = [r for r in ci_runs if r.get("head_sha") == commit and r.get("head_branch") == "main" and r.get("event") == "push"]
    latest = max(runs, key=lambda r: (r.get("run_number", 0), r.get("run_attempt", 0)), default={})
    if latest.get("status") != "completed" or latest.get("conclusion") != "success":
        raise DeliveryError("Latest main CI for this exact commit must pass first.")


def validate_release(tag, release, commit, ci_runs):
    if not re.fullmatch(r"v\d+\.\d+\.\d+", tag):
        raise DeliveryError("Use a stable release tag, not a branch or arbitrary ref.")
    if release.get("tag_name") != tag or release.get("draft") or release.get("prerelease"):
        raise DeliveryError("The release is not a published stable release.")
    validate_ci(commit, ci_runs)


def profile_check(profile, bundle, team, now=None):
    now = now or datetime.datetime.now(datetime.timezone.utc)
    expiry = profile.get("ExpirationDate")
    if not isinstance(expiry, datetime.datetime) or expiry.replace(tzinfo=datetime.timezone.utc) <= now:
        raise DeliveryError("An Ad Hoc profile is expired or lacks an expiry.")
    ent = profile.get("Entitlements", {})
    if profile.get("TeamIdentifier") != [team] or ent.get("com.apple.developer.team-identifier") != team:
        raise DeliveryError("Ad Hoc profile team does not match the configured team.")
    if ent.get("application-identifier") != f"{team}.{bundle}":
        raise DeliveryError("Ad Hoc profile bundle identity does not match.")
    devices = profile.get("ProvisionedDevices")
    if not isinstance(devices, list) or not devices or any(not isinstance(d, str) or not d for d in devices) or profile.get("ProvisionsAllDevices") or ent.get("get-task-allow") is not False:
        raise DeliveryError("Use registered-device Ad Hoc distribution profiles.")
    if bundle == "com.gtmux.app" and ent.get("aps-environment") != "production":
        raise DeliveryError("The main Ad Hoc profile must use production APNs.")
    uuid = profile.get("UUID", "")
    if not re.fullmatch(r"[0-9A-Fa-f-]{36}", uuid):
        raise DeliveryError("Profile UUID is invalid.")
    return set(devices), uuid


def preflight(root, environ):
    missing = [name for name in SIGNING_FIELDS + ("IOS_TEAM_ID", "IOS_RESULT_RECIPIENT", "IOS_PUBLISHER_SSH_KEY") if not environ.get(name)]
    if missing:
        raise DeliveryError("Missing maintainer setup: " + ", ".join(missing))
    if not re.fullmatch(r"[A-Z0-9]{10}", environ["IOS_TEAM_ID"]):
        raise DeliveryError("Invalid Apple team ID.")
    if not re.fullmatch(r"age1[023456789acdefghjklmnpqrstuvwxyz]{58}", environ["IOS_RESULT_RECIPIENT"]):
        raise DeliveryError("Configure a native age public recipient, not a private identity.")
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    (root / "logs").mkdir(mode=0o700, exist_ok=True)
    # Confirm the recipient checksum before importing signing material/building.
    run(["age", "-r", environ["IOS_RESULT_RECIPIENT"], "-o", str(root / "recipient-check.age"), "/dev/null"], root / "logs/setup.log")


def setup_signing(root, environ):
    signing = root / "signing"
    signing.mkdir(mode=0o700)
    log = root / "logs/setup.log"
    profiles = {}
    common_devices = None
    for bundle, key in BUNDLES.items():
        path = signing / (key + ".mobileprovision")
        path.write_bytes(base64.b64decode(environ[key], validate=True))
        decoded = signing / (key + ".plist")
        with decoded.open("wb") as output, log.open("ab") as errors:
            result = subprocess.run(["security", "cms", "-D", "-i", str(path)], stdout=output, stderr=errors)
        if result.returncode:
            raise DeliveryError("Could not decode an Ad Hoc profile.")
        devices, uuid = profile_check(plistlib.loads(decoded.read_bytes()), bundle, environ["IOS_TEAM_ID"])
        common_devices = devices if common_devices is None else common_devices & devices
        profiles[bundle] = uuid
        destination = Path.home() / "Library/MobileDevice/Provisioning Profiles"
        destination.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, destination / (uuid + ".mobileprovision"))
    if not common_devices:
        raise DeliveryError("App and extension profiles have no common registered test device.")
    (signing / "profiles.json").write_text(json.dumps(profiles))
    (signing / "certificate.p12").write_bytes(base64.b64decode(environ["IOS_CERT_P12"], validate=True))
    password = os.urandom(32).hex()
    keychain = str(signing / "delivery.keychain-db")
    (signing / "keychain-password").write_text(password)
    run(["security", "create-keychain", "-p", password, keychain], log)
    run(["security", "set-keychain-settings", "-lut", "21600", keychain], log)
    run(["security", "unlock-keychain", "-p", password, keychain], log)
    run(["security", "import", str(signing / "certificate.p12"), "-k", keychain, "-P", environ["IOS_CERT_PASSWORD"], "-T", "/usr/bin/codesign", "-T", "/usr/bin/security"], log)
    run(["security", "set-key-partition-list", "-S", "apple-tool:,apple:,codesign:", "-s", "-k", password, keychain], log)
    run(["security", "list-keychains", "-d", "user", "-s", keychain], log)
    options = {"method": "release-testing", "signingStyle": "manual", "signingCertificate": "Apple Distribution", "teamID": environ["IOS_TEAM_ID"], "provisioningProfiles": profiles, "destination": "export", "manageAppVersionAndBuildNumber": False}
    (signing / "ExportOptions.plist").write_bytes(plistlib.dumps(options))


def verify_export(root, version, build, team):
    ipas = list((root / "export").glob("*.ipa"))
    if len(ipas) != 1:
        raise DeliveryError("Expected exactly one exported IPA.")
    unpack = root / "verify"
    run(["ditto", "-x", "-k", str(ipas[0]), str(unpack)], root / "logs/verify.log")
    apps = list((unpack / "Payload").glob("*.app"))
    if len(apps) != 1:
        raise DeliveryError("Expected one main App in the IPA.")
    packages = [apps[0], *list((apps[0] / "PlugIns").glob("*.appex"))]
    bundles = set()
    common_devices = None
    for package in packages:
        info = plistlib.loads((package / "Info.plist").read_bytes())
        bundle = info.get("CFBundleIdentifier")
        if bundle not in BUNDLES or bundle in bundles or info.get("CFBundleShortVersionString") != version or info.get("CFBundleVersion") != build:
            raise DeliveryError("Exported App/extension identity or stamped version differs.")
        bundles.add(bundle)
        run(["codesign", "--verify", "--strict", str(package)], root / "logs/verify.log")
        with (root / "signing/export-profile.plist").open("wb") as out, (root / "logs/verify.log").open("ab") as err:
            result = subprocess.run(["security", "cms", "-D", "-i", str(package / "embedded.mobileprovision")], stdout=out, stderr=err)
        if result.returncode:
            raise DeliveryError("Could not verify exported provisioning profile.")
        devices, _ = profile_check(plistlib.loads((root / "signing/export-profile.plist").read_bytes()), bundle, team)
        common_devices = devices if common_devices is None else common_devices & devices
        signed = root / "signing/signed-entitlements.plist"
        with signed.open("wb") as out, (root / "logs/verify.log").open("ab") as err:
            result = subprocess.run(["codesign", "-d", "--entitlements", "-", str(package)], stdout=out, stderr=err)
        if result.returncode:
            raise DeliveryError("Could not verify signed entitlements.")
        ent = plistlib.loads(signed.read_bytes())
        if ent.get("get-task-allow") is not False or ent.get("application-identifier") != f"{team}.{bundle}" or (bundle == "com.gtmux.app" and ent.get("aps-environment") != "production"):
            raise DeliveryError("Exported signed entitlements are not the expected distribution entitlements.")
    if bundles != set(BUNDLES) or not common_devices:
        raise DeliveryError("Export is missing an extension or a common registered device.")
    return ipas[0]


def recovery_files(root):
    # Allowlist: never archive signing secrets, provisioning files or source checkouts.
    return [p for p in root.rglob("*") if p.is_file() and not p.is_symlink() and (p.parent == root / "logs" or (p.parent == root / "export" and p.suffix == ".ipa") or (p.parent == root and p.name in ("receipt.json", "publication.json", "publication.log")))]


def seal(root, destination, recipient):
    archive = root / "recovery.tar.gz"
    with tarfile.open(archive, "w:gz") as output:
        for path in recovery_files(root):
            output.add(path, arcname=str(path.relative_to(root)), recursive=False)
    run(["age", "-r", recipient, "-o", str(destination), str(archive)], root / "logs/encryption.log")
    archive.unlink()


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("release", "preview", "preflight", "signing", "verify", "seal", "cleanup"))
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--tag")
    parser.add_argument("--commit")
    parser.add_argument("--version")
    parser.add_argument("--build")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    root = args.root
    if args.action == "release":
        validate_release(args.tag, json.loads((root / "release.json").read_text()), args.commit, json.loads((root / "ci.json").read_text())["workflow_runs"])
    elif args.action == "preview":
        validate_ci(args.commit, json.loads((root / "ci.json").read_text())["workflow_runs"])
    elif args.action == "preflight":
        preflight(root, os.environ)
    elif args.action == "signing":
        setup_signing(root, os.environ)
    elif args.action == "verify":
        verify_export(root, args.version, args.build, os.environ["IOS_TEAM_ID"])
    elif args.action == "seal":
        seal(root, args.output, os.environ["IOS_RESULT_RECIPIENT"])
    elif args.action == "cleanup":
        keychain = root / "signing/delivery.keychain-db"
        if keychain.exists():
            run(["security", "delete-keychain", str(keychain)], root / "logs/cleanup.log")
        if root.exists():
            shutil.rmtree(root)


if __name__ == "__main__":
    try:
        main()
    except DeliveryError as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
    except (OSError, ValueError, TypeError, KeyError, plistlib.InvalidFileException):
        # Avoid echoing malformed credential contents or child process arguments.
        print("iOS delivery step failed. Check setup and encrypted diagnostics; no private output was printed.", file=sys.stderr)
        sys.exit(1)
