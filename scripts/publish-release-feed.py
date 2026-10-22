#!/usr/bin/env python3
"""Publish unchanged GitHub release assets, then atomically promote signed metadata."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
REPO = 'chenchaoyi/gtmux'
ORIGINS = ('https://downloads.ccy.dev', 'https://downloads.ccy.pub')
MAX_UPLOAD = 90 * 1024 * 1024
VERSION = re.compile(r'^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$')


def names(version):
    return [f'gtmux-{version}-{system}-{arch}.tar.gz' for system in ('darwin', 'linux') for arch in ('amd64', 'arm64')] + [f'Gtmux-{version}-macos.zip', 'SHASUMS256.txt']


def gh(path):
    return json.loads(subprocess.check_output(['gh', 'api', f'repos/{REPO}/{path}']))


def require_builds(source, run, version):
    candidates = [gh(f'actions/runs/{run}')] if run else gh(f'actions/workflows/release.yml/runs?head_sha={source}&event=push&per_page=100')['workflow_runs']
    candidates = [item for item in candidates if item.get('head_branch') == 'v'+version]
    # Never reuse an older successful run after the latest exact-tag run failed.
    for item in candidates[:1]:
        if item['head_sha'] != source or item['event'] != 'push' or item['path'] != '.github/workflows/release.yml':
            continue
        jobs = gh(f"actions/runs/{item['id']}/jobs?per_page=100")['jobs']
        expected = {'goreleaser', 'menu-bar app (macOS)'}
        success = {job['name'] for job in jobs if job['conclusion'] == 'success'}
        if not expected <= success:
            continue
        app = next(job for job in jobs if job['name'] == 'menu-bar app (macOS)' and job['conclusion'] == 'success')
        if not any(step['name'] == 'verify the app is notarized and stapled (tag)' and step['conclusion'] == 'success' for step in app.get('steps', [])):
            continue
        return item['id']
    raise ValueError('Both official CLI and notarized Mac jobs must succeed at the exact release source')


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


HTTP = urllib.request.build_opener(NoRedirect)


def transfer(url, path, token=None, sha=None):
    headers = {}
    if token:
        if not url.startswith('https://gtmux-downloads.pages.dev/publish/'):
            raise ValueError('Publication credential host is not allowed')
        headers = {'Authorization': f'Bearer {token}', 'Content-Length': str(path.stat().st_size),
                   'X-Artifact-SHA256': sha, 'Content-Type': 'application/octet-stream'}
        # A file object is streamed; the explicit length avoids chunked proxy ambiguity.
        with path.open('rb') as data:
            request = urllib.request.Request(url, data=data, headers=headers, method='PUT')
            with HTTP.open(request, timeout=120) as response:
                if response.status not in (200, 201, 204):
                    raise ValueError('Publication failed')
                response.read(65536)
    else:
        if not any(url.startswith(origin+'/gtmux/') for origin in ORIGINS):
            raise ValueError('Readback host is not allowed')
        with HTTP.open(url, timeout=120) as response, path.open('wb') as output:
            if response.status != 200:
                raise ValueError('Readback failed')
            size=0
            while chunk := response.read(1024*1024):
                size += len(chunk)
                if size > MAX_UPLOAD:
                    raise ValueError('Readback too large')
                output.write(chunk)


def digest(path):
    with path.open('rb') as data:
        return hashlib.file_digest(data, 'sha256').hexdigest()


def prepare(version, source, installer_source, directory, metadata, installer, key):
    if not VERSION.fullmatch(version) or any(not re.fullmatch('[0-9a-f]{40}', value) for value in (source, installer_source)):
        raise ValueError('Invalid immutable release identity')
    if metadata.get('tag_name') != 'v'+version or metadata.get('draft') or metadata.get('prerelease') or not metadata.get('published_at'):
        raise ValueError('Only a published stable GitHub release may be mirrored')
    info = {asset['name']: asset for asset in metadata['assets']}
    assets = {}
    for name in names(version):
        path = directory/name
        sha = digest(path)
        if path.stat().st_size <= 0 or path.stat().st_size > MAX_UPLOAD or info.get(name, {}).get('size') != path.stat().st_size or info.get(name, {}).get('digest') != 'sha256:'+sha:
            raise ValueError(f'Official GitHub size/digest mismatch or upload limit: {name}')
        assets[name] = {'sha256': sha, 'size': path.stat().st_size}
    sums = {}
    for line in (directory/'SHASUMS256.txt').read_text().splitlines():
        parts=line.split()
        if len(parts)==2: sums[parts[1].lstrip('*')]=parts[0]
    for name in names(version)[:4]:
        if sums.get(name) != assets[name]['sha256']:
            raise ValueError('GitHub CLI checksum mismatch')
    target = directory/'install.sh'
    target.write_bytes(installer.read_bytes())
    if not 0 < target.stat().st_size <= 1048576:
        raise ValueError('Installer is too large')
    subprocess.run(['bash', '-n', str(target)], check=True)
    assets['install.sh']={'sha256':digest(target),'size':target.stat().st_size}
    manifest={'schema':1, 'version':version, 'source_commit':source, 'installer_source':installer_source,
              'published_at':metadata['published_at'], 'assets':assets}
    payload=(json.dumps(manifest, indent=2, sort_keys=True)+'\n').encode()
    payload_file=directory/'payload.json'; payload_file.write_bytes(payload)
    signature=subprocess.check_output(['openssl','dgst','-sha256','-sign',str(key),str(payload_file)])
    signature_file=directory/'signature';signature_file.write_bytes(signature)
    subprocess.run(['openssl','dgst','-sha256','-verify',str(ROOT/'internal/releasefeed/public.pem'),'-signature',str(signature_file),str(payload_file)],check=True,stdout=subprocess.DEVNULL)
    envelope=(json.dumps({'payload':base64.b64encode(payload).decode(),'signature':base64.b64encode(signature).decode()},separators=(',',':'))+'\n').encode()
    if len(envelope)>65536: raise ValueError('Signed manifest too large')
    (directory/'manifest.json').write_bytes(envelope)
    return manifest


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--source',required=True)
    parser.add_argument('--run',type=int)
    parser.add_argument('--prepare-only',action='store_true')
    args=parser.parse_args()
    version=args.version.removeprefix('v')
    if not VERSION.fullmatch(version): parser.error('a stable X.Y.Z version is required')
    head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
    tag_source=subprocess.check_output(['git','rev-parse',f'v{version}^{{commit}}'],cwd=ROOT,text=True).strip()
    if tag_source!=args.source: parser.error('source does not match the immutable tag')
    remote = gh(f'git/ref/tags/v{version}')['object']
    for _ in range(4):
        if remote.get('type') != 'tag': break
        remote = gh('git/tags/'+remote['sha'])['object']
    if remote.get('type') != 'commit' or remote.get('sha') != args.source:
        parser.error('official tag source differs; stop instead of relabelling assets')
    if (ROOT/'install.sh').read_bytes() != subprocess.check_output(['git','show',head+':install.sh'],cwd=ROOT):
        parser.error('installer differs from its recorded source commit')
    require_builds(args.source,args.run,version)
    metadata=gh(f'releases/tags/v{version}')
    key=Path(os.environ['GTMUX_RELEASE_SIGNING_KEY_FILE'])
    token=os.environ.get('GTMUX_RELEASE_PUBLISH_TOKEN','')
    if not args.prepare_only and len(token)<32: parser.error('publication secret is missing')
    with tempfile.TemporaryDirectory(prefix='gtmux-release-feed-') as temp:
        directory=Path(temp)
        subprocess.run(['gh','release','download','v'+version,'--repo',REPO,'--dir',temp,
                        *sum((['--pattern',name] for name in names(version)),[])],check=True)
        manifest=prepare(version,args.source,head,directory,metadata,ROOT/'install.sh',key)
        print(f'Prepared signed v{version}; unchanged official assets, installer source {head}')
        if args.prepare_only: return
        publisher='https://gtmux-downloads.pages.dev/publish/'
        for name, asset in manifest['assets'].items():
            transfer(f'{publisher}v{version}/{name}',directory/name,token,asset['sha256'])
        transfer(f'{publisher}v{version}/manifest.json',directory/'manifest.json',token,digest(directory/'manifest.json'))
        # Both public routes must serve exact bytes before stable moves.
        for origin in ORIGINS:
            for name,asset in manifest['assets'].items():
                readback=directory/'readback';transfer(f'{origin}/gtmux/v{version}/{name}',readback)
                if digest(readback)!=asset['sha256'] or readback.stat().st_size!=asset['size']:
                    raise ValueError('Public artifact readback differs')
        transfer(publisher+'stable.json',directory/'manifest.json',token,digest(directory/'manifest.json'))
        for origin in ORIGINS:
            readback=directory/'readback';transfer(origin+'/gtmux/stable.json',readback)
            if readback.read_bytes()!=(directory/'manifest.json').read_bytes(): raise ValueError('Stable readback differs')
        print(f'Published and verified v{version} on both official domains; GitHub backup retained')


if __name__=='__main__':
    try: main()
    except (ValueError,KeyError,OSError,subprocess.CalledProcessError,urllib.error.URLError) as error:
        # No request headers, credentials, key contents or service response bodies.
        raise SystemExit(f'Release publication stopped: {type(error).__name__}: {error if isinstance(error,ValueError) else "operation failed; inspect the bounded step"}') from None
