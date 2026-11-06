import copy
import hashlib
import contextlib
import io
import importlib.util
import json
import os
import sys
import time
from unittest import mock
from pathlib import Path
import subprocess
import tempfile
import unittest

PATH = Path(__file__).resolve().parents[1] / 'mobileapp/e2e/ci-scene-migration.py'
spec = importlib.util.spec_from_file_location('scene_migration', PATH)
scene = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scene)
B, P, MERGE = 'a' * 40, 'b' * 40, 'c' * 40


class SceneMigrationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='scene-migration-controls-')
        self.root = Path(self.temp.name)

    def tearDown(self):
        self.temp.cleanup()

    def result(self, unit):
        root = self.root / scene.tuple_key(unit)
        root.mkdir(parents=True, exist_ok=True)
        data = b'isolated synthetic acceptance evidence\n'
        names = ['evidence.txt', 'jest.json', 'harness.json', 'session/session.json', 'session/behavior-steps.jsonl', 'session/view.png', 'session/view.xml', 'session/journey-fake.json', 'session/journey-native.jsonl', 'session/host-geometry.json']
        for name in names:
            (root / name).parent.mkdir(parents=True, exist_ok=True); (root / name).write_bytes(data)
        identity = {**scene.TOOLS[unit['toolchain']], 'source': unit['source'], 'signature': 'adhoc',
                    'pid': 123, 'runtimeID': 'runtime', 'runtimeBuild': 'build', 'udid': 'test-udid',
                    'deviceType': 'test-phone', 'sdkroot': 'iphonesimulator' + scene.TOOLS[unit['toolchain']]['sdk'], 'linkSysroot': '/SDK/iPhoneSimulator' + scene.TOOLS[unit['toolchain']]['sdk'] + '.sdk',
                    'developerDir': 'xcode', 'wdaDeployment': '15.0', 'wdaTargetDeployment': '15.0', 'wdaBinarySHA': 'd' * 64,
                    'sessionID': 'session', 'installedAt': 'time1', 'launchedAt': 'time2',
                    'products': [{'kind': kind, 'architectures': ['arm64'], 'signature': 'adhoc',
                                  'sha256': 'e' * 64, 'sdk': 'iphonesimulator' + scene.TOOLS[unit['toolchain']]['sdk'], 'uuid': 'uuid', 'deployment': '15.1', 'entitlements': {}}
                                 for kind in ('app', 'liveActivity', 'notificationExtension')]}
        return {'unit': unit, 'state': 'completed', 'testExit': 0, 'identity': identity,
                'cases': [{'name': n, 'status': 'passed', 'startedAt': 'time1', 'completedAt': 'time2'}
                          for n in scene.CASES[unit['journey']]],
                'steps': list(scene.STEPS[unit['journey']]),
                'artifacts': [{'path': name, 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()} for name in names]}

    def test_real_event_sources_and_fixed_PR15_main11_instances(self):
        event = {'pull_request': {'base': {'sha': B}, 'head': {'sha': P}}}
        ids = scene.sources('pull_request', event, MERGE)
        self.assertEqual(ids, {'baseline': B, 'candidate': P})
        self.assertNotIn(MERGE, ids.values())
        both = scene.units(ids, '26') + scene.units(ids, '27')
        self.assertEqual(sum(len(scene.CASES[u['journey']]) for u in both), 15)
        main = scene.sources('push', {'ref': 'refs/heads/main', 'after': MERGE}, MERGE)
        candidate = scene.units(main, '26') + scene.units(main, '27')
        self.assertEqual(sum(len(scene.CASES[u['journey']]) for u in candidate), 11)
        self.assertTrue(all(u['variant'] == 'candidate' for u in candidate))
        self.assertEqual(scene.TOOLS['26']['xcode'], '26.3')
        self.assertEqual(scene.TOOLS['26']['sdk'], '26.2')

    def test_missing_bad_base_and_actualmain_mismatch_fail(self):
        for base in ('', 'main', '0' * 40):
            with self.subTest(base=base), self.assertRaises(ValueError):
                scene.sources('pull_request', {'pull_request': {'base': {'sha': base}, 'head': {'sha': P}}}, MERGE)
        with self.assertRaises(ValueError):
            scene.sources('push', {'ref': 'refs/heads/main', 'after': P}, MERGE)
        with self.assertRaises(ValueError):
            scene.sources('push', {'ref': 'refs/heads/unreviewed', 'after': P}, P)

    def test_each_source_and_journey_has_exclusive_work_and_evidence(self):
        root = scene.create_job_root(self.root, 'run1', '1', 'ios')
        units = scene.units({'baseline': B, 'candidate': P}, '26')
        allocated = [scene.allocate_unit(root, u) for u in units]
        self.assertEqual(len({str(p) for a in allocated for p in a.values()}), 8)
        for i, dirs in enumerate(allocated):
            scene.write_once(dirs['evidence'] / 'status.json', {'testExit': 7 if i == 0 else 0})
        self.assertEqual(json.loads((allocated[0]['evidence'] / 'status.json').read_text())['testExit'], 7)
        with self.assertRaises(FileExistsError):
            scene.allocate_unit(root, units[0])
        with self.assertRaises(FileExistsError):
            scene.create_job_root(self.root, 'run1', '1', 'ios')
        with self.assertRaises(FileExistsError):
            scene.write_once(allocated[0]['evidence'] / 'status.json', {'testExit': 0})
        with self.assertRaises(ValueError):
            scene.create_job_root(self.root, '../escape', '1', 'ios')

    def test_real_git_archive_from_root_and_mobileapp_rejects_unreadable_base(self):
        repo = self.root / 'repo'; repo.mkdir(); (repo / 'mobileapp').mkdir()
        (repo / 'mobileapp/proof.txt').write_text('exact source\n')
        def git(*args):
            return subprocess.check_output(['git', '-c', 'user.email=fixture@example.test', '-c', 'user.name=fixture',
                                            '-c', 'core.hooksPath=/dev/null', *args], cwd=repo, stderr=subprocess.STDOUT).decode().strip()
        git('init', '-q'); git('add', 'mobileapp/proof.txt'); git('commit', '-qm', 'fixture')
        sha = git('rev-parse', 'HEAD')
        for name, cwd in [('root', repo), ('child', repo / 'mobileapp')]:
            with self.subTest(cwd=name):
                dest = scene.archive_source(cwd, sha, self.root / name)
                self.assertEqual((dest / 'mobileapp/proof.txt').read_text(), 'exact source\n')
        with self.assertRaises(subprocess.CalledProcessError):
            scene.archive_source(repo / 'mobileapp', 'f' * 40, self.root / 'missing')

    def test_complete_synthetic_matrix_and_missing_unit_fail_closed(self):
        units = scene.units({'baseline': B, 'candidate': P}, '26')
        results = [self.result(u) for u in units]
        self.assertEqual(scene.verify_all(units, results, self.root), [])
        for damaged in (results[:-1], results + [results[0]]):
            self.assertTrue(scene.verify_all(units, damaged, self.root))
        bad = copy.deepcopy(results); bad[0]['testExit'] = 7
        self.assertTrue(scene.verify_all(units, bad, self.root))
        self.assertEqual(results[-1]['testExit'], 0)  # a later green does not clear7

    def test_offline_verifier_retains_preparation_failure_with_complete_case_results(self):
        units = scene.units({'candidate': P}, '26')
        results = [self.result(u) for u in units]
        report = self.root / 'acceptance.json'
        payload = {'eventName': 'push', 'event': {'ref': 'refs/heads/main', 'after': P},
                   'githubSHA': P, 'toolchain': '26', 'results': results, 'preparationErrors': []}
        arguments = ['scene', '--verify', str(report), '--evidence-root', str(self.root)]
        report.write_text(json.dumps(payload))
        with mock.patch.object(sys, 'argv', arguments): self.assertEqual(scene.main(), 0)
        payload['preparationErrors'] = ['source verification failed']
        report.write_text(json.dumps(payload))
        with mock.patch.object(sys, 'argv', arguments): self.assertEqual(scene.main(), 1)

    def test_missing_skipped_failed_cancelled_case_or_early_return_never_passes(self):
        unit = scene.units({'candidate': P}, '27')[1]
        original = self.result(unit)
        for status in ('skipped', 'pending', 'todo', 'failed', 'cancelled'):
            bad = copy.deepcopy(original); bad['cases'][0]['status'] = status
            with self.subTest(status=status):
                self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        for field in ('cases', 'steps', 'artifacts'):
            bad = copy.deepcopy(original); bad[field] = [] if field == 'artifacts' else bad[field][1:]
            with self.subTest(field=field):
                self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        bad = copy.deepcopy(original); bad['cases'][0].pop('completedAt')
        self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        bad = copy.deepcopy(original); bad['state'] = 'cancelled'
        self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))

    def test_wrong_or_missing_toolchain_WDA_signature_and_product_identity_fail(self):
        unit = scene.units({'candidate': P}, '27')[0]; original = self.result(unit)
        fields = ('xcode', 'sdk', 'runtime', 'driver', 'wda', 'wdaDeployment', 'wdaBinarySHA', 'udid', 'pid', 'signature')
        for field in fields:
            bad = copy.deepcopy(original); bad['identity'].pop(field)
            with self.subTest(missing=field):
                self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        bad = copy.deepcopy(original); bad['identity']['linkSysroot'] = '/wrong/iPhoneSimulator26.3.sdk'
        self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        bad = copy.deepcopy(original); bad['identity']['sdk'] = '26.3'
        self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        bad = copy.deepcopy(original); bad['identity']['signature'] = 'unsigned'
        self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        bad = copy.deepcopy(original); bad['identity']['products'].pop()
        self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))

    def test_evidence_byte_changes_and_escape_are_rejected(self):
        unit = scene.units({'candidate': P}, '27')[0]; result = self.result(unit)
        root = self.root / scene.tuple_key(unit)
        original = (root / 'evidence.txt').read_bytes()
        (root / 'evidence.txt').write_text('replacement')
        self.assertTrue(scene.unit_errors(unit, result, root))
        (root / 'evidence.txt').write_bytes(original)
        self.assertEqual(scene.unit_errors(unit, result, root), [])
        (root / 'linked.txt').symlink_to(root / 'evidence.txt')
        result['artifacts'][0]['path'] = 'linked.txt'
        self.assertTrue(scene.unit_errors(unit, result, root))
        (root.parent / 'outside.txt').write_bytes(original)
        result['artifacts'][0]['path'] = '../outside.txt'
        self.assertTrue(scene.unit_errors(unit, result, root))




class SceneExecutionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='scene-execution-controls-')
        self.root = Path(self.temp.name)

    def tearDown(self):
        self.temp.cleanup()

    def test_execute_refuses_local_environment_before_any_native_command(self):
        with mock.patch.object(scene, 'preflight') as preflight:
            with self.assertRaisesRegex(ValueError, 'restricted'):
                scene.execute('27', self.root, {'CI': 'false'})
            preflight.assert_not_called()

    def test_real_small_child_exit_timeout_and_output_cap_are_bounded(self):
        runner = scene.Runner(self.root, dict(os.environ))
        code, _ = runner.command([sys.executable, '-c', 'raise SystemExit(7)'], cwd=self.root, label='exit7', timeout=5, check=False)
        self.assertEqual(code, 7)
        started = time.monotonic()
        with self.assertRaisesRegex(RuntimeError, 'timeout'):
            runner.command([sys.executable, '-c', 'import time; time.sleep(30)'], cwd=self.root, label='timeout', timeout=.2)
        self.assertLess(time.monotonic() - started, 3)
        with self.assertRaisesRegex(RuntimeError, 'output-limit'):
            runner.command([sys.executable, '-c', 'print("x" * 2048)'], cwd=self.root, label='output', timeout=5, limit=1024)
        records = [json.loads(p.read_text()) for p in self.root.glob('*.json')]
        self.assertEqual({r['state'] for r in records}, {'completed', 'timeout', 'output-limit'})

    def test_device_lookup_requires_actual_runtime_and_unique_hardware(self):
        runtime = {'runtimes': [{'identifier': 'ios-26', 'version': '26.2', 'buildversion': 'fixture', 'isAvailable': True}]}
        device = {'udid': 'fixture-device', 'deviceTypeIdentifier': 'phone-type', 'name': 'iPhone 17', 'isAvailable': True}
        devices = {'devices': {'ios-26': [device]}}
        self.assertEqual(scene.exact_device(devices, runtime, '26.2', 'iPhone 17')['runtimeBuild'], 'fixture')
        with self.assertRaises(ValueError): scene.exact_device(devices, runtime, '26.3', 'iPhone 17')
        devices['devices']['ios-26'].append(device)
        with self.assertRaises(ValueError): scene.exact_device(devices, runtime, '26.2', 'iPhone 17')

    def test_missing_diagnostic_evidence_keeps_original_test_zero_or_seven_and_cleans(self):
        for status in (0, 7):
            for error in (OSError('mkdir failed'), OSError('write failed')):
                with self.subTest(status=status, error=error):
                    root = self.root / str(status) / str(error); root.mkdir(parents=True)
                    unit = scene.units({'candidate': P}, '27')[0]
                    paths = {k: root / k for k in ('work', 'evidence')}
                    for p in paths.values(): p.mkdir()
                    (paths['work'] / 'harness.json').write_text('[]')
                    class FakeRunner:
                        def __init__(self, *args): pass
                        def command(self, args, **kwargs):
                            return (status if kwargs['label'] == 'journey' else 0), root / 'synthetic.log'
                    with mock.patch.object(scene, 'Runner', FakeRunner), mock.patch.object(scene, 'prepare_work', return_value=paths['work']), \
                         mock.patch.object(scene, 'parse_jest', return_value=[]), mock.patch.object(scene, 'collect_identity', side_effect=error), \
                         mock.patch.object(scene, 'cleanup_unit', return_value=[]) as cleanup:
                        result = scene.run_unit(unit, paths, root, root, {'app': '/synthetic/app'},
                              {'devices': {'phone': {'udid': 'fixture', 'name': 'iPhone 17'}}}, {})
                    self.assertEqual(result['testExit'], status)
                    self.assertTrue(any(str(error) in p for p in result['problems']))
                    cleanup.assert_called_once()
                    self.assertEqual(json.loads((paths['evidence'] / 'result.json').read_text())['testExit'], status)

    def controlled_execute(self, toolchain='27', event_name='pull_request', status=0, fault=None, cleanup_raw=None):
        # Execute the production coordinator, Runner and run_unit. Only external
        # Git/native/SDK/Jest commands become tiny real children, not a Runner mock.
        root = self.root / f'execute-{len(list(self.root.iterdir()))}'; root.mkdir()
        repo = root / 'repository'; mobile = repo / 'mobileapp'; mobile.mkdir(parents=True)
        for name in ('package-lock.json', 'package.json', 'Gemfile.lock'): (mobile / name).write_text('{}')
        for name in ('node_modules', 'vendor'): (mobile / name).mkdir()
        event = {'pull_request': {'base': {'sha': B}, 'head': {'sha': P}}} if event_name == 'pull_request' else {'ref': 'refs/heads/main', 'after': P}
        event_path = root / 'event.json'; event_path.write_text(json.dumps(event))
        env = {**os.environ, 'CI': 'true', 'RUNNER_OS': 'macOS', 'RUNNER_ARCH': 'ARM64',
               'GITHUB_EVENT_NAME': event_name, 'GITHUB_EVENT_PATH': str(event_path), 'GITHUB_SHA': MERGE if event_name == 'pull_request' else P,
               'RUNNER_TEMP': str(root), 'GITHUB_RUN_ID': '1', 'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_JOB': 'test'}
        builds, readable = [], []
        real_popen, real_write, real_record = scene.subprocess.Popen, scene.write_once, scene.file_record
        real_stat = Path.stat
        post_wait, stat_faults = set(), []
        cleanup_fault = fault in {'last-shutdown', 'cleanup-data', 'query-exit', 'query-timeout', 'query-io', 'cleanup-exception'}
        def child(args, **kwargs):
            script = 'pass'
            if args[0] == 'git':
                readable.append(args[-1][:40])
                if fault == 'unreadable-base' and args[-1].startswith(B): script = 'raise SystemExit(1)'
            if args[0] == 'npm': script = f'raise SystemExit({status})'
            if cleanup_fault:
                if args[:3] == ['xcrun', 'simctl', 'shutdown'] and '/ipad/' in kwargs['stdout'].name and kwargs['stdout'].name.endswith('-shutdown-after.log'):
                    script = 'raise SystemExit(7)'
                if args[:4] == ['xcrun', 'simctl', 'list', '-j']:
                    if fault in {'query-timeout', 'query-io'}:
                        if fault == 'query-timeout': raise subprocess.TimeoutExpired(args, 1)
                        raise OSError('injected query IO')
                    if fault == 'query-exit': script = 'raise SystemExit(7)'
                    else: script = 'print(' + repr(cleanup_raw if fault == 'cleanup-data' else '{"devices":{"runtime":[{"udid":"pad","state":"Booted"}]}}') + ')'
            process = real_popen([sys.executable, '-c', script], **kwargs)
            if fault == 'wait-stat' or cleanup_fault:
                # Fast, actual completed child; Runner still calls the real wait.
                process.wait(timeout=5)
            if args[0] == 'npm' and fault == 'wait-stat':
                original_wait = process.wait
                def wait(*a, **k):
                    code = original_wait(*a, **k)
                    post_wait.add(kwargs['stdout'].name)
                    return code
                process.wait = wait
            return process
        def stat(path, *a, **k):
            if str(path) in post_wait:
                post_wait.remove(str(path)); stat_faults.append(str(path))
                raise OSError('injected post-wait log.stat')
            return real_stat(path, *a, **k)
        actual_cleanup = scene.cleanup_unit
        def cleanup(*a):
            if fault == 'cleanup-exception' and '/ipad' in str(a[1]): raise AttributeError('injected cleanup boundary')
            return actual_cleanup(*a) if cleanup_fault else []
        def archive(repository, sha, destination):
            (destination / 'mobileapp').mkdir(parents=True)
            for name in ('package-lock.json', 'package.json', 'Gemfile.lock'): (destination / 'mobileapp' / name).write_text('{}')
            return destination
        def prepare(repository, sha, harness, job, deps, runner):
            target = job / 'source' / sha / 'mobileapp'; target.mkdir(parents=True); return target
        def build(runner, target, sha, job):
            builds.append(sha); return {'app': '/synthetic/app', 'resolvedPods': {'sha256': 'same'}}
        def work(harness, destination, deps):
            scene.write_once(destination / 'harness.json', []); return destination
        def write(path, value):
            if ((fault == 'metadata' and Path(path).name.endswith('-journey.json'))
                    or (fault == 'result' and Path(path).name == 'result.json')
                    or (fault == 'acceptance' and Path(path).name == 'acceptance.json')
                    or (fault == 'archives' and Path(path).name in {'result.json', 'acceptance.json', 'verdict.json'})):
                raise OSError('injected ' + fault)
            return real_write(path, value)
        def record(path, relative_to=None):
            if fault == 'artifact' and Path(path).name.endswith('-journey.log'): raise OSError('injected artifact read/hash')
            return real_record(path, relative_to)
        stderr = io.StringIO()
        with mock.patch.object(Path, 'stat', new=stat), mock.patch.object(scene.subprocess, 'Popen', side_effect=child), \
                mock.patch.object(scene, 'preflight', return_value={'devices': {'phone': {'udid': 'fixture', 'name': 'phone'}, 'ipad': {'udid': 'pad', 'name': 'pad'}}}), \
                mock.patch.object(scene, 'archive_source', side_effect=archive), mock.patch.object(scene, 'prepare_source', side_effect=prepare), \
                mock.patch.object(scene, 'build_product', side_effect=build), mock.patch.object(scene, 'prepare_work', side_effect=work), \
                mock.patch.object(scene, 'parse_jest', return_value=[]), mock.patch.object(scene, 'collect_identity', return_value={}), \
                mock.patch.object(scene, 'cleanup_unit', side_effect=cleanup), mock.patch.object(scene, 'write_once', side_effect=write), \
                mock.patch.object(scene, 'file_record', side_effect=record), contextlib.redirect_stderr(stderr), contextlib.redirect_stdout(io.StringIO()):
            code = scene.execute(toolchain, repo, env)
        if fault == 'wait-stat': self.assertEqual(len(stat_faults), 4)
        job = root / 'scene-migration/1-1-test'
        payload = json.loads((job / 'acceptance.json').read_text()) if (job / 'acceptance.json').exists() else None
        return code, builds, readable, payload, stderr.getvalue()

    def test_real_runner_wait_then_stat_failure_preserves_unit_and_aggregate(self):
        for status in (0, 7):
            with self.subTest(status=status):
                code, _, _, payload, _ = self.controlled_execute(status=status, fault='wait-stat')
                self.assertEqual(code, 1); self.assertEqual(len(payload['results']), 4)
                for result in payload['results']:
                    self.assertEqual((result['testExit'], result['state']), (status, 'completed'))
                    self.assertTrue(any('post-wait log.stat' in p for p in result['problems']))

    def test_cleanup_bad_query_shapes_and_errors_keep_final_executed_unit(self):
        malformed = [None, [], {'devices': None}, {'devices': []}, {'devices': {'r': None}},
                     {'devices': {'r': {}}}, {'devices': {'r': [None]}}, {'devices': {'r': [1]}},
                     {'devices': {'r': [{}]}}, {'devices': {'r': [{'udid': 1, 'state': 'Shutdown'}]}},
                     {'devices': {'r': [{'udid': 'pad', 'state': None}]}}]
        cases = [('cleanup-data', json.dumps(x), False) for x in malformed]
        cases += [('cleanup-data', '{', False), ('query-exit', None, False), ('query-timeout', None, False),
                  ('query-io', None, False), ('cleanup-exception', None, False),
                  ('cleanup-data', json.dumps({'devices': {'r': [{'udid': 'pad', 'state': 'Shutdown'}]}}), True),
                  ('last-shutdown', None, False)]
        for status in (0, 7):
            for fault, raw, allowed in cases:
                with self.subTest(status=status, fault=fault, raw=raw):
                    code, _, _, payload, _ = self.controlled_execute(status=status, fault=fault, cleanup_raw=raw)
                    self.assertEqual(code, 1)  # missing synthetic case identity also keeps acceptance red
                    self.assertEqual(len(payload['results']), 4)
                    final = payload['results'][-1]
                    self.assertEqual((final['testExit'], final['state']), (status, 'completed'))
                    self.assertEqual(any('cleanup' in p for p in final['problems']), not allowed)
                    self.assertFalse(any('cleanup' in p for row in payload['results'][:-1] for p in row['problems']))

    def test_runner_distinguishes_not_started_from_started_evidence_failure(self):
        runner = scene.Runner(self.root, dict(os.environ))
        with mock.patch.object(scene.subprocess, 'Popen', side_effect=OSError('spawn unavailable')):
            with self.assertRaises(scene.CommandFailure) as stopped:
                runner.command(['unused'], cwd=self.root, label='not-started', timeout=1)
        self.assertEqual((stopped.exception.outcome['state'], stopped.exception.outcome['exit']), ('not-started', None))
        real_stat = Path.stat
        def stat(path, *a, **k):
            if path.name.endswith('-started.log'): raise OSError('polling evidence unreadable')
            return real_stat(path, *a, **k)
        with mock.patch.object(Path, 'stat', new=stat):
            with self.assertRaises(scene.CommandFailure) as stopped:
                runner.command([sys.executable, '-c', 'import time; time.sleep(5)'], cwd=self.root, label='started', timeout=1)
        self.assertEqual(stopped.exception.outcome['state'], 'error')
        self.assertEqual(stopped.exception.outcome['exit'], -9)

    def test_executor_builds_only_expected_sources_but_checks_real_base(self):
        for chain, event, wanted in [('26', 'pull_request', [P, B]), ('27', 'pull_request', [P]),
                                     ('26', 'push', [P]), ('27', 'push', [P])]:
            with self.subTest(chain=chain, event=event):
                _, builds, readable, payload, _ = self.controlled_execute(chain, event)
                self.assertEqual(builds, wanted)
                self.assertEqual(set(readable), {P, B} if event == 'pull_request' else {P})
                self.assertEqual(len(payload['results']), len(scene.units(scene.sources(event, payload['event'], payload['githubSHA']), chain)))
        code, builds, readable, payload, _ = self.controlled_execute(fault='unreadable-base')
        self.assertEqual(code, 1); self.assertEqual(builds, []); self.assertIn(B, readable)
        self.assertTrue(payload['preparationErrors'])

    def test_real_runner_metadata_and_archival_failures_preserve_executed_units(self):
        for status in (0, 7):
            for fault in ('metadata', 'artifact', 'result', 'acceptance', 'archives'):
                with self.subTest(status=status, fault=fault):
                    code, _, _, payload, stderr = self.controlled_execute(status=status, fault=fault)
                    self.assertEqual(code, 1)
                    if fault in {'acceptance', 'archives'}:
                        line = next(line for line in stderr.splitlines() if line.startswith('scene acceptance archive failed: '))
                        payload = json.loads(line.split(': ', 1)[1])
                        self.assertIn('acceptance archive: injected ' + fault, payload['preparationErrors'])
                    self.assertEqual(len(payload['results']), 4)
                    for result in payload['results']:
                        self.assertEqual((result['testExit'], result['state']), (status, 'completed'))
                        if fault != 'acceptance': self.assertTrue(any('injected ' + fault in p for p in result['problems']))
                        if fault == 'artifact': self.assertTrue(any(a['path'].endswith('-journey.json') for a in result['artifacts']))
                    if fault == 'result': self.assertIn('scene unit archive failed:', stderr)

    def test_real_executor_keeps_last_shutdown_failure_and_original_exit(self):
        for status in (0, 7):
            code, _, _, payload, _ = self.controlled_execute(status=status, fault='last-shutdown')
            self.assertEqual(code, 1)
            self.assertEqual(len(payload['results']), 4)
            self.assertFalse(any('device cleanup:' in p for r in payload['results'][:-1] for p in r['problems']))
            self.assertTrue(any('device cleanup:' in p for p in payload['results'][-1]['problems']))
            self.assertEqual(payload['results'][-1]['testExit'], status)

    def test_shutdown_failure_or_timeout_of_final_unit_cannot_pass(self):
        # Existing full acceptance fixture makes every other criterion green.
        units = scene.units({'candidate': P}, '27')
        results = [SceneMigrationTests.result(self, u) for u in units]
        for state, code, timeout, accepted in [('Shutdown', 1, False, True), ('Booted', 1, False, False),
                                              ('missing', 1, False, False), ('duplicate', 1, False, False),
                                              ('Shutdown', 1, True, False), ('Booted', 0, False, True)]:
            with self.subTest(state=state, code=code, timeout=timeout):
                runner = mock.Mock()
                devices = [] if state == 'missing' else [{'udid': 'fixture', 'state': state}]
                if state == 'duplicate': devices *= 2
                runner.text.side_effect = [(0, ''), (0, json.dumps({'devices': {'runtime': devices}}))]
                if timeout: runner.command.side_effect = scene.CommandFailure('timeout', {'state': 'timeout', 'exit': -9})
                else: runner.command.return_value = (code, self.root / 'shutdown.log')
                problems = scene.cleanup_unit(runner, self.root, self.root / 'wda', 'fixture')
                self.assertEqual(not problems, accepted)
                for status in (0, 7):
                    final = copy.deepcopy(results); final[-1]['problems'] = problems; final[-1]['testExit'] = status
                    errors = scene.verify_all(units, final, self.root)
                    self.assertEqual(not errors, accepted and status == 0)
                    self.assertEqual(final[-1]['testExit'], status)
        for elapsed, accepted in [(29.5, True), (30, False)]:
            runner = mock.Mock(); runner.command.return_value = (1, self.root / 'shutdown.log')
            runner.text.side_effect = [(0, ''), (0, json.dumps({'devices': {'runtime': [{'udid': 'fixture', 'state': 'Shutdown'}]}}))]
            with mock.patch.object(scene.time, 'monotonic', side_effect=[100, 100 + elapsed]):
                problems = scene.cleanup_unit(runner, self.root, self.root / 'wda', 'fixture')
            self.assertEqual(not problems, accepted)
            if accepted: self.assertEqual(runner.text.call_args.kwargs['timeout'], .5)
            else: self.assertEqual(runner.text.call_count, 1)  # process query only, no late device probe

    def test_harness_journeys_share_only_verified_dependencies_and_keep_source_bytes(self):
        source = self.root / 'harness/mobileapp'; (source / 'e2e').mkdir(parents=True)
        (source / 'e2e/case.ts').write_text('frozen fixture source')
        (source / 'package-lock.json').write_text('frozen dependency input')
        dependencies = self.root / 'dependencies'; (dependencies / 'node_modules').mkdir(parents=True)
        first = self.root / 'one'; first.mkdir()
        second = self.root / 'two'; second.mkdir()
        one = scene.prepare_work(source.parent, first, dependencies)
        two = scene.prepare_work(source.parent, second, dependencies)
        self.assertEqual((one / 'node_modules').resolve(), (two / 'node_modules').resolve())
        self.assertEqual(json.loads((first / 'harness.json').read_text()), json.loads((second / 'harness.json').read_text()))
        (one / 'e2e/case.ts').write_text('one journey temporary state')
        self.assertEqual((two / 'e2e/case.ts').read_text(), 'frozen fixture source')
        self.assertEqual((source / 'e2e/case.ts').read_text(), 'frozen fixture source')
        (source / 'e2e/linked.ts').symlink_to(source / 'e2e/case.ts')
        bad = self.root / 'bad'; bad.mkdir()
        with self.assertRaisesRegex(ValueError, 'Unexpected link'):
            scene.prepare_work(source.parent, bad, dependencies)

    def test_cleanup_targets_only_saved_matching_server_and_exact_unit_wda(self):
        (self.root / 'session').mkdir(); (self.root / 'session/server.pid').write_text('41')
        derived = self.root / 'unit-wda'
        fake = mock.Mock()
        fake.command.return_value = (0, self.root / 'shutdown.log')
        fake.text.return_value = (0, f'41 node appium --port 4723\n42 xcodebuild -derivedDataPath {derived} test\n43 xcodebuild -derivedDataPath {derived}-other test\n44 appium --port 4730')
        with mock.patch.object(scene.os, 'killpg') as groups, mock.patch.object(scene.os, 'kill') as singles:
            self.assertEqual(scene.cleanup_unit(fake, self.root, derived, 'fixture-device'), [])
            groups.assert_called_once_with(41, scene.signal.SIGKILL); singles.assert_called_once_with(42, scene.signal.SIGKILL)
        fake.text.return_value = (0, '41 unrelated-process')
        with mock.patch.object(scene.os, 'killpg') as groups:
            self.assertTrue(scene.cleanup_unit(fake, self.root, derived, 'fixture-device'))
            groups.assert_not_called()
        self.assertEqual(fake.command.call_args.args[0][-1], 'fixture-device')

if __name__ == '__main__':
    unittest.main()
