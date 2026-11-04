#!/usr/bin/env python3
"""Resolve an immutable product tag separately from a reviewed recovery definition."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'mobileapp/scripts'))
from delivery_source import DeliveryError, validate_ci

REPO = 'chenchaoyi/gtmux'
WORKFLOW = '.github/workflows/release.yml'
SHA = re.compile(r'[0-9a-f]{40}')
TAG = re.compile(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)')


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def gh(path):
    return json.loads(subprocess.check_output(['gh', 'api', f'repos/{REPO}/{path}']))


def recovery(tag, definition, api=gh, *, before_build=True):
    """Read-only proof; a CLI success never substitutes for Mac signing or stapling."""
    require(TAG.fullmatch(tag) and SHA.fullmatch(definition), 'Exact stable tag and definition required')
    if before_build:
        require(api('git/ref/heads/main')['object']['sha'] == definition, 'Recovery must use current reviewed main')
    validate_ci(definition, api(f'actions/workflows/ci.yml/runs?head_sha={definition}&event=push&per_page=100')['workflow_runs'])
    ref = api('git/ref/tags/' + tag)['object']
    require(ref.get('type') == 'tag' and SHA.fullmatch(ref.get('sha', '')), 'Annotated product tag required')
    annotated = api('git/tags/' + ref['sha'])
    product = annotated.get('object', {})
    require(annotated.get('tag') == tag and product.get('type') == 'commit' and SHA.fullmatch(product.get('sha', '')),
            'Product tag must name an exact commit')
    source = product['sha']
    validate_ci(source, api(f'actions/workflows/ci.yml/runs?head_sha={source}&event=push&per_page=100')['workflow_runs'])
    release = api('releases/tags/' + tag)
    require(release.get('tag_name') == tag and release.get('draft') is False and release.get('prerelease') is False
            and release.get('published_at') and api('releases/latest').get('tag_name') == tag,
            'Only the current published stable release can be completed')
    version = tag[1:]
    expected = {f'gtmux-{version}-{system}-{arch}.tar.gz' for system in ('darwin', 'linux') for arch in ('amd64', 'arm64')} | {'SHASUMS256.txt'}
    assets = {a['name']: a for a in release.get('assets', [])}
    require(expected <= assets.keys() and all(type(assets[n].get('size')) is int and 0 < assets[n]['size'] <= 90 * 1024**2
            and re.fullmatch(r'sha256:[0-9a-f]{64}', assets[n].get('digest', '')) for n in expected),
            'Complete checksummed official CLI assets required')
    if before_build:
        require(f'Gtmux-{version}-macos.zip' not in assets, 'Do not replace an already published Mac package')
    rows = api('actions/workflows/release.yml/runs?event=push&per_page=100')['workflow_runs']
    rows = [r for r in rows if r.get('event') == 'push' and r.get('head_branch') == tag]
    original = max(rows, key=lambda r: (r.get('run_number', 0), r.get('run_attempt', 0)), default={})
    require(original.get('head_sha') == source and original.get('path') == WORKFLOW and original.get('repository', {}).get('full_name') == REPO
            and original.get('status') == 'completed' and original.get('conclusion') == 'failure',
            'The latest exact-tag official release must have failed')
    jobs = api(f"actions/runs/{original['id']}/jobs?per_page=100")['jobs']
    cli = [j for j in jobs if j.get('name') == 'goreleaser']
    app = [j for j in jobs if j.get('name') == 'menu-bar app (macOS)']
    require(len(cli) == len(app) == 1 and cli[0].get('run_id') == original['id'] and cli[0].get('head_sha') == source
            and cli[0].get('status') == 'completed' and cli[0].get('conclusion') == 'success'
            and app[0].get('conclusion') == 'failure', 'Successful exact-source CLI and failed Mac build required')
    return {'source': source, 'ref': 'refs/tags/' + tag, 'tag': tag, 'definition': definition,
            'recovery': 'true', 'release': 'true', 'cli_run': str(original['id'])}


def resolve(env, api=gh):
    require(env.get('GITHUB_REPOSITORY') == REPO and SHA.fullmatch(env.get('GITHUB_SHA', '')), 'Trusted repository and definition required')
    tag = env.get('RECOVERY_TAG', '')
    if tag:
        require(env.get('GITHUB_EVENT_NAME') == 'workflow_dispatch' and env.get('GITHUB_REF') == 'refs/heads/main'
                and env.get('GITHUB_ACTOR') == REPO.split('/')[0], 'Recovery dispatch must use owner and main')
        return recovery(tag, env['GITHUB_SHA'], api)
    ref = env.get('GITHUB_REF', '')
    release = ref.startswith('refs/tags/')
    require((release and env.get('GITHUB_EVENT_NAME') == 'push' and TAG.fullmatch(ref[10:])) or
            (not release and env.get('GITHUB_EVENT_NAME') == 'workflow_dispatch' and ref.startswith('refs/heads/')),
            'Expected a tag push or branch snapshot')
    return {'source': env['GITHUB_SHA'], 'ref': ref, 'tag': ref[10:] if release else '',
            'definition': env['GITHUB_SHA'], 'recovery': 'false', 'release': str(release).lower(), 'cli_run': ''}


def require_recovery_builds(source, run_id, version, definition, api=gh):
    proof = recovery('v' + version, definition, api, before_build=False)
    require(proof['source'] == source, 'Recovery product source differs')
    run = api(f'actions/runs/{run_id}')
    require(run.get('event') == 'workflow_dispatch' and run.get('head_branch') == 'main' and run.get('head_sha') == definition
            and run.get('path') == WORKFLOW and run.get('repository', {}).get('full_name') == REPO,
            'Recovery run does not match this reviewed definition')
    jobs = api(f'actions/runs/{run_id}/jobs?per_page=100')['jobs']
    for name in ('Resolve release source', 'menu-bar app (macOS)', 'publish notarized Mac app'):
        matches = [j for j in jobs if j.get('name') == name]
        require(len(matches) == 1 and matches[0].get('status') == 'completed' and matches[0].get('conclusion') == 'success',
                'Recovery build and publication must all succeed')
    cli = [j for j in jobs if j.get('name') == 'goreleaser']
    require(len(cli) == 1 and cli[0].get('conclusion') == 'skipped', 'Recovery must reuse the original CLI, not rebuild it')
    app = next(j for j in jobs if j['name'] == 'menu-bar app (macOS)')
    require(any(s.get('name') == 'verify the app is notarized and stapled (tag)' and s.get('conclusion') == 'success'
                for s in app.get('steps', [])), 'Independent stapling validation must succeed')
    return run_id


def attach(env, api=gh, upload=subprocess.run):
    import hashlib
    tag, sha = env.get('PRODUCT_TAG', ''), env.get('TRUSTED_MAC_SHA', '')
    archive = Path(env.get('ZIP', ''))
    require(TAG.fullmatch(tag) and re.fullmatch(r'[0-9a-f]{64}', sha), 'Exact published tag and trusted Mac digest required')
    require(archive.is_file() and not archive.is_symlink() and archive.name == f'Gtmux-{tag[1:]}-macos.zip'
            and 0 < archive.stat().st_size <= 90 * 1024**2, 'Unexpected Mac package')
    with archive.open('rb') as stream:
        require(hashlib.file_digest(stream, 'sha256').hexdigest() == sha, 'Mac package changed before upload')
    expected_source = env.get('PRODUCT_SOURCE', '')
    require(SHA.fullmatch(expected_source), 'Exact product source required before attachment')
    remote = api('git/ref/tags/' + tag)['object']
    require(remote.get('type') == 'tag' and SHA.fullmatch(remote.get('sha', '')), 'Annotated tag required before attachment')
    annotated = api('git/tags/' + remote['sha'])
    require(annotated.get('tag') == tag and annotated.get('object', {}).get('type') == 'commit' and annotated.get('object', {}).get('sha') == expected_source, 'Tag product changed before attachment')
    metadata = api('releases/tags/' + tag)
    require(metadata.get('tag_name') == tag and metadata.get('draft') is False and metadata.get('prerelease') is False
            and metadata.get('published_at'), 'Stable official release required')
    matches = [a for a in metadata.get('assets', []) if a.get('name') == archive.name]
    if matches:
        require(len(matches) == 1 and matches[0].get('digest') == 'sha256:' + sha and matches[0].get('size') == archive.stat().st_size,
                'Published Mac bytes differ; do not overwrite')
        return
    upload(['gh', 'release', 'upload', tag, str(archive), '--repo', REPO], check=True)


def main():
    try:
        if sys.argv[1:] == ['attach']:
            attach(os.environ)
            print('Exact Mac artifact attached or identical existing upload verified')
            return
        require(not sys.argv[1:], 'Unknown release operation')
        result = resolve(os.environ)
        with open(os.environ['GITHUB_OUTPUT'], 'a') as output:
            for key, value in result.items():
                output.write(f'{key}={value}\n')
        print(json.dumps(result, sort_keys=True))
    except (DeliveryError, ValueError, KeyError, OSError, subprocess.SubprocessError) as error:
        raise SystemExit('Release source resolution failed: ' + type(error).__name__) from None


if __name__ == '__main__':
    main()
