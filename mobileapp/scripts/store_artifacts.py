"""Artifact contracts for unsigned Store captures; no credentials or ASC writes."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import struct
import subprocess
import tarfile

SCHEMA = 1
GROUPS = {slot + '-' + lang: names for slot, names in {
    'phone': ('00-demo-entry', '01-radar', '02-terminal-approval', '03-hq', '04-console', '05-usage', '06-servers'),
    'ipad': ('00-demo-entry', '01-split', '02-hq', '03-panes', '04-knowledge'),
}.items() for lang in ('en', 'zh')}
RENDER_FILES = ('frame-shots.mjs', 'frame-creative.mjs', 'render-store-creative.mjs', 'creative-layout.mjs',
                'shot-captions.json', 'creative-captions.json')
CREATIVE_SIZES = {'header-21x9.png': [3840, 1646], 'search-3x2.png': [3840, 2560]}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def save(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + '.tmp')
    temp.write_text(json.dumps(data, indent=2, sort_keys=True) + '\n')
    temp.replace(path)


def tree_hash(root):
    records = []
    for path in sorted(root.rglob('*')):
        if path.is_symlink():
            if not path.resolve().is_relative_to(root.resolve()) or not path.exists():
                raise ValueError('Artifact contains an external or broken symlink')
            value = 'link:' + str(path.readlink())
        elif path.is_file():
            value = digest(path)
        else:
            continue
        records.append([str(path.relative_to(root)), value, path.lstat().st_mode & 0o111])
    if not records:
        raise ValueError('Empty application artifact')
    return hashlib.sha256(json.dumps(records).encode()).hexdigest()


def png_record(path, root):
    data = path.read_bytes()
    if len(data) < 24 or data[:8] != b'\x89PNG\r\n\x1a\n':
        raise ValueError('Invalid capture PNG: ' + path.name)
    return {'file': str(path.relative_to(root)), 'sha256': digest(path),
            'bytes': len(data), 'size': list(struct.unpack('>II', data[16:24]))}


def verify_records(root, records):
    if not records:
        raise ValueError('No validated images')
    names = set()
    for record in records:
        name = record['file']
        path = root / name
        if name in names or Path(name).is_absolute() or '..' in Path(name).parts or path.is_symlink() or not path.resolve().is_relative_to(root.resolve()):
            raise ValueError('Unsafe or duplicate artifact path')
        names.add(name)
        if not path.is_file() or png_record(path, root) != record:
            raise ValueError('Capture artifact content differs: ' + name)


def build_manifest(app, tag, commit, xcode, arch):
    info = plistlib.loads((app / 'Info.plist').read_bytes())
    if not re.fullmatch(r'v\d+\.\d+\.\d+', tag) or not re.fullmatch(r'[a-f0-9]{40}', commit):
        raise ValueError('Need a stable tag and exact source commit')
    if info.get('CFBundleIdentifier') != 'com.gtmux.app' or info.get('CFBundleShortVersionString') != tag[1:] or info.get('CFBundleSupportedPlatforms') != ['iPhoneSimulator']:
        raise ValueError('Artifact must be the requested Simulator application')
    if (app / 'embedded.mobileprovision').exists() or (app / '_CodeSignature').exists():
        raise ValueError('Only unsigned simulator applications may be public artifacts')
    return {'schema': SCHEMA, 'release_tag': tag, 'source_commit': commit, 'xcode': xcode,
            'arch': arch, 'app_sha256': tree_hash(app), 'unsigned_simulator_only': True}


def verify_build(app, manifest, tag, commit, xcode, arch):
    if manifest != build_manifest(app, tag, commit, xcode, arch):
        raise ValueError('Simulator artifact source, toolchain, architecture or content differs; build again')


def verify_run(run, repository, workflow_id):
    if run.get('repository', {}).get('full_name') != repository or run.get('head_repository', {}).get('full_name') != repository or run.get('workflow_id') != workflow_id or run.get('event') != 'workflow_dispatch' or run.get('head_branch') != 'main' or run.get('status') != 'completed':
        raise ValueError('Reuse requires a completed manual run of this workflow on this repository main')


def load_checkpoint(output, context=None):
    path = output / 'checkpoint.json'
    if not path.exists():
        return {'schema': SCHEMA, 'context': context, 'groups': {}}
    data = json.loads(path.read_text())
    if data.get('schema') != SCHEMA or not isinstance(data.get('context'), dict) or not isinstance(data.get('groups'), dict) or not set(data['groups']).issubset(GROUPS):
        raise ValueError('Invalid capture checkpoint')
    for key, records in data['groups'].items():
        expected = {f'raw/{key}/{name}.png' for name in GROUPS[key]}
        if {r['file'] for r in records} != expected:
            raise ValueError('Incomplete group in capture checkpoint')
        verify_records(output, records)
    if context is not None and data['context'] != context:
        # Validate old bytes first, then invalidate capture completion. The unsigned
        # application can still be reused independently of changed harness inputs.
        return {'schema': SCHEMA, 'context': context, 'groups': {}}
    return data


def checkpoint_group(output, context, key, raw, origin=None):
    data = load_checkpoint(output, context)
    records = [png_record(raw / (name + '.png'), raw) for name in GROUPS[key]]
    expected = [2752, 2064] if key.startswith('ipad-') else [1320, 2868]
    if any(r['size'] != expected for r in records) or len({r['sha256'] for r in records}) != len(records):
        raise ValueError('Capture group has wrong orientation/size or duplicate screens')
    for record in records:
        target = output / 'raw' / key / record['file']
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(raw / record['file'], target)
    if origin is not None:
        data.setdefault('origins', {})[key] = origin
    data['groups'][key] = [png_record(output / 'raw' / key / (name + '.png'), output) for name in GROUPS[key]]
    save(output / 'checkpoint.json', data)
    return data


def complete_checkpoint(output, tag, commit):
    data = load_checkpoint(output)
    if set(data['groups']) != set(GROUPS) or data['context'].get('release_tag') != tag or data['context'].get('source_commit') != commit:
        raise ValueError('Rendering requires all four verified capture groups for the exact release')
    return data


def render(source, renderer, captures, output, tag, commit, render_commit):
    data = complete_checkpoint(captures, tag, commit)
    if output.exists():
        raise ValueError('Use a fresh render output directory')
    render_hashes = {name: digest(renderer / name) for name in RENDER_FILES}
    captions = json.loads((renderer / 'shot-captions.json').read_text())
    for key in GROUPS:
        locale = key.replace('phone-', '')
        entries = captions[locale]
        if len(entries) != (4 if key.startswith('ipad') else 7):
            raise ValueError('Screenshot plan must contain seven phone/four iPad entries per locale')
        for entry in entries:
            if entry['file'] not in GROUPS[key] and not (key.startswith('phone-') and entry['file'] == '02-lockscreen'):
                raise ValueError('Caption references an uncaptured screen')
    output.mkdir(parents=True)
    for key in GROUPS:
        destination = output / 'raw' / key.replace('phone-', '')
        destination.mkdir(parents=True)
        for record in data['groups'][key]:
            shutil.copyfile(captures / record['file'], destination / Path(record['file']).name)
    for language, locale in (('en', 'en-US'), ('zh', 'zh-Hans')):
        phone, ipad = output / 'raw' / language, output / 'raw' / ('ipad-' + language)
        # Widget definition and renderer stay tied to the released application.
        subprocess.run(['node', 'scripts/render-lockscreen.mjs', '--lang', language, '--out', str(phone)], cwd=source, check=True, timeout=180)
        for slot, raw in (('phone', phone), ('ipad', ipad)):
            options = ['--prefix', 'ipad-'] if slot == 'ipad' else []
            subprocess.run(['node', str(renderer / 'frame-shots.mjs'), '--slot', slot, '--in', str(raw),
                            '--lang', ('ipad-' if slot == 'ipad' else '') + language,
                            '--out', str(output / 'screenshots' / locale), *options], check=True, timeout=300)
        subprocess.run(['node', str(renderer / 'frame-creative.mjs'), '--lang', language, '--ipad', str(ipad),
                        '--phone', str(phone), '--out', str(output / 'creative' / locale)], check=True, timeout=180)
    images = [png_record(path, output) for path in sorted(output.rglob('*.png'))]
    for prefix, count, size in [('screenshots/', 22, None), ('creative/', 4, None)]:
        group = [r for r in images if r['file'].startswith(prefix)]
        if len(group) != count:
            raise ValueError('Incomplete rendered ' + prefix)
        for record in group:
            expected = CREATIVE_SIZES.get(Path(record['file']).name) if prefix == 'creative/' else (
                [2752, 2064] if Path(record['file']).name.startswith('ipad-') else [1320, 2868])
            if record['size'] != expected:
                raise ValueError('Wrong rendered dimensions')
    save(output / 'manifest.json', {**data['context'], 'schema': SCHEMA, 'render_commit': render_commit,
                                   'render_files': render_hashes, 'capture_checkpoint_sha256': digest(captures / 'checkpoint.json'),
                                   'capture_attempts': data.get('attempts', []), 'capture_origins': data.get('origins', {}),
                                   'images': images, 'visual_review_required': True, 'submitted': False})


def fetch_artifact(root, run_id, kind, repository, optional=False):
    if not re.fullmatch(r"[1-9][0-9]*", str(run_id)) or kind not in ("simulator", "checkpoint"):
        raise ValueError("Invalid reuse run or artifact kind")
    def api(path):
        return json.loads(subprocess.check_output(["gh", "api", "repos/" + repository + path], text=True, timeout=60))
    workflow = api("/actions/workflows/ios-store-assets.yml")
    run = api("/actions/runs/" + str(run_id))
    verify_run(run, repository, workflow["id"])
    page = api("/actions/runs/" + str(run_id) + "/artifacts?per_page=100")
    if page.get("total_count", 0) > 100:
        raise ValueError("Too many reuse artifacts; select a newer run")
    candidates = [a for a in page["artifacts"] if a["name"].startswith("store-" + kind + "-")]
    artifact = max(candidates, key=lambda a: a["id"], default=None)
    if artifact is None and optional:
        print("No completed " + kind + " artifact; this stage will run fresh", flush=True)
        return
    if artifact is None or artifact.get("expired"):
        raise ValueError("Reuse artifact is missing or expired; start a fresh capture")
    root.mkdir(parents=True, exist_ok=True)
    subprocess.run(["gh", "run", "download", str(run_id), "--repo", repository, "--name", artifact["name"], "--dir", str(root)], check=True, timeout=300)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('pack-build', 'unpack-build', 'verify-build', 'verify-run', 'render', 'fetch'))
    parser.add_argument('--run')
    parser.add_argument('--kind')
    parser.add_argument('--optional', action='store_true')
    parser.add_argument('--app', type=Path)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--tag')
    parser.add_argument('--commit')
    parser.add_argument('--xcode')
    parser.add_argument('--arch')
    parser.add_argument('--repository')
    parser.add_argument('--workflow-id', type=int)
    parser.add_argument('--source', type=Path)
    parser.add_argument('--renderer', type=Path)
    parser.add_argument('--captures', type=Path)
    parser.add_argument('--render-commit')
    args = parser.parse_args()
    if args.action == 'fetch':
        fetch_artifact(args.root, args.run, args.kind, args.repository or os.environ['GITHUB_REPOSITORY'], args.optional)
    elif args.action == 'verify-run':
        verify_run(json.loads((args.root / 'run.json').read_text()), args.repository, args.workflow_id)
    elif args.action == 'render':
        render(args.source.resolve(), args.renderer.resolve(), args.captures.resolve(), args.root.resolve(), args.tag, args.commit, args.render_commit)
    else:
        manifest_path = args.root / 'build.json'
        if args.action == 'pack-build':
            manifest = build_manifest(args.app, args.tag, args.commit, args.xcode, args.arch)
            args.root.mkdir(parents=True, exist_ok=True)
            with tarfile.open(args.root / 'simulator-app.tar.gz', 'w:gz') as archive:
                archive.add(args.app, arcname='gtmux.app')
            save(manifest_path, manifest)
        else:
            if args.action == 'unpack-build':
                with tarfile.open(args.root / 'simulator-app.tar.gz') as archive:
                    if any(Path(m.name).is_absolute() or '..' in Path(m.name).parts or not (m.name == 'gtmux.app' or m.name.startswith('gtmux.app/')) for m in archive):
                        raise ValueError('Unexpected path in simulator archive')
                    archive.extractall(args.root, filter='data')
            verify_build(args.root / 'gtmux.app', json.loads(manifest_path.read_text()), args.tag, args.commit, args.xcode, args.arch)


if __name__ == '__main__':
    main()
