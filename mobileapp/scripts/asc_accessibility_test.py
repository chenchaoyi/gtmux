"""Accessibility workflow regressions use synthetic metadata; no ASC key or network."""
from pathlib import Path
import json
import subprocess
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parent


class AccessibilityWorkflowTests(unittest.TestCase):
    def test_readonly_review_and_publication_boundaries(self):
        result = subprocess.run(['ruby', str(Path(__file__).with_suffix('.rb'))], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def run_preparation(self, folder, command, valid):
        # Execute the actual CLI path; stub only external data/work for a tiny package.
        driver = r'''
require 'digest'
require ARGV.shift
Object.prepend(Module.new do
def metadata_plan(*) = {}
def package_receipt_ok(*) = true
def accessibility_status(*)
  {ok: ENV['SYNTHETIC_REVIEW_VALID'] == 'yes', errors: ['Complete candidate review'], post_live_claims: []}
end
def library_id = 'lib'
def place_screenshots(*) = File.write('MUTATED', 'write')
def place_creative(*) = File.write('MUTATED', 'write')
def preparation_status(state:, **)
  StoreProgress.save(state, {status: 'ready_for_user_review', submitted: false})
  true
end
end)
$0 = ARGV.shift
load $0
'''
        script = str(SCRIPTS / 'asc-preparation.rb')
        import os
        return subprocess.run(
            ['ruby', '-e', driver, str(SCRIPTS / 'asc-asset-library.rb'), script, command, '--version', '1.2.3', '--build', '26',
             '--ipa', 'app.ipa', '--package-receipt', 'package.json', '--state', 'state.json'],
            cwd=folder, capture_output=True, text=True,
            env={**os.environ, 'SYNTHETIC_REVIEW_VALID': 'yes' if valid else 'no'})

    def test_assets_review_failure_blocks_writes_and_invalidates_old_ready_receipt(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / 'package.json').write_text('{}')
            (root / 'state.json').write_text('{"status":"ready_for_user_review"}')
            result = self.run_preparation(folder, 'assets', False)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('Accessibility review incomplete', result.stderr)
            self.assertFalse((root / 'MUTATED').exists())
            self.assertEqual(json.loads((root / 'state.json').read_text())['status'], 'incomplete')

    def test_successful_cli_exit_does_not_invalidate_new_ready_receipt(self):
        with tempfile.TemporaryDirectory() as folder:
            result = self.run_preparation(folder, 'status', True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(json.loads((Path(folder) / 'state.json').read_text())['status'], 'ready_for_user_review')


if __name__ == '__main__':
    unittest.main()
