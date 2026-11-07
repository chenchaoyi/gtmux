#!/usr/bin/env python3
"""One manual, fail-closed cache-tail observation. No retries or installation."""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import time

BASE = '650f1ec1a415551a547db10886181666572c8768'
SELECTOR = 'GtmuxBarTests.HQWorkspaceTests/testCachedConversationOpensAtLatestAndReselectionReturnsFromHistory'
FILTER = '^' + re.escape(SELECTOR) + '$'
GIB = 1024 ** 3
SCRATCH_LIMIT = 2 * GIB
EVIDENCE_LIMIT = 64 * 1024 ** 2
SAFETY_FLOOR = 5 * GIB // 2
ALLOWED = {
    '.github/workflows/ci.yml', 'scripts/macos_cache_tail_trace.py',
    'macapp/Sources/GtmuxBar/HQConversationTrace.swift',
    'macapp/Sources/GtmuxBar/HQConversationTailReveal.swift',
    'macapp/Sources/GtmuxBar/HQConversationScrollIntent.swift',
    'macapp/Sources/GtmuxBar/HQWorkspaceConversation.swift',
    'macapp/Tests/GtmuxBarTests/HQWorkspaceTests.swift',
}


class RunInterrupted(Exception):
    def __init__(self, signum):
        self.signum = signum
        super().__init__(f'interrupted by signal {signum}')


def output(command, cwd=None):
    try:
        result = subprocess.run(command, cwd=cwd, text=True, stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, timeout=30)
        return {'command': command, 'status': result.returncode, 'output': result.stdout.strip()}
    except RunInterrupted:
        raise
    except Exception as error:
        return {'command': command, 'status': None, 'output': '', 'error': str(error)}


def environment_mismatches(facts):
    expected = {
        'os_version': '26.7.2', 'os_build': '25G332', 'architecture': 'arm64',
        'xcode': 'Xcode 26.6\nBuild version 17F113',
    }
    errors = [key for key, value in expected.items()
              if facts[key]['status'] != 0 or facts[key]['output'] != value]
    swift = facts['swift']
    if swift['status'] or 'Apple Swift version 6.3.3 (swiftlang-6.3.3.1.3 clang-2100.1.1.101)' not in swift['output']:
        errors.append('swift')
    if facts['image_version'] != '20261102.0473.1':
        errors.append('image_version')
    if facts['developer_dir'] != '/Applications/Xcode_26.6.app/Contents/Developer':
        errors.append('developer_dir')
    return errors


def allocated(path):
    if not path.exists():
        return 0
    return int(subprocess.check_output(['du', '-sk', str(path)], text=True, timeout=5).split()[0]) * 1024


def resources(scratch, evidence):
    snapshot = {'errors': []}
    for key, read in [('free_bytes', lambda: shutil.disk_usage(evidence).free),
                      ('scratch_bytes', lambda: allocated(scratch)),
                      ('evidence_bytes', lambda: allocated(evidence))]:
        try:
            snapshot[key] = read()
        except RunInterrupted:
            raise
        except Exception as error:
            snapshot[key] = None
            snapshot['errors'].append(key + ': ' + str(error))
    return snapshot


def budget_errors(snapshot, suffix='', initial=False, polling=False):
    errors = ['resource-measurement-failed' + suffix] if snapshot['errors'] else []
    required = SAFETY_FLOOR + (SCRATCH_LIMIT + EVIDENCE_LIMIT + GIB // 2 if initial else 0)
    if snapshot['free_bytes'] is not None and snapshot['free_bytes'] < required:
        errors.append('free-space-floor' + suffix)
    limit = SCRATCH_LIMIT - GIB // 4 if polling else SCRATCH_LIMIT
    if snapshot['scratch_bytes'] is not None and snapshot['scratch_bytes'] > limit:
        errors.append('scratch-budget' + suffix)
    if snapshot['evidence_bytes'] is not None and snapshot['evidence_bytes'] > EVIDENCE_LIMIT:
        errors.append('evidence-budget' + suffix)
    return errors


def group_exists(pgid):
    try:
        os.killpg(pgid, 0)
        return True
    except ProcessLookupError:
        return False


def stop_owned(process, pgid):
    # Only the session/group created by this Popen. Leader exit is not group exit.
    info = {'pgid': pgid, 'signals': [], 'group_gone': False, 'errors': []}
    started = time.monotonic()
    try:
        for signum in [signal.SIGTERM, signal.SIGKILL]:
            process.poll()  # Reap the leader without mistaking it for the group.
            if not group_exists(pgid):
                info['group_gone'] = True
                break
            try:
                os.killpg(pgid, signum)
                info['signals'].append(signum)
            except ProcessLookupError:
                info['group_gone'] = True
                break
            deadline = time.monotonic() + 2
            while time.monotonic() < deadline:
                process.poll()
                if not group_exists(pgid):
                    info['group_gone'] = True
                    break
                time.sleep(0.1)
            if info['group_gone']:
                break
        if not info['group_gone']:
            info['group_gone'] = not group_exists(pgid)
    except Exception as error:
        info['errors'].append(str(error))
    info['elapsed_seconds'] = time.monotonic() - started
    return info


def bounded(command, cwd, env, log, seconds, scratch, evidence, stage, checkpoint):
    started = time.monotonic()
    process = None
    stage.update(command=command, state='starting', returncode=None, stopped=None)
    checkpoint()
    try:
        with log.open('xb') as stream:
            if stage.get('interrupted_signal'):
                raise RunInterrupted(stage['interrupted_signal'])
            process = subprocess.Popen(command, cwd=cwd, env=env, stdout=stream,
                                       stderr=subprocess.STDOUT, start_new_session=True)
            stage.update(pgid=process.pid, state='running')
            if stage.get('interrupted_signal'):
                raise RunInterrupted(stage['interrupted_signal'])
            checkpoint()
            while process.poll() is None:
                if time.monotonic() - started > seconds:
                    stage['stopped'] = 'wall-clock-budget'
                elif log.stat().st_size > 16 * 1024 ** 2:
                    stage['stopped'] = 'log-budget'
                else:
                    stage['resource_sample'] = resources(scratch, evidence)
                    failures = budget_errors(stage['resource_sample'], polling=True)
                    if failures:
                        stage['stopped'] = failures[0]
                if stage['stopped']:
                    break
                time.sleep(2)
    except (Exception, KeyboardInterrupt) as error:
        stage['stopped'] = 'interrupted' if isinstance(error, (RunInterrupted, KeyboardInterrupt)) else 'supervisor-error'
        stage['error'] = str(error)
        if isinstance(error, RunInterrupted):
            stage['interrupted_signal'] = error.signum
    finally:
        stage['state'] = 'cleaning'
        if process is not None:
            stage['returncode_before_cleanup'] = process.poll()
            stage['cleanup'] = stop_owned(process, process.pid)
            stage['returncode'] = process.poll()
            if not stage['cleanup']['group_gone'] or stage['cleanup']['errors']:
                stage['stopped'] = stage['stopped'] or 'owned-group-cleanup-incomplete'
        stage['state'] = 'finished'
        stage['elapsed_seconds'] = time.monotonic() - started
        checkpoint()
    stage['resources_at_exit'] = resources(scratch, evidence)
    stage['budget_errors'] = budget_errors(stage['resources_at_exit'], '-at-exit')
    if log.exists() and log.stat().st_size > 16 * 1024 ** 2:
        stage['budget_errors'].append('log-budget-at-exit')
    if stage['budget_errors']:
        stage['stopped'] = stage['stopped'] or stage['budget_errors'][0]
    checkpoint()
    return stage


def provenance_errors(commit, raw_diff):
    errors = []
    parents = [line[7:] for line in commit.split('\n\n', 1)[0].splitlines() if line.startswith('parent ')]
    if parents != [BASE]:
        errors.append('expected-unique-parent-650f')
    paths = []
    added = {'scripts/macos_cache_tail_trace.py', 'macapp/Sources/GtmuxBar/HQConversationTrace.swift'}
    for line in raw_diff.splitlines():
        match = re.fullmatch(r':([0-7]{6}) ([0-7]{6}) ([0-9a-f]{40}) ([0-9a-f]{40}) ([AM])\t(.+)', line)
        if not match:
            errors.append('invalid-raw-diff'); continue
        old_mode, new_mode, _, _, change, path = match.groups()
        paths.append(path)
        expected = ('000000', '100644', 'A') if path in added else ('100644', '100644', 'M')
        if (old_mode, new_mode, change) != expected:
            errors.append('unexpected-mode-or-change: ' + path)
    if len(paths) != len(ALLOWED) or set(paths) != ALLOWED:
        errors.append('expected-exact-seven-path-delta')
    return errors


def write_evidence(path, data, result):
    try:
        path.write_text(json.dumps(data, indent=2) + '\n')
        return True
    except RunInterrupted:
        raise
    except Exception as error:
        result.setdefault('evidence_errors', []).append(str(path.name) + ': ' + str(error))
        print('Evidence write failed: ' + str(error), file=sys.stderr)
        return False


def validate_trace(path):
    if not path.is_file() or path.stat().st_size > 8 * 1024 ** 2:
        return ['missing-or-oversize-trace']
    try:
        rows = [json.loads(line) for line in path.read_text().splitlines()]
    except (ValueError, OSError):
        return ['invalid-trace']
    events = [row.get('event') for row in rows]
    errors = []
    for name in ['case-begin', 'entry-assertion', 'case-end', 'trace-summary']:
        if events.count(name) != 1:
            errors.append('expected-one-' + name)
    for name in ['resize-applied', 'geometry', 'reveal-exit', 'task-end']:
        if name not in events:
            errors.append('missing-' + name)
    if not rows or rows[-1].get('complete') is not True or rows[-1].get('dropped') != 0:
        errors.append('incomplete-trace')
    begins = [r for r in rows if r.get('event') == 'case-begin']
    if begins and begins[0].get('fields', {}).get('selector') != SELECTOR:
        errors.append('selector-mismatch')
    for begin in [r for r in rows if r.get('event') == 'task-begin']:
        attempt = begin.get('fields', {}).get('attempt')
        for end in ['reveal-exit', 'task-end']:
            if sum(r.get('event') == end and r.get('fields', {}).get('attempt') == attempt for r in rows) != 1:
                errors.append('unbalanced-' + end)
    return errors


def observe(repo, root, evidence, scratch, result, checkpoint):
    commands = {'os_version': ['sw_vers', '-productVersion'], 'os_build': ['sw_vers', '-buildVersion'],
                'architecture': ['uname', '-m'], 'xcode': ['xcodebuild', '-version'],
                'swift': ['xcrun', 'swift', '--version'], 'sdk': ['xcrun', '--sdk', 'macosx', '--show-sdk-path'],
                'disk': ['df', '-k', '/System/Volumes/Data', '/System/Volumes/VM']}
    facts = result['environment'] = {}
    for key, command in commands.items():
        facts[key] = output(command)
        checkpoint()
    facts.update(image_version=os.environ.get('ImageVersion'), image_os=os.environ.get('ImageOS'),
                 developer_dir=os.environ.get('DEVELOPER_DIR'))
    result['environment_mismatches'] = environment_mismatches(facts)
    expected = os.environ.get('TRACE_EXPECTED_SHA', '')
    head = output(['git', 'rev-parse', 'HEAD'], repo)
    commit = output(['git', 'cat-file', '-p', 'HEAD'], repo)
    changes = output(['git', 'diff', '--raw', '--no-abbrev', '--no-renames', BASE, 'HEAD'], repo)
    result.update(expected_sha=expected, head=head['output'], commit=commit, changes=changes)
    if head['status'] != 0 or not re.fullmatch('[0-9a-f]{40}', expected) or expected != head['output'] or expected != os.environ.get('GITHUB_SHA'):
        raise ValueError('Workflow SHA, checkout SHA and explicit input SHA must match; external review is separate.')
    result['provenance_errors'] = provenance_errors(commit['output'], changes['output'])
    if commit['status'] != 0 or changes['status'] != 0 or result['provenance_errors']:
        raise ValueError('Require unique parent 650f and the exact seven-path delta with expected modes/change types.')
    dirty = output(['git', 'status', '--porcelain=v1'], repo)
    if dirty['status'] != 0 or dirty['output']:
        raise ValueError('Checkout must be clean before building.')
    result['source_sha256'] = {p: hashlib.sha256((repo / p).read_bytes()).hexdigest() for p in sorted(ALLOWED)}
    result['baseline_trees'] = {p: output(['git', 'rev-parse', BASE + ':' + p], repo)
                                for p in ['macapp', 'shared/UsageWidgets']}
    (evidence / 'instrumentation.diff').write_bytes(subprocess.check_output(
        ['git', 'diff', '--binary', BASE, 'HEAD'], cwd=repo, timeout=30))
    checkpoint()
    if result['environment_mismatches']:
        result['outcome'] = 'environment-mismatch-no-build'
        return 78
    result['initial_resources'] = resources(scratch, evidence)
    result['initial_budget_errors'] = budget_errors(result['initial_resources'], '-before-build', initial=True)
    if result['initial_budget_errors'] or result.get('evidence_errors'):
        result['outcome'] = 'resource-or-evidence-preflight-failed-no-build'
        return 78
    env = os.environ.copy()
    for key, name in [('GTMUX_WORKSPACE_PREVIEW_DIR', 'workspace-previews'),
                      ('GTMUX_WIDGET_RENDER_DIR', 'widget-previews'), ('GTMUX_CACHE_TAIL_TRACE_DIR', 'trace')]:
        path = evidence / name
        path.mkdir()
        env[key] = str(path)
    env['CLANG_MODULE_CACHE_PATH'] = str(scratch / 'clang-module-cache')
    env['SWIFTPM_MODULECACHE_OVERRIDE'] = str(scratch / 'swift-module-cache')
    result['build_ran'] = True
    result['build'] = {}
    bounded(['swift', 'build', '--scratch-path', str(scratch), '--build-tests'], repo / 'macapp',
            env, evidence / 'build.log', 600, scratch, evidence, result['build'], checkpoint)
    if result['build']['returncode'] != 0 or result['build']['stopped']:
        result['outcome'] = 'build-or-supervisor-failed'
        return 1
    result['test_preflight_resources'] = resources(scratch, evidence)
    result['test_preflight_budget_errors'] = budget_errors(result['test_preflight_resources'], '-before-test')
    checkpoint()
    if result['test_preflight_budget_errors'] or result.get('evidence_errors'):
        result['outcome'] = 'resource-or-evidence-preflight-failed-no-test'
        return 1
    result['test_ran'] = True
    result['test'] = {}
    bounded(['swift', 'test', '--scratch-path', str(scratch), '--skip-build', '--parallel', '--num-workers', '1',
             '--filter', FILTER], repo / 'macapp', env, evidence / 'test.log', 60, scratch, evidence, result['test'], checkpoint)
    result['trace_errors'] = validate_trace(evidence / 'trace/cache-tail.jsonl')
    failed = result['test']['returncode'] != 0 or result['test']['stopped'] or result['trace_errors']
    result['outcome'] = 'test-or-trace-failed-review-evidence' if failed else 'not-reproduced-in-this-instrumented-single-case-context'
    return 1 if failed else 0


def finalize(result, evidence, scratch, status):
    hashes = {}
    try:
        paths = list(evidence.rglob('*'))
    except Exception as error:
        paths = []
        result.setdefault('evidence_errors', []).append('inventory: ' + str(error))
    for path in paths:
        if path.name in ['result.json', 'manifest.json']:
            continue
        try:
            if path.is_file():
                data = path.read_bytes()
                hashes[str(path.relative_to(evidence))] = {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
        except Exception as error:
            result.setdefault('evidence_errors', []).append('hash ' + str(path.name) + ': ' + str(error))
    result['final_resources'] = resources(scratch, evidence)
    result['final_budget_errors'] = budget_errors(result['final_resources'], '-at-finalization')
    if result['final_budget_errors'] or result.get('evidence_errors'):
        status = 1
        result['provisional_outcome'] = result['outcome']
        result['outcome'] = 'observation-incomplete-or-budget-failed'
    if result['signals'] and status == 0:
        status = 128 + result['signals'][0]
        result['provisional_outcome'] = result['outcome']
        result['outcome'] = 'interrupted'
    result['helper_returncode'] = status
    if not write_evidence(evidence / 'result.json', result, result):
        status = 1
    else:
        try:
            data = (evidence / 'result.json').read_bytes()
            hashes['result.json'] = {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
        except Exception as error:
            result.setdefault('evidence_errors', []).append('hash result: ' + str(error))
            status = 1
    if not write_evidence(evidence / 'manifest.json', hashes, result):
        status = 1
    if result.get('evidence_errors'):
        status = 1
        result.update(helper_returncode=1, outcome='evidence-incomplete')
        # A failed write cannot be guaranteed recoverable. Keep the prior checkpoint and stderr.
        hashes.pop('result.json', None)
        if write_evidence(evidence / 'result.json', result, result):
            try:
                data = (evidence / 'result.json').read_bytes()
                hashes['result.json'] = {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
            except Exception as error:
                print('Final result hash unavailable: ' + str(error), file=sys.stderr)
        write_evidence(evidence / 'manifest.json', hashes, result)
    return status


def main():
    if os.environ.get('GITHUB_ACTIONS') != 'true' or os.environ.get('GITHUB_EVENT_NAME') != 'workflow_dispatch':
        raise SystemExit('This helper is only for the reviewed manual Actions diagnostic lane.')
    repo = Path(__file__).resolve().parents[1]
    root = Path(os.environ['RUNNER_TEMP']) / 'macos-cache-tail'
    root.mkdir(exist_ok=False)
    evidence = root / 'evidence'
    evidence.mkdir()
    scratch = root / 'build'
    result = {'baseline': BASE, 'selector': SELECTOR, 'root_cause': 'undetermined',
              'build_ran': False, 'test_ran': False, 'outcome': 'preflight-incomplete', 'signals': [],
              'budgets': {'scratch_bytes': SCRATCH_LIMIT, 'evidence_bytes': EVIDENCE_LIMIT,
                          'free_floor_bytes': SAFETY_FLOOR, 'build_seconds': 600, 'test_seconds': 60}}
    def checkpoint():
        write_evidence(evidence / 'result.json', result, result)
    def interrupted(signum, _frame):
        first = not result['signals']
        result['signals'].append(signum)
        # Do not throw across Popen's ownership handoff or interrupt group cleanup.
        deferred = any(result.get(phase, {}).get('state') in ['starting', 'cleaning']
                       for phase in ['build', 'test'])
        for phase in ['build', 'test']:
            stage = result.get(phase, {})
            if stage.get('state') in ['starting', 'running', 'cleaning']:
                stage['interrupted_signal'] = signum
                stage['stopped'] = stage.get('stopped') or 'interrupted'
        if first and not deferred:
            raise RunInterrupted(signum)
    previous = {sig: signal.signal(sig, interrupted) for sig in [signal.SIGINT, signal.SIGTERM]}
    status = 1
    try:
        checkpoint()
        status = observe(repo, root, evidence, scratch, result, checkpoint)
    except (Exception, KeyboardInterrupt) as error:
        result['error'] = str(error)
        result['outcome'] = 'interrupted' if isinstance(error, (RunInterrupted, KeyboardInterrupt)) else 'supervisor-error'
    finally:
        # Further soft cancellation records signals without interrupting bounded cleanup/finalization.
        for sig in previous:
            signal.signal(sig, lambda signum, _frame: result['signals'].append(signum))
        if result['signals']:
            status = 128 + result['signals'][0]
        try:
            status = finalize(result, evidence, scratch, status)
        except Exception as error:
            status = 1
            result.update(outcome='finalization-incomplete', finalization_error=str(error), helper_returncode=1)
            checkpoint()
            print('Finalization incomplete: ' + str(error), file=sys.stderr)
        finally:
            for sig, handler in previous.items():
                signal.signal(sig, handler)
    print(json.dumps({k: result[k] for k in ['head', 'outcome', 'build_ran', 'test_ran', 'helper_returncode'] if k in result}))
    return status


if __name__ == '__main__':
    sys.exit(main())
