"""Cross-job provenance, corrupt transport and fail-closed publication regressions."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

import macos_release_artifact as artifact

ROOT = Path(__file__).resolve().parents[1]


class ArtifactTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.expected = artifact.identity('a' * 40, 'refs/tags/v1.2.3', 55, 1)
        self.archive = self.root / self.expected['file']
        self.archive.write_bytes(b'zip bytes verified by the trusted Mac build')
        self.directory = self.root / 'stage'
        self.sha = artifact.seal(self.directory, self.archive, self.expected)

    def test_exact_bytes_and_original_successful_build_attempt(self):
        path = artifact.verify(self.directory, self.expected, self.sha)
        self.assertEqual(path.read_bytes(), self.archive.read_bytes())
        # A failed-only publication rerun must consume the original build output.
        result = subprocess.run(['python3', str(ROOT / 'scripts/macos_release_artifact.py'),
                                 'verify', '--directory', str(self.directory), '--source', 'a' * 40,
                                 '--ref', 'refs/tags/v1.2.3', '--run', '55', '--attempt', '1',
                                 '--sha256', self.sha], env={**os.environ, 'GITHUB_RUN_ATTEMPT': '2'},
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_corrupt_zip_even_with_modified_receipt_is_refused(self):
        (self.directory / self.expected['file']).write_bytes(b'replaced ZIP')
        receipt = self.directory / 'BUILD.json'
        metadata = json.loads(receipt.read_text())
        metadata.update(bytes=12, sha256=artifact.digest(self.directory / self.expected['file']))
        receipt.write_text(json.dumps(metadata))
        with self.assertRaises(ValueError):
            artifact.verify(self.directory, self.expected, self.sha)

    def test_other_source_ref_run_attempt_or_missing_trusted_hash_refused(self):
        for field, value in [('source', 'b' * 40), ('ref', 'refs/tags/v1.2.4'), ('run', 56), ('attempt', 2)]:
            with self.subTest(field=field), self.assertRaises(ValueError):
                artifact.verify(self.directory, {**self.expected, field: value}, self.sha)
        with self.assertRaises(ValueError):
            artifact.verify(self.directory, self.expected, '')

    def test_extra_missing_and_symlink_files_refused(self):
        extra = self.directory / 'unknown'
        extra.write_bytes(b'unknown')
        with self.assertRaises(ValueError):
            artifact.verify(self.directory, self.expected, self.sha)
        extra.unlink()
        target = self.directory / self.expected['file']
        target.unlink()
        target.symlink_to(self.archive)
        with self.assertRaises(ValueError):
            artifact.verify(self.directory, self.expected, self.sha)
        target.unlink()
        with self.assertRaises(ValueError):
            artifact.verify(self.directory, self.expected, self.sha)

    def test_snapshot_branch_is_never_a_notarized_tag(self):
        snapshot = artifact.identity('a' * 40, 'refs/heads/perf/nested-branch', 55, 1)
        self.assertEqual(snapshot['file'], 'Gtmux-snapshot-macos.zip')
        self.assertFalse(snapshot['notarized_tag'])
        for ref in ('refs/tags/main', 'refs/tags/v1.2.3;bad', 'main'):
            with self.assertRaises(ValueError):
                artifact.identity('a' * 40, ref, 55, 1)

    def test_definition_is_bound_separately_from_immutable_product(self):
        expected = artifact.identity('a' * 40, 'refs/tags/v1.2.3', 55, 1, 'b' * 40)
        with self.assertRaises(ValueError):
            artifact.verify(self.directory, expected, self.sha)
        other = self.root / 'recovery'
        sha = artifact.seal(other, self.archive, expected)
        artifact.verify(other, expected, sha)
        with self.assertRaises(ValueError):
            artifact.verify(other, {**expected, 'definition': 'c' * 40}, sha)
        self.assertEqual(expected['source'], 'a' * 40)

    def test_staging_does_not_replace_an_existing_build(self):
        with self.assertRaises(FileExistsError):
            artifact.seal(self.directory, self.archive, self.expected)


class WorkflowTests(unittest.TestCase):
    def test_builds_overlap_but_publication_and_mirror_wait_for_all_successes(self):
        text = (ROOT / '.github/workflows/release.yml').read_text()
        jobs = dict(re.findall(r'^  ([\w-]+):\n(.*?)(?=^  [\w-]+:|\Z)', text.split('jobs:\n', 1)[1], re.M | re.S))
        self.assertIn('runs-on: macos-26', jobs['app'])
        self.assertIn('DEVELOPER_DIR: /Applications/Xcode_26.6.app/Contents/Developer', jobs['app'])
        self.assertNotIn('macos-15-intel', jobs['app'])
        self.assertIn('needs: source', jobs['app'])
        self.assertIn('needs: source', jobs['goreleaser'])
        self.assertNotIn('needs: goreleaser', jobs['app'])
        self.assertIn('contents: read', jobs['app'])
        self.assertNotIn('gh release upload', jobs['app'])
        self.assertNotIn('HOMEBREW_TAP_TOKEN', jobs['app'])
        self.assertIn('xcrun stapler validate macapp/build/Gtmux.app', jobs['app'])
        self.assertIn('needs: [source, goreleaser, app]', jobs['app-publish'])
        self.assertIn('needs.app.outputs.artifact', jobs['app-publish'])
        self.assertIn('needs.app.outputs.attempt', jobs['app-publish'])
        self.assertIn('needs.app.outputs.sha256', jobs['app-publish'])
        self.assertIn('needs: [source, goreleaser, app, app-publish]', jobs['mirror'])
        self.assertIn("needs.app.result == 'success'", jobs['app-publish'])
        self.assertIn("needs.goreleaser.result == 'success'", jobs['app-publish'])
        self.assertIn("needs.source.outputs.recovery == 'true' && needs.goreleaser.result == 'skipped'", jobs['app-publish'])
        self.assertNotIn('continue-on-error:', jobs['app-publish'])
        self.assertLess(jobs['app-publish'].index('Verify exact-source Mac artifact'),
                        jobs['app-publish'].index('python3 scripts/macos_release_source.py attach'))
        self.assertNotIn('MACOS_CERT_P12', jobs['app-publish'])


if __name__ == '__main__':
    unittest.main()
