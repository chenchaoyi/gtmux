import io
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch
from contextlib import redirect_stderr
import openspec_check


class OpenSpecFailureTests(unittest.TestCase):
    def report(self, valid=True, issues=None):
        return {"items": [{"id": "fixture", "type": "spec", "valid": valid, "issues": [] if issues is None else issues}],
                "summary": {"totals": {"items": 1, "passed": int(valid), "failed": int(not valid)}},
                "root": {"path": str(Path.cwd())}}

    def test_invalid_spec_is_not_a_launcher_failure(self):
        status, _ = openspec_check.verdict(1, json.dumps(self.report(False)), Path.cwd())
        self.assertEqual(status, "invalid")

    def test_cache_and_startup_errors_have_no_spec_verdict(self):
        self.assertEqual(openspec_check.verdict(1, "npm ERR! EACCES cache", Path.cwd())[0], "tool")
        self.assertEqual(openspec_check.verdict(127, "", Path.cwd())[0], "tool")
        self.assertEqual(openspec_check.verdict(1, json.dumps(self.report()), Path.cwd())[0], "tool")

    def test_false_green_mismatched_summary_and_other_checkout_fail_closed(self):
        self.assertEqual(openspec_check.verdict(0, json.dumps(self.report(False)), Path.cwd())[0], "invalid")
        report = self.report(); report["summary"]["totals"]["failed"] = 1
        self.assertEqual(openspec_check.verdict(0, json.dumps(report), Path.cwd())[0], "tool")
        report = self.report(); report["root"]["path"] = "/other-checkout"
        self.assertEqual(openspec_check.verdict(0, json.dumps(report), Path.cwd())[0], "tool")

    def test_missing_or_timed_out_launcher_reports_tool_failure(self):
        for error in [FileNotFoundError(), subprocess.TimeoutExpired("npx", 120)]:
            with self.subTest(error=type(error)), patch("openspec_check.subprocess.run", side_effect=error), redirect_stderr(io.StringIO()) as output:
                self.assertEqual(openspec_check.main(), 2)
                self.assertIn("tool/setup failure", output.getvalue())
                self.assertNotIn("drifted specs", output.getvalue())

    def test_success_requires_complete_machine_output(self):
        self.assertEqual(openspec_check.verdict(0, json.dumps(self.report()), Path.cwd())[0], "ok")
        self.assertEqual(openspec_check.verdict(0, "success", Path.cwd())[0], "tool")

    def test_v2_legacy_length_advisories_pass_while_other_warnings_fail(self):
        len_warn = {"level": "WARNING", "path": "requirements[0]", "message": openspec_check.LEGACY_PROSE_LEN_WARNING}
        purpose_warn = {"level": "WARNING", "path": "overview", "message": "Purpose section is still a placeholder"}
        self.assertEqual(openspec_check.verdict(1, json.dumps(self.report(False, [len_warn])), Path.cwd())[0], "ok")
        self.assertEqual(openspec_check.verdict(2, json.dumps(self.report(False, [len_warn])), Path.cwd())[0], "tool")
        status, parsed = openspec_check.verdict(1, json.dumps(self.report(False, [len_warn, purpose_warn])), Path.cwd())
        self.assertEqual(status, "invalid")
        self.assertEqual(openspec_check.blocking_issues(parsed["items"][0]), [purpose_warn])
