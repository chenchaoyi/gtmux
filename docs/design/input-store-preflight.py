#!/usr/bin/env python3
"""Read-only terminal input layout and inventory, with no request text or IDs in output."""

import argparse
from collections import Counter
import datetime
import json
import os
from pathlib import Path
import re
import stat
import time


LEGACY = 'terminal-input'
ACTIVE = 'terminal-input-v2'
JOURNAL = 'terminal-input-cutover.json'
LEASES = 'terminal-clients'
SCAN_LIMIT = 8192
TOKEN = re.compile(r'[0-9a-f]{32}')
SPOOL_NAME = re.compile(r'[0-9a-f]{32}\.(index|pending|claimed|receipt)\.json')
DIR_FLAGS = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
FILE_FLAGS = os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW


class PreflightError(Exception):
    pass


def private(info):
    return info.st_uid == os.getuid() and not info.st_mode & 0o077


def identity(info):
    return {'dev': str(info.st_dev), 'ino': str(info.st_ino)}


def info_at(fd, name):
    try:
        return os.stat(name, dir_fd=fd, follow_symlinks=False)
    except FileNotFoundError:
        return None


def unique_object(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError('duplicate JSON key')
        value[key] = item
    return value


def invalid_constant(_):
    raise ValueError('invalid JSON constant')


def validate_json(value, depth=0):
    if isinstance(value, str):
        value.encode('utf-8')
    elif isinstance(value, (dict, list)):
        if depth == 64:
            raise ValueError('too many nested containers')
        if isinstance(value, dict):
            for key, item in value.items():
                key.encode('utf-8')
                validate_json(item, depth + 1)
        else:
            for item in value:
                validate_json(item, depth + 1)


def document(fd, name, limit):
    raw = os.open(name, FILE_FLAGS, dir_fd=fd)
    try:
        info = os.fstat(raw)
        if not private(info) or not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > limit:
            raise PreflightError('invalid private file')
        with os.fdopen(raw, 'rb', closefd=False) as stream:
            data = stream.read(limit + 1)
        if len(data) != info.st_size:
            raise PreflightError('document changed while reading')
        try:
            value = json.loads(data.decode('utf-8'), object_pairs_hook=unique_object, parse_constant=invalid_constant)
            validate_json(value)
        except (ValueError, RecursionError) as error:
            raise PreflightError('invalid JSON') from error
        if not isinstance(value, dict):
            raise PreflightError('expected JSON object')
        return info, value
    finally:
        os.close(raw)


def layout(fd):
    legacy, active, journal = (info_at(fd, name) for name in (LEGACY, ACTIVE, JOURNAL))
    if journal is None:
        if active is not None:
            raise PreflightError('active path has no journal')
        if legacy is not None and (not private(legacy) or not stat.S_ISDIR(legacy.st_mode)):
            raise PreflightError('invalid legacy directory')
        return LEGACY, legacy, None
    journal_info, record = document(fd, JOURNAL, 4096)
    if identity(journal_info) != identity(journal):
        raise PreflightError('journal changed during inspection')
    if record.keys() != {'version', 'phase', 'token', 'directory', 'marker'} or (
        type(record['version']) not in (int, float) or record['version'] != 1 or record['phase'] != 'active'
        or not isinstance(record['token'], str) or TOKEN.fullmatch(record['token']) is None
    ):
        raise PreflightError('journal does not select an active layout')
    if active is None or not private(active) or not stat.S_ISDIR(active.st_mode) or identity(active) != record['directory']:
        raise PreflightError('active directory identity differs')
    marker_info, marker = document(fd, LEGACY, 4096)
    if (legacy is None or identity(marker_info) != identity(legacy) or marker_info.st_dev != active.st_dev
        or identity(marker_info) != record['marker'] or marker.keys() != {'version', 'token'}
        or type(marker['version']) not in (int, float) or marker['version'] != 1 or marker['token'] != record['token']
    ):
        raise PreflightError('legacy marker differs from journal')
    return ACTIVE, active, record['token']


def inventory(root, name, baseline):
    fd = os.open(name, DIR_FLAGS, dir_fd=root)
    try:
        if identity(os.fstat(fd)) != identity(baseline):
            raise PreflightError('queue directory changed during inspection')
        counts = Counter()
        logical = allocated = missing = 0
        with os.scandir(fd) as entries:
            for i, entry in enumerate(entries, start=1):
                if i > SCAN_LIMIT:
                    raise PreflightError('queue exceeds scan limit')
                info = info_at(fd, entry.name)
                if info is None:
                    missing += 1
                    continue
                match = SPOOL_NAME.fullmatch(entry.name)
                counts[match[1] if match and stat.S_ISREG(info.st_mode) else 'other'] += 1
                logical += info.st_size
                allocated += info.st_blocks * 512
        return {'entries_by_filename': dict(counts), 'record_validation': 'not_checked', 'logical_bytes': logical,
                'allocated_bytes': allocated, 'vanished_during_scan': missing}
    finally:
        os.close(fd)


def lease_summary(root, generation, directory_present):
    baseline = info_at(root, LEASES)
    if baseline is None:
        return {'directory_present': False}
    fd = os.open(LEASES, DIR_FLAGS, dir_fd=root)
    try:
        info = os.fstat(fd)
        if not private(info) or identity(info) != identity(baseline):
            raise PreflightError('invalid lease directory')
        now = time.time_ns() // 1_000_000
        counts = Counter()
        with os.scandir(fd) as entries:
            for i, entry in enumerate(entries, start=1):
                if i > 256:
                    raise PreflightError('lease directory exceeds scan limit')
                if re.fullmatch(r'[0-9]+\.json', entry.name) is None:
                    counts['other'] += 1
                    continue
                try:
                    _, lease = document(fd, entry.name, 16 << 10)
                except (OSError, PreflightError):
                    counts['unreadable'] += 1
                    continue
                if lease.get('agent') != 'dsh' or lease.get('client') != 'dst':
                    counts['other'] += 1
                    continue
                updated = lease.get('updated_at')
                if type(updated) is not int or not -1000 <= now - updated <= 5000:
                    counts['dsh_stale_or_invalid_clock'] += 1
                elif lease.get('input_ready') is not True:
                    counts['dsh_fresh_not_ready'] += 1
                elif not isinstance(lease.get('input_store_generation', ''), str):
                    counts['unreadable'] += 1
                elif not directory_present:
                    counts['dsh_fresh_ready_without_store'] += 1
                elif lease.get('input_store_generation', '') == (generation or ''):
                    counts['dsh_fresh_ready_matching_generation'] += 1
                elif not lease.get('input_store_generation'):
                    counts['dsh_fresh_ready_without_generation'] += 1
                else:
                    counts['dsh_fresh_ready_other_generation'] += 1
        return {'directory_present': True, 'lease_files_by_self_report': dict(counts)}
    finally:
        os.close(fd)


def inspect(root):
    try:
        fd = os.open(root, DIR_FLAGS)
    except FileNotFoundError:
        return {'layout': 'fresh', 'root_exists': False}
    try:
        root_info = os.fstat(fd)
        if not private(root_info):
            raise PreflightError('invalid state root')
        name, info, token = layout(fd)
        space = os.fstatvfs(fd)
        result = {'layout': 'active' if name == ACTIVE else ('legacy' if info else 'fresh'),
                  'root_exists': True, 'queue_identity': identity(info) if info else None,
                  'available_bytes': space.f_bavail * space.f_frsize}
        if info is not None:
            result['inventory'] = inventory(fd, name, info)
        result['leases'] = lease_summary(fd, token, info is not None)
        after_name, after_info, after_token = layout(fd)
        if name != after_name or (identity(after_info) if after_info else None) != result['queue_identity'] or token != after_token:
            raise PreflightError('layout changed during inspection')
        if identity(root_info) != identity(root.lstat()):
            raise PreflightError('state root changed during inspection')
        return result
    finally:
        os.close(fd)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('state_root', type=Path, help='explicit gtmux state directory')
    args = parser.parse_args()
    report = {'observed_at': datetime.datetime.now().astimezone().isoformat(),
              'maintenance_ready': 'unverified'}
    try:
        report.update(inspect(args.state_root))
    except (OSError, PreflightError) as error:
        reason = error.strerror if isinstance(error, OSError) else str(error)
        report.update(layout='unverified', reason=reason)
        print(json.dumps(report, indent=2))
        return 2
    print(json.dumps(report, indent=2))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
