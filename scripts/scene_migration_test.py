import copy
import hashlib
import importlib.util
import json
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
        (root / 'evidence.txt').write_bytes(data)
        identity = {**scene.TOOLS[unit['toolchain']], 'source': unit['source'], 'signature': 'adhoc',
                    'pid': 123, 'runtimeID': 'runtime', 'runtimeBuild': 'build', 'udid': 'test-udid',
                    'deviceType': 'test-phone', 'sdkroot': 'iphonesimulator' + scene.TOOLS[unit['toolchain']]['sdk'], 'linkSysroot': '/SDK/iPhoneSimulator' + scene.TOOLS[unit['toolchain']]['sdk'] + '.sdk',
                    'developerDir': 'xcode', 'wdaDeployment': '15.0', 'wdaBinarySHA': 'd' * 64,
                    'sessionID': 'session', 'installedAt': 'time1', 'launchedAt': 'time2',
                    'products': [{'kind': kind, 'architectures': ['arm64'], 'signature': 'adhoc',
                                  'sha256': 'e' * 64, 'uuid': 'uuid', 'deployment': '15.1', 'entitlements': {}}
                                 for kind in ('app', 'liveActivity', 'notificationExtension')]}
        return {'unit': unit, 'state': 'completed', 'testExit': 0, 'identity': identity,
                'cases': [{'name': n, 'status': 'passed', 'startedAt': 'time1', 'completedAt': 'time2'}
                          for n in scene.CASES[unit['journey']]],
                'steps': list(scene.STEPS[unit['journey']]),
                'artifacts': [{'path': 'evidence.txt', 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}]}

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

    def test_missing_skipped_failed_cancelled_case_or_early_return_never_passes(self):
        unit = scene.units({'candidate': P}, '27')[1]
        original = self.result(unit)
        for status in ('skipped', 'pending', 'todo', 'failed', 'cancelled'):
            bad = copy.deepcopy(original); bad['cases'][0]['status'] = status
            with self.subTest(status=status):
                self.assertTrue(scene.unit_errors(unit, bad, self.root / scene.tuple_key(unit)))
        for field in ('cases', 'steps', 'artifacts'):
            bad = copy.deepcopy(original); bad[field] = bad[field][1:]
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


if __name__ == '__main__':
    unittest.main()
