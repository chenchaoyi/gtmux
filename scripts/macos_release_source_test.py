"""Immutable tag, original CLI, independent reviewed definition and failure gates."""
import copy
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock

import macos_release_source as source
from delivery_source import DeliveryError


class RecoveryTests(unittest.TestCase):
    def setUp(self):
        self.definition, self.product, self.tag = 'a' * 40, 'b' * 40, 'v1.2.3'
        ci = lambda sha: {'workflow_runs': [{'head_sha': sha, 'head_branch': 'main', 'event': 'push',
            'status': 'completed', 'conclusion': 'success', 'run_number': 2, 'run_attempt': 1}]}
        self.release = {'tag_name': self.tag, 'draft': False, 'prerelease': False, 'published_at': 'synthetic',
                        'assets': [{'name': name, 'size': 5, 'digest': 'sha256:' + 'c' * 64} for name in
                            [*(f'gtmux-1.2.3-{system}-{arch}.tar.gz' for system in ('darwin', 'linux') for arch in ('amd64', 'arm64')), 'SHASUMS256.txt']]}
        self.original = {'id': 10, 'run_number': 1, 'run_attempt': 2, 'event': 'push', 'head_branch': self.tag,
            'head_sha': self.product, 'path': source.WORKFLOW, 'repository': {'full_name': source.REPO},
            'status': 'completed', 'conclusion': 'failure'}
        self.valid_notes = "v1.2.3\n\nuser:\n- Ship feature A across CLI and Mac.\n\nuser-zh:\n- 支持功能 A，覆盖命令行与菜单栏应用。\n"
        self.table = {
            'git/ref/heads/main': {'object': {'sha': self.definition}},
            'git/ref/tags/' + self.tag: {'object': {'type': 'tag', 'sha': 'd' * 40}},
            'git/tags/' + 'd' * 40: {'tag': self.tag, 'message': self.valid_notes, 'object': {'type': 'commit', 'sha': self.product}},
            'releases/tags/' + self.tag: self.release, 'releases/latest': self.release,
            'actions/workflows/release.yml/runs?event=push&per_page=100': {'workflow_runs': [self.original]},
            'actions/runs/10/jobs?per_page=100': {'jobs': [
                {'name': 'goreleaser', 'run_id': 10, 'head_sha': self.product, 'status': 'completed', 'conclusion': 'success'},
                {'name': 'menu-bar app (macOS)', 'conclusion': 'failure'}]},
            'actions/runs/20': {'event': 'workflow_dispatch', 'head_branch': 'main', 'head_sha': self.definition,
                'path': source.WORKFLOW, 'repository': {'full_name': source.REPO}},
            'actions/runs/20/jobs?per_page=100': {'jobs': [
                {'name': name, 'status': 'completed', 'conclusion': 'success', 'steps': [
                    {'name': 'verify the app is notarized and stapled (tag)', 'conclusion': 'success'}]}
                for name in ('Resolve release source', 'menu-bar app (macOS)', 'publish notarized Mac app')] + [
                    {'name': 'goreleaser', 'status': 'completed', 'conclusion': 'skipped'}]},
        }
        for sha in (self.definition, self.product):
            self.table[f'actions/workflows/ci.yml/runs?head_sha={sha}&event=push&per_page=100'] = ci(sha)
        self.api = lambda path: copy.deepcopy(self.table[path])
        self.env = {'GITHUB_REPOSITORY': source.REPO, 'GITHUB_SHA': self.definition, 'GITHUB_EVENT_NAME': 'workflow_dispatch',
            'GITHUB_REF': 'refs/heads/main', 'GITHUB_ACTOR': 'chenchaoyi', 'RECOVERY_TAG': self.tag}

    def test_exact_product_is_separate_from_workflow_definition(self):
        result = source.resolve(self.env, self.api)
        self.assertEqual((result['source'], result['definition'], result['cli_run']), (self.product, self.definition, '10'))
        self.assertEqual(result['ref'], 'refs/tags/' + self.tag)

    def test_normal_tag_and_snapshot_do_not_fetch_recovery_authority(self):
        # Snapshot dispatch touches no GitHub API at all.
        snap = source.resolve({**self.env, 'RECOVERY_TAG': '', 'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_REF': 'refs/heads/topic'},
                              Mock(side_effect=AssertionError('unexpected API')))
        self.assertEqual((snap['source'], snap['release'], snap['recovery']), (self.definition, 'false', 'false'))

        # Official tag push verifies annotated tag, main CI, and bilingual What's New without consulting recovery runs.
        called = []
        def tag_api(path):
            called.append(path)
            self.assertNotIn('release.yml/runs', path)
            return self.api(path)
        rel = source.resolve({**self.env, 'RECOVERY_TAG': '', 'GITHUB_SHA': self.product, 'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/tags/' + self.tag}, tag_api)
        self.assertEqual((rel['source'], rel['release'], rel['recovery']), (self.product, 'true', 'false'))
        self.assertIn('git/tags/' + 'd' * 40, called)

    def test_RWN_001_release_tag_requires_verified_ci_and_bilingual_whatsnew_blocks(self):
        push_env = {**self.env, 'RECOVERY_TAG': '', 'GITHUB_SHA': self.product, 'GITHUB_EVENT_NAME': 'push', 'GITHUB_REF': 'refs/tags/' + self.tag}
        # Lightweight tag is rejected on tag push.
        orig_ref = self.table['git/ref/tags/' + self.tag]
        self.table['git/ref/tags/' + self.tag] = {'object': {'type': 'commit', 'sha': self.product}}
        with self.assertRaises(ValueError):
            source.resolve(push_env, self.api)
        self.table['git/ref/tags/' + self.tag] = orig_ref

        # Tag push without passing main push CI is rejected.
        ci_key = f'actions/workflows/ci.yml/runs?head_sha={self.product}&event=push&per_page=100'
        orig_ci = copy.deepcopy(self.table[ci_key])
        self.table[ci_key]['workflow_runs'][0]['conclusion'] = 'failure'
        with self.assertRaises(DeliveryError):
            source.resolve(push_env, self.api)
        self.table[ci_key] = orig_ci

        # Invalid What's New tag bodies fail closed before any build starts.
        bad_messages = [
            "v1.2.3\n\n- English bullet without user: header\n\n---\n\n- 无标题头中文条目\n",
            "v1.2.3\n\nuser:\n- English only\n",
            "v1.2.3\n\nuser-zh:\n- 只有中文\n",
            "v1.2.3\n\nuser:\n\n- Blank line right after header\n\nuser-zh:\n- 中文\n",
            "v1.2.3\n\nuser:\n- First item\n\n- Split item after blank line\n\nuser-zh:\n- 中文\n",
            "v1.2.3\n\n```\nuser:\n- Fenced example only\nuser-zh:\n- 代码块示例\n```\n",
        ]
        for msg in bad_messages:
            self.table['git/tags/' + 'd' * 40]['message'] = msg
            with self.subTest(msg=msg), self.assertRaises(ValueError):
                source.resolve(push_env, self.api)
        self.table['git/tags/' + 'd' * 40]['message'] = self.valid_notes

        # Local --check-notes preflight accepts valid bilingual notes and rejects malformed ones.
        with tempfile.TemporaryDirectory() as tmp:
            good = Path(tmp) / 'good.txt'
            good.write_text(self.valid_notes, encoding='utf-8')
            en, zh = source.validate_tag_notes(good.read_text(encoding='utf-8'))
            self.assertEqual((len(en), len(zh)), (1, 1))

    def test_dispatch_identity_branch_and_tag_cannot_inject_source(self):
        for field, value in [('GITHUB_ACTOR', 'other'), ('GITHUB_REF', 'refs/heads/topic'), ('GITHUB_EVENT_NAME', 'push'),
                             ('GITHUB_REPOSITORY', 'other/repo'), ('RECOVERY_TAG', 'v1.2.3\nsource=evil'), ('GITHUB_SHA', 'main')]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                source.resolve({**self.env, field: value}, self.api)

    def test_moved_main_lightweight_tag_or_wrong_annotated_target_refused(self):
        cases = [('git/ref/heads/main', {'object': {'sha': 'e' * 40}}),
                 ('git/ref/tags/' + self.tag, {'object': {'type': 'commit', 'sha': self.product}}),
                 ('git/tags/' + 'd' * 40, {'tag': 'v1.2.4', 'object': {'type': 'commit', 'sha': self.product}})]
        for path, value in cases:
            original = self.table[path]; self.table[path] = value
            with self.subTest(path=path), self.assertRaises(ValueError): source.resolve(self.env, self.api)
            self.table[path] = original

    def test_both_latest_product_and_definition_main_ci_required(self):
        for sha in (self.product, self.definition):
            key = f'actions/workflows/ci.yml/runs?head_sha={sha}&event=push&per_page=100'
            original = copy.deepcopy(self.table[key])
            self.table[key]['workflow_runs'].append({**original['workflow_runs'][0], 'run_attempt': 2, 'conclusion': 'failure'})
            with self.subTest(sha=sha), self.assertRaises(DeliveryError): source.resolve(self.env, self.api)
            self.table[key] = original

    def test_cli_assets_and_current_stable_release_required(self):
        for field, value in [('draft', True), ('prerelease', True), ('published_at', None), ('assets', [])]:
            original = self.release[field]; self.release[field] = value
            with self.subTest(field=field), self.assertRaises(ValueError): source.resolve(self.env, self.api)
            self.release[field] = original
        self.table['releases/latest'] = {'tag_name': 'v1.2.4'}
        with self.assertRaises(ValueError): source.resolve(self.env, self.api)

    def test_existing_mac_is_not_rebuilt(self):
        self.release['assets'].append({'name': 'Gtmux-1.2.3-macos.zip'})
        with self.assertRaises(ValueError): source.resolve(self.env, self.api)
        source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)

    def test_newer_failed_cli_cannot_reuse_old_success(self):
        self.table['actions/workflows/release.yml/runs?event=push&per_page=100']['workflow_runs'].append({**self.original, 'run_attempt': 3, 'head_sha': 'e' * 40})
        with self.assertRaises(ValueError): source.resolve(self.env, self.api)

    def test_wrong_cli_source_run_failure_or_successful_mac_refused(self):
        jobs = self.table['actions/runs/10/jobs?per_page=100']['jobs']
        for index, field, value in [(0, 'head_sha', self.definition), (0, 'run_id', 99), (0, 'conclusion', 'failure'), (1, 'conclusion', 'success')]:
            original = jobs[index].get(field); jobs[index][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError): source.resolve(self.env, self.api)
            jobs[index][field] = original

    def test_recovery_publication_requires_independent_staple_and_all_jobs(self):
        source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)
        jobs = self.table['actions/runs/20/jobs?per_page=100']['jobs']
        for job in jobs[:3]:
            job['conclusion'] = 'skipped'
            with self.subTest(job=job['name']), self.assertRaises(ValueError):
                source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)
            job['conclusion'] = 'success'
        jobs[3]['conclusion'] = 'success'
        with self.assertRaises(ValueError): source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)
        jobs[3]['conclusion'] = 'skipped'
        jobs[1]['steps'][0]['conclusion'] = 'skipped'
        with self.assertRaises(ValueError): source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)

    def test_publication_is_bound_to_its_reviewed_definition_if_main_advances(self):
        self.table['git/ref/heads/main']['object']['sha'] = 'e' * 40
        source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)
        with self.assertRaises(ValueError): source.resolve(self.env, self.api)

    def test_definition_and_product_cannot_be_swapped_at_publication(self):
        with self.assertRaises(ValueError): source.require_recovery_builds(self.definition, 20, '1.2.3', self.definition, self.api)
        self.table['actions/runs/20']['head_sha'] = self.product
        with self.assertRaises(ValueError): source.require_recovery_builds(self.product, 20, '1.2.3', self.definition, self.api)

    def test_attach_is_idempotent_only_for_identical_published_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / 'Gtmux-1.2.3-macos.zip'; archive.write_bytes(b'signed and stapled synthetic bytes')
            sha = hashlib.sha256(archive.read_bytes()).hexdigest(); env = {'PRODUCT_TAG': self.tag, 'ZIP': str(archive), 'TRUSTED_MAC_SHA': sha, 'PRODUCT_SOURCE': self.product}
            upload = Mock(); source.attach(env, self.api, upload); upload.assert_called_once()
            self.release['assets'].append({'name': archive.name, 'size': archive.stat().st_size, 'digest': 'sha256:' + sha})
            upload.reset_mock(); source.attach(env, self.api, upload); upload.assert_not_called()
            self.release['assets'][-1]['digest'] = 'sha256:' + 'e' * 64
            with self.assertRaises(ValueError): source.attach(env, self.api, upload)
            self.release['assets'].pop(); archive.write_bytes(b'changed')
            with self.assertRaises(ValueError): source.attach(env, self.api, upload)
            upload.assert_not_called()


if __name__ == '__main__':
    unittest.main()
