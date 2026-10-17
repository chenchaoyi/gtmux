#!/usr/bin/env python3
"""Verify an App Store IPA against its archive/original before any upload."""
import argparse
import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import stat
import subprocess
import tempfile
import zipfile

_spec = importlib.util.spec_from_file_location('delivery', Path(__file__).with_name('hosted-ios-delivery.py'))
_delivery = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_delivery)
BUNDLES = set(_delivery.BUNDLES)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def command(args):
    result = subprocess.run(args, capture_output=True, timeout=120)
    if result.returncode:
        raise ValueError('Package verification command failed: ' + args[0])
    return result.stdout


def unpack(ipa, target):
    with zipfile.ZipFile(ipa) as archive:
        if sum(i.file_size for i in archive.infolist()) > 2 * 1024**3:
            raise ValueError('IPA exceeds the verification size limit')
        for member in archive.infolist():
            path = Path(member.filename)
            if path.is_absolute() or '..' in path.parts or stat.S_ISLNK(member.external_attr >> 16):
                raise ValueError('Unsafe IPA path or symlink')
        archive.extractall(target)
    return application(target / 'Payload')


def application(folder):
    apps = list(folder.glob('*.app'))
    if len(apps) != 1:
        raise ValueError('Expected exactly one application')
    return apps[0]


def packages(app):
    result = {}
    for path in [app, *list((app / 'PlugIns').glob('*.appex'))]:
        info = plistlib.loads((path / 'Info.plist').read_bytes())
        bundle = info.get('CFBundleIdentifier')
        if bundle not in BUNDLES or bundle in result:
            raise ValueError('Unexpected or duplicate app/extension identity')
        result[bundle] = (path, info)
    if set(result) != BUNDLES:
        raise ValueError('Missing required extension')
    return result


def signed(path, run=command):
    run(['codesign', '--verify', '--deep', '--strict', str(path)])
    return _delivery.der_entitlements(run(['codesign', '-d', '--entitlements', '-', '--der', str(path)]))


def check_entitlements(original, candidate, bundle, team):
    expected = dict(original)
    expected['beta-reports-active'] = True
    if candidate != expected:
        raise ValueError('Signed entitlement drift: ' + bundle)
    if candidate.get('application-identifier') != team + '.' + bundle or candidate.get('com.apple.developer.team-identifier') != team or candidate.get('get-task-allow') is not False:
        raise ValueError('Invalid Store identity or debugger permission: ' + bundle)
    if bundle == 'com.gtmux.app' and candidate.get('aps-environment') != 'production':
        raise ValueError('Store app must use production APNs')


def check_profile(profile, bundle, team, now=None):
    now = now or datetime.datetime.now(datetime.timezone.utc)
    ent = profile.get('Entitlements', {})
    expiry = profile.get('ExpirationDate')
    if not isinstance(expiry, datetime.datetime) or expiry.replace(tzinfo=datetime.timezone.utc) <= now:
        raise ValueError('Missing/expired Store profile')
    if profile.get('TeamIdentifier') != [team] or ent.get('application-identifier') != team + '.' + bundle or ent.get('com.apple.developer.team-identifier') != team:
        raise ValueError('Store profile identity mismatch')
    if 'ProvisionedDevices' in profile or profile.get('ProvisionsAllDevices') or ent.get('get-task-allow') is not False or ent.get('beta-reports-active') is not True:
        raise ValueError('Expected App Store distribution profile')
    if bundle == 'com.gtmux.app' and ent.get('aps-environment') != 'production':
        raise ValueError('Store profile must authorize production APNs')


def uuids(binary, run=command):
    result = sorted(re.findall(r'UUID: ([0-9A-Fa-f-]+) \(([^)]+)\)', run(['dwarfdump', '--uuid', str(binary)]).decode()))
    if not result:
        raise ValueError('No native UUID evidence')
    return result


def resources(app, bundle_packages, run=command):
    # Resigning changes signature blobs, embedded profiles, executable signature
    # regions and only the declared build number in package Info.plists.
    info_paths = {p / 'Info.plist' for p, _ in bundle_packages.values()}
    binaries = {p / i['CFBundleExecutable'] for p, i in bundle_packages.values()}
    result = {}
    for path in sorted(app.rglob('*')):
        if path.is_symlink():
            raise ValueError('Unexpected package symlink')
        if not path.is_file() or '_CodeSignature' in path.parts or path.name == 'embedded.mobileprovision':
            continue
        if path in binaries or path.read_bytes()[:4] in (b'\xcf\xfa\xed\xfe', b'\xfe\xed\xfa\xcf', b'\xca\xfe\xba\xbe', b'\xbe\xba\xfe\xca'):
            # Compare executable content after removing only signatures from private
            # copies. Frameworks are re-signed on export as well as the three targets.
            with tempfile.TemporaryDirectory(prefix='gtmux-code-compare-') as folder:
                copy = Path(folder) / path.name
                shutil.copyfile(path, copy)
                run(['codesign', '--remove-signature', str(copy)])
                value = sha(copy)
        elif path in info_paths:
            info = plistlib.loads(path.read_bytes())
            info.pop('CFBundleVersion', None)
            value = hashlib.sha256(plistlib.dumps(info, sort_keys=True)).hexdigest()
        else:
            value = sha(path)
        result[str(path.relative_to(app))] = value
    return result


def verify(ipa, baseline, version, build, team, run=command):
    if not re.fullmatch(r'\d+\.\d+\.\d+', version) or not re.fullmatch(r'\d+(?:\.\d+){0,2}', build) or not re.fullmatch(r'[A-Z0-9]{10}', team):
        raise ValueError('Invalid version, build or team')
    with tempfile.TemporaryDirectory(prefix='gtmux-store-verify-') as tmp:
        root = Path(tmp)
        inputs = [ipa] + ([] if baseline.is_dir() else [baseline])
        required = 256 * 1024**2
        for item in inputs:
            with zipfile.ZipFile(item) as archive:
                sizes = [entry.file_size for entry in archive.infolist()]
                required += sum(sizes) + max(sizes, default=0)
        if shutil.disk_usage(root).free < required:
            raise ValueError('Not enough disk space for private verification copies')
        candidate = unpack(ipa, root / 'candidate')
        archive = baseline if baseline.is_dir() and baseline.suffix == '.xcarchive' else None
        original = application(archive / 'Products/Applications') if archive else unpack(baseline, root / 'original')
        originals, candidates = packages(original), packages(candidate)
        if resources(original, originals, run) != resources(candidate, candidates, run):
            raise ValueError('Original package resources or Info fields changed')
        symbols = {}
        if archive:
            for path in (archive / 'dSYMs').glob('*.dSYM/Contents/Resources/DWARF/*'):
                for uuid, arch in uuids(path, run):
                    symbols[(uuid, arch)] = True
        checks = []
        for bundle in sorted(BUNDLES):
            path, info = candidates[bundle]
            original_path, original_info = originals[bundle]
            if info.get('CFBundleShortVersionString') != version or info.get('CFBundleVersion') != build:
                raise ValueError('App/extension version or build mismatch')
            original_ent = signed(original_path, run)
            candidate_ent = signed(path, run)
            check_entitlements(original_ent, candidate_ent, bundle, team)
            profile = plistlib.loads(run(['security', 'cms', '-D', '-i', str(path / 'embedded.mobileprovision')]))
            check_profile(profile, bundle, team)
            native = uuids(path / info['CFBundleExecutable'], run)
            if native != uuids(original_path / original_info['CFBundleExecutable'], run):
                raise ValueError('Original native UUIDs changed')
            if archive and any(tuple(pair) not in symbols for pair in native):
                raise ValueError('Archive lacks matching app/extension dSYM')
            checks.append({'bundle': bundle, 'native_uuids': native})
        return {'schema': 1, 'version': version, 'build': build, 'ipa_sha256': sha(ipa),
                'baseline': 'archive' if archive else 'original-ipa', 'matching_symbols': bool(archive),
                'packages': checks, 'verified_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                'upload_allowed': True, 'submitted': False}


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ipa', type=Path, required=True)
    parser.add_argument('--baseline', type=Path, required=True, help='Original signed IPA or canonical .xcarchive')
    parser.add_argument('--version', required=True)
    parser.add_argument('--build', required=True)
    parser.add_argument('--team', required=True)
    parser.add_argument('--receipt', type=Path, required=True)
    args = parser.parse_args()
    result = verify(args.ipa, args.baseline, args.version, args.build, args.team)
    args.receipt.parent.mkdir(parents=True, exist_ok=True)
    temp = args.receipt.with_suffix('.tmp')
    with temp.open('w', opener=lambda path, flags: os.open(path, flags, 0o600)) as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    temp.replace(args.receipt)
    print('Store package verified before upload: app and both extensions; private receipt saved.', flush=True)


if __name__ == '__main__':
    main()
