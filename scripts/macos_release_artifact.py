#!/usr/bin/env python3
"""Move one already-verified Mac ZIP between jobs, without rebuilding or extracting it."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil

LIMIT = 90 * 1024**2


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def identity(source, ref, run, attempt):
    require(re.fullmatch(r'[0-9a-f]{40}', source) and run > 0 and attempt > 0,
            'Exact source, run and build attempt required')
    tag = ref.startswith('refs/tags/')
    require(re.fullmatch(r'refs/tags/v\d+\.\d+\.\d+', ref) if tag else
            ref.startswith('refs/heads/') and len(ref) > len('refs/heads/'), 'Invalid build ref')
    filename = f'Gtmux-{ref.removeprefix("refs/tags/v")}-macos.zip' if tag else 'Gtmux-snapshot-macos.zip'
    return {'source': source, 'ref': ref, 'run': run, 'attempt': attempt,
            'file': filename, 'notarized_tag': tag}


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def seal(directory, archive, expected):
    require(archive.is_file() and not archive.is_symlink() and archive.name == expected['file']
            and 0 < archive.stat().st_size <= LIMIT, 'Unexpected Mac ZIP')
    directory.mkdir(mode=0o700)  # A second build must select a new artifact, never replace one.
    target = directory / archive.name
    shutil.copyfile(archive, target)
    sha = digest(archive)
    require(digest(target) == sha, 'Mac ZIP changed during staging')
    (directory / 'BUILD.json').write_text(json.dumps({**expected, 'bytes': target.stat().st_size,
                                                     'sha256': sha}, sort_keys=True) + '\n')
    return sha


def verify(directory, expected, sha):
    require(re.fullmatch(r'[0-9a-f]{64}', sha), 'Trusted build checksum required')
    require(directory.is_dir() and not directory.is_symlink(), 'Missing Mac artifact directory')
    require({p.name for p in directory.iterdir()} == {expected['file'], 'BUILD.json'},
            'Incomplete or additional Mac artifact files')
    for name in (expected['file'], 'BUILD.json'):
        path = directory / name
        require(path.is_file() and not path.is_symlink(), 'Only regular artifact files are allowed')
    receipt = directory / 'BUILD.json'
    require(receipt.stat().st_size <= 4096, 'Mac build receipt is too large')
    metadata = json.loads(receipt.read_text())
    archive = directory / expected['file']
    require(isinstance(metadata, dict) and set(metadata) == set(expected) | {'bytes', 'sha256'}
            and all(metadata.get(key) == value for key, value in expected.items()),
            'Mac artifact belongs to a different source, ref, run or build attempt')
    require(type(metadata['bytes']) is int and 0 < metadata['bytes'] == archive.stat().st_size <= LIMIT
            and metadata['sha256'] == sha == digest(archive), 'Mac artifact length or checksum differs')
    return archive


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('seal', 'verify'))
    parser.add_argument('--directory', type=Path, required=True)
    parser.add_argument('--zip', type=Path)
    parser.add_argument('--sha256', default='')
    parser.add_argument('--source', required=True)
    parser.add_argument('--ref', required=True)
    parser.add_argument('--run', type=int, required=True)
    parser.add_argument('--attempt', type=int, required=True)
    args = parser.parse_args()
    try:
        expected = identity(args.source, args.ref, args.run, args.attempt)
        if args.action == 'seal':
            require(args.zip is not None, 'Mac ZIP required')
            sha = seal(args.directory, args.zip, expected)
            if os.environ.get('GITHUB_OUTPUT'):
                with open(os.environ['GITHUB_OUTPUT'], 'a') as out:
                    out.write(f'sha256={sha}\nattempt={args.attempt}\n')
        else:
            archive = verify(args.directory, expected, args.sha256)
            if os.environ.get('GITHUB_ENV'):
                with open(os.environ['GITHUB_ENV'], 'a') as out:
                    out.write(f'ZIP={archive}\n')
        print('Mac artifact ' + args.action + ' passed')
    except (ValueError, OSError, TypeError):
        raise SystemExit('Mac artifact verification failed; no publication performed')


if __name__ == '__main__':
    main()
