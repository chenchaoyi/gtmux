#!/usr/bin/env python3
"""Bounded scene migration acceptance: exact sources, isolated evidence, fail closed.

This module never interprets a successful build or a zero Jest exit as complete
acceptance. Execution is allowed only on the reviewed hosted CI surface; the
command-line verifier can independently check already captured evidence.
"""
from pathlib import Path
import argparse
import contextlib
import datetime
import difflib
import os
import plistlib
import shutil
import signal
import time
import hashlib
import json
import re
import subprocess
import sys

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
                'draft-keyboard-retained', 'rotated-input-edited', 'returned-to-list', 'warm-background', 'warm-fake-once'),
    'links': ('cold-not-running', 'cold-native-new-scene-url', 'cold-usage-visible',
              'warm-native-new-scene-url', 'warm-usage-visible'),
    'ipad': ('sidebar-reader-landscape', 'reader-selected', 'trace-used', 'diff-used', 'draft-entered',
             'sidebar-reader-portrait', 'draft-keyboard-retained', 'rotated-input-edited'),
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
                'developerDir', 'wdaDeployment', 'wdaTargetDeployment', 'wdaBinarySHA', 'sessionID', 'installedAt', 'launchedAt'):
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
                or product.get('sdk') != 'iphonesimulator' + TOOLS[unit['toolchain']]['sdk']
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
    if result.get('problems'): errors.append('execution evidence or cleanup failed')
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
    required = {'jest.json', 'harness.json', 'session/session.json', 'session/behavior-steps.jsonl'}
    names = {a.get('path') for a in artifacts if a.get('bytes', 0) > 0}
    if not required <= names or not any(str(n).endswith('.png') for n in names) or not any(str(n).endswith('.xml') for n in names):
        errors.append('required execution, harness or UI evidence missing')
    if not any(str(n).endswith('-fake.json') or str(n).startswith('session/requests-') for n in names):
        errors.append('fake request evidence missing')
    if unit['journey'] in {'actions', 'links', 'ipad'} and not any(str(n).endswith('-native.jsonl') for n in names):
        errors.append('native journey evidence missing')
    if unit['journey'] in {'actions', 'ipad'} and not any(str(n).endswith('-geometry.json') for n in names):
        errors.append('host geometry evidence missing')
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
            if len(data) != artifact['bytes'] or hashlib.sha256(data).hexdigest() != artifact['sha256']:
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


# Execution is confined to an ephemeral hosted runner. Local controls exercise
# these functions with small synthetic commands and files, never a simulator.
class CommandFailure(RuntimeError):
    """A completed child outcome survives a later diagnostic write failure."""
    def __init__(self, message, outcome):
        super().__init__(message)
        self.outcome = outcome


class CommandClock:
    """Fixed event slots: observations, not inferred kernel exit times."""
    EVENTS = ('commandRequested', 'popenReturned', 'deadlineObserved', 'killRequested',
              'exitObserved', 'reaped', 'archived', 'firstByte')

    def __init__(self):
        self.data = {'events': {name: None for name in self.EVENTS},
                     'deadlineMonotonic': None, 'maxPollGap': 0}
        self.last_poll = None

    def mark(self, name):
        if self.data['events'][name] is None:
            self.data['events'][name] = {'at': now(), 'monotonic': time.monotonic()}
        return self.data['events'][name]['monotonic']

    def poll(self):
        current = time.monotonic()
        if self.last_poll is not None:
            self.data['maxPollGap'] = max(self.data['maxPollGap'], current - self.last_poll)
        self.last_poll = current


class InstallStream:
    """One owned same-device observer; bounded pipes never gate primary startup."""
    LIMIT = 256 * 1024
    READ_QUOTA = 16 * 1024
    METADATA_LIMIT = 16 * 1024

    def __init__(self, stem, env, cwd, binding):
        self.stem, self.env, self.cwd = stem, env, cwd
        self.child = None
        self.outputs, self.pipes = {}, {}
        self.problems = []
        self.clock = CommandClock()
        self.turn = 0
        self.finished = False
        self.record = {'binding': binding, 'state': 'not-started', 'exit': None, 'observerPID': None,
                       'primaryPID': None, 'retainedBytes': {'stdout': 0, 'stderr': 0},
                       'eof': {'stdout': False, 'stderr': False}, 'truncated': False,
                       'droppedBytes': None, 'clock': self.clock.data}

    def problem(self, label, error=''):
        # Fixed failure sites and bounded text; metadata serialization is checked too.
        self.problems.append((label + ': ' + str(error))[:512])

    def start(self):
        args = ['xcrun', 'simctl', 'spawn', self.record['binding']['udid'], '/usr/bin/log', 'stream',
                '--style', 'compact', '--level', 'info', '--timeout', '60',
                '--predicate', 'process == "installd" OR process == "SpringBoard"']
        self.record['command'] = args
        self.clock.mark('commandRequested')
        try:
            for name in ('stdout', 'stderr'):
                self.outputs[name] = self.stem.with_suffix('.install-stream.' + name + '.log').open('xb', buffering=0)
            self.child = subprocess.Popen(args, cwd=self.cwd, env=self.env, stdout=subprocess.PIPE,
                                          stderr=subprocess.PIPE, start_new_session=True)
            self.clock.mark('popenReturned')
            self.record.update(state='running', observerPID=self.child.pid)
            self.pipes = {'stdout': self.child.stdout, 'stderr': self.child.stderr}
            for pipe in self.pipes.values(): os.set_blocking(pipe.fileno(), False)
        except Exception as error:
            self.record['state'] = 'error'
            self.problem('stream startup', error)
            self.stop('error')

    def bind_primary(self, child, deadline):
        self.record['primaryPID'] = child.pid
        self.clock.data['deadlineMonotonic'] = deadline

    def observe_exit(self):
        if self.child is None: return None
        code = self.child.poll()
        if code is not None:
            self.record['exit'] = code
            self.clock.mark('exitObserved')
        return code

    def stop(self, reason):
        if self.record['state'] == 'running': self.record['state'] = reason
        if self.child is None: return
        try:
            if self.observe_exit() is None:
                self.clock.mark('killRequested')
                os.killpg(self.child.pid, signal.SIGKILL)
        except Exception as error:
            self.problem('stream stop', error)

    def poll(self):
        if self.child is None or self.record['state'] != 'running': return
        self.clock.poll()
        try:
            deadline = self.clock.data['deadlineMonotonic']
            if deadline is not None and time.monotonic() >= deadline:
                self.clock.mark('deadlineObserved')
                self.stop('deadline')
                return
            code = self.observe_exit()
            # At most one shared quota, alternating the first serviced pipe.
            names = ('stdout', 'stderr') if self.turn % 2 == 0 else ('stderr', 'stdout')
            self.turn += 1
            budget = self.READ_QUOTA
            for name in names:
                if self.record['eof'][name]: continue
                remaining = self.LIMIT - sum(self.record['retainedBytes'].values())
                if remaining <= 0: break
                size = min(budget, self.READ_QUOTA // 2, remaining)
                try: data = os.read(self.pipes[name].fileno(), size)
                except BlockingIOError: continue
                budget -= len(data)
                if not data:
                    self.record['eof'][name] = True
                    continue
                self.clock.mark('firstByte')
                written = self.outputs[name].write(data)
                self.record['retainedBytes'][name] += written
                if written != len(data): raise OSError('partial stream write')
            # Exact capacity stops, with unknown dropped total; never probe/drain forever.
            if sum(self.record['retainedBytes'].values()) >= self.LIMIT:
                self.record['truncated'] = True
                self.problem('stream capacity reached')
                self.stop('output-limit')
            elif code is not None:
                self.problem('stream ended before primary', code)
                self.record['state'] = 'early-exit'
        except Exception as error:
            self.problem('stream observation', error)
            self.stop('error')

    def finish(self):
        if self.finished: return list(self.problems)
        self.finished = True
        self.stop('primary-ended')
        if self.child is not None:
            try:
                self.record['exit'] = self.child.wait(timeout=1)
                self.clock.mark('exitObserved')
                self.clock.mark('reaped')
            except Exception as error: self.problem('stream reap unconfirmed', error)
        for output in [*self.pipes.values(), *self.outputs.values()]:
            try: output.close()
            except Exception as error: self.problem('stream close', error)
        if not sum(self.record['retainedBytes'].values()): self.problem('stream empty/inconclusive')
        self.record['problems'] = self.problems
        self.clock.mark('archived')
        try:
            raw = (json.dumps(self.record, indent=2) + '\n').encode()
            if len(raw) > self.METADATA_LIMIT:
                self.problem('stream metadata capacity reached')
                # Preserve all fixed terminal slots and counters; never silently truncate identity.
                self.record = {k: self.record[k] for k in ('state', 'exit', 'observerPID', 'primaryPID',
                    'retainedBytes', 'eof', 'truncated', 'droppedBytes', 'clock')}
                self.record.update(metadataTruncated=True, omittedSHA256=hashlib.sha256(raw).hexdigest(),
                                   omittedBytes=len(raw), problems=[p[:160] for p in self.problems[:16]])
                raw = (json.dumps(self.record, indent=2) + '\n').encode()
            if len(raw) > self.METADATA_LIMIT: raise ValueError('fixed terminal metadata exceeds cap')
            with self.stem.with_suffix('.install-stream.json').open('xb') as output: output.write(raw)
        except Exception as error: self.problem('stream archive', error)
        return list(self.problems)


class InstallSample:
    """One bounded observation of this command's child, never a process search."""
    AFTER = 15
    TIMEOUT = 3
    LIMIT = 256 * 1024

    def __init__(self, stem, env, cwd):
        self.stem, self.env, self.cwd = stem, env, cwd
        self.child = self.output = None
        self.record = None
        self.clock = CommandClock()
        self.problems = []
        self.stack = stem.with_suffix('.sample.txt')
        self.log = stem.with_suffix('.sample.log')

    def poll(self, primary, elapsed):
        if self.record is None:
            if elapsed < self.AFTER or primary.poll() is not None:
                return
            args = ['/usr/bin/sample', str(primary.pid), '1', '10', '-file', str(self.stack)]
            self.record = {'primaryPID': primary.pid, 'command': args, 'startedAt': now(),
                           'state': 'not-started', 'exit': None, 'stack': str(self.stack), 'log': str(self.log)}
            self.clock.mark('commandRequested')
            self.record['clock'] = self.clock.data
            self.deadline = time.monotonic() + self.TIMEOUT
            self.clock.data['deadlineMonotonic'] = self.deadline
            try:
                if self.stack.exists(): raise ValueError('Sample destination already exists')
                self.output = self.log.open('xb')
                self.child = subprocess.Popen(args, cwd=self.cwd, env=self.env, stdout=self.output,
                                              stderr=subprocess.STDOUT, start_new_session=True)
                self.clock.mark('popenReturned')
                self.record.update(state='running', observerPID=self.child.pid)
            except (OSError, ValueError, subprocess.SubprocessError) as error:
                self.record['state'] = 'error'
                self.problems.append('sample startup: ' + str(error))
        if self.child is None or self.record['state'] != 'running':
            return
        try:
            self.clock.poll()
            size = sum(path.stat().st_size for path in (self.stack, self.log) if path.exists())
            code = self.child.poll()
            if code is not None: self.clock.mark('exitObserved')
            if size > self.LIMIT:
                self.record['state'] = 'output-limit'
            elif time.monotonic() >= self.deadline:
                self.clock.mark('deadlineObserved')
                self.record['state'] = 'timeout'
            elif code is not None:
                self.record.update(state='completed', exit=code)
                if code != 0 or not self.stack.is_file() or self.stack.stat().st_size == 0:
                    self.problems.append('sample did not produce a successful stack')
            if self.record['state'] in ('output-limit', 'timeout'):
                self.problems.append('sample ' + self.record['state'])
                if code is None:
                    self.clock.mark('killRequested')
                    os.killpg(self.child.pid, signal.SIGKILL)
        except (OSError, ValueError, subprocess.SubprocessError) as error:
            self.record['state'] = 'error'
            self.problems.append('sample observation: ' + str(error))
            try:
                if self.child.poll() is None:
                    self.clock.mark('killRequested')
                    os.killpg(self.child.pid, signal.SIGKILL)
            except OSError as stop_error: self.problems.append('sample stop: ' + str(stop_error))

    def finish(self, primary):
        if self.record is None:
            return []  # Install completed before the observation threshold.
        try:
            while self.record['state'] == 'running':
                self.poll(primary, self.AFTER)
                if self.record['state'] == 'running': time.sleep(.05)
        finally:
            # Even observer evidence failures cannot leave a background driver.
            if self.child is not None:
                try:
                    if self.child.poll() is None:
                        self.clock.mark('killRequested')
                        os.killpg(self.child.pid, signal.SIGKILL)
                    self.record['exit'] = self.child.wait(timeout=1)
                    self.clock.mark('exitObserved')
                    self.clock.mark('reaped')
                except (OSError, subprocess.SubprocessError) as error:
                    self.problems.append('sample reap: ' + str(error))
            if self.output is not None:
                try: self.output.close()
                except OSError as error: self.problems.append('sample log close: ' + str(error))
            self.clock.mark('archived')
            self.record.update(completedAt=now(), problems=self.problems)
            try: write_once(self.stem.with_suffix('.sample.json'), self.record)
            except OSError as error: self.problems.append('sample archive: ' + str(error))
        return list(self.problems)


class Runner:
    def __init__(self, root, env):
        self.root, self.env = Path(root), dict(env)
        self.sequence = 0
        self.observation_problems = []

    def command(self, args, *, cwd, label, timeout, check=True, limit=64 * 1024 * 1024, separate_stderr=False, sample_install=False, install_stream=None):
        self.sequence += 1
        stem = self.root / f'{self.sequence:03d}-{label}'
        log = stem.with_suffix('.log')
        outcome = {'command': [str(x) for x in args], 'cwd': str(cwd), 'startedAt': now(),
                   'state': 'not-started', 'exit': None, 'log': str(log)}
        clock = CommandClock()
        clock.mark('commandRequested')
        outcome['clock'] = clock.data
        sample = InstallSample(stem, self.env, cwd) if sample_install else None
        observer = InstallStream(stem, self.env, cwd, install_stream) if install_stream is not None else None
        stderr_log = stem.with_suffix('.stderr.log') if separate_stderr else None
        if stderr_log is not None: outcome['stderrLog'] = str(stderr_log)
        # Structured output must not contain diagnostics. Both files still share
        # the original byte budget; default text retains combined stderr (codesign).
        def output_size():
            return log.stat().st_size + (stderr_log.stat().st_size if stderr_log is not None else 0)
        problems = []
        state = 'completed'
        try:
            with contextlib.ExitStack() as streams:
                output = streams.enter_context(log.open('xb'))
                error_output = streams.enter_context(stderr_log.open('xb')) if stderr_log is not None else subprocess.STDOUT
                if observer is not None: observer.start()
                child = subprocess.Popen([str(x) for x in args], cwd=cwd, env=self.env,
                                         stdout=output, stderr=error_output, start_new_session=True)
                outcome['state'] = 'running'
                started = clock.mark('popenReturned')
                deadline = started + timeout
                clock.data['deadlineMonotonic'] = deadline
                if observer is not None: observer.bind_primary(child, deadline)
                try:
                    while True:
                        clock.poll()
                        observed_code = child.poll()
                        if observed_code is not None:
                            outcome.update(state=state, exit=observed_code)
                            clock.mark('exitObserved')
                            break
                        if time.monotonic() >= deadline or output_size() > limit:
                            state = 'timeout' if time.monotonic() >= deadline else 'output-limit'
                            outcome['state'] = state
                            if state == 'timeout': clock.mark('deadlineObserved')
                            clock.mark('killRequested')
                            os.killpg(child.pid, signal.SIGKILL)
                            if observer is not None:
                                if state == 'timeout': observer.clock.mark('deadlineObserved')
                                observer.stop('deadline' if state == 'timeout' else 'primary-ended')
                            break
                        if observer is not None: observer.poll()
                        if sample is not None: sample.poll(child, time.monotonic() - started)
                        time.sleep(.1)
                    code = child.wait(timeout=10)
                    clock.mark('exitObserved')
                    clock.mark('reaped')
                    # Record execution before any fallible log/diagnostic work.
                    outcome.update(state=state, exit=code)
                    outcome['completedAt'] = now()
                    primary_finished = time.monotonic()
                    if state == 'completed' and output_size() > limit: state = 'output-limit'
                    if state == 'completed' and primary_finished > deadline: state = 'timeout'
                    outcome['state'] = state
                finally:
                    try:
                        if child.poll() is None:
                            clock.mark('killRequested')
                            os.killpg(child.pid, signal.SIGKILL)
                            cleanup_code = child.wait(timeout=10)
                            clock.mark('exitObserved')
                            clock.mark('reaped')
                            if outcome['exit'] is None: outcome['exit'] = cleanup_code
                    finally:
                        if observer is not None and not observer.finished:
                            self.observation_problems.extend(observer.finish())
                        if sample is not None:
                            try: self.observation_problems.extend(sample.finish(child))
                            except Exception as error: self.observation_problems.append('sample finalization: ' + str(error))
        except (OSError, RuntimeError, subprocess.SubprocessError) as error:
            problems.append(f'{label}: execution/log evidence: {error}')
            if outcome['state'] == 'running': outcome['state'] = 'error'
        finally:
            if observer is not None and not observer.finished:
                self.observation_problems.extend(observer.finish())
        outcome.setdefault('completedAt', now())
        clock.mark('archived')
        outcome['problems'] = problems
        try:
            write_once(stem.with_suffix('.json'), outcome)
        except OSError as error:
            problems.append(f'{label}: command evidence: {error}')
        if problems or outcome['state'] != 'completed' or (check and outcome['exit'] != 0):
            raise CommandFailure('; '.join(problems) or f"{label}: {outcome['state']}, exit={outcome['exit']}; see {log}", outcome)
        return outcome['exit'], log

    def text(self, args, *, cwd, label, timeout=30, check=True, separate_stderr=False):
        code, log = self.command(args, cwd=cwd, label=label, timeout=timeout, check=check, limit=4 * 1024 * 1024, separate_stderr=separate_stderr)
        return code, log.read_text(errors='replace')


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def file_record(path, relative_to=None):
    path = Path(path)
    digest = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            digest.update(block)
    return {'path': str(path.relative_to(relative_to) if relative_to else path),
            'bytes': path.stat().st_size, 'sha256': digest.hexdigest()}


def exact_device(devices, runtimes, runtime_version, name):
    runtime_id = 'com.apple.CoreSimulator.SimRuntime.iOS-' + runtime_version.replace('.', '-')
    runtime = [r for r in runtimes['runtimes'] if r.get('identifier') == runtime_id
               and r.get('version') == runtime_version and r.get('isAvailable')]
    if len(runtime) != 1:
        raise ValueError('Required available runtime is not unique')
    selected = [d for d in devices['devices'].get(runtime[0]['identifier'], []) if d.get('name') == name and d.get('isAvailable')]
    if len(selected) != 1 or not selected[0].get('deviceTypeIdentifier'):
        raise ValueError('Required device and hardware identity are not unique')
    return {**selected[0], 'runtimeID': runtime[0]['identifier'], 'runtimeBuild': runtime[0]['buildversion'],
            'runtime': runtime[0]['version']}


def preflight(runner, toolchain, cwd):
    expected = TOOLS[toolchain]
    _, version = runner.text(['xcodebuild', '-version'], cwd=cwd, label='xcode-version')
    _, sdk = runner.text(['xcrun', '--sdk', 'iphonesimulator', '--show-sdk-version'], cwd=cwd, label='sdk-version')
    _, sdkroot = runner.text(['xcrun', '--sdk', 'iphonesimulator', '--show-sdk-path'], cwd=cwd, label='sdk-root')
    if not re.search(r'^Xcode ' + re.escape(expected['xcode']) + r'$', version, re.M) or sdk.strip() != expected['sdk']:
        raise ValueError('Actual Xcode/SDK differs from reviewed matrix')
    _, developer = runner.text(['xcode-select', '-p'], cwd=cwd, label='developer-directory')
    developer = runner.env.get('DEVELOPER_DIR') or developer.strip()
    if not Path(developer).is_dir():
        raise ValueError('Missing actual developer directory')
    _, raw_runtimes = runner.text(['xcrun', 'simctl', 'list', '-j', 'runtimes'], cwd=cwd, label='runtimes')
    _, raw_devices = runner.text(['xcrun', 'simctl', 'list', '-j', 'devices', 'available'], cwd=cwd, label='devices')
    devices = {'phone': exact_device(json.loads(raw_devices), json.loads(raw_runtimes), expected['runtime'], 'iPhone 17')}
    if toolchain == '27':
        devices['ipad'] = exact_device(json.loads(raw_devices), json.loads(raw_runtimes), expected['runtime'], 'iPad Pro 13-inch (M5)')
    package = Path(runner.env['APPIUM_HOME']) / 'node_modules/appium-xcuitest-driver/package.json'
    script = "const{createRequire}=require('module');const p=require(process.argv[1]);const r=createRequire(process.argv[1]);const w=r.resolve('appium-webdriveragent/package.json');console.log(JSON.stringify({driver:p.version,wda:r('appium-webdriveragent/package.json').version,wdaPackage:w}));"
    _, packages = runner.text(['node', '-e', script, str(package)], cwd=cwd, label='driver-packages')
    packages = json.loads(packages)
    if any(packages[k] != expected[k] for k in ('driver', 'wda')):
        raise ValueError('Actual isolated driver/WDA differs from reviewed matrix')
    root_package_path = Path(runner.env['APPIUM_HOME']) / 'package.json'
    root_package_bytes = root_package_path.read_bytes()
    root_package = json.loads(root_package_bytes)
    if not isinstance(root_package, dict):
        raise ValueError('Installed isolated Appium root package must be an object')
    if toolchain == '26' and (root_package.get('private') is not True or root_package.get('overrides') != {
            'appium-xcuitest-driver': {'appium-webdriveragent': expected['wda']}}):
        raise ValueError('Installed isolated Appium root differs from reviewed override')
    host = {'imageOS': runner.env.get('ImageOS'), 'imageVersion': runner.env.get('ImageVersion'),
            'appiumHome': str(Path(runner.env['APPIUM_HOME']).resolve())}
    for key, command in {'osVersion': ['sw_vers', '-productVersion'], 'osBuild': ['sw_vers', '-buildVersion'],
                         'architecture': ['uname', '-m'], 'host': ['hostname'],
                         'nodeVersion': ['node', '--version'], 'rubyVersion': ['ruby', '--version']}.items():
        _, value = runner.text(command, cwd=cwd, label='host-' + key)
        host[key] = value.strip()
    match = re.search(r'^Build version (\S+)$', version, re.M)
    if not match or not all(host.values()): raise ValueError('Actual host/image/Xcode build identity missing')
    host['xcodeBuild'] = match[1]
    return {**expected, 'sdkroot': sdkroot.strip(), 'developerDir': developer, 'devices': devices,
            **host, 'wdaPackage': packages['wdaPackage'], 'driverPackage': content_record(package),
            'wdaPackageRecord': content_record(packages['wdaPackage']),
            'appiumRootPackage': {'path': str(root_package_path), 'bytes': len(root_package_bytes),
                                  'sha256': hashlib.sha256(root_package_bytes).hexdigest(), 'raw': root_package_bytes.decode()},
            'appiumRootContents': root_package}


def prepare_source(repository, source, harness_source, root, dependency_root, runner):
    destination = Path(root) / 'source' / source
    archive_source(repository, source, destination)
    mobile, harness = destination / 'mobileapp', Path(harness_source) / 'mobileapp'
    for name in ('package.json', 'package-lock.json', 'Gemfile.lock', 'ios/Podfile.lock', '.bundle/config'):
        if (mobile / name).read_bytes() != (harness / name).read_bytes():
            raise ValueError(f'{source}: committed dependency input differs: {name}')
    for name in ('node_modules', 'vendor'):
        target = Path(dependency_root) / name
        if not target.is_dir():
            raise ValueError('Missing verified job dependency: ' + str(target))
        (mobile / name).symlink_to(target, target_is_directory=True)
    observed = ('src/state/AppContext.tsx', 'src/screens/ServersScreen.tsx', 'src/push/index.ts', 'metro.config.js')
    before = {name: (mobile / name).read_text() for name in observed}
    runner.command(['python3', str(harness / 'e2e/instrument-baseline.py'), str(destination)],
                   cwd=destination, label='instrument-source', timeout=30)
    with (mobile / 'metro.config.js').open('a') as stream:
        stream.write("\nmodule.exports.watchFolders = [...(module.exports.watchFolders || []), process.env.GTMUX_COMPARE_NODE_MODULES];\nmodule.exports.resolver.nodeModulesPaths = [...(module.exports.resolver.nodeModulesPaths || []), process.env.GTMUX_COMPARE_NODE_MODULES];\n")
    edits = []
    for name, original in before.items():
        final = (mobile / name).read_text()
        edits.append({'path': name, 'beforeSHA': hashlib.sha256(original.encode()).hexdigest(),
                      'afterSHA': hashlib.sha256(final.encode()).hexdigest(),
                      'patch': ''.join(difflib.unified_diff(original.splitlines(True), final.splitlines(True), fromfile=name, tofile=name))})
    write_once(destination / 'instrumentation.json', {'source': source, 'edits': edits})
    return mobile


def prepare_work(harness_source, destination, dependencies):
    """One immutable harness source, independent writable state per journey."""
    source = Path(harness_source) / 'mobileapp'
    target = Path(destination) / 'mobileapp'
    ignore = shutil.ignore_patterns('node_modules', 'vendor', 'ios', 'android', '.e2e-artifacts', '.git', '.gradle', 'build', 'coverage')
    shutil.copytree(source, target, ignore=ignore, symlinks=True)
    manifest = []
    for path in sorted(target.rglob('*')):
        if path.is_symlink():
            raise ValueError('Unexpected link in immutable test harness')
        if path.is_file():
            relative = path.relative_to(target)
            if path.read_bytes() != (source / relative).read_bytes():
                raise ValueError('Harness bytes changed in copy')
            manifest.append(file_record(path, target))
    (target / 'node_modules').symlink_to(Path(dependencies) / 'node_modules', target_is_directory=True)
    write_once(Path(destination) / 'harness.json', manifest)
    return target


def build_product(runner, mobile, source, root):
    derived = Path(root) / 'build' / source / 'adhoc'
    runner.command(['bundle', 'exec', 'pod', 'install'], cwd=mobile / 'ios', label='pods-' + source[:8], timeout=600)
    args = ['xcodebuild', '-workspace', 'ios/GtmuxMobile.xcworkspace', '-scheme', 'GtmuxMobile',
            '-configuration', 'Release', '-sdk', 'iphonesimulator', '-destination', 'generic/platform=iOS Simulator',
            '-jobs', '2', '-derivedDataPath', str(derived), 'ARCHS=arm64', 'ONLY_ACTIVE_ARCH=NO',
            'CODE_SIGNING_ALLOWED=YES', 'CODE_SIGN_IDENTITY=-', 'build']
    _, log = runner.command(args, cwd=mobile, label='build-' + source[:8], timeout=1200)
    app = derived / 'Build/Products/Release-iphonesimulator/gtmux.app'
    bundles = [('app', app)] + [('liveActivity' if p.name == 'GtmuxWidget.appex' else 'notificationExtension' if p.name == 'GtmuxNotificationService.appex' else 'unknown', p) for p in sorted((app / 'PlugIns').glob('*.appex'))]
    products = []
    for kind, bundle in bundles:
        info = plistlib.loads((bundle / 'Info.plist').read_bytes()); binary = bundle / info['CFBundleExecutable']
        _, arch = runner.text(['lipo', '-archs', str(binary)], cwd=mobile, label=kind + '-arch')
        _, uuid = runner.text(['dwarfdump', '--uuid', str(binary)], cwd=mobile, label=kind + '-uuid')
        _, signature = runner.text(['codesign', '-dv', str(bundle)], cwd=mobile, label=kind + '-signature')
        _, entitlements = runner.text(['codesign', '-d', '--entitlements', ':-', str(bundle)], cwd=mobile, label=kind + '-entitlements')
        products.append({**file_record(binary), 'kind': kind, 'architectures': arch.strip().split(),
                         'uuid': uuid.strip(), 'deployment': info.get('MinimumOSVersion'),
                         'bundleID': info.get('CFBundleIdentifier'), 'sdk': info.get('DTSDKName'),
                         'signature': 'adhoc' if 'Signature=adhoc' in signature else 'unexpected',
                         'entitlements': entitlements, 'info': file_record(bundle / 'Info.plist')})
    build_text = log.read_text(errors='replace')
    sysroots = re.findall(r'-isysroot\s+(\S+)', build_text)
    if not sysroots:
        raise ValueError('No actual compiler sysroot in build log')
    return {'app': str(app), 'products': products, 'linkSysroot': sysroots[0], 'buildLog': file_record(log),
            'resolvedPods': content_record(mobile / 'ios/Podfile.lock')}


def parse_jest(path):
    data = json.loads(Path(path).read_text())
    cases = []
    for suite in data.get('testResults', []):
        for item in suite.get('assertionResults', []):
            cases.append({'name': item['fullName'], 'status': item['status'],
                          'startedAt': suite.get('startTime'), 'completedAt': suite.get('endTime'),
                          'failureMessages': item.get('failureMessages', [])})
    return cases


def collect_identity(runner, unit, evidence, device, product, preflight_data, installed_at, launched_at, derived):
    identity = {k: preflight_data[k] for k in ('xcode', 'sdk', 'runtime', 'driver', 'wda', 'sdkroot', 'developerDir')}
    identity.update({k: device[k] for k in ('runtimeID', 'runtimeBuild', 'udid')})
    identity.update({'source': unit['source'], 'deviceType': device['deviceTypeIdentifier'],
                     'signature': 'adhoc', 'products': product['products'], 'linkSysroot': product['linkSysroot'],
                     'installedAt': installed_at, 'journeyStartedAt': launched_at})
    session = json.loads((Path(evidence) / 'session/session.json').read_text())
    identity['sessionID'] = session['sessionID']
    capabilities = session['capabilities']
    if capabilities.get('udid', capabilities.get('appium:udid')) != device['udid']:
        raise ValueError('Actual WDA session is not attached to the selected device')
    _, launchctl = runner.text(['xcrun', 'simctl', 'spawn', device['udid'], 'launchctl', 'list'], cwd=evidence, label='process-identity')
    pids = [int(line.split()[0]) for line in launchctl.splitlines() if 'UIKitApplication:com.gtmux.app[' in line and line.split()[0].isdigit()]
    if len(pids) != 1:
        raise ValueError('Installed application PID is missing or ambiguous')
    identity['pid'] = pids[0]
    _, launched = runner.text(['ps', '-p', str(pids[0]), '-o', 'lstart='], cwd=evidence, label='actual-process-launch')
    if not launched.strip(): raise ValueError('Actual process launch time missing')
    identity['launchedAt'] = launched.strip()
    wdas = list(Path(derived).glob('Build/Products/*/WebDriverAgentRunner-Runner.app'))
    if len(wdas) != 1:
        raise ValueError('Actual built WDA runner is missing or ambiguous')
    info = plistlib.loads((wdas[0] / 'Info.plist').read_bytes())
    bundle = wdas[0] / 'PlugIns/WebDriverAgentRunner.xctest'
    test_info = plistlib.loads((bundle / 'Info.plist').read_bytes())
    binary = bundle / test_info['CFBundleExecutable']
    identity['wdaBinarySHA'] = file_record(binary)['sha256']
    identity['wdaDeployment'] = info.get('MinimumOSVersion')
    project = Path(preflight_data['wdaPackage']).parent / 'WebDriverAgent.xcodeproj'
    _, settings = runner.text(['xcodebuild', '-project', str(project), '-scheme', 'WebDriverAgentRunner', '-sdk', 'iphonesimulator',
                    '-showBuildSettings', '-json'], cwd=evidence, label='wda-build-settings', timeout=60, separate_stderr=True)
    targets = json.loads(settings)
    actual = [t['buildSettings']['IPHONEOS_DEPLOYMENT_TARGET'] for t in targets if t.get('target') == 'WebDriverAgentRunner']
    if len(actual) != 1 or not actual[0]: raise ValueError('Actual WDA target deployment is missing')
    identity['wdaTargetDeployment'] = actual[0]
    return identity


TEST_FILES = {'servers': 'server-switch-compare.test.ts', 'coldstart': 'coldstart-push.test.ts',
              'actions': 'scene-migration-actions.test.ts', 'links': 'scene-migration-links.test.ts', 'ipad': 'ipad-adaptive.test.ts'}


def cleanup_unit(runner, evidence, derived, udid):
    """End only processes bound to this unit's PID file/derived-data path."""
    problems = []
    try:
        _, processes = runner.text(['ps', '-axo', 'pid=,command='], cwd=evidence, label='owned-processes')
        rows = {}
        for row in processes.splitlines():
            parts = row.strip().split(None, 1)
            if len(parts) == 2 and parts[0].isdigit(): rows[int(parts[0])] = parts[1]
        pid_file = Path(evidence) / 'session/server.pid'
        if pid_file.exists():
            pid = int(pid_file.read_text())
            command = rows.get(pid, '')
            if command:
                if 'appium' not in command or not re.search(r'--port(?:=|\s+)4723(?:\s|$)', command):
                    raise ValueError('Saved server PID no longer identifies this unit Appium')
                try: os.killpg(pid, signal.SIGKILL)
                except ProcessLookupError: pass
        marker = '-derivedDataPath ' + str(derived)
        for pid, command in rows.items():
            if marker + ' ' in command or command.endswith(marker):
                if 'xcodebuild' not in command: raise ValueError('Unexpected command using unit WDA path')
                try: os.kill(pid, signal.SIGKILL)
                except ProcessLookupError: pass
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        problems.append('process cleanup: ' + str(error))
    try:
        deadline = time.monotonic() + 30
        code, _ = runner.command(['xcrun', 'simctl', 'shutdown', udid], cwd=evidence,
                                 label='shutdown-after', timeout=30, check=False)
        if code != 0:
            remaining = deadline - time.monotonic()
            if remaining <= 0: raise RuntimeError('shutdown verification exceeded the 30s cleanup budget')
            _, raw = runner.text(['xcrun', 'simctl', 'list', '-j', 'devices'], cwd=evidence,
                                 label='shutdown-state', timeout=remaining)
            data = json.loads(raw)
            if not isinstance(data, dict) or not isinstance(data.get('devices'), dict):
                raise ValueError('device query requires an object with a devices object')
            matches = []
            for group in data['devices'].values():
                if not isinstance(group, list): raise ValueError('device query group is not an array')
                for device in group:
                    if (not isinstance(device, dict) or not isinstance(device.get('udid'), str)
                            or not device['udid'] or not isinstance(device.get('state'), str) or not device['state']):
                        raise ValueError('device query record lacks a string UDID/state')
                    if device['udid'] == udid: matches.append(device)
            if len(matches) != 1 or matches[0].get('state') != 'Shutdown':
                raise ValueError(f'shutdown exit={code}; exact device is not verified Shutdown')
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, subprocess.SubprocessError) as error:
        problems.append('device cleanup: ' + str(error))
    return problems


def preparation_diagnostics(runner, evidence, udid):
    """Read only, after a failed simctl preparation, while its state still exists."""
    commands = [
        ('prep-host-disk', ['/bin/df', '-k', str(evidence)], 3),
        ('prep-host-processes', ['/bin/ps', '-axo', 'pid,ppid,%cpu,%mem,rss,etime,stat,comm'], 3),
        ('prep-host-log', ['/usr/bin/log', 'show', '--last', '2m', '--style', 'compact', '--predicate',
                           'process == "CoreSimulatorService" OR process == "simdiskimaged" OR process == "installd"'], 8),
        ('prep-device-log', ['xcrun', 'simctl', 'spawn', udid, 'log', 'show', '--last', '2m', '--style', 'compact',
                            '--predicate', 'process == "installd" OR process == "SpringBoard"'], 8),
    ]
    problems = []
    for label, args, timeout in commands:
        try:
            runner.command(args, cwd=evidence, label=label, timeout=timeout, limit=512 * 1024)
        except Exception as error:
            # Diagnostic failure must not erase the first failure or skip finally.
            problems.append(f'{label}: {type(error).__name__}: {error}')
    return problems


def run_unit(unit, paths, harness_source, dependencies, product, preflight_data, env, install_observation=None):
    evidence = paths['evidence']; command_root = evidence / 'commands'; command_root.mkdir()
    local_env = dict(env)
    device = preflight_data['devices']['ipad' if unit['journey'] == 'ipad' else 'phone']
    derived = paths['work'] / 'wda'
    local_env.update({'GTMUX_E2E_UDID': device['udid'], 'GTMUX_E2E_OS': unit['runtime'],
                     'GTMUX_E2E_DEVICE': device['name'], 'GTMUX_E2E_SESSION_CI': '1',
                     'GTMUX_E2E_HEADLESS': '1', 'GTMUX_E2E_SOFT_KEYBOARD': '1', 'GTMUX_E2E_ACCEPT_ALERTS': '1',
                     'GTMUX_E2E_WDA_DERIVED': str(derived), 'GTMUX_E2E_APPIUM_PORT': '4723',
                     'GTMUX_SCENE_EVIDENCE_ROOT': str(evidence), 'GTMUX_E2E_ARTIFACTS_DIR': str(evidence / 'session')})
    if unit['journey'] == 'servers':
        local_env['GTMUX_COMPARE_MANUAL_LAUNCH'] = '1'
    else:
        local_env.pop('GTMUX_COMPARE_MANUAL_LAUNCH', None)
    runner = Runner(command_root, local_env)
    result = {'unit': unit, 'state': 'not-started', 'testExit': None, 'cases': [], 'steps': [], 'identity': {}, 'artifacts': []}
    problems = []
    try:
        work = prepare_work(harness_source, paths['work'], dependencies)
        write_once(evidence / 'harness.json', json.loads((paths['work'] / 'harness.json').read_text()))
        runner.command(['xcrun', 'simctl', 'shutdown', device['udid']], cwd=work, label='shutdown-before', timeout=30, check=False)
        runner.command(['xcrun', 'simctl', 'erase', device['udid']], cwd=work, label='erase', timeout=60)
        runner.command(['xcrun', 'simctl', 'bootstatus', device['udid'], '-b'], cwd=work, label='boot', timeout=180)
        runner.command(['xcrun', 'simctl', 'install', device['udid'], product['app']], cwd=work, label='install', timeout=60,
                       sample_install=unit['toolchain'] == '27', install_stream=install_observation)
        installed = now(); launched = now()
        try:
            status, _ = runner.command(['npm', 'run', 'test:e2e', '--', '--runInBand', TEST_FILES[unit['journey']],
                                        '--json', '--outputFile', str(evidence / 'jest.json')],
                                       cwd=work, label='journey', timeout=2100, check=False)
            result['testExit'] = status; result['state'] = 'completed'
        except CommandFailure as error:
            result['testExit'] = error.outcome['exit']; result['state'] = error.outcome['state']
            problems.append(str(error))
        # Post-test evidence is isolated from the original test status. Any absent
        # evidence still makes acceptance red; never replace a test's exit7 with0.
        for label, collect in [
            ('cases', lambda: parse_jest(evidence / 'jest.json')),
            ('steps', lambda: [json.loads(s)['step'] for s in (evidence / 'session/behavior-steps.jsonl').read_text().splitlines()]),
            ('identity', lambda: collect_identity(runner, unit, evidence, device, product, preflight_data, installed, launched, derived)),
        ]:
            try: result[label] = collect()
            except (OSError, ValueError, KeyError, RuntimeError) as error: problems.append(f'{label}: {error}')
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        problems.append(str(error))
        if isinstance(error, CommandFailure) and error.outcome['command'][:2] == ['xcrun', 'simctl']:
            result['preparationFailure'] = error.outcome
            problems.extend(preparation_diagnostics(runner, evidence, device['udid']))
    finally:
        try:
            problems.extend(cleanup_unit(runner, evidence, derived, device['udid']))
        except Exception as error:
            # Cleanup is evidence, never authority to erase an executed unit.
            problems.append(f'cleanup error: {type(error).__name__}: {error}')
    problems.extend(runner.observation_problems)
    result['problems'] = problems
    try:
        for path in sorted(evidence.rglob('*')):
            try:
                if path.is_file() and not path.is_symlink():
                    result['artifacts'].append(file_record(path, evidence))
            except OSError as error:
                problems.append(f'artifact {path}: {error}')
    except OSError as error:
        problems.append(f'artifact enumeration: {error}')
    try:
        write_once(evidence / 'result.json', result)
    except OSError as error:
        problems.append(f'result archive: {error}')
        print('scene unit archive failed: ' + json.dumps(result), file=sys.stderr)
    return result


def execute(toolchain, repository, env):
    if env.get('CI') != 'true' or env.get('RUNNER_OS') != 'macOS' or env.get('RUNNER_ARCH') != 'ARM64':
        raise ValueError('Execution is restricted to the reviewed ARM64 macOS CI runner')
    event = json.loads(Path(env['GITHUB_EVENT_PATH']).read_text())
    ids = sources(env['GITHUB_EVENT_NAME'], event, env['GITHUB_SHA'])
    root = create_job_root(env['RUNNER_TEMP'], env['GITHUB_RUN_ID'], env['GITHUB_RUN_ATTEMPT'], env['GITHUB_JOB'])
    logs = root / 'commands'; logs.mkdir(); runner = Runner(logs, env)
    results, errors = [], []
    context = trusted_context(repository, env)
    role = env['GITHUB_JOB']
    if role not in ROLES or ROLES[role][0] != toolchain: raise ValueError('Unexpected execution role')
    expected = role_units(context, role)
    try:
        # A PR base remains a required readable object even when SDK27 only
        # executes the candidate. Do not pay for an unused baseline native build.
        for sha in dict.fromkeys(ids.values()):
            runner.command(['git', '-C', str(repository), 'cat-file', '-e', sha + '^{commit}'],
                           cwd=repository, label='source-readable', timeout=15)
        data = preflight(runner, toolchain, repository)
        write_once(root / 'preflight.json', data)
        harness = archive_source(repository, ids['candidate'], root / 'harness')
        checkout_mobile = Path(repository) / 'mobileapp'
        for name in ('package-lock.json', 'package.json', 'Gemfile.lock'):
            if (checkout_mobile / name).read_bytes() != (harness / 'mobileapp' / name).read_bytes():
                raise ValueError('Runner dependencies are not from the exact harness source')
        write_once(root / 'inputs.json', {name: content_record(harness / 'mobileapp' / name) for name in INPUTS})
        lock = file_record(harness / 'mobileapp/package-lock.json')['sha256']
        dependencies = root / 'deps' / lock; dependencies.mkdir(parents=True)
        for name in ('node_modules', 'vendor'):
            if not (checkout_mobile / name).is_dir(): raise ValueError('Missing preinstalled job dependency ' + name)
            (dependencies / name).symlink_to((checkout_mobile / name).resolve(), target_is_directory=True)
        # Metro must see physical symlink targets, not only the dependency alias.
        runner.env.update({'GTMUX_COMPARE_NODE_MODULES': str((dependencies / 'node_modules').resolve(strict=True)), 'GTMUX_COMPARE_JOURNEY': 'push'})
        products = {}
        for sha in dict.fromkeys(u['source'] for u in sorted(expected, key=lambda u: u['variant'] != 'candidate')):
            mobile = prepare_source(repository, sha, harness, root, dependencies, runner)
            product = build_product(runner, mobile, sha, root)
            products[sha] = product
            write_once(root / 'source' / sha / 'product.json', product)
        if len({p['resolvedPods']['sha256'] for p in products.values()}) != 1:
            raise ValueError('Resolved Pods differ between compared sources')
        for index, unit in enumerate(expected):
            observation = None
            if role == 'ios-scene' and index == 0 and unit['journey'] == 'coldstart':
                product = products[unit['source']]
                observation = {'context': context, 'role': role, 'unit': unit,
                               'udid': data['devices']['phone']['udid'], 'app': product['app'],
                               'products': product['products']}
            result = run_unit(unit, allocate_unit(root, unit), harness, dependencies, products[unit['source']], data, runner.env,
                              install_observation=observation)
            results.append(result)
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        errors.append(str(error))
    payload = {'eventName': env['GITHUB_EVENT_NAME'], 'event': event, 'githubSHA': env['GITHUB_SHA'],
               'binding': context, 'role': role, 'flags': FLAGS,
               'toolchain': toolchain, 'results': results, 'preparationErrors': errors}
    try:
        write_once(root / 'acceptance.json', payload)
    except OSError as error:
        errors.append('acceptance archive: ' + str(error))
        print('scene acceptance archive failed: ' + json.dumps(payload), file=sys.stderr)
    errors.extend(verify_all(expected, results, root / 'evidence'))
    try:
        write_once(root / 'verdict.json', {'errors': errors, 'passed': not errors})
    except OSError as error:
        errors.append('verdict archive: ' + str(error))
    try:
        publish_role(root, context, role)
    except (OSError, ValueError) as error:
        errors.append('publication archive: ' + str(error))
    for error in errors: print('scene acceptance failed:', error)
    print('Scene evidence:', root)
    return int(bool(errors))


# Fixed job names are roles, not a caller-supplied reduced acceptance matrix.
ROLES = {'ios-native': ('26', 'candidate'), 'ios-scene-baseline-26': ('26', 'baseline'),
         'ios-scene': ('27', 'candidate')}
INPUTS = ('package.json', 'package-lock.json', 'Gemfile.lock', 'ios/Podfile.lock', '.bundle/config')
FLAGS = {'configuration': 'Release', 'arch': 'arm64', 'signature': 'adhoc', 'jobs': 2,
         'instrumentation': 'instrument-baseline.py+physical-metro-v1'}


def trusted_context(repository, env):
    event = json.loads(Path(env['GITHUB_EVENT_PATH']).read_text())
    ids = sources(env['GITHUB_EVENT_NAME'], event, env['GITHUB_SHA'])
    ref = env['GITHUB_REF']
    if ((env['GITHUB_EVENT_NAME'] == 'pull_request' and not re.fullmatch(r'refs/pull/[1-9][0-9]*/merge', ref))
            or (env['GITHUB_EVENT_NAME'] == 'push' and ref != 'refs/heads/main')):
        raise ValueError('Unexpected workflow ref')
    for name in ('GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT'):
        if not re.fullmatch(r'[1-9][0-9]*', env[name]): raise ValueError('Invalid run identity')
    def git(*args):
        return subprocess.check_output(['git', '-C', str(repository), *args], timeout=15)
    if git('rev-parse', 'HEAD').decode().strip() != env['GITHUB_SHA']:
        raise ValueError('Checkout does not match trusted workflow SHA')
    # Validate every immutable source, including PR base even in candidate-only jobs.
    for sha in ids.values(): git('cat-file', '-e', sha + '^{commit}')
    blob = git('show', ids['candidate'] + ':mobileapp/e2e/ci-scene-migration.py')
    if blob != Path(__file__).read_bytes(): raise ValueError('Coordinator is not the exact candidate verifier')
    return {'eventName': env['GITHUB_EVENT_NAME'], 'ref': ref, 'runID': env['GITHUB_RUN_ID'],
            'attempt': env['GITHUB_RUN_ATTEMPT'], 'githubSHA': env['GITHUB_SHA'], 'sources': ids,
            'harnessTree': git('rev-parse', ids['candidate'] + ':mobileapp').decode().strip(),
            'coordinatorSHA256': hashlib.sha256(blob).hexdigest()}


def role_units(context, role):
    chain, variant = ROLES[role]
    if variant not in context['sources']: raise ValueError('Role is not selected for this event')
    return [u for u in units(context['sources'], chain) if u['variant'] == variant]


def selected_roles(context):
    return [role for role, (_, variant) in ROLES.items() if variant in context['sources']]


def published_files(root):
    root = Path(root)
    files = list(root.glob('*.json')) + list((root / 'commands').rglob('*')) + list((root / 'evidence').rglob('*'))
    for name in ('instrumentation.json', 'product.json'):
        files += list((root / 'source').glob('*/' + name))
    return sorted(p for p in files if p.is_file() and p.name != 'publication.json')


def publish_role(root, context, role):
    # Uploaded bytes, not Actions' warning-only digest, form the immutable inventory.
    records = [file_record(p, root) for p in published_files(root)]
    write_once(Path(root) / 'publication.json', {'binding': context, 'role': role,
               'job': role, 'toolchain': ROLES[role][0], 'files': records})


def content_record(path):
    path = Path(path); raw = path.read_bytes()
    return {'path': str(path), 'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest(), 'raw': raw.decode()}


def checked_content(record):
    if not isinstance(record, dict) or not isinstance(record.get('raw'), str) or not record.get('path'):
        raise ValueError('Missing captured dependency contents')
    raw = record['raw'].encode()
    if not raw or record['bytes'] != len(raw) or record['sha256'] != hashlib.sha256(raw).hexdigest():
        raise ValueError('Captured dependency bytes/hash mismatch')
    return record['sha256']


def comparable_identity(preflight_data, product, inputs, results):
    """Check each host before comparing stable fields. Host/PID/path identities stay independent."""
    p = preflight_data
    chain = results[0]['unit']['toolchain']
    for key, value in TOOLS[chain].items():
        if p.get(key) != value: raise ValueError('Preflight matrix mismatch: ' + key)
    required = ('osVersion', 'osBuild', 'imageOS', 'imageVersion', 'architecture', 'xcodeBuild',
                'nodeVersion', 'rubyVersion', 'sdkroot', 'developerDir', 'host', 'appiumHome')
    if any(not isinstance(p.get(k), str) or not p[k].strip() for k in required):
        raise ValueError('Host/toolchain identity missing')
    if p['architecture'] != 'arm64' or not p['nodeVersion'].startswith('v22.'):
        raise ValueError('Unreviewed architecture/Node major')
    if p['imageOS'] != ('macos15' if chain == '26' else p['imageOS']):
        raise ValueError('Unexpected SDK26 image OS')
    if not re.fullmatch(r'\d+\.\d+\.\d+', p['osVersion']): raise ValueError('Invalid actual OS version')
    if not re.fullmatch(r'\d+[A-Z]\w+', p['osBuild']): raise ValueError('Invalid actual OS build')
    if not re.fullmatch(r'\d+[A-Z]\w+', p['xcodeBuild']): raise ValueError('Invalid Xcode build')
    compare = {k: p[k] for k in required if k not in {'host', 'appiumHome'}}
    compare.update({k: p[k] for k in TOOLS[chain]})
    for key in ('driverPackage', 'wdaPackageRecord', 'appiumRootPackage'):
        record = p[key]; compare[key] = checked_content(record)
        Path(record['path']).relative_to(Path(p['appiumHome']))
    if p['wdaPackage'] != p['wdaPackageRecord']['path']: raise ValueError('Resolved WDA path mismatch')
    if json.loads(p['driverPackage']['raw'])['version'] != p['driver']: raise ValueError('Installed driver version mismatch')
    if json.loads(p['wdaPackageRecord']['raw'])['version'] != p['wda']: raise ValueError('Installed WDA version mismatch')
    root_package = json.loads(p['appiumRootPackage']['raw'])
    if root_package != p['appiumRootContents'] or not isinstance(root_package, dict): raise ValueError('Installed root content mismatch')
    if chain == '26' and (root_package.get('private') is not True or root_package.get('overrides') != {
            'appium-xcuitest-driver': {'appium-webdriveragent': p['wda']}}):
        raise ValueError('Installed root override mismatch')
    compare['inputs'] = {name: checked_content(inputs[name]) for name in INPUTS}
    compare['resolvedPods'] = checked_content(product['resolvedPods'])
    compare['flags'] = FLAGS
    compare['linkSysroot'] = product['linkSysroot']
    wda_deployments = {(r['identity']['wdaDeployment'], r['identity']['wdaTargetDeployment']) for r in results}
    if len(wda_deployments) != 1: raise ValueError('WDA deployment differs between journeys')
    compare['wdaDeployments'] = sorted(wda_deployments)
    compare['deployments'] = sorted((x['kind'], x['deployment']) for x in product['products'])
    device_fields = ('runtimeID', 'runtimeBuild', 'runtime', 'deviceTypeIdentifier')
    compare['devices'] = {k: {f: v[f] for f in device_fields} for k, v in p['devices'].items()}
    for result in results:
        identity = result['identity']; device = p['devices']['ipad' if result['unit']['journey'] == 'ipad' else 'phone']
        for key in ('runtimeID', 'runtimeBuild', 'runtime', 'udid'):
            if not device.get(key) or identity[key] != device[key]: raise ValueError('Session/device mismatch: ' + key)
        if identity['deviceType'] != device['deviceTypeIdentifier']: raise ValueError('Session hardware mismatch')
        for key in ('sdkroot', 'developerDir', *TOOLS[chain]):
            if identity[key] != p[key]: raise ValueError('Session/preflight mismatch: ' + key)
        if identity['products'] != product['products'] or identity['linkSysroot'] != product['linkSysroot']:
            raise ValueError('Session/product mismatch')
    return compare


def verify_union(directory, context):
    """Trusted current-event expectation AND every exact role, file and original unit oracle."""
    errors, compared = [], {}
    directory = Path(directory)
    roles = selected_roles(context)
    expected_names = {f'scene-migration-{role}-{context["runID"]}-{context["attempt"]}' for role in roles}
    if not directory.is_dir() or {p.name for p in directory.iterdir()} != expected_names:
        return ['Missing, unexpected or duplicate role artifact']
    for role in roles:
        try:
            artifact = directory / f'scene-migration-{role}-{context["runID"]}-{context["attempt"]}'
            roots = list(artifact.iterdir())
            expected_root = f'{context["runID"]}-{context["attempt"]}-{role}'
            if len(roots) != 1 or roots[0].name != expected_root or not roots[0].is_dir():
                raise ValueError('Ambiguous job artifact root')
            root = roots[0]
            if any(p.is_symlink() for p in [artifact, root, *root.rglob('*')]): raise ValueError('Linked artifact')
            publication = json.loads((root / 'publication.json').read_text())
            if (publication['binding'] != context or publication['role'] != role or publication['job'] != role
                    or publication['toolchain'] != ROLES[role][0]): raise ValueError('Untrusted role/run/source binding')
            files = publication['files']
            paths = [r['path'] for r in files]
            actual = {str(p.relative_to(root)) for p in root.rglob('*') if p.is_file() and p != root / 'publication.json'}
            if len(set(paths)) != len(paths) or set(paths) != actual: raise ValueError('Incomplete/duplicate artifact inventory')
            for record in files:
                path = Path(record['path'])
                if path.is_absolute() or '..' in path.parts: raise ValueError('Unsafe inventory path')
                if record != file_record(root / path, root): raise ValueError('Artifact bytes/hash mismatch')
            payload = json.loads((root / 'acceptance.json').read_text())
            verdict = json.loads((root / 'verdict.json').read_text())
            if payload['binding'] != context or payload['role'] != role or payload['toolchain'] != ROLES[role][0]:
                raise ValueError('Acceptance binding mismatch')
            if payload['preparationErrors'] or verdict != {'errors': [], 'passed': True}:
                raise ValueError('Role preparation/execution/diagnostic failed')
            expected = role_units(context, role)
            role_errors = verify_all(expected, payload['results'], root / 'evidence')
            if role_errors: raise ValueError('; '.join(role_errors))
            source = expected[0]['source']
            product = json.loads((root / 'source' / source / 'product.json').read_text())
            instrument = json.loads((root / 'source' / source / 'instrumentation.json').read_text())
            if instrument['source'] != source or payload['flags'] != FLAGS: raise ValueError('Instrumentation/source mismatch')
            if {e['path'] for e in instrument['edits']} != {'src/state/AppContext.tsx', 'src/screens/ServersScreen.tsx', 'src/push/index.ts', 'metro.config.js'}:
                raise ValueError('Instrumentation edits missing')
            compared[role] = comparable_identity(json.loads((root / 'preflight.json').read_text()), product,
                                                  json.loads((root / 'inputs.json').read_text()), payload['results'])
        except (OSError, ValueError, KeyError, TypeError, IndexError, AttributeError) as error:
            errors.append(role + ': ' + str(error))
    if 'ios-scene-baseline-26' in roles:
        if not all(r in compared for r in ('ios-native', 'ios-scene-baseline-26')):
            errors.append('Cross-host SDK26 comparison incomplete')
        elif compared['ios-native'] != compared['ios-scene-baseline-26']:
            errors.append('Cross-host SDK26 identity/inputs/resolved Pods mismatch')
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--verify', type=Path)
    parser.add_argument('--verify-union', type=Path)
    parser.add_argument('--execute', choices=tuple(TOOLS))
    parser.add_argument('--repository', type=Path, default=Path.cwd())
    parser.add_argument('--evidence-root', type=Path)
    args = parser.parse_args()
    if args.verify_union:
        if args.execute or args.verify or args.evidence_root: parser.error('Union verification is separate')
        try:
            errors = verify_union(args.verify_union, trusted_context(args.repository.resolve(), os.environ))
        except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
            errors = [str(error)]
        for error in errors: print('scene union failed:', error)
        return int(bool(errors))
    if args.execute:
        if args.verify or args.evidence_root: parser.error('Execution and verification are separate')
        return execute(args.execute, args.repository.resolve(), os.environ)
    if not args.verify or not args.evidence_root: parser.error('Verification requires input and evidence root')
    try:
        data = json.loads(args.verify.read_text())
        identities = sources(data['eventName'], data['event'], data['githubSHA'])
        expected = units(identities, data['toolchain'])
        errors = list(data.get('preparationErrors', []))
        errors.extend(verify_all(expected, data['results'], args.evidence_root))
    except (OSError, ValueError, KeyError, TypeError) as error:
        errors = [str(error)]
    for error in errors:
        print('scene acceptance failed:', error)
    return int(bool(errors))


if __name__ == '__main__':
    raise SystemExit(main())
