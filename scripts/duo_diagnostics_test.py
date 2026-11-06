"""Execute the actual phone diagnostic shell tail with no simulator access."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]


class PhoneDiagnosticTests(unittest.TestCase):
    def test_diagnostic_failures_preserve_the_original_phone_exit_status(self):
        workflow = (ROOT / '.github/workflows/ci.yml').read_text()
        step = workflow.split('      - name: Ordinary iPhone simulator regression\n', 1)[1]
        step = step.split('      - name: Preserve iPhone diagnostics immediately\n', 1)[0]
        tail = '          phone_status=0\n' + step.split('          phone_status=0\n', 1)[1]
        script = 'set -euo pipefail\n' + textwrap.dedent(tail)
        for status in (0, 7):
            for fault in ('none', 'mkdir', 'write'):
                with self.subTest(status=status, fault=fault), tempfile.TemporaryDirectory(prefix='duo-diagnostics-') as tmp:
                    root = Path(tmp)
                    binary = root / 'bin'
                    binary.mkdir()
                    (binary / 'python3').symlink_to(sys.executable)
                    (binary / 'npm').write_text('#!/bin/sh\nexit "$FIXTURE_PHONE_STATUS"\n')
                    (binary / 'xcrun').write_text('#!/bin/sh\nprintf "%s\\n" "$FIXTURE_CONTAINER"\n')
                    for name in ('npm', 'xcrun'):
                        (binary / name).chmod(0o700)
                    evidence = root / '.e2e-artifacts/phone-notification-system'
                    if fault == 'mkdir':
                        (root / '.e2e-artifacts').write_text('block directory creation')
                    elif fault == 'write':
                        evidence.mkdir(parents=True)
                        (evidence / 'system.log').mkdir()
                        (evidence / 'diagnostic-error.txt').mkdir()
                    env = {**os.environ, 'PATH': str(binary) + os.pathsep + os.environ['PATH'],
                           'GTMUX_E2E_UDID': 'fixture-only', 'FIXTURE_PHONE_STATUS': str(status),
                           'FIXTURE_CONTAINER': str(root / 'app')}
                    result = subprocess.run(['bash', '-c', script], cwd=root, env=env, capture_output=True, text=True, timeout=10)
                    self.assertEqual(result.returncode, status, result.stderr)
                    if fault != 'none':
                        self.assertIn('phone notification diagnostics failed', result.stderr)
                    else:
                        self.assertEqual((evidence / 'status.txt').read_text(), 'log_exit=0\n')


if __name__ == '__main__':
    unittest.main()
