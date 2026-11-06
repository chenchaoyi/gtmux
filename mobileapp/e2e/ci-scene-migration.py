#!/usr/bin/env python3
"""Bounded scene migration acceptance: exact sources, isolated evidence, fail closed.

This module never interprets a successful build or a zero Jest exit as complete
acceptance. Runner commands are added only after the reviewed product closure is
resolved. The command-line verifier requires already captured execution evidence.
"""
from pathlib import Path
import argparse
import hashlib
import json
import re
import subprocess

SHA = re.compile(r'^[0-9a-f]{40}$')
CASES = {
    'servers': (
        'selects a server after a clean launch',
        'switches between two saved servers',
    ),
    'coldstart': (
        'a cold-start push is handled once A: cold start with an active Mac, switches, background, a new warm tap',
        'a cold-start push is handled once C: a launch with no notification switches where it is pointed',
    ),
    'actions': ('delivers a cold notification action exactly once', 'delivers a warm notification action exactly once'),
    'links': ('opens a cold usage link through the scene', 'opens a warm usage link through the scene'),
    'ipad': ('keeps the iPad sidebar and selected session through rotation',),
}
STEPS = {
    'servers': ('select-after', 'switch-before', 'switch-after'),
    'coldstart': ('A0-home-connected', 'A1-after-cold-tap', 'A2-switch-office', 'A3-switch-home',
                  'A4-switch-office-again', 'A5-after-background', 'A6-after-warm-tap-office13',
                  'A7-switch-home-after-warm', 'C1-home', 'C2-office'),
    'actions': ('cold-native-headless', 'cold-fake-once', 'same-host-attached', 'fresh-root-before',
                'reader-selected', 'trace-used', 'diff-used', 'draft-entered', 'fresh-root-rotated',
                'draft-keyboard-retained', 'returned-to-list', 'warm-background', 'warm-fake-once'),
    'links': ('cold-not-running', 'cold-native-new-scene-url', 'cold-usage-visible',
              'warm-native-new-scene-url', 'warm-usage-visible'),
    'ipad': ('sidebar-reader-landscape', 'reader-selected', 'trace-used', 'diff-used', 'draft-entered',
             'sidebar-reader-portrait', 'draft-keyboard-retained'),
}
TOOLS = {
    '26': {'xcode': '26.3', 'sdk': '26.2', 'runtime': '26.2', 'driver': '11.14.0', 'wda': '15.0.0'},
    '27': {'xcode': '27.1', 'sdk': '27.1', 'runtime': '27.0', 'driver': '12.20.0', 'wda': '16.17.1'},
}


def revision(value):
    if not isinstance(value, str) or not SHA.fullmatch(value) or set(value) == {'0'}:
        raise ValueError('Missing or invalid exact revision')
    return value


def sources(event_name, event, github_sha):
    """Use real event identities; never replace an unreadable base with HEAD."""
    revision(github_sha)  # For PR this may be the test merge; record, never use as P.
    if event_name == 'pull_request':
        pr = event['pull_request']
        return {'baseline': revision(pr['base']['sha']), 'candidate': revision(pr['head']['sha'])}
    if event_name == 'push' and event.get('ref') == 'refs/heads/main':
        actual = revision(event['after'])
        if actual != github_sha:
            raise ValueError('actualmain does not match event.after')
        return {'candidate': actual}
    raise ValueError('Unsupported migration event')


def units(identities, toolchain):
    if toolchain not in TOOLS or 'candidate' not in identities or set(identities) - {'candidate', 'baseline'}:
        raise ValueError('Invalid migration matrix')
    for sha in identities.values():
        revision(sha)
    result = []
    for variant in ('baseline', 'candidate'):
        if variant not in identities or (toolchain == '27' and variant == 'baseline'):
            continue
        journeys = ('servers', 'coldstart') if toolchain == '26' else ('coldstart', 'actions', 'links', 'ipad')
        for journey in journeys:
            result.append({'toolchain': toolchain, 'runtime': TOOLS[toolchain]['runtime'],
                           'variant': variant, 'source': identities[variant], 'journey': journey})
    return result


def tuple_key(unit):
    if not isinstance(unit, dict) or set(unit) != {'toolchain', 'runtime', 'variant', 'source', 'journey'}:
        raise ValueError('Invalid unit fields')
    toolchain = unit['toolchain']
    if toolchain not in TOOLS or unit['runtime'] != TOOLS[toolchain]['runtime']:
        raise ValueError('Unexpected toolchain/runtime')
    if unit['variant'] not in {'baseline', 'candidate'} or (toolchain == '27' and unit['variant'] != 'candidate'):
        raise ValueError('Unexpected variant')
    allowed = ('servers', 'coldstart') if toolchain == '26' else ('coldstart', 'actions', 'links', 'ipad')
    if unit['journey'] not in allowed:
        raise ValueError('Unexpected journey')
    revision(unit['source'])
    return '/'.join((toolchain, unit['runtime'], unit['source'], unit['journey']))


def create_job_root(runner_temp, run_id, attempt, job):
    if not all(re.fullmatch(r'[A-Za-z0-9_-]+', str(x)) for x in (run_id, attempt, job)):
        raise ValueError('Unsafe job identity')
    parent = Path(runner_temp).resolve() / 'scene-migration'
    parent.mkdir(exist_ok=True)
    root = parent / f'{run_id}-{attempt}-{job}'
    root.mkdir()  # Exclusive: an existing attempt cannot overwrite its evidence.
    return root


def allocate_unit(root, unit):
    key = tuple_key(unit)
    paths = {name: Path(root) / name / key for name in ('work', 'evidence')}
    for path in paths.values():
        path.mkdir(parents=True, exist_ok=False)
    return paths


def archive_source(repository, sha, destination):
    """Real git root handling works even when invoked from mobileapp/."""
    revision(sha)
    repo = Path(subprocess.check_output(['git', '-C', str(repository), 'rev-parse', '--show-toplevel'], text=True, timeout=15).strip())
    subprocess.run(['git', '-C', str(repo), 'cat-file', '-e', sha + '^{commit}'], check=True, timeout=15)
    target = Path(destination)
    target.mkdir(parents=True, exist_ok=False)
    archive = target.parent / (target.name + '.tar')
    # Disk-backed archive, no unbounded PIPE buffer; remove only this newly created
    # transient tar after successful extraction. No shared worktree is touched.
    with archive.open('xb') as output:
        subprocess.run(['git', '-C', str(repo), 'archive', sha], stdout=output, check=True, timeout=60)
    subprocess.run(['tar', '-xf', str(archive), '-C', str(target)], check=True, timeout=60)
    archive.unlink()
    return target


def write_once(path, value):
    with Path(path).open('x') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')


def product_errors(unit, evidence):
    errors = []
    for key, value in TOOLS[unit['toolchain']].items():
        if evidence.get(key) != value:
            errors.append(f'{key} missing or mismatched')
    for key in ('runtimeID', 'runtimeBuild', 'udid', 'deviceType', 'sdkroot', 'linkSysroot',
                'developerDir', 'wdaDeployment', 'wdaBinarySHA', 'sessionID', 'installedAt', 'launchedAt'):
        if not evidence.get(key):
            errors.append(f'{key} missing')
    if type(evidence.get('pid')) is not int or evidence.get('pid', 0) <= 0:
        errors.append('PID missing')
    if evidence.get('source') != unit['source'] or evidence.get('signature') != 'adhoc':
        errors.append('source/signature mismatch')
    if not re.fullmatch(r'[0-9a-f]{64}', evidence.get('wdaBinarySHA', '')):
        errors.append('WDA binary hash invalid')
    for key in ('sdkroot', 'linkSysroot'):
        if ('iphonesimulator' + TOOLS[unit['toolchain']]['sdk']) not in str(evidence.get(key, '')).lower():
            errors.append(key + ' does not identify the required SDK')
    products = evidence.get('products', [])
    if len(products) != 3 or {p.get('kind') for p in products} != {'app', 'liveActivity', 'notificationExtension'}:
        errors.append('app and two extensions missing')
    for product in products:
        if (product.get('architectures') != ['arm64'] or product.get('signature') != 'adhoc'
                or not re.fullmatch(r'[0-9a-f]{64}', product.get('sha256', ''))
                or not product.get('uuid') or not product.get('deployment')
                or 'entitlements' not in product):
            errors.append('product identity incomplete')
    app = next((p for p in products if p.get('kind') == 'app'), {})
    if app.get('deployment') != '15.1':
        errors.append('app deployment changed')
    return errors


def unit_errors(unit, result, artifact_root):
    errors = []
    if result.get('unit') != unit:
        errors.append('unit identity mismatch')
    if type(result.get('testExit')) is not int or result.get('testExit') != 0 or result.get('state') != 'completed':
        errors.append('test did not complete successfully')
    errors.extend(product_errors(unit, result.get('identity', {})))
    expected = set(CASES[unit['journey']])
    cases = result.get('cases', [])
    if len(cases) != len(expected) or {c.get('name') for c in cases} != expected:
        errors.append('missing or duplicate registered case')
    if any(c.get('status') != 'passed' or not c.get('startedAt') or not c.get('completedAt') for c in cases):
        errors.append('case skipped, failed, cancelled or incomplete')
    if (len(result.get('steps', [])) != len(STEPS[unit['journey']])
            or set(result.get('steps', [])) != set(STEPS[unit['journey']])):
        errors.append('required behavior step missing or unexpected')
    artifacts = result.get('artifacts', [])
    if not artifacts:
        errors.append('evidence missing')
    root = Path(artifact_root).resolve()
    for artifact in artifacts:
        try:
            relative = Path(artifact['path'])
            if relative.is_absolute() or '..' in relative.parts:
                raise ValueError('unsafe evidence path')
            unresolved = root / relative
            if unresolved.is_symlink():
                raise ValueError('linked evidence')
            path = unresolved.resolve(strict=True)
            path.relative_to(root)
            if not path.is_file():
                raise ValueError('nonfile evidence')
            data = path.read_bytes()
            if not data or len(data) != artifact['bytes'] or hashlib.sha256(data).hexdigest() != artifact['sha256']:
                raise ValueError('artifact changed')
        except (OSError, ValueError, KeyError, TypeError):
            errors.append('artifact identity invalid')
    return errors


def verify_all(expected_units, results, root):
    errors = []
    if len(results) != len(expected_units):
        errors.append('matrix execution incomplete')
    by_key = {}
    for result in results:
        unit = result.get('unit', {})
        try:
            key = tuple_key(unit)
        except (KeyError, ValueError, TypeError):
            errors.append('invalid result unit')
            continue
        if key in by_key:
            errors.append('duplicate execution result')
        by_key[key] = result
    for unit in expected_units:
        key = tuple_key(unit)
        if key not in by_key:
            errors.append('missing ' + key)
        else:
            errors.extend(key + ': ' + e for e in unit_errors(unit, by_key[key], Path(root) / key))
    if set(by_key) - {tuple_key(u) for u in expected_units}:
        errors.append('unexpected execution unit')
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--verify', required=True, type=Path)
    parser.add_argument('--evidence-root', required=True, type=Path)
    args = parser.parse_args()
    try:
        data = json.loads(args.verify.read_text())
        identities = sources(data['eventName'], data['event'], data['githubSHA'])
        expected = units(identities, data['toolchain'])
        errors = verify_all(expected, data['results'], args.evidence_root)
    except (OSError, ValueError, KeyError, TypeError) as error:
        errors = [str(error)]
    for error in errors:
        print('scene acceptance failed:', error)
    return int(bool(errors))


if __name__ == '__main__':
    raise SystemExit(main())
