import copy
import hashlib
import contextlib
import io
import importlib.util
import json
import os
import plistlib
import shutil
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


    def test_collect_identity_parses_only_stdout_with_retained_real_child_diagnostics(self):
        unit = scene.units({'candidate': P}, '26')[0]
        evidence = self.root / 'identity'; (evidence / 'session').mkdir(parents=True)
        (evidence / 'session/session.json').write_text(json.dumps({'sessionID': 'session-123', 'capabilities': {'udid': 'device-123'}}))
        derived = self.root / 'derived'
        app = derived / 'Build/Products/Debug-iphonesimulator/WebDriverAgentRunner-Runner.app'
        bundle = app / 'PlugIns/WebDriverAgentRunner.xctest'; bundle.mkdir(parents=True)
        (app / 'Info.plist').write_bytes(plistlib.dumps({'MinimumOSVersion': '15.0'}))
        (bundle / 'Info.plist').write_bytes(plistlib.dumps({'CFBundleExecutable': 'WDA'}))
        binary = b'controlled WDA binary'; (bundle / 'WDA').write_bytes(binary)
        device = {'runtimeID': 'runtime26', 'runtimeBuild': 'build26', 'udid': 'device-123', 'deviceTypeIdentifier': 'phone'}
        product = {'products': ['app', 'extension', 'widget'], 'linkSysroot': 'sdk26.2'}
        preflight = {**scene.TOOLS['26'], 'sdkroot': 'sdk26.2', 'developerDir': 'xcode26', 'wdaPackage': str(self.root / 'wda/package.json')}
        payload = json.dumps([{'target': 'WebDriverAgentRunner', 'buildSettings': {'IPHONEOS_DEPLOYMENT_TARGET': '15.0'}}])
        warning = 'xcodebuild: WARNING: Using the first of multiple matching destinations\n'
        real_popen = subprocess.Popen
        for name, stdout, stderr, exit_code, error in (
                ('valid', payload, warning, 0, None),
                ('prefix', warning + payload, '', 0, json.JSONDecodeError),
                ('stderr-only', '', payload, 0, json.JSONDecodeError),
                ('exit7', payload, warning, 7, scene.CommandFailure)):
            with self.subTest(name=name):
                commands = self.root / name; commands.mkdir()
                runner = scene.Runner(commands, os.environ)
                def native_boundary(args, **kwargs):
                    if args[0] == 'xcodebuild':
                        self.assertEqual(args[-2:], ['-showBuildSettings', '-json'])
                        out, err, code = stdout, stderr, exit_code
                    elif args[0] == 'ps': out, err, code = 'Fri Nov 7 00:00:00 2026', '', 0
                    else:
                        self.assertEqual(args[:4], ['xcrun', 'simctl', 'spawn', 'device-123'])
                        out, err, code = '123 0 UIKitApplication:com.gtmux.app[fixture]\n', '', 0
                    child_code = 'import sys; sys.stdout.write(' + repr(out) + '); sys.stderr.write(' + repr(err) + '); sys.exit(' + str(code) + ')'
                    return real_popen([sys.executable, '-c', child_code], **kwargs)
                with mock.patch.object(scene.subprocess, 'Popen', side_effect=native_boundary):
                    if error:
                        with self.assertRaises(error):
                            scene.collect_identity(runner, unit, evidence, device, product, preflight, 'installed', 'journey', derived)
                    else:
                        actual = scene.collect_identity(runner, unit, evidence, device, product, preflight, 'installed', 'journey', derived)
                        self.assertEqual(actual['wdaTargetDeployment'], '15.0')
                        self.assertEqual(actual['sessionID'], 'session-123')
                        self.assertEqual(actual['pid'], 123)
                        self.assertEqual(actual['launchedAt'], 'Fri Nov 7 00:00:00 2026')
                        self.assertEqual(actual['wdaBinarySHA'], hashlib.sha256(binary).hexdigest())
                record = json.loads((commands / '003-wda-build-settings.json').read_text())
                self.assertEqual(record['exit'], exit_code)
                self.assertEqual(record['state'], 'completed')
                self.assertEqual(Path(record['log']).read_text(), stdout)
                self.assertEqual(Path(record['stderrLog']).read_text(), stderr)

    def test_split_stream_runner_preserves_exit_and_combined_default(self):
        runner = scene.Runner(self.root, os.environ)
        for code in (0, 7):
            program = 'import sys; print("stdout"); print("warning",file=sys.stderr); sys.exit(' + str(code) + ')'
            got, text = runner.text([sys.executable, '-c', program], cwd=self.root, label='split-' + str(code),
                                    check=False, separate_stderr=True)
            self.assertEqual(got, code); self.assertEqual(text, 'stdout\n')
            record = json.loads((self.root / f'{runner.sequence:03d}-split-{code}.json').read_text())
            self.assertEqual(record['exit'], code)
            self.assertEqual(Path(record['stderrLog']).read_text(), 'warning\n')
        _, combined = runner.text([sys.executable, '-c', program.replace('sys.exit(7)', 'sys.exit(0)')], cwd=self.root, label='default')
        self.assertIn('stdout\n', combined); self.assertIn('warning\n', combined)
        self.assertNotIn('stderrLog', json.loads((self.root / '003-default.json').read_text()))

    def test_split_stream_runner_bounds_total_bytes_and_timeout(self):
        runner = scene.Runner(self.root, os.environ)
        for name, program, limit, timeout, state in (
                ('stderr-cap', 'import os; os.write(2,b"x"*256)', 128, 5, 'output-limit'),
                ('total-cap', 'import os; os.write(1,b"x"*96); os.write(2,b"y"*96)', 128, 5, 'output-limit'),
                ('timeout', 'import time; time.sleep(5)', 128, .15, 'timeout')):
            with self.subTest(name=name):
                start = time.monotonic()
                with self.assertRaises(scene.CommandFailure) as raised:
                    runner.command([sys.executable, '-c', program], cwd=self.root, label=name, timeout=timeout,
                                   limit=limit, separate_stderr=True)
                self.assertLess(time.monotonic() - start, 3)
                self.assertEqual(raised.exception.outcome['state'], state)
                self.assertIsInstance(raised.exception.outcome['exit'], int)
                self.assertTrue(Path(raised.exception.outcome['stderrLog']).is_file())

    def test_device_lookup_requires_actual_runtime_and_unique_hardware(self):
        runtime = {'runtimes': [{'identifier': 'com.apple.CoreSimulator.SimRuntime.iOS-26-2', 'version': '26.2', 'buildversion': 'fixture', 'isAvailable': True}]}
        device = {'udid': 'fixture-device', 'deviceTypeIdentifier': 'phone-type', 'name': 'iPhone 17', 'isAvailable': True}
        devices = {'devices': {'com.apple.CoreSimulator.SimRuntime.iOS-26-2': [device]}}
        self.assertEqual(scene.exact_device(devices, runtime, '26.2', 'iPhone 17')['runtimeBuild'], 'fixture')
        with self.assertRaises(ValueError): scene.exact_device(devices, runtime, '26.3', 'iPhone 17')
        devices['devices']['com.apple.CoreSimulator.SimRuntime.iOS-26-2'].append(device)
        with self.assertRaises(ValueError): scene.exact_device(devices, runtime, '26.2', 'iPhone 17')

    def test_device_lookup_separates_same_version_platforms_without_fallback(self):
        ios_id = 'com.apple.CoreSimulator.SimRuntime.iOS-27-0'
        ios = {'identifier': ios_id, 'version': '27.0', 'isAvailable': True, 'buildversion': '24A434'}
        foreign = [{**ios, 'identifier': ios_id.replace('iOS-', family + '-')}
                   for family in ('tvOS', 'watchOS', 'xrOS')]
        devices = {'devices': {ios_id: [
            {'name': name, 'udid': name, 'deviceTypeIdentifier': kind, 'isAvailable': True}
            for name, kind in [('iPhone 17', 'phone'), ('iPad Pro 13-inch (M5)', 'ipad')]]}}
        # Foreign runtimes first: selecting the first version match is not valid.
        runtimes = {'runtimes': [*foreign, ios]}
        for name in ('iPhone 17', 'iPad Pro 13-inch (M5)'):
            selected = scene.exact_device(devices, runtimes, '27.0', name)
            self.assertEqual((selected['runtimeID'], selected['runtimeBuild'], selected['udid']),
                             (ios_id, '24A434', name))
        invalid = [foreign, [*foreign, {**ios, 'isAvailable': False}],
                   [*foreign, {**ios, 'version': '27.1'}], [*foreign, ios, ios],
                   [*foreign, {**ios, 'identifier': ios_id + '-different-device'}]]
        for values in invalid:
            with self.subTest(runtimes=values), self.assertRaises(ValueError):
                scene.exact_device(devices, {'runtimes': values}, '27.0', 'iPhone 17')
        self.assertEqual(scene.TOOLS['27']['runtime'], '27.0')
        self.assertEqual(scene.TOOLS['26']['runtime'], '26.2')

    def test_preflight_preserves_installed_appium_root_and_rejects_missing_or_changed_override(self):
        for chain in ('26', '27'):
            expected = scene.TOOLS[chain]
            root = self.root / ('appium-' + chain); root.mkdir()
            home = root / 'isolated'; driver = home / 'node_modules/appium-xcuitest-driver'
            wda = driver / 'node_modules/appium-webdriveragent'; wda.mkdir(parents=True)
            (driver / 'package.json').write_text(json.dumps({'name': 'appium-xcuitest-driver', 'version': expected['driver']}))
            (wda / 'package.json').write_text(json.dumps({'name': 'appium-webdriveragent', 'version': expected['wda']}))
            package = home / 'package.json'
            valid = {'private': True, 'overrides': {'appium-xcuitest-driver': {'appium-webdriveragent': expected['wda']}},
                     'dependencies': {'appium-xcuitest-driver': expected['driver']}}
            runtime_id = 'com.apple.CoreSimulator.SimRuntime.iOS-' + expected['runtime'].replace('.', '-')
            runtimes = {'runtimes': [{'identifier': runtime_id, 'version': expected['runtime'], 'buildversion': 'fixture', 'isAvailable': True}]}
            devices = {'devices': {runtime_id: [{'name': name, 'udid': name, 'deviceTypeIdentifier': 'fixture-hardware', 'isAvailable': True}
                                               for name in ('iPhone 17', 'iPad Pro 13-inch (M5)')]}}
            outputs = {('xcodebuild', '-version'): 'Xcode ' + expected['xcode'] + '\nBuild version 17C100',
                       ('xcrun', '--sdk', 'iphonesimulator', '--show-sdk-version'): expected['sdk'],
                       ('xcrun', '--sdk', 'iphonesimulator', '--show-sdk-path'): '/fixture/sdk',
                       ('xcode-select', '-p'): str(root),
                       ('sw_vers', '-productVersion'): '15.8.2', ('sw_vers', '-buildVersion'): '24H232',
                       ('uname', '-m'): 'arm64', ('hostname',): 'fixture-host', ('ruby', '--version'): 'ruby 3.3.0',
                       ('xcrun', 'simctl', 'list', '-j', 'runtimes'): json.dumps(runtimes),
                       ('xcrun', 'simctl', 'list', '-j', 'devices', 'available'): json.dumps(devices)}
            real_popen = subprocess.Popen
            def child(args, **kwargs):
                if args[0] == 'node':
                    return real_popen(args, **kwargs)  # Actual createRequire and installed fixture package resolution.
                return real_popen([sys.executable, '-c', 'print(' + repr(outputs[tuple(args)]) + ')'], **kwargs)
            variants = [('valid', valid), ('missing', None), ('malformed', '{'), ('null', 'null'), ('array', '[]')]
            if chain == '26':
                variants += [('wrong-wda', {**valid, 'overrides': {'appium-xcuitest-driver': {'appium-webdriveragent': '15.1.0'}}}),
                             ('not-private', {**valid, 'private': False})]
            for index, (label, content) in enumerate(variants):
                with self.subTest(chain=chain, fault=label):
                    if content is None:
                        package.unlink()  # Owned temporary fixture only.
                    else:
                        package.write_text(content if isinstance(content, str) else json.dumps(content))
                    logs = root / str(index); logs.mkdir()
                    runner = scene.Runner(logs, {**os.environ, 'APPIUM_HOME': str(home), 'DEVELOPER_DIR': str(root), 'ImageOS': 'macos15', 'ImageVersion': '20261104.0514.1'})
                    with mock.patch.object(scene.subprocess, 'Popen', side_effect=child):
                        if label == 'valid':
                            data = scene.preflight(runner, chain, root)
                            self.assertEqual(data['appiumRootContents'], valid)
                            self.assertEqual(data['appiumRootPackage'], scene.content_record(package))
                            self.assertEqual((data['driver'], data['wda']), (expected['driver'], expected['wda']))
                        else:
                            with self.assertRaises((ValueError, OSError)):
                                scene.preflight(runner, chain, root)

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
                        def __init__(self, *args): self.observation_problems = []
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

    def controlled_execute(self, toolchain='27', event_name='pull_request', status=0, fault=None, cleanup_raw=None, dependency_observations=None, install_observations=None, role=None, stream_observations=None):
        # Execute the production coordinator, Runner and run_unit. Only external
        # Git/native/SDK/Jest commands become tiny real children, not a Runner mock.
        root = self.root / f'execute-{len(list(self.root.iterdir()))}'; root.mkdir()
        repo = root / 'repository'; mobile = repo / 'mobileapp'; mobile.mkdir(parents=True)
        for name in ('package-lock.json', 'package.json', 'Gemfile.lock'): (mobile / name).write_text('{}')
        (mobile / 'vendor').mkdir()
        if dependency_observations is None:
            (mobile / 'node_modules').mkdir()
        else:
            physical = root / 'physical-dependencies'; physical.mkdir()
            (physical / 'proof.txt').write_text('verified fixture dependency')
            link = root / 'dependency-link'; link.symlink_to(physical, target_is_directory=True)
            (mobile / 'node_modules').symlink_to(link, target_is_directory=True)
            dependency_observations.update(physical=str(physical.resolve()), builds=[], child_logs=[])
        event = {'pull_request': {'base': {'sha': B}, 'head': {'sha': P}}} if event_name == 'pull_request' else {'ref': 'refs/heads/main', 'after': P}
        event_path = root / 'event.json'; event_path.write_text(json.dumps(event))
        env = {**os.environ, 'CI': 'true', 'RUNNER_OS': 'macOS', 'RUNNER_ARCH': 'ARM64',
               'GITHUB_EVENT_NAME': event_name, 'GITHUB_EVENT_PATH': str(event_path), 'GITHUB_SHA': MERGE if event_name == 'pull_request' else P,
               'RUNNER_TEMP': str(root), 'GITHUB_RUN_ID': '1', 'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_JOB': role or ('ios-native' if toolchain == '26' else 'ios-scene')}
        builds, readable = [], []
        ready, release = root/'stream-bound', root/'stream-release'
        original_bind, original_finish = scene.InstallStream.bind_primary, scene.InstallStream.finish
        def bind(stream,*args):
            original_bind(stream,*args); ready.write_text('bound')
        def finish(stream):
            if stream_observations and stream_observations['mode']=='final-exit' and stream.child is not None:
                release.write_text('exit now'); stream.child.wait(timeout=2)
            return original_finish(stream)
        real_popen, real_write, real_record = scene.subprocess.Popen, scene.write_once, scene.file_record
        real_stat = Path.stat
        post_wait, stat_faults = set(), []
        cleanup_fault = fault in {'last-shutdown', 'cleanup-data', 'query-exit', 'query-timeout', 'query-io', 'cleanup-exception'}
        def child(args, **kwargs):
            script = 'pass'
            stdout_name = getattr(kwargs['stdout'], 'name', '')
            if fault == 'first-boot' and args[:3] == ['xcrun','simctl','bootstatus'] and '/coldstart/' in stdout_name:
                script = 'raise SystemExit(7)'
            if args[0] == 'git':
                readable.append(args[-1][:40])
                if fault == 'unreadable-base' and args[-1].startswith(B): script = 'raise SystemExit(1)'
            if args[0] == 'npm':
                script = f'raise SystemExit({status})'
                if dependency_observations is not None:
                    # This reads the environment in the actual child spawned by
                    # Runner, after run_unit's per-journey environment handling.
                    script = ("import json, os; from pathlib import Path; "
                              "p=Path(os.environ['GTMUX_COMPARE_NODE_MODULES']); "
                              "print(json.dumps({'root': str(p), 'proof': (p/'proof.txt').read_text()})); "
                              f"raise SystemExit({status})")
                    dependency_observations['child_logs'].append(stdout_name)
            if cleanup_fault:
                if args[:3] == ['xcrun', 'simctl', 'shutdown'] and '/ipad/' in stdout_name and stdout_name.endswith('-shutdown-after.log'):
                    script = 'raise SystemExit(7)'
                if args[:4] == ['xcrun', 'simctl', 'list', '-j']:
                    if fault in {'query-timeout', 'query-io'}:
                        if fault == 'query-timeout': raise subprocess.TimeoutExpired(args, 1)
                        raise OSError('injected query IO')
                    if fault == 'query-exit': script = 'raise SystemExit(7)'
                    else: script = 'print(' + repr(cleanup_raw if fault == 'cleanup-data' else '{"devices":{"runtime":[{"udid":"pad","state":"Booted"}]}}') + ')'
            if fault in {'last-install', 'last-install-diagnostic-exit', 'last-install-diagnostic-write', 'last-install-diagnostic-cap'}:
                if args[:3] == ['xcrun', 'simctl', 'install'] and '/ipad/' in stdout_name:
                    script = 'raise SystemExit(7)'
                if '-prep-host-disk.log' in stdout_name:
                    if fault == 'last-install-diagnostic-exit': script = 'raise SystemExit(9)'
                    if fault == 'last-install-diagnostic-cap': script = 'print("x" * (600 * 1024))'
            if install_observations is not None:
                if args[:3] == ['xcrun', 'simctl', 'install']:
                    script = 'import time; time.sleep(.4)'
                elif args[0] == '/usr/bin/sample':
                    target = install_observations['primary'][int(args[1])]
                    self.assertIsNone(target.poll(), 'sample must target the still-live install child')
                    self.assertEqual(args[2:5], ['1', '10', '-file'])
                    install_observations['calls'].append(list(args))
                    script = 'from pathlib import Path; Path(' + repr(args[5]) + ').write_text("controlled stack")'
                    if fault == 'sample-exit': script += '; raise SystemExit(9)'
            if stream_observations is not None:
                if args[0]=='npm':
                    # Isolate diagnostic propagation; empty synthetic steps remain
                    # insufficient for the actual behavior gate, but are readable.
                    script=("import os; from pathlib import Path; p=Path(os.environ['GTMUX_SCENE_EVIDENCE_ROOT'])/'session'; "
                            "p.mkdir(exist_ok=True); (p/'behavior-steps.jsonl').write_text(''); "+script)
                if args[:3]==['xcrun','simctl','install'] and '/coldstart/' in stdout_name:
                    script='import time; time.sleep(.4)'
                if args[:3]==['xcrun','simctl','spawn'] and '/usr/bin/log' in args:
                    mode=stream_observations['mode']
                    script=synthetic_stream_script('valid' if mode=='final-exit' else mode,ready,
                                                   release if mode=='final-exit' else None)
            process = real_popen([sys.executable, '-c', script], **kwargs)
            if install_observations is not None:
                if args[:3] == ['xcrun', 'simctl', 'install']: install_observations['primary'][process.pid] = process
                if args[0] == '/usr/bin/sample': install_observations['children'].append(process)
            if fault == 'wait-stat' or cleanup_fault:
                # Fast, actual completed child; Runner still calls the real wait.
                process.wait(timeout=5)
            if args[0] == 'npm' and fault == 'wait-stat':
                original_wait = process.wait
                def wait(*a, **k):
                    code = original_wait(*a, **k)
                    post_wait.add(stdout_name)
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
            return actual_cleanup(*a) if cleanup_fault or str(fault).startswith('last-install') else []
        def archive(repository, sha, destination):
            (destination / 'mobileapp').mkdir(parents=True)
            for name in scene.INPUTS:
                path = destination / 'mobileapp' / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_text('{}')
            return destination
        def prepare(repository, sha, harness, job, deps, runner):
            target = job / 'source' / sha / 'mobileapp'; target.mkdir(parents=True); return target
        def build(runner, target, sha, job):
            if dependency_observations is not None:
                dependency_observations['builds'].append({'source': sha, 'root': runner.env['GTMUX_COMPARE_NODE_MODULES']})
            builds.append(sha); return {'app': '/synthetic/app', 'products':[{'kind':'app','sha256':'d'*64}], 'resolvedPods': {'sha256': 'same'}}
        def work(harness, destination, deps):
            scene.write_once(destination / 'harness.json', []); return destination
        def write(path, value):
            if ((fault == 'metadata' and Path(path).name.endswith('-journey.json'))
                    or (fault == 'result' and Path(path).name == 'result.json')
                    or (fault == 'acceptance' and Path(path).name == 'acceptance.json')
                    or (fault == 'archives' and Path(path).name in {'result.json', 'acceptance.json', 'verdict.json'})
                    or (fault == 'last-install-diagnostic-write' and Path(path).name.endswith('-prep-host-disk.json'))):
                raise OSError('injected ' + fault)
            return real_write(path, value)
        def record(path, relative_to=None):
            if fault == 'artifact' and Path(path).name.endswith('-journey.log'): raise OSError('injected artifact read/hash')
            return real_record(path, relative_to)
        stderr = io.StringIO()
        with mock.patch.object(scene, 'trusted_context', return_value={'sources': scene.sources(event_name, event, env['GITHUB_SHA'])}), \
                mock.patch.object(Path, 'stat', new=stat), mock.patch.object(scene.subprocess, 'Popen', side_effect=child), \
                mock.patch.object(scene, 'preflight', return_value={'devices': {'phone': {'udid': 'fixture', 'name': 'phone'}, 'ipad': {'udid': 'pad', 'name': 'pad'}}}), \
                mock.patch.object(scene, 'archive_source', side_effect=archive), mock.patch.object(scene, 'prepare_source', side_effect=prepare), \
                mock.patch.object(scene, 'build_product', side_effect=build), mock.patch.object(scene, 'prepare_work', side_effect=work), \
                mock.patch.object(scene, 'parse_jest', return_value=[]), mock.patch.object(scene, 'collect_identity', return_value={}), \
                mock.patch.object(scene, 'cleanup_unit', side_effect=cleanup), mock.patch.object(scene, 'write_once', side_effect=write), \
                mock.patch.object(scene, 'file_record', side_effect=record), \
                mock.patch.object(scene.InstallStream,'bind_primary',new=bind), mock.patch.object(scene.InstallStream,'finish',new=finish), contextlib.redirect_stderr(stderr), contextlib.redirect_stdout(io.StringIO()):
            code = scene.execute(toolchain, repo, env)
        if fault == 'wait-stat': self.assertEqual(len(stat_faults), 4)
        job = root / ('scene-migration/1-1-' + env['GITHUB_JOB'])
        payload = json.loads((job / 'acceptance.json').read_text()) if (job / 'acceptance.json').exists() else None
        return code, builds, readable, payload, stderr.getvalue()

    def test_install_sample_observes_owned_live_child_once_without_changing_exit(self):
        for status in (0, 7):
            with self.subTest(status=status), mock.patch.object(scene.InstallSample, 'AFTER', .05):
                root = self.root / str(status); root.mkdir()
                runner = scene.Runner(root, os.environ)
                real_popen = subprocess.Popen
                primary, observers = [], []
                def spawn(args, **kwargs):
                    if args[0] == '/usr/bin/sample':
                        self.assertEqual(args[1], str(primary[0].pid))
                        self.assertIsNone(primary[0].poll())
                        self.assertEqual(args[2:5], ['1', '10', '-file'])
                        child = real_popen([sys.executable, '-c', 'from pathlib import Path; Path(' + repr(args[5]) + ').write_text("controlled stack")'], **kwargs)
                        observers.append(child); return child
                    child = real_popen(args, **kwargs); primary.append(child); return child
                with mock.patch.object(scene.subprocess, 'Popen', side_effect=spawn):
                    code, _ = runner.command([sys.executable, '-c', f'import time; time.sleep(.4); raise SystemExit({status})'],
                                             cwd=root, label='install', timeout=2, check=False, sample_install=True)
                self.assertEqual(code, status); self.assertEqual(len(observers), 1)
                self.assertTrue(all(c.poll() is not None for c in observers + primary))
                self.assertEqual(runner.observation_problems, [])
                record = json.loads((root / '001-install.sample.json').read_text())
                self.assertEqual((record['primaryPID'], record['observerPID'], record['state'], record['exit']),
                                 (primary[0].pid, observers[0].pid, 'completed', 0))
                self.assertEqual(Path(record['stack']).read_text(), 'controlled stack')
                events = record['clock']['events']
                self.assertLessEqual(events['commandRequested']['monotonic'],events['popenReturned']['monotonic'])
                self.assertLessEqual(events['exitObserved']['monotonic'],events['reaped']['monotonic'])
                self.assertLessEqual(events['reaped']['monotonic'],events['archived']['monotonic'])
                self.assertEqual(json.loads((root / '001-install.json').read_text())['exit'], status)
        runner = scene.Runner(self.root, os.environ)
        with mock.patch.object(scene.subprocess, 'Popen', wraps=subprocess.Popen) as spawn:
            runner.command([sys.executable, '-c', 'pass'], cwd=self.root, label='quick', timeout=3, sample_install=True)
        self.assertEqual(spawn.call_count, 1)
        self.assertFalse((self.root / '001-quick.sample.json').exists())

    def test_install_sample_faults_keep_primary_outcome_and_reap_observer(self):
        for fault in ('exit', 'hang', 'cap', 'archive', 'stat', 'startup', 'stop'):
            for status in (0, 7):
                with self.subTest(fault=fault, status=status):
                    root = self.root / f'{fault}-{status}'; root.mkdir()
                    runner = scene.Runner(root, os.environ)
                    real_popen, real_write, real_stat, real_kill = subprocess.Popen, scene.write_once, Path.stat, os.killpg
                    observers, stop_errors = [], []
                    def spawn(args, **kwargs):
                        if args[0] != '/usr/bin/sample': return real_popen(args, **kwargs)
                        if fault == 'startup': raise OSError('sample startup failure')
                        script = 'from pathlib import Path; Path(' + repr(args[5]) + ').write_text("controlled stack")'
                        if fault == 'exit': script += '; raise SystemExit(9)'
                        if fault in ('hang', 'stop', 'stat'): script += '; import time; time.sleep(30)'
                        if fault == 'cap': script += '; print("x" * 4096, flush=True); import time; time.sleep(30)'
                        child = real_popen([sys.executable, '-c', script], **kwargs); observers.append(child); return child
                    def write(path, value):
                        if fault == 'archive' and str(path).endswith('.sample.json'): raise OSError('sample archive failure')
                        return real_write(path, value)
                    def stat(path, *a, **k):
                        if fault == 'stat' and str(path).endswith('.sample.txt'): raise OSError('sample stat failure')
                        return real_stat(path, *a, **k)
                    def kill(pid, sig):
                        if fault == 'stop' and observers and pid == observers[0].pid and not stop_errors:
                            stop_errors.append(pid); raise OSError('injected sample stop failure')
                        return real_kill(pid, sig)
                    with mock.patch.object(scene.InstallSample, 'AFTER', .05), mock.patch.object(scene.InstallSample, 'TIMEOUT', .2 if fault in ('hang', 'stop') else 3), \
                            mock.patch.object(scene.InstallSample, 'LIMIT', 1024), mock.patch.object(scene.subprocess, 'Popen', side_effect=spawn), \
                            mock.patch.object(scene, 'write_once', side_effect=write), mock.patch.object(Path, 'stat', new=stat), \
                            mock.patch.object(scene.os, 'killpg', side_effect=kill):
                        code, _ = runner.command([sys.executable, '-c', f'import time; time.sleep(.4); raise SystemExit({status})'],
                                                 cwd=root, label='install', timeout=2, check=False, sample_install=True)
                    self.assertEqual(code, status)
                    self.assertEqual(json.loads((root / '001-install.json').read_text())['exit'], status)
                    self.assertTrue(runner.observation_problems)
                    record_path = root / '001-install.sample.json'
                    if fault == 'archive':
                        self.assertFalse(record_path.exists())
                        self.assertTrue(any('sample archive failure' in p for p in runner.observation_problems))
                    else:
                        record = json.loads(record_path.read_text())
                        expected = {'exit': 'completed', 'hang': 'timeout', 'cap': 'output-limit',
                                    'stat': 'error', 'startup': 'error', 'stop': 'error'}[fault]
                        self.assertEqual(record['state'], expected)
                        if fault == 'exit': self.assertEqual(record['exit'], 9)
                        if fault == 'hang':
                            events = record['clock']['events']
                            self.assertLessEqual(events['deadlineObserved']['monotonic'],events['killRequested']['monotonic'])
                            self.assertLessEqual(events['killRequested']['monotonic'],events['exitObserved']['monotonic'])
                            self.assertLessEqual(events['exitObserved']['monotonic'],events['reaped']['monotonic'])
                    self.assertTrue(all(c.poll() is not None for c in observers), 'no observer survives Runner')
        root = self.root / 'primary-timeout'; root.mkdir()
        runner = scene.Runner(root, os.environ)
        # An observer that cannot start must not replace the primary timeout/-9.
        real_popen = subprocess.Popen
        def spawn(args, **kwargs):
            if args[0] == '/usr/bin/sample': raise OSError('controlled observation unavailable')
            return real_popen(args, **kwargs)
        with mock.patch.object(scene.InstallSample, 'AFTER', .05), mock.patch.object(scene.subprocess, 'Popen', side_effect=spawn):
            with self.assertRaises(scene.CommandFailure) as caught:
                runner.command([sys.executable, '-c', 'import time; time.sleep(30)'], cwd=root, label='install', timeout=.25, sample_install=True)
        self.assertEqual((caught.exception.outcome['state'], caught.exception.outcome['exit']), ('timeout', -9))
        self.assertTrue(runner.observation_problems)

    def test_install_exit_during_observation_keeps_completion_and_reaps_on_wait_error(self):
        for wait_error in (False, True):
            root = self.root / str(wait_error); root.mkdir()
            runner = scene.Runner(root, os.environ)
            real_popen = subprocess.Popen
            observers = []
            def spawn(args, **kwargs):
                if args[0] != '/usr/bin/sample': return real_popen(args, **kwargs)
                script = 'from pathlib import Path; Path(' + repr(args[5]) + ').write_text("controlled stack"); import time; time.sleep(.25)'
                child = real_popen([sys.executable, '-c', script], **kwargs)
                observers.append(child)
                if wait_error:
                    original_wait = child.wait
                    def wait(*a, **k):
                        original_wait(*a, **k)
                        raise subprocess.TimeoutExpired(args, 1)
                    child.wait = wait
                return child
            with self.subTest(wait_error=wait_error), mock.patch.object(scene.InstallSample, 'AFTER', .05), \
                    mock.patch.object(scene.InstallSample, 'TIMEOUT', .6), mock.patch.object(scene.subprocess, 'Popen', side_effect=spawn):
                code, _ = runner.command([sys.executable, '-c', 'import time; time.sleep(.15)'],
                                         cwd=root, label='install', timeout=2, sample_install=True)
            primary = json.loads((root / '001-install.json').read_text())
            sample = json.loads((root / '001-install.sample.json').read_text())
            self.assertEqual((code, primary['exit'], primary['state']), (0, 0, 'completed'))
            self.assertLess(primary['completedAt'], sample['completedAt'])
            self.assertEqual(len(observers), 1)
            self.assertIsNotNone(observers[0].poll())
            self.assertEqual(any('sample reap' in p for p in runner.observation_problems), wait_error)

    def test_actual_units_keep_install_sample_diagnostics_separate_from_journey_exit(self):
        for toolchain, fault, status in (('27', None, 0), ('27', 'sample-exit', 0), ('27', 'sample-exit', 7), ('26', None, 0)):
            observations = {'primary': {}, 'children': [], 'calls': []}
            # The exact pre-change module has no sampler; this inert timing knob
            # lets its actual run_unit execute and fail the missing-evidence oracle.
            observer_type = getattr(scene, 'InstallSample', type('AbsentObserver', (), {'AFTER': 15}))
            with self.subTest(toolchain=toolchain, fault=fault, status=status), mock.patch.object(observer_type, 'AFTER', .05):
                code, _, _, payload, _ = self.controlled_execute(toolchain=toolchain, fault=fault, status=status, install_observations=observations)
            self.assertEqual(code, 1)  # Synthetic identity/behavior is intentionally incomplete.
            self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
            self.assertEqual(len(observations['calls']), 4 if toolchain == '27' else 0)
            self.assertTrue(all(c.poll() is not None for c in observations['children']))
            for result in payload['results']:
                self.assertEqual((result['state'], result['testExit']), ('completed', status))
                self.assertEqual(any('sample did not produce' in p for p in result['problems']), fault == 'sample-exit')
                samples = [a for a in result['artifacts'] if a['path'].endswith('.sample.json')]
                self.assertEqual(len(samples), 1 if toolchain == '27' else 0)
            # Last-unit observation failure stays in the acceptance aggregate.
            if fault: self.assertTrue(any('sample did not produce' in p for p in payload['results'][-1]['problems']))

    def test_final_preparation_failure_keeps_original_outcome_and_bounds_diagnostics(self):
        actual_command = scene.Runner.command
        for fault in ('last-install', 'last-install-diagnostic-exit', 'last-install-diagnostic-write', 'last-install-diagnostic-cap'):
            with self.subTest(fault=fault), mock.patch.object(scene.Runner, 'command', autospec=True, side_effect=actual_command) as commands:
                code, _, _, payload, _ = self.controlled_execute(fault=fault, status=7)
                self.assertEqual(code, 1)
                self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
                self.assertTrue(all(r['testExit'] == 7 for r in payload['results'][:-1]))
                final = payload['results'][-1]
                self.assertEqual((final['state'], final['testExit']), ('not-started', None))
                self.assertEqual(final['preparationFailure']['exit'], 7)
                self.assertEqual(final['preparationFailure']['state'], 'completed')
                self.assertEqual(final['preparationFailure']['command'][:3], ['xcrun', 'simctl', 'install'])
                diagnostic = [c for c in commands.call_args_list if c.kwargs['label'].startswith('prep-')]
                self.assertEqual([c.kwargs['label'] for c in diagnostic], ['prep-host-disk', 'prep-host-processes', 'prep-host-log', 'prep-device-log'])
                self.assertEqual([c.kwargs['timeout'] for c in diagnostic], [3, 3, 8, 8])
                self.assertTrue(all(c.kwargs['limit'] == 512 * 1024 for c in diagnostic))
                self.assertEqual(diagnostic[-1].args[1][:4], ['xcrun', 'simctl', 'spawn', 'pad'])
                self.assertIn('comm', diagnostic[1].args[1][-1]); self.assertNotIn('command', diagnostic[1].args[1][-1])
                after = [c for c in commands.call_args_list if c.kwargs['label'] == 'shutdown-after']
                self.assertEqual(after[-1].args[1][-1], 'pad')
                self.assertLessEqual(after[-1].kwargs['timeout'], 30)
                self.assertEqual(any('prep-host-disk:' in p for p in final['problems']), fault != 'last-install')
                self.assertTrue(any(a['path'].endswith('-prep-device-log.log') for a in final['artifacts']))

    def test_successful_preparation_does_not_collect_failure_diagnostics(self):
        with mock.patch.object(scene, 'preparation_diagnostics') as diagnostic:
            _, _, _, payload, _ = self.controlled_execute()
        diagnostic.assert_not_called()
        self.assertTrue(all('preparationFailure' not in r for r in payload['results']))

    def test_physical_dependency_root_reaches_builds_and_real_journey_children(self):
        for chain, event, role in [('26', 'pull_request', 'ios-native'), ('26', 'pull_request', 'ios-scene-baseline-26'),
                                   ('27', 'pull_request', 'ios-scene'), ('26', 'push', 'ios-native'), ('27', 'push', 'ios-scene')]:
            with self.subTest(toolchain=chain, event=event, role=role):
                observed = {}
                _, built, _, payload, _ = self.controlled_execute(chain, event, dependency_observations=observed, role=role)
                ids = {'baseline': B, 'candidate': P} if event == 'pull_request' else {'candidate': P}
                expected = scene.role_units(payload['binding'], role)
                self.assertEqual(set(built), {unit['source'] for unit in expected})
                self.assertEqual(len(observed['builds']), len(set(built)))
                self.assertEqual(len(payload['results']), len(expected))
                self.assertEqual(len(observed['child_logs']), len(expected))
                for build in observed['builds']:
                    self.assertEqual(build['root'], observed['physical'])
                for name in observed['child_logs']:
                    child = json.loads(Path(name).read_text())
                    self.assertEqual(child, {'root': observed['physical'], 'proof': 'verified fixture dependency'})

    def test_real_runner_wait_then_stat_failure_preserves_unit_and_aggregate(self):
        for status in (0, 7):
            with self.subTest(status=status):
                code, _, _, payload, _ = self.controlled_execute(status=status, fault='wait-stat')
                self.assertEqual(code, 1); self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
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
                    self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
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
        for chain, event, wanted in [('26', 'pull_request', [P]), ('27', 'pull_request', [P]),
                                     ('26', 'push', [P]), ('27', 'push', [P])]:
            with self.subTest(chain=chain, event=event):
                _, builds, readable, payload, _ = self.controlled_execute(chain, event)
                self.assertEqual(builds, wanted)
                self.assertEqual(set(readable), {P, B} if event == 'pull_request' else {P})
                self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
        _, builds, readable, payload, _ = self.controlled_execute('26', role='ios-scene-baseline-26')
        self.assertEqual(builds, [B]); self.assertEqual(set(readable), {P, B})
        self.assertEqual(len(payload['results']), 2)
        self.assertTrue(all(r['unit']['variant'] == 'baseline' for r in payload['results']))
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
                    self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
                    for result in payload['results']:
                        self.assertEqual((result['testExit'], result['state']), (status, 'completed'))
                        if fault != 'acceptance': self.assertTrue(any('injected ' + fault in p for p in result['problems']))
                        if fault == 'artifact': self.assertTrue(any(a['path'].endswith('-journey.json') for a in result['artifacts']))
                    if fault == 'result': self.assertIn('scene unit archive failed:', stderr)

    def test_real_executor_keeps_last_shutdown_failure_and_original_exit(self):
        for status in (0, 7):
            code, _, _, payload, _ = self.controlled_execute(status=status, fault='last-shutdown')
            self.assertEqual(code, 1)
            self.assertEqual(len(payload['results']), len(scene.role_units(payload['binding'], payload['role'])))
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


def synthetic_stream_script(mode='valid', ready=None, release=None):
    # Deliberately synthetic log(1) text, never reported as captured UIKit evidence.
    return f'''
import datetime, os, time
from pathlib import Path
mode={mode!r}; ready={str(ready) if ready else None!r}; release={str(release) if release else None!r}
if ready:
    limit=time.monotonic()+2
    while not Path(ready).exists() and time.monotonic()<limit: time.sleep(.005)
    if not Path(ready).exists(): raise RuntimeError('controlled primary clock was not bound')
stamp=datetime.datetime.now(datetime.timezone.utc)
if mode=='outside': stamp-=datetime.timedelta(seconds=60)
if mode=='future': stamp+=datetime.timedelta(seconds=60)
if mode=='offset': stamp=stamp.astimezone(datetime.timezone(datetime.timedelta(hours=8)))
text=stamp.strftime('%Y-%m-%d %H:%M:%S.%f%z')
if mode=='naive': text=stamp.strftime('%Y-%m-%d %H:%M:%S.%f')
service='other' if mode=='service' else 'installd'
app='com.gtmux.app.other' if mode=='app' else 'com.gtmux.app'
line=text+' 0x123 Default 0x0 42 0 '+service+': synthetic install '+app+'\\n'
if mode=='header': print('Filtering the log data using predicate',flush=True)
elif mode=='stderr': os.write(2,line.encode())
elif mode=='late':
    print('Filtering the log data using predicate',flush=True); time.sleep(2); print(line,end='',flush=True)
elif mode=='oversize': print(line.rstrip()+'x'*5000,flush=True)
elif mode=='unterminated': print(line.rstrip(),end='',flush=True)
elif mode=='fragment':
    data=line.encode(); os.write(1,data[:30]); time.sleep(.15); os.write(1,data[30:])
else: print(line,end='',flush=True)
if release:
    limit=time.monotonic()+3
    while not Path(release).exists() and time.monotonic()<limit: time.sleep(.005)
    raise SystemExit(0 if mode=='exit-zero' else 9)
time.sleep(30)
'''


class InstallStreamControls(unittest.TestCase):
    setUp = SceneMigrationTests.setUp
    tearDown = SceneMigrationTests.tearDown

    def run_child(self, primary='import time; time.sleep(.35)', observer=None,
                  timeout=2, fault=None, binding=None, sample=False, legacy_style=False, stream_mode='valid'):
        root = self.root / str(len(list(self.root.iterdir()))); root.mkdir()
        runner = scene.Runner(root, os.environ)
        ready, release = root/'primary-bound', root/'observer-release'
        observer = observer or synthetic_stream_script(stream_mode, ready=ready, release=release if fault in ('final-exit','stop-race') else None)
        original_bind, original_finish = scene.InstallStream.bind_primary, scene.InstallStream.finish
        def bind(stream, *args):
            original_bind(stream,*args); ready.write_text('bound')
        def finish(stream):
            if fault=='final-exit' and stream.child is not None:
                release.write_text('exit now'); stream.child.wait(timeout=2)
            return original_finish(stream)
        children, kills = [], []
        real_spawn, real_open, real_kill, real_read = subprocess.Popen, Path.open, os.killpg, os.read
        def spawn(args, **kwargs):
            is_observer = args[:3] == ['xcrun', 'simctl', 'spawn']
            if is_observer:
                if not legacy_style: self.assertEqual(args[7], 'default')
                self.assertIn(args[7],('compact','default'))
                self.assertEqual(args[3:], ['fixture', '/usr/bin/log', 'stream', '--style', args[7], '--level', 'info',
                                           '--timeout', '60', '--predicate', 'process == "installd" OR process == "SpringBoard"'])
                if fault == 'startup': raise OSError('controlled startup')
                if fault == 'spawn-delay': time.sleep(.15)
                script = observer
            else:
                if fault == 'primary-startup': raise OSError('controlled primary Popen')
                script = primary
            child = real_spawn([sys.executable, '-c', script], **kwargs)
            children.append((is_observer, child))
            if fault == 'primary-poll' and not is_observer:
                poll = child.poll; first = [True]
                def failed_poll():
                    if first:
                        first.pop()
                        raise OSError('controlled primary poll')
                    return poll()
                child.poll = failed_poll
            if fault == 'after-exit-poll' and not is_observer:
                poll = child.poll; saw_exit = [False]; failed = [False]
                def after_exit_poll():
                    if saw_exit[0] and not failed[0]:
                        failed[0] = True
                        raise OSError('controlled poll after known exit')
                    code = poll()
                    if code is not None: saw_exit[0] = True
                    return code
                child.poll = after_exit_poll
            def cleanup_owned_fixture():
                if child.poll() is None:
                    real_kill(child.pid, 9)
                    child.wait(timeout=2)
            self.addCleanup(cleanup_owned_fixture)
            if is_observer and fault in ('reap', 'slow-reap'):
                wait = child.wait
                def delayed_wait(*a, **k):
                    code = wait(*a, **k)
                    if fault == 'reap': raise subprocess.TimeoutExpired('owned observer', 1)
                    time.sleep(.4)
                    return code
                child.wait = delayed_wait
            return child
        def opened(path, *a, **k):
            if fault == 'open' and str(path).endswith('install-stream.stderr.log'): raise OSError('controlled stream open')
            if fault == 'archive' and str(path).endswith('install-stream.json'): raise OSError('controlled stream archive')
            output = real_open(path, *a, **k)
            if fault == 'write' and str(path).endswith('install-stream.stdout.log'):
                class FailingOutput:
                    def write(self, data): raise OSError('controlled stream write')
                    def close(self): output.close()
                return FailingOutput()
            return output
        def kill(pid, sig):
            self.assertIn(pid, [c.pid for _, c in children], 'only actual owned children')
            kills.append((pid, time.monotonic()))
            if fault == 'stop-race' and children and pid == children[0][1].pid:
                release.write_text('exit before signal acknowledgement')
                children[0][1].wait(timeout=2)
                return None  # controlled syscall acknowledgement; real child exits9
            if fault == 'kill' and children and pid == children[0][1].pid:
                # Actual signal still occurs; a raced OS error must be recorded, not hidden.
                real_kill(pid, sig); raise OSError('controlled kill acknowledgement')
            return real_kill(pid, sig)
        def read(fd, size):
            if fault == 'read' and any(owned and fd in (child.stdout.fileno(),child.stderr.fileno()) for owned,child in children):
                raise OSError('controlled observer read')
            return real_read(fd,size)
        result = None
        with mock.patch.object(scene.subprocess, 'Popen', side_effect=spawn), mock.patch.object(Path, 'open', new=opened), \
                mock.patch.object(scene.os, 'killpg', side_effect=kill), mock.patch.object(scene.os, 'read', side_effect=read), \
                mock.patch.object(scene.InstallStream,'bind_primary',new=bind), mock.patch.object(scene.InstallStream,'finish',new=finish):
            try:
                result = runner.command(['controlled-primary'], cwd=root, label='install', timeout=timeout, check=False,
                                        install_stream=binding or {'udid': 'fixture'}, sample_install=sample)
            except scene.CommandFailure as error: result = error.outcome
        self.assertTrue(all(c.poll() is not None for _, c in children), 'own children reaped or actual exit observed')
        primary_record = json.loads((root / '001-install.json').read_text())
        meta = root / '001-install.install-stream.json'
        observer_record = json.loads(meta.read_text()) if meta.exists() else None
        return runner, primary_record, observer_record, root, children, kills, result

    def test_stream_independent_exit_clocks_stop_before_sample_and_primary_deadline(self):
        for status in (0, 7):
            runner, primary, record, root, children, kills, result = self.run_child(
                primary=f'import time; time.sleep(.35); raise SystemExit({status})')
            self.assertEqual((primary['state'], primary['exit']), ('completed', status))
            self.assertEqual(runner.observation_problems, [])
            self.assertEqual(record['state'], 'primary-ended')
            self.assertEqual(record['exit'], -9)
            events = record['clock']['events']; main = primary['clock']['events']
            self.assertLessEqual(events['popenReturned']['monotonic'], main['popenReturned']['monotonic'])
            self.assertEqual(record['clock']['deadlineMonotonic'], primary['clock']['deadlineMonotonic'])
            self.assertAlmostEqual(primary['clock']['deadlineMonotonic']-main['popenReturned']['monotonic'], 2)
            self.assertLessEqual(main['exitObserved']['monotonic'], events['killRequested']['monotonic'])
            self.assertLessEqual(events['killRequested']['monotonic'], events['exitObserved']['monotonic'])
            self.assertLessEqual(events['exitObserved']['monotonic'], events['reaped']['monotonic'])
            self.assertLessEqual(events['reaped']['monotonic'], events['archived']['monotonic'])
            self.assertIsNone(events['deadlineObserved'])
            self.assertLessEqual(len(events), 32)
            self.assertTrue(kills)
            self.assertEqual(record['retainedBytes']['stdout'], (root/'001-install.install-stream.stdout.log').stat().st_size)
        original_finish = scene.InstallSample.finish
        def finish(sample, child):
            stream = json.loads(sample.stem.with_suffix('.install-stream.json').read_text())
            self.assertIsNotNone(stream['clock']['events']['reaped'])
            return original_finish(sample, child)
        with mock.patch.object(scene.InstallSample, 'finish', new=finish): self.run_child(sample=True)
        # Slow observer reap cannot retroactively timeout a completed primary.
        _, primary, record, _, _, _, _ = self.run_child(timeout=.5, fault='slow-reap')
        self.assertEqual((primary['state'], primary['exit']), ('completed', 0))
        self.assertGreater(record['clock']['events']['archived']['monotonic'], primary['clock']['deadlineMonotonic'])

    def test_stream_first_observed_primary_exit_survives_later_poll_failure(self):
        for status in (0, 7):
            _, primary, stream, _, _, _, result = self.run_child(
                primary=f'import time; time.sleep(.35); raise SystemExit({status})', fault='after-exit-poll')
            self.assertEqual((primary['state'], primary['exit']), ('completed', status))
            self.assertIsNotNone(primary['clock']['events']['exitObserved'])
            self.assertIsNotNone(stream['clock']['events']['reaped'])
            self.assertTrue(any('after known exit' in p for p in primary['problems']))
            self.assertIsInstance(result, dict)  # diagnostic failure still raises CommandFailure

    def test_stream_timeout_has_absolute_deadline_and_owned_kill(self):
        began = time.monotonic()
        _, primary, record, _, children, kills, _ = self.run_child(primary='import time; time.sleep(30)', timeout=.3)
        self.assertLess(time.monotonic()-began, 2)
        self.assertEqual((primary['state'], primary['exit']), ('timeout', -9))
        self.assertEqual(record['clock']['deadlineMonotonic'], primary['clock']['deadlineMonotonic'])
        self.assertIsNotNone(record['clock']['events']['deadlineObserved'])
        self.assertIsNotNone(primary['clock']['events']['deadlineObserved'])
        self.assertEqual({pid for pid, _ in kills}, {c.pid for _, c in children})
        self.assertLess(record['clock']['events']['killRequested']['monotonic']-primary['clock']['deadlineMonotonic'], .4)
        self.assertLess(record['clock']['maxPollGap'], .4)

    def test_stream_failures_do_not_replace_primary_zero_seven_or_start_failure(self):
        for fault in ('startup', 'open', 'archive', 'write', 'read', 'kill', 'reap'):
            for status in (0, 7):
                with self.subTest(fault=fault, status=status):
                    runner, primary, record, _, _, _, _ = self.run_child(
                        primary=f'import time; time.sleep(.35); raise SystemExit({status})', fault=fault)
                    self.assertEqual((primary['state'], primary['exit']), ('completed', status))
                    self.assertTrue(runner.observation_problems)
                    if fault == 'reap': self.assertIsNone(record['clock']['events']['reaped'])
        _, failed, observed, _, _, _, _ = self.run_child(fault='primary-poll')
        self.assertEqual((failed['state'],failed['exit']),('error',-9))
        self.assertIsNotNone(observed['clock']['events']['reaped'])
        runner, primary, record, _, children, _, _ = self.run_child(fault='primary-startup')
        self.assertEqual((primary['state'], primary['exit']), ('not-started', None))
        self.assertEqual(len(children), 1)
        self.assertIsNone(record['clock']['deadlineMonotonic'])
        self.assertIsNone(primary['clock']['events']['popenReturned'])
        self.assertIsNotNone(record['clock']['events']['reaped'])
        runner, primary, _, _, _, _, _ = self.run_child(primary='import time; time.sleep(30)', timeout=.3, fault='startup')
        self.assertEqual((primary['state'], primary['exit']), ('timeout', -9))
        self.assertTrue(runner.observation_problems)

    def test_stream_empty_late_early_and_eof_are_not_successful_coverage(self):
        for script, expected in [('import time; time.sleep(30)', 'empty'),
                                 ('import time; time.sleep(2); print("late", flush=True)', 'empty'),
                                 ('print("early", flush=True)', 'ended before'),
                                 ('raise SystemExit(9)', 'ended before')]:
            with self.subTest(script=script):
                runner, primary, record, _, _, _, _ = self.run_child(observer=script)
                self.assertEqual(primary['exit'], 0)
                self.assertTrue(any(expected in p for p in runner.observation_problems), runner.observation_problems)
        runner, _, record, _, _, _, _ = self.run_child(observer='import os,time; os.close(1); os.write(2,b"stderr event"); time.sleep(30)')
        self.assertEqual(record['retainedBytes']['stdout'], 0)
        self.assertGreater(record['retainedBytes']['stderr'], 0)
        self.assertTrue(record['eof']['stdout'])
        self.assertTrue(any('window inconclusive' in p for p in runner.observation_problems))

    def test_stream_shared_hard_cap_exact_cross_and_terminal_metadata(self):
        self.assertEqual((scene.InstallStream.LIMIT, scene.InstallStream.READ_QUOTA, scene.InstallStream.METADATA_LIMIT), (262144,16384,16384))
        for count in (2500, 5000):
            with self.subTest(bytes_per_pipe=count), mock.patch.object(scene.InstallStream, 'LIMIT', 5000), \
                    mock.patch.object(scene.InstallStream, 'READ_QUOTA', 2048):
                script=f'import os,time; os.write(1,b"a"*{count}); os.write(2,b"b"*{count}); time.sleep(30)'
                runner, primary, record, root, _, _, _ = self.run_child(primary='import time; time.sleep(1)', observer=script)
                retained = sum((root/('001-install.install-stream.'+name+'.log')).stat().st_size for name in ('stdout','stderr'))
                self.assertEqual(retained, 5000)
                self.assertEqual(sum(record['retainedBytes'].values()), retained)
                self.assertGreater(record['retainedBytes']['stderr'], 0)
                self.assertEqual(record['state'], 'output-limit')
                self.assertTrue(record['truncated']); self.assertIsNone(record['droppedBytes'])
                self.assertEqual(primary['exit'], 0); self.assertTrue(runner.observation_problems)
                self.assertLess(record['clock']['events']['killRequested']['monotonic'],primary['clock']['events']['exitObserved']['monotonic'])
        runner, primary, record, root, _, _, _ = self.run_child(binding={'udid':'fixture','long':'\"\\\n'*20000})
        self.assertLessEqual((root/'001-install.install-stream.json').stat().st_size, 16384)
        self.assertTrue(record['metadataTruncated'])
        self.assertEqual(record['exit'], -9)
        self.assertIsNotNone(record['clock']['events']['reaped'])
        self.assertIsNotNone(record['clock']['events']['archived'])
        self.assertTrue(runner.observation_problems); self.assertEqual(primary['exit'],0)

    def test_stream_spawn_overhead_visible_without_resetting_primary_budget(self):
        _, primary, record, _, _, _, _ = self.run_child(fault='spawn-delay')
        observer = record['clock']['events']; main = primary['clock']['events']
        self.assertGreaterEqual(observer['popenReturned']['monotonic']-observer['commandRequested']['monotonic'], .15)
        self.assertGreaterEqual(main['popenReturned']['monotonic']-main['commandRequested']['monotonic'], .15)
        self.assertAlmostEqual(record['clock']['deadlineMonotonic']-main['popenReturned']['monotonic'],2)
        self.assertEqual(primary['state'],'completed')

    def test_stream_real_execute_selects_only_first_27_install_and_never_failed_boot(self):
        control = SceneExecutionTests(); control.root = self.root
        actual_command = scene.Runner.command
        for chain, event, role, fault in [('26','pull_request','ios-native',None),
                ('26','pull_request','ios-scene-baseline-26',None),('27','pull_request','ios-scene',None),
                ('27','push','ios-scene',None),('27','pull_request','ios-scene','first-boot')]:
            with self.subTest(chain=chain,fault=fault), mock.patch.object(scene.Runner,'command',autospec=True,side_effect=actual_command) as calls:
                _, _, _, payload, _ = control.controlled_execute(chain, event_name=event, role=role, fault=fault)
            streams = [c for c in calls.call_args_list if c.kwargs.get('install_stream') is not None]
            self.assertEqual(len(streams), 1 if chain=='27' and fault is None else 0)
            if streams:
                binding = streams[0].kwargs['install_stream']
                self.assertEqual(binding['unit']['journey'],'coldstart')
                self.assertEqual(binding['unit']['source'],P)
                self.assertEqual(binding['context'],payload['binding'])
                self.assertEqual(binding['role'],'ios-scene')
                self.assertEqual(binding['udid'],'fixture')
                self.assertEqual(binding['products'][0]['sha256'],'d'*64)
                self.assertEqual(streams[0].kwargs['timeout'],60)
            self.assertEqual(len(payload['results']),len(scene.role_units(payload['binding'],payload['role'])))
            if fault:
                self.assertEqual(payload['results'][0]['preparationFailure']['exit'],7)
                self.assertEqual(payload['results'][0]['testExit'],None)
            else:
                self.assertTrue(all(r['testExit']==0 for r in payload['results']))
                if chain=='27':
                    r=payload['results'][0]
                    self.assertTrue(any(a['path'].endswith('install-stream.json') for a in r['artifacts']))
                    self.assertTrue(any('stream' in p for p in r['problems']))
                    self.assertIn('execution evidence or cleanup failed',scene.unit_errors(r['unit'],r, self.root))


    def test_stream_final_exit_race_keeps_primary_and_reports_terminal_failure(self):
        for fault, observer_exit in (('final-exit',9),('final-exit',0),('stop-race',9)):
            for status in (0,7):
                with self.subTest(fault=fault,status=status):
                    runner, primary, stream, _, _, _, _ = self.run_child(
                        primary=f'import time; time.sleep(.35); raise SystemExit({status})',fault=fault,legacy_style=True,
                        stream_mode='exit-zero' if observer_exit==0 else 'valid')
                    self.assertEqual((primary['state'],primary['exit']),('completed',status))
                    self.assertEqual(stream['exit'],observer_exit)
                    self.assertTrue(any('terminal exit inconclusive' in p for p in runner.observation_problems),
                                    'final wait9 must become an independent unit problem')
                    self.assertEqual(stream['termination'],'natural-or-unexpected')
                    self.assertEqual(stream['stopSignalReturned'],fault=='stop-race')
                    self.assertEqual(stream['stopObservedExit'],observer_exit if fault=='final-exit' else None)
                    self.assertGreater(stream['window']['qualifiedEvents'],0)
                    self.assertIsNotNone(stream['clock']['events']['reaped'])
        runner, primary, stream, _, _, _, _ = self.run_child()
        self.assertEqual(primary['exit'],0)
        self.assertEqual((stream['exit'],stream['termination']),(-9,'requested-sigkill'))
        self.assertEqual(runner.observation_problems,[])

    def test_stream_requires_qualified_event_not_headers_diagnostics_or_outside_window(self):
        for mode in ('header','stderr','outside','future','naive','app','service','late','oversize','unterminated',
                     'valid','offset','fragment'):
            with self.subTest(mode=mode):
                runner, primary, stream, _, _, _, _ = self.run_child(stream_mode=mode,legacy_style=True)
                valid = mode in ('valid','offset','fragment')
                self.assertEqual((primary['state'],primary['exit']),('completed',0))
                self.assertGreater(sum(stream['retainedBytes'].values()),0)
                inconclusive=any('window inconclusive' in p for p in runner.observation_problems)
                self.assertEqual(inconclusive,not valid, f'{mode}: bytes are not window evidence')
                self.assertEqual(stream['window']['qualifiedEvents']>0,valid)
                if valid:
                    first=stream['window']['first']
                    self.assertEqual(first['process'],'installd')
                    self.assertGreaterEqual(first['received']['monotonic'],stream['window']['primaryStart']['monotonic'])
                    self.assertLessEqual(first['received']['monotonic'],stream['clock']['deadlineMonotonic'])
                    self.assertEqual(runner.observation_problems,[])
                else: self.assertIsNone(stream['window']['first'])
                if mode=='oversize':self.assertEqual(stream['window']['ignored']['oversize'],1)

    def test_stream_receipt_clock_discontinuity_stays_unknown(self):
        inspect = scene.InstallStream.inspect_stdout
        def skew(stream,data):
            # Controlled parent clock boundary, not a claim about device clocks.
            future=(scene.datetime.datetime.now(scene.datetime.timezone.utc)+scene.datetime.timedelta(seconds=2)).isoformat()
            with mock.patch.object(scene,'now',return_value=future):inspect(stream,data)
        with mock.patch.object(scene.InstallStream,'inspect_stdout',new=skew):
            runner, primary, stream, _, _, _, _ = self.run_child()
        self.assertEqual(primary['exit'],0)
        self.assertEqual(stream['window']['qualifiedEvents'],0)
        self.assertGreater(stream['window']['ignored']['clock'],0)
        self.assertTrue(any('window inconclusive' in p for p in runner.observation_problems))

    def test_stream_final_failure_and_window_inconclusive_reach_real_unit_and_aggregate(self):
        control=SceneExecutionTests();control.root=self.root
        for mode in ('valid','header','stderr','outside','final-exit'):
            for status in (0,7):
                with self.subTest(mode=mode,status=status):
                    code,_,_,payload,_=control.controlled_execute(status=status,stream_observations={'mode':mode})
                    first=payload['results'][0]
                    self.assertEqual((first['state'],first['testExit']),('completed',status))
                    expected='terminal exit inconclusive' if mode=='final-exit' else 'window inconclusive'
                    self.assertEqual(any(expected in p for p in first['problems']),mode!='valid')
                    if mode=='valid':self.assertEqual(first['problems'],[])
                    commands=[a for a in first['artifacts'] if a['path'].endswith('install-stream.json')]
                    self.assertEqual(len(commands),1)
                    expected_units=scene.role_units(payload['binding'],payload['role'])
                    self.assertEqual(len(payload['results']),len(expected_units))
                    # Other synthetic identities are deliberately incomplete; assert
                    # the observer's own failure term rather than just overall red.
                    root=next(self.root.glob(f'execute-{len(list(self.root.iterdir()))-1}'))/'scene-migration/1-1-ios-scene'
                    errors=scene.verify_all(expected_units,payload['results'],root/'evidence')
                    term=scene.tuple_key(first['unit'])+': execution evidence or cleanup failed'
                    self.assertEqual(term in errors,mode!='valid')
                    self.assertEqual(code,1)  # No synthetic identity fixture becomes device acceptance.


class SceneRoleUnionTests(unittest.TestCase):
    setUp = SceneMigrationTests.setUp
    tearDown = SceneMigrationTests.tearDown
    result = SceneMigrationTests.result
    def fixture(self, event_name='pull_request'):
        repo = self.root / 'repo'; (repo / 'mobileapp/e2e').mkdir(parents=True)
        (repo / 'mobileapp/e2e/ci-scene-migration.py').write_bytes(PATH.read_bytes())
        def git(*args):
            return subprocess.check_output(['git', '-C', str(repo), '-c', 'user.name=fixture',
                '-c', 'user.email=fixture@example.test', '-c', 'core.hooksPath=/dev/null', *args], stderr=subprocess.STDOUT).decode().strip()
        git('init', '-q'); git('add', '.'); git('commit', '-qm', 'base'); baseline = git('rev-parse', 'HEAD')
        (repo / 'identity.txt').write_text('candidate'); git('add', '.'); git('commit', '-qm', 'candidate'); candidate = git('rev-parse', 'HEAD')
        event = {'pull_request': {'base': {'sha': baseline}, 'head': {'sha': candidate}}} if event_name == 'pull_request' else {'ref': 'refs/heads/main', 'after': candidate}
        event_path = self.root / 'event.json'; event_path.write_text(json.dumps(event))
        env = {**os.environ, 'GITHUB_EVENT_NAME': event_name, 'GITHUB_EVENT_PATH': str(event_path),
               'GITHUB_REF': 'refs/pull/1825/merge' if event_name == 'pull_request' else 'refs/heads/main',
               'GITHUB_RUN_ID': '123', 'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_SHA': candidate}
        context = scene.trusted_context(repo, env)
        artifacts = self.root / 'download'; artifacts.mkdir()
        for role in scene.selected_roles(context):
            root = artifacts / f'scene-migration-{role}-123-2' / f'123-2-{role}'; root.mkdir(parents=True)
            expected = scene.role_units(context, role); chain = expected[0]['toolchain']; source = expected[0]['source']
            results = [self.result(u) for u in expected]
            for u in expected:
                destination = root / 'evidence' / scene.tuple_key(u); destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.move(str(self.root / scene.tuple_key(u)), destination)
            device = {'runtimeID': 'runtime', 'runtimeBuild': 'build', 'runtime': scene.TOOLS[chain]['runtime'],
                      'deviceTypeIdentifier': 'test-phone', 'udid': 'test-udid'}
            def content(raw, path):
                raw = raw if isinstance(raw, str) else json.dumps(raw)
                return {'path': path, 'raw': raw, 'bytes': len(raw.encode()), 'sha256': hashlib.sha256(raw.encode()).hexdigest()}
            root_package = {'private': True, 'overrides': {'appium-xcuitest-driver': {'appium-webdriveragent': scene.TOOLS[chain]['wda']}}}
            preflight = {**scene.TOOLS[chain], 'osVersion': '15.8.2', 'osBuild': '24H232', 'architecture': 'arm64',
                'imageOS': 'macos15', 'imageVersion': '20261104.0514.1', 'xcodeBuild': '17C100',
                'nodeVersion': 'v22.20.0', 'rubyVersion': 'ruby 3.3.0', 'host': role, 'appiumHome': '/isolated',
                'sdkroot': 'iphonesimulator'+scene.TOOLS[chain]['sdk'], 'developerDir': 'xcode',
                'driverPackage': content({'version': scene.TOOLS[chain]['driver']}, '/isolated/driver/package.json'),
                'wdaPackageRecord': content({'version': scene.TOOLS[chain]['wda']}, '/isolated/wda/package.json'),
                'wdaPackage': '/isolated/wda/package.json', 'appiumRootContents': root_package,
                'appiumRootPackage': content(root_package, '/isolated/package.json'),
                'devices': {'phone': device, **({'ipad': device} if chain == '27' else {})}}
            scene.write_once(root / 'preflight.json', preflight)
            scene.write_once(root / 'inputs.json', {n: content('committed-lock', '/source/'+n) for n in scene.INPUTS})
            product_root = root / 'source' / source; product_root.mkdir(parents=True)
            scene.write_once(product_root / 'product.json', {'products': results[0]['identity']['products'],
                'linkSysroot': results[0]['identity']['linkSysroot'], 'resolvedPods': content('actual-resolved-lock', '/source/Podfile.lock')})
            scene.write_once(product_root / 'instrumentation.json', {'source': source, 'edits': [{'path': n} for n in
                ('src/state/AppContext.tsx', 'src/screens/ServersScreen.tsx', 'src/push/index.ts', 'metro.config.js')]})
            scene.write_once(root / 'acceptance.json', {'binding': context, 'role': role, 'toolchain': chain,
                'flags': scene.FLAGS, 'results': results, 'preparationErrors': []})
            scene.write_once(root / 'verdict.json', {'errors': [], 'passed': True})
            scene.publish_role(root, context, role)
        return repo, artifacts, env, context

    def cli(self, repo, artifacts, env):
        return subprocess.run([sys.executable, str(PATH), '--verify-union', str(artifacts), '--repository', str(repo)],
                              env=env, capture_output=True, text=True, timeout=15)

    def republish(self, root, context, role):
        (root / 'publication.json').unlink()  # This test's own tiny fixture only.
        scene.publish_role(root, context, role)

    def test_union_real_cli_PR15_main11_and_trusted_current_event(self):
        for event in ('pull_request', 'push'):
            with tempfile.TemporaryDirectory(prefix='scene-union-') as temp, self.subTest(event=event):
                saved = self.root; self.root = Path(temp)
                try:
                    repo, artifacts, env, ctx = self.fixture(event)
                    result = self.cli(repo, artifacts, env); self.assertEqual(result.returncode, 0, result.stdout+result.stderr)
                    expected = [u for r in scene.selected_roles(ctx) for u in scene.role_units(ctx, r)]
                    self.assertEqual(len(expected), 8 if event == 'pull_request' else 6)
                    self.assertEqual(sum(len(scene.CASES[u['journey']]) for u in expected), 15 if event == 'pull_request' else 11)
                    for key, value in [('GITHUB_RUN_ATTEMPT', '3'), ('GITHUB_RUN_ID', '999'), ('GITHUB_SHA', 'f'*40),
                                       ('GITHUB_REF', 'refs/heads/foreign'), ('GITHUB_EVENT_NAME', 'workflow_dispatch')]:
                        damaged = self.cli(repo, artifacts, {**env, key: value})
                        self.assertNotEqual(damaged.returncode, 0, key)
                finally: self.root = saved

    def test_union_rejects_missing_duplicate_wrong_binding_hash_and_cases(self):
        repo, artifacts, env, ctx = self.fixture()
        role = 'ios-scene-baseline-26'; artifact = artifacts / f'scene-migration-{role}-123-2'; root = artifact / f'123-2-{role}'
        self.assertEqual(scene.verify_union(artifacts, ctx), [])
        for name in ('acceptance.json', 'publication.json', 'verdict.json'):
            path = root / name; raw = path.read_bytes(); path.unlink()
            try: self.assertTrue(scene.verify_union(artifacts, ctx), name)
            finally: path.write_bytes(raw)
        acceptance = root / 'acceptance.json'; original = acceptance.read_bytes()
        mutations = [lambda d: d['results'].pop(), lambda d: d['results'].append(d['results'][0]),
            lambda d: d['results'][0]['cases'].pop(), lambda d: d['results'][0].update(testExit=7),
            lambda d: d['results'][0].update(state='cancelled'), lambda d: d['results'][0].update(problems=['cleanup']),
            lambda d: d['results'][0]['unit'].update(variant='candidate'),
            lambda d: d['results'][0]['identity'].update(pid=None), lambda d: d.update(preparationErrors=['archive failure']),
            lambda d: d['binding'].update(attempt='1')]
        for mutate in mutations:
            data = json.loads(original); mutate(data); acceptance.write_text(json.dumps(data)); self.republish(root, ctx, role)
            self.assertTrue(scene.verify_union(artifacts, ctx))
        changed_context = {**ctx, 'githubSHA': 'f'*40}
        data = json.loads(original); data['binding'] = changed_context
        acceptance.write_text(json.dumps(data)); self.republish(root, changed_context, role)
        self.assertTrue(scene.verify_union(artifacts, ctx), 'self-consistent foreign binding is not trusted')
        acceptance.write_bytes(original); self.republish(root, ctx, role)
        metadata = root / 'preflight.json'; metadata_raw = metadata.read_bytes(); metadata.write_bytes(metadata_raw+b'\n')
        self.assertTrue(scene.verify_union(artifacts, ctx), 'metadata byte changes without new manifest must fail')
        metadata.write_bytes(metadata_raw)
        binary = next((root / 'evidence').rglob('*.png')); raw = binary.read_bytes(); binary.write_bytes(raw+b'changed')
        self.assertTrue(scene.verify_union(artifacts, ctx)); binary.write_bytes(raw)
        (artifacts / 'duplicate').mkdir(); self.assertTrue(scene.verify_union(artifacts, ctx)); (artifacts / 'duplicate').rmdir()
        publication = root / 'publication.json'; data = json.loads(publication.read_text()); data['files'].append(data['files'][0])
        publication.write_text(json.dumps(data)); self.assertTrue(scene.verify_union(artifacts, ctx)); self.republish(root, ctx, role)
        with mock.patch.object(scene, 'write_once', side_effect=OSError('upload manifest write failure')):
            with self.assertRaises(OSError): self.republish(root, ctx, role)
        self.assertTrue(scene.verify_union(artifacts, ctx))

    def test_union_cross_host_identity_and_resolved_pods_are_required(self):
        repo, artifacts, env, ctx = self.fixture(); role = 'ios-scene-baseline-26'
        root = artifacts / f'scene-migration-{role}-123-2' / f'123-2-{role}'
        preflight = root / 'preflight.json'; original = preflight.read_bytes()
        for key in ('osVersion', 'osBuild', 'imageOS', 'imageVersion', 'xcodeBuild', 'rubyVersion', 'nodeVersion', 'developerDir'):
            for value in ('', 'different'):
                with self.subTest(key=key, value=value):
                    data = json.loads(original); data[key] = value; preflight.write_text(json.dumps(data)); self.republish(root, ctx, role)
                    self.assertTrue(scene.verify_union(artifacts, ctx))
        preflight.write_bytes(original); self.republish(root, ctx, role)
        for name in ('inputs.json', f'source/{ctx["sources"]["baseline"]}/product.json'):
            path = root / name; raw = path.read_bytes(); data = json.loads(raw)
            record = data['resolvedPods'] if name.endswith('product.json') else data['Gemfile.lock']
            record.update(raw='changed-lock', bytes=12, sha256=hashlib.sha256(b'changed-lock').hexdigest())
            path.write_text(json.dumps(data)); self.republish(root, ctx, role)
            self.assertTrue(scene.verify_union(artifacts, ctx), name)
            path.write_bytes(raw); self.republish(root, ctx, role)
        self.assertEqual(scene.verify_union(artifacts, ctx), [])

    def test_trusted_sources_require_readable_base_and_exact_coordinator_bytes(self):
        repo, artifacts, env, _ = self.fixture()
        path = Path(env['GITHUB_EVENT_PATH']); data = json.loads(path.read_text()); original = path.read_bytes()
        data['pull_request']['base']['sha'] = 'f'*40; path.write_text(json.dumps(data))
        self.assertNotEqual(self.cli(repo, artifacts, env).returncode, 0); path.write_bytes(original)
        with mock.patch.object(scene.Path, 'read_bytes', return_value=b'foreign verifier'):
            with self.assertRaises(ValueError): scene.trusted_context(repo, env)


if __name__ == '__main__':
    unittest.main()
