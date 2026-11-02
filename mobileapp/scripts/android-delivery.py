#!/usr/bin/env python3
"""Build and verify Android deliveries. No debug-signing fallback or embedded credentials."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import zipfile

from delivery_source import DeliveryError, validate_ci

ROOT = Path(__file__).resolve().parents[1]
PACKAGE = "com.gtmux.app"
BUNDLETOOL_VERSION = "1.18.3"
BUNDLETOOL_SHA256 = "a099cfa1543f55593bc2ed16a70a7c67fe54b1747bb7301f37fdfd6d91028e29"


def require(value, reason):
    if not value:
        raise DeliveryError(reason)


def sha(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def version_code(version, number):
    require(re.fullmatch(r"\d+\.\d+\.\d+", version), "Expected a semantic version")
    major, minor, patch = map(int, version.split("."))
    require(0 <= minor <= 99 and 0 <= patch <= 999 and 1 <= number <= 999, "Version/build outside the Android ordinal range")
    code = major * 100_000_000 + minor * 1_000_000 + patch * 1000 + number
    require(0 < code <= 2_100_000_000, "Version code outside the Play range")
    return code


def run(command, *, private_log=None, **kwargs):
    result = subprocess.run([str(x) for x in command], capture_output=True, **kwargs)
    if private_log:
        with Path(private_log).open("ab") as log:
            log.write(result.stdout + result.stderr)
    require(result.returncode == 0, "Android tool failed; inspect the private build log")
    return result.stdout.decode("utf-8", errors="replace")


def elf_alignment(data):
    require(data[:4] == b"\x7fELF" and len(data) >= 64 and data[4] == 2 and data[5] == 1, "Expected a little-endian 64-bit ELF")
    offset = struct.unpack_from("<Q", data, 32)[0]
    entry_size, count = struct.unpack_from("<HH", data, 54)
    require(entry_size >= 56 and count > 0 and offset + entry_size * count <= len(data), "Invalid ELF program table")
    loads = 0
    for index in range(count):
        position = offset + index * entry_size
        kind, _, file_offset, address, _, _, _, alignment = struct.unpack_from("<IIQQQQQQ", data, position)
        if kind == 1:
            loads += 1
            require(alignment >= 16384 and alignment & (alignment - 1) == 0 and file_offset % alignment == address % alignment, "Native library is not compatible with 16 KB pages")
    require(loads > 0, "ELF has no loadable segments")
    return loads


def inspect_zip(path, prefix):
    natives, js = {}, None
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        require(len(names) == len(set(names)), "Duplicate package entry")
        require(all(not name.startswith("/") and ".." not in Path(name).parts for name in names), "Unsafe package entry")
        for name in names:
            require(not name.endswith((".keystore", ".p12", ".p8", "service-account.json", ".agekey")), "Signing/relay secret found in package")
            if name.startswith(prefix + "lib/") and name.endswith(".so"):
                relative = name.removeprefix(prefix)
                require(relative.split("/")[1] in {"arm64-v8a", "x86_64"}, "Unexpected Android ABI")
                data = archive.read(name)
                elf_alignment(data)
                natives[relative] = hashlib.sha256(data).hexdigest()
            if name == prefix + "assets/index.android.bundle":
                data = archive.read(name)
                require(len(data) > 100_000, "Missing/empty bundled JavaScript")
                js = hashlib.sha256(data).hexdigest()
    require(natives and js, "Android package lacks native libraries or bundled JS")
    require({name.split('/')[1] for name in natives} == {"arm64-v8a", "x86_64"}, "Release must include both declared 64-bit Android ABIs")
    return {"native_libraries": natives, "js_sha256": js}


def verify(apk, aab, certificate, source, version, code, sdk, bundletool, output):
    require(re.fullmatch(r"[a-f0-9]{64}", certificate) and re.fullmatch(r"[a-f0-9]{40}", source), "Exact source/certificate required")
    tools = Path(sdk) / "build-tools" / "36.0.0"
    output.mkdir(parents=True, exist_ok=True, mode=0o700)
    log = output / "verification-private.log"
    signing = run([tools / "apksigner", "verify", "--verbose", "--print-certs", apk], private_log=log)
    found = re.findall(r"Signer #\d+ certificate SHA-256 digest: ([a-fA-F0-9]+)", signing)
    require(found == [certificate] and "Android Debug" not in signing, "APK signer differs from the pinned release certificate")
    run([tools / "zipalign", "-c", "-P", "16", "4", apk], private_log=log)
    badging = run([tools / "aapt", "dump", "badging", apk], private_log=log)
    require(f"package: name='{PACKAGE}'" in badging and f"versionCode='{code}'" in badging and f"versionName='{version}'" in badging, "APK package/version differs")
    require("application-debuggable" not in badging, "Release package is debuggable")
    require("sdkVersion:'24'" in badging and "targetSdkVersion:'36'" in badging, "Unexpected Android SDK contract")
    manifest = run([tools / "aapt2", "dump", "xmltree", apk, "--file", "AndroidManifest.xml"], private_log=log)
    require(source in manifest, "APK source provenance missing")
    apk_parts = inspect_zip(apk, "")
    require(sha(bundletool) == BUNDLETOOL_SHA256, "Bundletool differs from the pinned Google release")
    java = Path(os.environ["JAVA_HOME"]) / "bin"
    run([java / "java", "-jar", bundletool, "validate", "--bundle=" + str(aab)], private_log=log)
    run([java / "jarsigner", "-verify", aab], private_log=log)
    certificate_pem = run([java / "keytool", "-printcert", "-jarfile", aab, "-rfc"], private_log=log)
    import base64
    certificates = re.findall(r"-----BEGIN CERTIFICATE-----\s+([A-Za-z0-9+/=\s]+)-----END CERTIFICATE-----", certificate_pem)
    require(len(certificates) == 1 and hashlib.sha256(base64.b64decode(certificates[0])).hexdigest() == certificate, "AAB signing certificate differs")
    aab_manifest = run([java / "java", "-jar", bundletool, "dump", "manifest", "--bundle=" + str(aab), "--module=base"], private_log=log)
    require(f'package="{PACKAGE}"' in aab_manifest and f'android:versionCode="{code}"' in aab_manifest and source in aab_manifest, "AAB identity/provenance differs")
    require('android:debuggable="true"' not in aab_manifest, "AAB is debuggable")
    aab_parts = inspect_zip(aab, "base/")
    require(apk_parts == aab_parts, "APK/AAB native or JS bytes differ")
    receipt = {"package": PACKAGE, "source": source, "version": version, "version_code": code, "certificate_sha256": certificate,
               "apk": {"bytes": Path(apk).stat().st_size, "sha256": sha(apk)}, "aab": {"bytes": Path(aab).stat().st_size, "sha256": sha(aab)},
               "zip_16kb_aligned": True, "elf_16kb_aligned": True, **apk_parts,
               "device_installation": "not verified by package validation", "store_submission": False}
    (output / "VERIFICATION.json").write_text(json.dumps(receipt, indent=2) + "\n")
    return receipt


def build(args):
    require(re.fullmatch(r"[a-f0-9]{40}", args.source), "Exact source commit required")
    head = run(["git", "rev-parse", "HEAD"], cwd=ROOT).strip()
    require(head == args.source, "Checkout does not match the requested source")
    require(not run(["git", "status", "--porcelain", "--untracked-files=normal"], cwd=ROOT).strip(), "Delivery checkout must be clean")
    version = re.search(r"APP_VERSION = '([^']+)'", (ROOT / "src/version.ts").read_text()).group(1)
    version_code(version, args.number)
    notes = json.loads((ROOT / "release-notes/android.json").read_text())
    require(any(row.get("version") == version and row.get("en") and row.get("zh") for row in notes), "This Android version needs its own bilingual release notes")
    if not args.unsigned:
        for field in ["GTMUX_ANDROID_KEYSTORE", "GTMUX_ANDROID_STORE_PASSWORD", "GTMUX_ANDROID_KEY_ALIAS", "GTMUX_ANDROID_KEY_PASSWORD"]:
            require(bool(os.environ.get(field)), "Android release signing input is missing")
    require(args.output.is_dir() and not any(args.output.iterdir()), "Provide an existing empty private output directory")
    env = dict(os.environ, GTMUX_SOURCE_SHA=args.source)
    variant = "unsigned" if args.unsigned else "release"
    task = variant[0].upper() + variant[1:]
    run([ROOT / "android/gradlew", ":app:assemble" + task, ":app:bundle" + task, "-PgtmuxBuildNumber=" + str(args.number), "-PreactNativeArchitectures=arm64-v8a,x86_64", "--no-daemon", "--max-workers=2", "--console=plain"], cwd=ROOT / "android", env=env, private_log=args.output / "build-private.log")
    apks = list((ROOT / f"android/app/build/outputs/apk/{variant}").glob("*.apk"))
    require(len(apks) == 1, "Expected one complete APK")
    prefix = "gtmux-unsigned" if args.unsigned else "gtmux"
    shutil.copyfile(apks[0], args.output / (prefix + ".apk"))
    shutil.copyfile(ROOT / f"android/app/build/outputs/bundle/{variant}/app-{variant}.aab", args.output / (prefix + ".aab"))
    mapping = ROOT / f"android/app/build/outputs/mapping/{variant}/mapping.txt"
    require(mapping.is_file(), "Release obfuscation map missing")
    shutil.copyfile(mapping, args.output / "mapping.txt")
    sourcemap = ROOT / f"android/app/build/generated/sourcemaps/react/{variant}/index.android.bundle.map"
    symbols = ROOT / f"android/app/build/outputs/native-debug-symbols/{variant}/native-debug-symbols.zip"
    require(sourcemap.is_file() and symbols.is_file(), "Release JS map or native debug symbols missing")
    shutil.copyfile(sourcemap, args.output / "index.android.bundle.map")
    shutil.copyfile(symbols, args.output / "native-debug-symbols.zip")
    run(["git", "archive", "--format=tar.gz", "--output=" + str(args.output / "source.tar.gz"), args.source], cwd=ROOT)
    if args.unsigned:
        require(inspect_zip(args.output / "gtmux-unsigned.apk", "") == inspect_zip(args.output / "gtmux-unsigned.aab", "base/"), "Unsigned APK/AAB native or JS differ")
    records = [{"file":item.name,"bytes":item.stat().st_size,"sha256":sha(item)} for item in sorted(args.output.iterdir()) if item.is_file() and item.name != "build-private.log"]
    (args.output / "DELIVERY.json").write_text(json.dumps({"source":args.source,"version":version,"number":args.number,"signed":not args.unsigned,"files":records},indent=2)+"\n")


def sign(args):
    require(re.fullmatch(r"[a-f0-9]{40}", args.source), "Exact reviewed source required")
    manifest = args.input / "DELIVERY.json"
    require(manifest.is_file() and not manifest.is_symlink() and manifest.stat().st_size <= 1024 * 1024, "Bounded delivery receipt required")
    receipt = json.loads(manifest.read_text())
    require(isinstance(receipt, dict), "Object-shaped delivery receipt required")
    expected = {"gtmux-unsigned.apk", "gtmux-unsigned.aab", "mapping.txt", "index.android.bundle.map", "native-debug-symbols.zip", "source.tar.gz"}
    rows = receipt.get("files")
    require(isinstance(rows, list) and len(rows) == len(expected) and all(isinstance(row, dict) for row in rows), "Complete delivery inventory required")
    require({row.get("file") for row in rows} == expected, "Unexpected or missing delivery member")
    require(receipt.get("signed") is False and receipt.get("source") == args.source, "Exact unsigned source receipt required")
    require(args.output.is_dir() and not any(args.output.iterdir()), "Provide an existing empty signing output directory")
    for row in rows:
        require(Path(row["file"]).name == row["file"] and type(row.get("bytes")) is int and 0 < row["bytes"] <= 1024 ** 3 and re.fullmatch(r"[a-f0-9]{64}", str(row.get("sha256", ""))), "Unsafe delivery member")
        file = args.input / row["file"]
        require(file.is_file() and not file.is_symlink() and file.stat().st_size == row["bytes"] and sha(file) == row["sha256"], "Unsigned delivery file differs")
    apk, aab = args.input / "gtmux-unsigned.apk", args.input / "gtmux-unsigned.aab"
    require(inspect_zip(apk, "") == inspect_zip(aab, "base/"), "Unsigned JS/native bytes differ")
    with zipfile.ZipFile(aab) as bundle:
        require(not any(re.fullmatch(r"META-INF/[^/]+\.(?:RSA|DSA|EC|SF)", name, re.I) for name in bundle.namelist()), "Expected an unsigned AAB")
    for field in ["GTMUX_ANDROID_KEYSTORE", "GTMUX_ANDROID_STORE_PASSWORD", "GTMUX_ANDROID_KEY_ALIAS", "GTMUX_ANDROID_KEY_PASSWORD"]:
        require(bool(os.environ.get(field)), "Android signing input is missing")
    tools = Path(args.sdk) / "build-tools/36.0.0"
    java = Path(os.environ["JAVA_HOME"]) / "bin"
    log = args.output / "signing-private.log"
    aligned = args.output / "aligned.apk"
    run([tools / "zipalign", "-P", "16", "-f", "4", apk, aligned], private_log=log)
    run([tools / "apksigner", "sign", "--ks", os.environ["GTMUX_ANDROID_KEYSTORE"], "--ks-type", "PKCS12", "--ks-key-alias", os.environ["GTMUX_ANDROID_KEY_ALIAS"], "--ks-pass", "env:GTMUX_ANDROID_STORE_PASSWORD", "--key-pass", "env:GTMUX_ANDROID_KEY_PASSWORD", "--out", args.output / "gtmux.apk", aligned], private_log=log)
    aligned.unlink()
    shutil.copyfile(aab, args.output / "gtmux.aab")
    run([java / "jarsigner", "-keystore", os.environ["GTMUX_ANDROID_KEYSTORE"], "-storetype", "PKCS12", "-storepass:env", "GTMUX_ANDROID_STORE_PASSWORD", "-keypass:env", "GTMUX_ANDROID_KEY_PASSWORD", "-sigalg", "SHA256withRSA", "-digestalg", "SHA-256", args.output / "gtmux.aab", os.environ["GTMUX_ANDROID_KEY_ALIAS"]], private_log=log)
    for name in ["source.tar.gz", "mapping.txt", "index.android.bundle.map", "native-debug-symbols.zip", "DELIVERY.json"]:
        shutil.copyfile(args.input / name, args.output / name)
    (args.output / "SIGNING.json").write_text(json.dumps({"source":args.source,"version":receipt["version"],"number":receipt["number"],"mode":"trusted CI build; private local signing","unsigned_apk_sha256":sha(apk),"unsigned_aab_sha256":sha(aab),"apk_sha256":sha(args.output/"gtmux.apk"),"aab_sha256":sha(args.output/"gtmux.aab")},indent=2)+"\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    b = sub.add_parser("build")
    b.add_argument("--unsigned", action="store_true"); b.add_argument("--source", required=True); b.add_argument("--number", type=int, required=True); b.add_argument("--output", type=Path, required=True)
    local = sub.add_parser("sign")
    local.add_argument("--input", type=Path, required=True); local.add_argument("--output", type=Path, required=True)
    local.add_argument("--source", required=True); local.add_argument("--sdk", required=True)
    v = sub.add_parser("verify")
    for flag in ["apk", "aab", "certificate", "source", "version", "sdk", "bundletool"]: v.add_argument("--" + flag, required=True)
    v.add_argument("--code", type=int, required=True); v.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.command == "build": build(args)
    elif args.command == "sign": sign(args)
    else: verify(args.apk, args.aab, args.certificate, args.source, args.version, args.code, args.sdk, args.bundletool, args.output)


if __name__ == "__main__":
    try: main()
    except (DeliveryError, OSError, ValueError, zipfile.BadZipFile) as error:
        print("Android delivery failed: " + str(error), file=sys.stderr)
        sys.exit(1)
