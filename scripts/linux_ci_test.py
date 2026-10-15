"""Run the actual CI lifecycle step with synthetic commands, never real accounts."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[1]


class LinuxCILifecycleTests(unittest.TestCase):
    def run_step(self, test_exit=0, cleanup_exit=0):
        source = (ROOT / ".github/workflows/ci.yml").read_text()
        section = source.split("      - name: Linux systemd user-service acceptance", 1)[1].split("\n      - name:", 1)[0]
        script = textwrap.dedent(section.split("        run: |\n", 1)[1])
        with tempfile.TemporaryDirectory(prefix="gtmux-linux-ci-test-") as tmp:
            root = Path(tmp)
            tools = root / "tools"
            tools.mkdir()
            for name, body in {"go": "exit 0", "install": "exit 0", "sleep": "exit 0", "id": "echo 1002"}.items():
                tool = tools / name
                tool.write_text("#!/bin/sh\n" + body + "\n")
                tool.chmod(0o700)
            sudo = tools / "sudo"
            sudo.write_text('''#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
with open(os.environ["CI_TEST_CALLS"], "a") as log:
    log.write(json.dumps(args) + "\\n")
if args[0] == "-u": sys.exit(int(os.environ["CI_TEST_EXIT"]))
if args[:2] == ["loginctl", "terminate-user"]: sys.exit(1)
if args[0] == "userdel": sys.exit(int(os.environ["CI_CLEANUP_EXIT"]))
''')
            sudo.chmod(0o700)
            calls = root / "calls"
            result = subprocess.run(["bash", "-e"], input=script, text=True,
                                    env={**os.environ, "PATH": str(tools) + ":" + os.environ["PATH"],
                                         "RUNNER_TEMP": str(root), "CI_TEST_CALLS": str(calls),
                                         "CI_TEST_EXIT": str(test_exit), "CI_CLEANUP_EXIT": str(cleanup_exit)},
                                    capture_output=True, timeout=10)
            commands = [json.loads(line) for line in calls.read_text().splitlines()]
            self.assertEqual(commands[-1], ["userdel", "--remove", "gtmux-ci"], commands)
            return result

    def test_stopped_user_manager_does_not_mask_runtime_result(self):
        for code in (0, 17):
            with self.subTest(code=code):
                result = self.run_step(test_exit=code)
                self.assertEqual(result.returncode, code, result.stdout + result.stderr)

    def test_account_cleanup_failure_is_visible_without_hiding_test_failure(self):
        for test_exit, expected in ((0, 1), (17, 17)):
            with self.subTest(test_exit=test_exit):
                result = self.run_step(test_exit=test_exit, cleanup_exit=4)
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
                self.assertIn("Could not remove", result.stdout)
