import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import Mock

import test_design as td


class TestDesignTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.path = 'sample.test.ts'
        (self.root / self.path).write_text("test('case A', () => expect(1).toBe(1));\n")
        self.plan_name = 'docs/test/designs/sample.json'
        self.plan = {'id': 'sample', 'status': 'reviewed', 'review': {'reviewer': 'fixture self-review', 'date': '2026-11-05', 'decision': 'proceed', 'notes': 'failure and scope reviewed'}, 'cases': [{'id': 'S-1', 'layer': 'L1', 'description': 'observable result', 'setup': 'synthetic', 'action': 'exercise', 'expected': 'equal', 'planned_files': [self.path]}]}
        self.plan['seal'] = td.seal(self.plan)
        self.catalogue = {'files': {self.path: {'sha256': td.sha((self.root / self.path).read_bytes()), 'layer': 'L1', 'stack': 'fixture', 'cases': []}}}
        self.mappings = {self.path: {'cases': ['S-1'], 'selectors': ['case A']}}

    def check(self):
        return td.validate(self.root, self.catalogue, self.mappings, {self.plan_name: self.plan}, {self.path})

    def test_TD_001_valid_review_and_changed_source_guard(self):
        self.assertEqual(self.check(), [])
        (self.root / self.path).write_text('changed')
        self.assertTrue(any('source changed' in e for e in self.check()))
        with self.assertRaises(ValueError):
            td.refresh_catalogue(self.root, self.catalogue, {}, {self.plan_name:self.plan}, {self.path})
        (self.root / self.path).write_text('case A changed')
        refreshed = td.refresh_catalogue(self.root, self.catalogue, self.mappings, {self.plan_name:self.plan}, {self.path})
        self.assertEqual(td.validate(self.root, refreshed, self.mappings, {self.plan_name:self.plan}, {self.path}), [])
        (self.root / self.path).write_text('changed')
        self.assertTrue(any('selector' in e for e in self.check()))

    def test_TD_002_seal_required_fields_layers_paths(self):
        original = copy.deepcopy(self.plan)
        for key, value in [('expected', ''), ('layer', 'RN'), ('planned_files', ['../escape'])]:
            self.plan = copy.deepcopy(original)
            self.plan['cases'][0][key] = value
            self.assertTrue(self.check())
        self.plan = copy.deepcopy(original)
        self.plan['review']['notes'] = 'changed after review'
        self.assertTrue(any('changed design' in e for e in self.check()))
        self.plan = original
        self.mappings['../escape'] = self.mappings[self.path]
        self.assertTrue(any('Unsafe' in e for e in self.check()))

    def test_TD_003_unlisted_test_missing_source_and_missing_case(self):
        self.catalogue['files'].clear()
        self.assertTrue(any('catalogue' in e for e in self.check()))
        self.catalogue['files'][self.path] = {'sha256': '0'*64}
        (self.root / self.path).unlink()
        self.assertTrue(any('Missing/unsafe' in e for e in self.check()))
        self.mappings[self.path]['cases'] = ['unknown']
        self.assertTrue(any('outside reviewed' in e for e in self.check()))

    def git(self, *args):
        return subprocess.check_output(['git', *args], cwd=self.root, stderr=subprocess.DEVNULL).decode().strip()

    def commit(self):
        self.git('add', '.')
        self.git('-c', 'user.name=Synthetic Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture')
        return self.git('rev-parse', 'HEAD')

    def test_TD_004_history_rejects_same_commit_review_and_accepts_prior_review(self):
        self.git('init', '-q')
        (self.root / self.path).unlink()
        (self.root / 'README').write_text('fixture')
        base = self.commit()
        design = self.root / self.plan_name
        design.parent.mkdir(parents=True)
        design.write_text(json.dumps(self.plan))
        (self.root / self.path).write_text('case A')
        together = self.commit()
        errors = td.history_errors(self.root, base, together, self.mappings, {self.plan_name: self.plan})
        self.assertTrue(any('earlier commit' in e for e in errors))
        # A second independent fixture starts with the reviewed design already present.
        (self.root / self.path).unlink()
        reviewed = self.commit()
        (self.root / self.path).write_text('case A')
        head = self.commit()
        self.assertEqual(td.history_errors(self.root, reviewed, head, self.mappings, {self.plan_name: self.plan}), [])
        (self.root / self.path).write_text('case A changed again')
        reused = self.commit()
        self.assertTrue(td.history_errors(self.root, head, reused, self.mappings, {self.plan_name:self.plan}))
        self.assertTrue(td.history_errors(self.root, 'bad', head, self.mappings, {self.plan_name: self.plan}))
        self.assertTrue(td.history_errors(self.root, reviewed, head, {}, {self.plan_name: self.plan}))
        self.plan['cases'][0]['planned_files'].append('new-helper.cjs')
        self.plan['seal'] = td.seal(self.plan)
        design.write_text(json.dumps(self.plan))
        (self.root / 'new-helper.cjs').write_text('case A helper')
        helper_head = self.commit()
        helper_errors = td.history_errors(self.root, head, helper_head, {'new-helper.cjs':{'cases':['S-1'],'selectors':['case A']}}, {self.plan_name:self.plan})
        self.assertTrue(any('new-helper.cjs' in error and 'earlier commit' in error for error in helper_errors))


    def test_TD_005_removal_requires_rationale_and_absence(self):
        self.mappings[self.path] = {'cases': ['S-1'], 'removed': True}
        self.assertTrue(any('Removal' in e for e in self.check()))
        (self.root / self.path).unlink()
        self.catalogue['files'].clear()
        self.mappings[self.path]['reason'] = 'Moved to explicit manual tools; capability retained'
        self.assertEqual(td.validate(self.root, self.catalogue, self.mappings, {self.plan_name: self.plan}, set()), [])
        # A rename must account for the deleted name, even when Git would detect
        # an exact rename and otherwise report only the destination.
        (self.root / self.path).write_text('case A')
        self.git('init', '-q')
        (self.root / 'README').write_text('fixture')
        base = self.commit()
        self.plan['cases'][0]['planned_files'].append('renamed.test.ts')
        self.plan['seal'] = td.seal(self.plan)
        design = self.root / self.plan_name
        design.parent.mkdir(parents=True)
        design.write_text(json.dumps(self.plan))
        self.commit()
        (self.root / self.path).rename(self.root / 'renamed.test.ts')
        head = self.commit()
        errors = td.history_errors(self.root, base, head, {'renamed.test.ts': {'cases':['S-1'], 'selectors':['case A']}}, {self.plan_name:self.plan})
        self.assertIn('Changed test lacks reviewed mapping: ' + self.path, errors)


    def test_ND_003_native_runner_uses_production_sources_and_propagates_failures(self):
        path = td.ROOT / 'mobileapp/scripts/check-native-documents.py'
        spec = importlib.util.spec_from_file_location('native_oracles', path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        execute = Mock()
        module.run(Path('/synthetic/mobileapp'), execute)
        self.assertEqual(execute.call_count, 8)
        for call in (execute.call_args_list[0], execute.call_args_list[2]):
            argv = call.args[0]
            self.assertIn('-Onone', argv)
            self.assertIn('-module-cache-path', argv)
            self.assertTrue(any('/ios/Documents/' in a for a in argv))
            self.assertEqual(call.kwargs, {'check': True})
        for index, name in ((4, 'NotificationResponsePayload'), (6, 'NotificationResponseCompletion')):
            call = execute.call_args_list[index]
            self.assertIn('/synthetic/mobileapp/ios/GtmuxMobile/NotificationResponsePayload.swift', call.args[0])
            self.assertIn('/synthetic/mobileapp/ios/tests/' + name + 'Checks.swift', call.args[0])
            self.assertIn('-Onone', call.args[0])
            self.assertEqual(call.kwargs, {'check': True})
            self.assertEqual(execute.call_args_list[index + 1].args[0], [call.args[0][-1]])
        for call in execute.call_args_list[::2]:
            for argument in call.args[0]:
                if argument.endswith('.swift'):
                    actual = td.ROOT / argument.removeprefix('/synthetic/')
                    self.assertTrue(actual.is_file(), str(actual))
        for fail_at in range(8):
            calls = [None]*8
            calls[fail_at] = subprocess.CalledProcessError(1, ['synthetic'])
            with self.assertRaises(subprocess.CalledProcessError):
                module.run(Path('/synthetic/mobileapp'), Mock(side_effect=calls))
        workflow = (td.ROOT / '.github/workflows/ci.yml').read_text()
        self.assertIn('python3 mobileapp/scripts/check-native-documents.py', workflow)


if __name__ == '__main__':
    unittest.main()
