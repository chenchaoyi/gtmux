"""Invoke the real shell gate with a synthetic scanner, no dependency/network scan."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().with_name("vulncheck.sh")
CONFIG = {"config": {"protocol_version": "v1.0.0"}}


class VulncheckTests(unittest.TestCase):
    def scan(self, data, rc=0):
        with tempfile.TemporaryDirectory(prefix="gtmux-vuln-test-") as temp:
            scanner = Path(temp, "go")
            scanner.write_text("#!/bin/sh\ncat \"$FAKE_SCAN\"\nexit \"$FAKE_STATUS\"\n")
            scanner.chmod(0o700)
            output = Path(temp, "scan.json"); output.write_text(data)
            env = {**os.environ, "PATH": temp + os.pathsep + os.environ["PATH"],
                   "FAKE_SCAN": str(output), "FAKE_STATUS": str(rc)}
            return subprocess.run(["bash", str(SCRIPT)], env=env, capture_output=True, text=True)

    def test_execution_failure_even_with_partial_output_fails(self):
        for data in ("", json.dumps(CONFIG)):
            result = self.scan(data, 42)
            self.assertEqual(result.returncode, 42)
            self.assertIn("scanner did not complete", result.stderr)
            self.assertNotIn("vulncheck: OK", result.stdout)

    def test_empty_malformed_or_missing_protocol_fails(self):
        for data in ("", "  \n", "{bad", "{}", "[]", json.dumps({"config": {"protocol_version": "v9"}}), json.dumps(CONFIG) + "{incomplete"):
            with self.subTest(data=data):
                self.assertNotEqual(self.scan(data).returncode, 0)

    def test_valid_scan_without_findings_passes(self):
        self.assertEqual(self.scan(json.dumps(CONFIG)).returncode, 0)

    def test_reachable_fixable_fails_unfixed_or_unreachable_passes(self):
        for fix, function, expected in (("v2.0.1", "Vulnerable", 1), ("", "Vulnerable", 0), ("v2.0.1", "", 0)):
            finding = {"finding": {"osv": "GO-TEST-1", "fixed_version": fix,
                                    "trace": [{"module": "example.invalid/pkg", "function": function}]}}
            result = self.scan(json.dumps(CONFIG) + "\n" + json.dumps(finding))
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            if function:
                self.assertIn("GO-TEST-1", result.stdout)


if __name__ == "__main__":
    unittest.main()
